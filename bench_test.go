// bench_test.go 多格式真实样本批量评测（TASK-112 评测基准）：
// 对 eval-samples 样本库（pdf/md/docx/xlsx）批量执行 docling 解析，
// 产出逐文档与汇总指标（解析成功率/乱码率分布/label 分布/表格产出/耗时），
// 用于质量优化前后对比与 Docling 复刻效果的量化验证。
//
// 运行方式（样本库不进 git，位于 .temp/eval-samples/）：
//
//	DOCPARSE_EVAL_SAMPLE_DIR=/home/ubuntu/saas/.temp/eval-samples \
//	  go test ./docling/ -run TestBenchmarkSamples -count=1 -v
//
// 未设置环境变量时跳过（默认单测不依赖本地样本库）。
package docling

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

// benchDocStats 单文档解析指标。
type benchDocStats struct {
	File               string         `json:"file"`                           // File 是样本文件名。
	Format             string         `json:"format"`                         // Format 是样本扩展名。
	OK                 bool           `json:"ok"`                             // OK 表示解析是否成功。
	Error              string         `json:"error,omitempty"`                // Error 是解析失败原因。
	MS                 int64          `json:"ms"`                             // MS 是解析耗时毫秒数。
	Texts              int            `json:"texts"`                          // Texts 是文本元素数。
	Tables             int            `json:"tables"`                         // Tables 是结构化表格数。
	Pictures           int            `json:"pictures"`                       // Pictures 是图片元素数。
	PicturesWithURI    int            `json:"pictures_with_uri,omitempty"`    // PicturesWithURI 是携带可用资产 URI 的图片数。
	PictureBytes       int64          `json:"picture_bytes,omitempty"`        // PictureBytes 是可解码 data URI 的原始字节总数。
	MaxPictureCoverage float64        `json:"max_picture_coverage,omitempty"` // MaxPictureCoverage 是单图最大页面覆盖率。
	Formulas           int            `json:"formulas,omitempty"`             // Formulas 是公式元素数。
	FormulaChars       int            `json:"formula_chars,omitempty"`        // FormulaChars 是公式 LaTeX 字符总数。
	FormulaCells       int            `json:"formula_cells,omitempty"`        // FormulaCells 是保留原始公式的表格单元格数。
	Pages              int            `json:"pages"`                          // Pages 是页面数。
	Labels             map[string]int `json:"labels,omitempty"`               // Labels 是文本标签计数。
	Garbage            float64        `json:"garbage,omitempty"`              // Garbage 是全文乱码率。
	TableRows          int64          `json:"table_rows,omitempty"`           // TableRows 是最大表格行数。
	TableCols          int64          `json:"table_cols,omitempty"`           // TableCols 是最大表格列数。
	TableCells         int            `json:"table_cells,omitempty"`          // TableCells 是全部显式表格单元格数。
	MergedTableCells   int            `json:"merged_table_cells,omitempty"`   // MergedTableCells 是跨行或跨列单元格数。
	Bytes              int            `json:"bytes"`                          // Bytes 是输入文件字节数。
}

// collectBenchDocumentStats 从解析结果收集与格式无关的质量指标。
func collectBenchDocumentStats(stats *benchDocStats, doc *DoclingDocument) {
	if stats == nil || doc == nil {
		return
	}
	stats.OK = true
	stats.Texts = len(doc.Texts)
	stats.Tables = len(doc.Tables)
	stats.Pictures = len(doc.Pictures)
	stats.Pages = len(doc.Pages)
	stats.Labels = map[string]int{}
	for _, item := range doc.Texts {
		stats.Labels[string(item.Label)]++
		if item.Label == LabelFormula {
			stats.Formulas++
			stats.FormulaChars += len([]rune(strings.TrimSpace(item.Text)))
		}
	}
	for _, table := range doc.Tables {
		if table.Data == nil {
			continue
		}
		stats.TableCells += len(table.Data.TableCells)
		for _, cell := range table.Data.TableCells {
			if cell.RowSpan > 1 || cell.ColSpan > 1 {
				stats.MergedTableCells++
			}
			if cell.Ref != nil && cell.Ref.Kind == refTexts && cell.Ref.Idx >= 0 &&
				cell.Ref.Idx < int64(len(doc.Texts)) && len(doc.Texts[cell.Ref.Idx].Meta[xlsxFormulaMetaKey]) > 0 {
				stats.FormulaCells++
			}
		}
		if table.Data.NumRows > stats.TableRows {
			stats.TableRows = table.Data.NumRows
			stats.TableCols = table.Data.NumCols
		}
	}
	for _, picture := range doc.Pictures {
		if picture.Image == nil || strings.TrimSpace(picture.Image.URI) == "" {
			continue
		}
		stats.PicturesWithURI++
		stats.PictureBytes += benchDataURIBytes(picture.Image.URI)
		for _, prov := range picture.Prov {
			coverage := benchPictureCoverage(doc, prov)
			if coverage > stats.MaxPictureCoverage {
				stats.MaxPictureCoverage = coverage
			}
		}
	}
}

// benchDataURIBytes 返回合法 base64 data URI 的原始字节数。
func benchDataURIBytes(uri string) int64 {
	comma := strings.IndexByte(uri, ',')
	if comma < 0 || !strings.HasSuffix(strings.ToLower(uri[:comma]), ";base64") {
		return 0
	}
	data, err := base64.StdEncoding.DecodeString(strings.TrimSpace(uri[comma+1:]))
	if err != nil {
		return 0
	}
	return int64(len(data))
}

// benchPictureCoverage 计算单个图片来源框占对应页面的面积比例。
func benchPictureCoverage(doc *DoclingDocument, prov ProvenanceItem) float64 {
	if doc == nil || prov.BBox == nil {
		return 0
	}
	page, ok := doc.Pages[fmt.Sprintf("%d", prov.PageNo)]
	if !ok || page.Size == nil || page.Size.Width <= 0 || page.Size.Height <= 0 {
		return 0
	}
	width := prov.BBox.R - prov.BBox.L
	height := prov.BBox.T - prov.BBox.B
	if width <= 0 || height <= 0 {
		return 0
	}
	coverage := width * height / (page.Size.Width * page.Size.Height)
	if coverage > 1 {
		return 1
	}
	return coverage
}

// TestBenchmarkSamples 批量评测样本库并输出汇总 JSON 报告。
func TestBenchmarkSamples(t *testing.T) {
	root := os.Getenv("DOCPARSE_EVAL_SAMPLE_DIR")
	if root == "" {
		t.Skip("DOCPARSE_EVAL_SAMPLE_DIR 未设置，跳过批量评测（默认不依赖本地样本库）")
	}
	formats := []string{"pdf", "md", "docx", "xlsx"}
	var all []benchDocStats
	for _, format := range formats {
		dir := filepath.Join(root, format)
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Logf("跳过格式 %s：%v", format, err)
			continue
		}
		for _, e := range entries {
			if e.IsDir() || !strings.EqualFold(filepath.Ext(e.Name()), "."+format) {
				continue
			}
			path := filepath.Join(dir, e.Name())
			data, err := os.ReadFile(path)
			if err != nil {
				t.Errorf("读取 %s 失败: %v", path, err)
				continue
			}
			stats := benchDocStats{File: e.Name(), Format: format, Bytes: len(data)}
			start := time.Now()
			doc, err := ParseByExt(path, data)
			stats.MS = time.Since(start).Milliseconds()
			if err != nil {
				stats.Error = err.Error()
				all = append(all, stats)
				continue
			}
			collectBenchDocumentStats(&stats, doc)
			if format == "pdf" {
				stats.Garbage = textGarbageRatio(doc.Text())
			}
			all = append(all, stats)
		}
	}
	if len(all) == 0 {
		t.Skip("样本库为空")
	}
	// 汇总
	byFormat := map[string][]benchDocStats{}
	for _, s := range all {
		byFormat[s.Format] = append(byFormat[s.Format], s)
	}
	formatsSeen := make([]string, 0, len(byFormat))
	for f := range byFormat {
		formatsSeen = append(formatsSeen, f)
	}
	sort.Strings(formatsSeen)
	var summary []string
	for _, f := range formatsSeen {
		ss := byFormat[f]
		okCount, totalMS := 0, int64(0)
		var maxMS int64
		for _, s := range ss {
			if s.OK {
				okCount++
			}
			totalMS += s.MS
			if s.MS > maxMS {
				maxMS = s.MS
			}
		}
		summary = append(summary, fmt.Sprintf(
			"%s: %d/%d 成功, 平均 %dms, 最大 %dms",
			f, okCount, len(ss), totalMS/int64(len(ss)), maxMS))
	}
	t.Logf("评测样本 %d 篇\n汇总: %s", len(all), strings.Join(summary, "; "))
	// 报告落盘（便于优化前后对比）
	out := filepath.Join(root, "bench-report.json")
	data, _ := json.MarshalIndent(all, "", "  ")
	if err := os.WriteFile(out, data, 0644); err != nil {
		t.Errorf("写报告失败: %v", err)
	} else {
		t.Logf("逐文档报告: %s", out)
	}
	// 失败明细
	for _, s := range all {
		if !s.OK {
			t.Errorf("解析失败 %s/%s: %s", s.Format, s.File, s.Error)
		}
	}
}

// TestCollectBenchDocumentStats 验证公式、合并单元格和图片资产指标可重复计算。
func TestCollectBenchDocumentStats(t *testing.T) {
	doc := NewDoclingDocument("bench")
	doc.AddPage(1, 100, 200)
	doc.AddFormula(`x^{2}`, nil, nil)
	doc.AddTable([]DoclingTableCell{{
		Text: "标题", RowSpan: 1, ColSpan: 2,
		StartRowOffsetIdx: 0, EndRowOffsetIdx: 1,
		StartColOffsetIdx: 0, EndColOffsetIdx: 2,
	}}, 2, 2, nil, nil)
	formulaRef := doc.AddText(LabelText, "3", nil, nil)
	setXLSXFormulaMeta(&doc.Texts[formulaRef.Idx], "SUM(A1:A2)")
	doc.Tables[0].Data.TableCells[0].Ref = &formulaRef
	doc.AddPicture(&ImageRef{
		Mimetype: "image/png", Dpi: 72, Size: &ImageSize{Width: 10, Height: 10},
		URI: "data:image/png;base64,AQID",
	}, []ProvenanceItem{{
		PageNo: 1,
		BBox:   &DoclingBBox{L: 0, B: 0, R: 50, T: 100, CoordOrigin: CoordOriginBottomLeft},
	}}, nil)

	stats := benchDocStats{}
	collectBenchDocumentStats(&stats, doc)
	if !stats.OK || stats.Formulas != 1 || stats.FormulaChars != 5 {
		t.Fatalf("formula stats = %+v", stats)
	}
	if stats.TableCells != 1 || stats.MergedTableCells != 1 || stats.FormulaCells != 1 || stats.TableRows != 2 || stats.TableCols != 2 {
		t.Fatalf("table stats = %+v", stats)
	}
	if stats.Pictures != 1 || stats.PicturesWithURI != 1 || stats.PictureBytes != 3 || stats.MaxPictureCoverage != 0.25 {
		t.Fatalf("picture stats = %+v", stats)
	}
}

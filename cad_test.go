// cad_test.go 覆盖 CAD（DWG/DXF）解析后端：格式识别分派（DWG 版本串 /
// DXF ASCII / DXF 二进制哨兵 / 非法数据拒绝）、文档结构（页数=图框数或
// 兜底 1 页、图名节标题、PictureItem PNG data URI、文本 provenance）、
// ParseByExt 路由与 origin MIME、Markdown/JSON 导出，并沿用仓库约定
// 接入可选的官方 Docling Core schema 校验（DOCLING_PYTHON）。
package docling

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// readCADFixture 读取仓库 testdata 下的 CAD 样例；样例随仓库分发，
// 缺失视为测试环境损坏（fatal 而非 skip）。
func readCADFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("读取 CAD 样例 %s: %v", name, err)
	}
	return data
}

// TestDetectCADFormat 验证容器格式识别分派：DWG 版本串优先于 DXF 特征，
// DXF 覆盖 ASCII 特征与二进制哨兵两种编码，非法数据明确报错。
func TestDetectCADFormat(t *testing.T) {
	cases := []struct {
		name   string
		data   []byte
		want   cadFormat
		wantOK bool
	}{
		{name: "dwg样例", data: readCADFixture(t, "lw_example2018.dwg"), want: cadFormatDWG, wantOK: true},
		{name: "dxf ascii样例", data: readCADFixture(t, "sample_2018.dxf"), want: cadFormatDXF, wantOK: true},
		{name: "dxf多图框样例", data: readCADFixture(t, "frame_two.dxf"), want: cadFormatDXF, wantOK: true},
		// 二进制 DXF：哨兵后跟任意载荷即识别，载荷合法性由解析层把关
		{name: "dxf二进制哨兵", data: append([]byte("AutoCAD Binary DXF\r\n\x1a\x00"), 0x01, 0x02), want: cadFormatDXF, wantOK: true},
		// DWG 版本串抽样：R2.5 起数字串 + pre-R9 点分串（含 NUL 补齐形态）
		{name: "dwg版本串R2.5", data: []byte("AC1002\x00\x00"), want: cadFormatDWG, wantOK: true},
		{name: "dwg版本串R9", data: []byte("AC1004\x00\x00"), want: cadFormatDWG, wantOK: true},
		{name: "dwg版本串R2018", data: []byte("AC1032rest"), want: cadFormatDWG, wantOK: true},
		{name: "dwg版本串R1.4", data: []byte("AC1.40\x00"), want: cadFormatDWG, wantOK: true},
		{name: "dwg版本串R2.1", data: []byte("AC2.10\x00"), want: cadFormatDWG, wantOK: true},
		{name: "dwg版本串MC", data: []byte("MC0.0\x00\x00"), want: cadFormatDWG, wantOK: true},
		// 非法形态：未知版本串、普通文本与空数据均不得猜测分发
		{name: "未知版本串", data: []byte("AC1099\x00\x00"), wantOK: false},
		{name: "普通文本", data: []byte("hello cad world, definitely not a drawing"), wantOK: false},
		{name: "空数据", data: nil, wantOK: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			format, err := detectCADFormat(tc.data)
			if !tc.wantOK {
				if err == nil {
					t.Fatalf("期望识别失败，实际返回 %q", format)
				}
				if !strings.Contains(err.Error(), "无法识别 CAD 格式") {
					t.Fatalf("错误信息应明确拒绝原因，实际: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("识别失败: %v", err)
			}
			if format != tc.want {
				t.Fatalf("识别结果 = %q, 期望 %q", format, tc.want)
			}
		})
	}
}

// TestParseCADRejectsGarbage 非法数据必须以明确错误拒绝（识别层拦截，
// 不进入 go-cad 解析）。
func TestParseCADRejectsGarbage(t *testing.T) {
	if _, err := ParseCAD([]byte("not a cad file")); err == nil {
		t.Fatal("非法数据应报错")
	} else if !strings.Contains(err.Error(), "无法识别 CAD 格式") {
		t.Fatalf("错误信息不明确: %v", err)
	}
}

// TestParseCADWholeFallback 验证无图框样例的兜底结构：单页整图、图名
// 节标题、整幅渲染 PNG 的 PictureItem 与带 provenance 的图纸文本。
// lw_example2018.dwg 另有图纸文本可断言文本条数与 bbox 结构。
func TestParseCADWholeFallback(t *testing.T) {
	cases := []struct {
		filename  string
		wantTexts int // 正文（label=text）条数下限；0 表示不断言
	}{
		{filename: "lw_example2018.dwg", wantTexts: 1},
		{filename: "line_2000.dwg", wantTexts: 0},
		{filename: "sample_2018.dxf", wantTexts: 1},
	}
	for _, tc := range cases {
		t.Run(tc.filename, func(t *testing.T) {
			data := readCADFixture(t, tc.filename)
			doc, err := ParseCADWithOptions(data, PDFOptions{Filename: tc.filename})
			if err != nil {
				t.Fatalf("解析失败: %v", err)
			}
			// 文档名取文件主干名
			wantName := strings.TrimSuffix(tc.filename, filepath.Ext(tc.filename))
			if doc.Name != wantName {
				t.Fatalf("doc.Name = %q, 期望 %q", doc.Name, wantName)
			}
			// 无图框兜底：单页整图（go-cad 对三个样例均识别不出图框）
			if len(doc.Pages) != 1 {
				t.Fatalf("兜底页数 = %d, 期望 1", len(doc.Pages))
			}
			page := doc.Pages["1"]
			if page.Size == nil || page.Size.Width <= 0 || page.Size.Height <= 0 {
				t.Fatalf("页面尺寸无效: %+v", page.Size)
			}
			if doc.Meta == nil || doc.Meta.PageCount != 1 {
				t.Fatalf("DocMeta.PageCount 应为 1: %+v", doc.Meta)
			}
			// 图名节标题：section_header 恰 1 个且与分组名一致
			headers := 0
			for i := range doc.Texts {
				tx := &doc.Texts[i]
				if tx.Label != LabelSectionHeader {
					continue
				}
				headers++
				if strings.TrimSpace(tx.Text) == "" {
					t.Fatalf("节标题文本为空: %+v", tx)
				}
				if len(tx.Prov) != 1 || tx.Prov[0].PageNo != 1 {
					t.Fatalf("节标题 prov 应归第 1 页: %+v", tx.Prov)
				}
			}
			if headers != 1 {
				t.Fatalf("section_header 数 = %d, 期望 1", headers)
			}
			if len(doc.Groups) != 1 || doc.Groups[0].Name != doc.Texts[0].Text {
				t.Fatalf("应存在与图名一致的 section 分组: groups=%d", len(doc.Groups))
			}
			// PictureItem：整幅渲染 PNG，URI 为 data URI，尺寸与页面一致
			if len(doc.Pictures) != 1 {
				t.Fatalf("PictureItem 数 = %d, 期望 1", len(doc.Pictures))
			}
			pic := doc.Pictures[0]
			if pic.Image == nil || pic.Image.Mimetype != "image/png" {
				t.Fatalf("PictureItem 应为 image/png: %+v", pic.Image)
			}
			if !strings.HasPrefix(pic.Image.URI, "data:image/png;base64,") {
				t.Fatalf("URI 前缀错误: %q", pic.Image.URI[:min(40, len(pic.Image.URI))])
			}
			if pic.Image.Size == nil || pic.Image.Size.Width != page.Size.Width || pic.Image.Size.Height != page.Size.Height {
				t.Fatalf("图片尺寸应与页面一致: %+v vs %+v", pic.Image.Size, page.Size)
			}
			if len(pic.Prov) != 1 || pic.Prov[0].PageNo != 1 || pic.Prov[0].BBox == nil {
				t.Fatalf("PictureItem prov 应整页归第 1 页: %+v", pic.Prov)
			}
			// 图纸文本：条数下限 + provenance bbox 结构
			texts := 0
			for i := range doc.Texts {
				tx := &doc.Texts[i]
				if tx.Label != LabelText || strings.TrimSpace(tx.Text) == "" {
					continue
				}
				texts++
				if len(tx.Prov) != 1 || tx.Prov[0].PageNo != 1 {
					t.Fatalf("文本 prov 应归第 1 页: %+v", tx.Prov)
				}
				if tx.Prov[0].BBox == nil {
					t.Fatalf("文本应携带 provenance bbox: %+v", tx)
				}
			}
			if texts < tc.wantTexts {
				t.Fatalf("正文文本数 = %d, 期望 >= %d", texts, tc.wantTexts)
			}
		})
	}
}

// TestParseCADMultiSheet 验证合成多图框 DXF（testdata/frame_two.dxf，
// 两个 INSERT 图框各含一个框内 TEXT）的页组织：页数=图框数、页号与
// 图框序号对齐、每页图名标题 + 渲染 PNG + 框内文本按图框归属。
func TestParseCADMultiSheet(t *testing.T) {
	data := readCADFixture(t, "frame_two.dxf")
	doc, err := ParseCADWithOptions(data, PDFOptions{Filename: "frame_two.dxf"})
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if len(doc.Pages) != 2 {
		t.Fatalf("页数 = %d, 期望 2（图框数）", len(doc.Pages))
	}
	if doc.Meta == nil || doc.Meta.PageCount != 2 {
		t.Fatalf("DocMeta.PageCount 应为 2: %+v", doc.Meta)
	}
	// 每页一个 section 分组（图名）+ 一张渲染 PNG
	if len(doc.Groups) != 2 || len(doc.Pictures) != 2 {
		t.Fatalf("分组/图片数 = %d/%d, 期望 2/2", len(doc.Groups), len(doc.Pictures))
	}
	for i := range doc.Pictures {
		pic := &doc.Pictures[i]
		pageNo := pic.Prov[0].PageNo
		if pageNo != int64(i+1) {
			t.Fatalf("图片页号 = %d, 期望 %d", pageNo, i+1)
		}
		page := doc.Pages[pageNoKey(pageNo)]
		if pic.Image == nil || pic.Image.Mimetype != "image/png" ||
			!strings.HasPrefix(pic.Image.URI, "data:image/png;base64,") {
			t.Fatalf("图片 %d 应为 PNG data URI: %+v", i, pic.Image)
		}
		if pic.Image.Size == nil || pic.Image.Size.Width != page.Size.Width || pic.Image.Size.Height != page.Size.Height {
			t.Fatalf("图片 %d 尺寸应与页面一致", i)
		}
	}
	// 框内文本按图框归属：页号与 go-cad SheetText.Sheet 严格对齐
	assertTextOnPage := func(needle string, wantPage int64) {
		t.Helper()
		for i := range doc.Texts {
			tx := &doc.Texts[i]
			if tx.Label == LabelText && strings.Contains(tx.Text, needle) {
				if tx.Prov[0].PageNo != wantPage {
					t.Fatalf("文本 %q 应归第 %d 页, 实际第 %d 页", needle, wantPage, tx.Prov[0].PageNo)
				}
				bbox := tx.Prov[0].BBox
				page := doc.Pages[pageNoKey(wantPage)]
				if bbox == nil || bbox.L < 0 || bbox.T < 0 || bbox.R > page.Size.Width+1 || bbox.B > page.Size.Height+1 {
					t.Fatalf("文本 %q bbox 应落在页面内: %+v", needle, bbox)
				}
				return
			}
		}
		t.Fatalf("未找到文本 %q", needle)
	}
	assertTextOnPage("SHEET-ONE PLAN", 1)
	assertTextOnPage("SHEET-TWO DIAGRAM", 2)

	// Markdown 导出（export 路径）含图名与图纸文本
	md := doc.ToMarkdown()
	if !strings.Contains(md, "# FRAME") {
		t.Fatalf("Markdown 应含图名标题: %.200s", md)
	}
	if !strings.Contains(md, "SHEET-ONE PLAN") || !strings.Contains(md, "SHEET-TWO DIAGRAM") {
		t.Fatalf("Markdown 应含图纸文本: %.400s", md)
	}

	// JSON 导出合法
	raw, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("JSON 序列化失败: %v", err)
	}
	if !json.Valid(raw) {
		t.Fatal("JSON 导出非法")
	}
	// 沿用仓库约定接入可选官方 schema 校验（未配置 DOCLING_PYTHON 时跳过）
	validateDocumentWithDoclingCore110ForTest(t, "cad-frame-two", doc)
}

// pageNoKey 页号 → pages map 键（字符串化数字，Docling 协议口径，
// 与 AddPage 的 fmt.Sprintf("%d") 一致）。
func pageNoKey(pageNo int64) string {
	return strconv.FormatInt(pageNo, 10)
}

// TestParseByExtCADRouting 验证统一入口按扩展名路由到 CAD 后端：
// origin MIME 按 .dwg/.dxf/.dxfb 区分，文档名取主干名，非法 DWG 数据
// 明确报错。
func TestParseByExtCADRouting(t *testing.T) {
	dwg := readCADFixture(t, "lw_example2018.dwg")
	dxf := readCADFixture(t, "frame_two.dxf")

	cases := []struct {
		filename    string
		data        []byte
		wantMIME    string
		wantErrText string
	}{
		{filename: "平面图.dwg", data: dwg, wantMIME: "image/vnd.dwg"},
		{filename: "平面图.dxf", data: dxf, wantMIME: "image/vnd.dxf"},
		// .dxfb 与 .dxf 同 MIME：二进制只是编码差异
		{filename: "平面图.dxfb", data: dxf, wantMIME: "image/vnd.dxf"},
		{filename: "bad.dwg", data: []byte("junk"), wantErrText: "无法识别 CAD 格式"},
	}
	for _, tc := range cases {
		t.Run(tc.filename, func(t *testing.T) {
			doc, err := ParseByExt(tc.filename, tc.data)
			if tc.wantErrText != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErrText) {
					t.Fatalf("期望错误含 %q, 实际: %v", tc.wantErrText, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("解析失败: %v", err)
			}
			if doc.Origin == nil || doc.Origin.Mimetype != tc.wantMIME {
				t.Fatalf("origin MIME = %+v, 期望 %q", doc.Origin, tc.wantMIME)
			}
			if doc.Name != "平面图" {
				t.Fatalf("doc.Name = %q, 期望主干名 %q", doc.Name, "平面图")
			}
		})
	}
}

// TestParseCADBinaryDXFRouting 验证 DXF 二进制变体经哨兵识别后路由到
// DXF 解析分支（载荷非法时错误来自 DXF 解析层而非识别层）。
func TestParseCADBinaryDXFRouting(t *testing.T) {
	bin := append([]byte("AutoCAD Binary DXF\r\n\x1a\x00"), bytes.Repeat([]byte{0xAB}, 64)...)
	if _, err := ParseByExt("binary.dxfb", bin); err == nil {
		t.Fatal("非法二进制 DXF 应报错")
	} else if !strings.Contains(err.Error(), "CAD dxf 解析失败") {
		t.Fatalf("应路由到 DXF 解析分支: %v", err)
	}
}

// TestExamplesCadGolden 是 examples/cad 对照集的端到端回归：sample.dwg
// 经 ParseByExtToMarkdown 的输出必须与 sample.expected.md 逐字一致。
// 样例源文件是真实 DWG 二进制（非代码构造），因此不进 examples_test.go
// 的 examplesSources 合成清单，独立在此对照；-update 复用全局
// updateExamples 标志重建期望输出，源文件保持原样。
func TestExamplesCadGolden(t *testing.T) {
	const (
		dir      = "examples/cad"
		sample   = "sample.dwg"
		expected = "sample.expected.md"
	)
	data, err := os.ReadFile(filepath.Join(dir, sample))
	if err != nil {
		t.Fatalf("读取样例 %s: %v", sample, err)
	}
	markdown, err := ParseByExtToMarkdown(sample, data)
	if err != nil {
		t.Fatalf("解析样例: %v", err)
	}
	expectedPath := filepath.Join(dir, expected)
	if *updateExamples {
		if err := os.WriteFile(expectedPath, []byte(markdown), 0o644); err != nil {
			t.Fatalf("重建期望输出: %v", err)
		}
		return
	}
	want, err := os.ReadFile(expectedPath)
	if err != nil {
		t.Fatalf("读取期望输出（首次生成请运行 go test -run TestExamplesCadGolden -update）: %v", err)
	}
	if markdown != string(want) {
		t.Fatalf("Markdown 输出与 %s 不一致（重跑 go test -run TestExamplesCadGolden -update 重建）", expected)
	}
}

// main_test.go 覆盖 CLI 的四种输出格式、章节/工作表过滤,以及"excel 公式
// 数字溯源"场景的全链路断言（公式写入富单元格 meta,表格单元格经 $ref 关联）。
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

// TestFormatsCommand 列出格式命令可用。
func TestFormatsCommand(t *testing.T) {
	var out bytes.Buffer
	if code := run([]string{"formats"}, &out, &bytes.Buffer{}); code != 0 {
		t.Fatalf("formats exit=%d", code)
	}
	if !strings.Contains(out.String(), "pdf") || !strings.Contains(out.String(), "xlsx") {
		t.Fatalf("formats output=%s", out.String())
	}
}

// TestParseOutlineXLSX 验证 outline 含分组名、表格行列与图片计数。
func TestParseOutlineXLSX(t *testing.T) {
	var out bytes.Buffer
	code := run([]string{"parse", filepath.Join("..", "..", "examples", "xlsx", "rich-chart.xlsx"), "--format", "outline"}, &out, &bytes.Buffer{})
	if code != 0 {
		t.Fatalf("parse exit=%d", code)
	}
	text := out.String()
	for _, want := range []string{"Sheet1", "表格:", "图片 #1"} {
		if !strings.Contains(text, want) {
			t.Fatalf("outline missing %q:\n%s", want, text)
		}
	}
}

// TestParseMarkdownSectionFilter 验证 --section 只保留命中章节的条目。
func TestParseMarkdownSectionFilter(t *testing.T) {
	book := filepath.Join("testdata", "book.md")
	if err := os.MkdirAll("testdata", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(book, []byte(testBookMarkdown), 0o644); err != nil {
		t.Fatal(err)
	}
	var all, filtered bytes.Buffer
	if code := run([]string{"parse", book, "--format", "md"}, &all, &bytes.Buffer{}); code != 0 {
		t.Fatalf("parse all exit=%d", code)
	}
	if !strings.Contains(all.String(), "使用手册") || !strings.Contains(all.String(), "三章内容") {
		t.Fatalf("full md missing chapters:\n%s", all.String())
	}
	if code := run([]string{"parse", book, "--format", "md", "--section", "第二章"}, &filtered, &bytes.Buffer{}); code != 0 {
		t.Fatalf("parse section exit=%d", code)
	}
	text := filtered.String()
	if !strings.Contains(text, "二章内容") {
		t.Fatalf("filtered md missing target section:\n%s", text)
	}
	if strings.Contains(text, "一章内容") || strings.Contains(text, "三章内容") {
		t.Fatalf("filtered md leaked other sections:\n%s", text)
	}
}

// TestParseContentListSheetFilter 验证 --sheet 按 SectionPath[0] 过滤。
func TestParseContentListSheetFilter(t *testing.T) {
	data := mustBuildFormulaXLSX(t)
	path := filepath.Join(t.TempDir(), "report.xlsx")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if code := run([]string{"parse", path, "--format", "content-list"}, &out, &bytes.Buffer{}); code != 0 {
		t.Fatalf("parse exit=%d", code)
	}
	var items []map[string]any
	if err := json.Unmarshal(out.Bytes(), &items); err != nil {
		t.Fatalf("decode content-list: %v", err)
	}
	if len(items) == 0 {
		t.Fatal("content-list empty")
	}
}

// TestJSONFormulaScenario 是"excel 某格数字怎么来的"场景的全链路校准：
// 公式写入 texts[].meta.docling__xlsx_formula,表格单元格 ref 以 "#/texts/N"
// 指回公式节点——skill 文档中的 jq 查询路径以此断言为准。
func TestJSONFormulaScenario(t *testing.T) {
	data := mustBuildFormulaXLSX(t)
	path := filepath.Join(t.TempDir(), "report.xlsx")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	outPath := filepath.Join(t.TempDir(), "doc.json")
	var stdout bytes.Buffer
	if code := run([]string{"parse", path, "--format", "json", "--out", outPath}, &stdout, &bytes.Buffer{}); code != 0 {
		t.Fatalf("parse exit=%d stdout=%s", code, stdout.String())
	}
	raw, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Texts []struct {
			Text string         `json:"text"`
			Meta map[string]any `json:"meta"`
		} `json:"texts"`
		Tables []struct {
			Data *struct {
				TableCells []struct {
					Text string `json:"text"`
					Ref  *struct {
						Ref string `json:"$ref"`
					} `json:"ref"`
					StartRowOffsetIdx int64 `json:"start_row_offset_idx"`
					StartColOffsetIdx int64 `json:"start_col_offset_idx"`
				} `json:"table_cells"`
			} `json:"data"`
		} `json:"tables"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("decode doc json: %v", err)
	}

	// 1. jq 场景:找公式节点
	formulaTexts := map[int]string{}
	for i, item := range doc.Texts {
		if f, ok := item.Meta["docling__xlsx_formula"].(string); ok {
			formulaTexts[i] = f
		}
	}
	if len(formulaTexts) != 1 {
		t.Fatalf("expect 1 formula text, got %d", len(formulaTexts))
	}
	// 2. jq 反查:表格单元格的 $ref 指回公式节点,并带行列坐标
	found := false
	for _, table := range doc.Tables {
		if table.Data == nil {
			continue
		}
		for _, cell := range table.Data.TableCells {
			if cell.Ref == nil {
				continue
			}
			var idx int
			if _, err := fmt.Sscanf(cell.Ref.Ref, "#/texts/%d", &idx); err != nil {
				t.Fatalf("bad ref %q", cell.Ref.Ref)
			}
			formula, ok := formulaTexts[idx]
			if !ok {
				continue
			}
			if formula != "SUM(A2:A3)" {
				t.Fatalf("formula=%q", formula)
			}
			if cell.StartRowOffsetIdx != 0 || cell.StartColOffsetIdx != 1 {
				t.Fatalf("cell coord=%d/%d", cell.StartRowOffsetIdx, cell.StartColOffsetIdx)
			}
			found = true
		}
	}
	if !found {
		t.Fatal("no table cell refs the formula node")
	}
}

// TestJSONIgnoresFilters 提示语义:json 格式带 --section 时仍输出完整文档。
func TestJSONIgnoresFilters(t *testing.T) {
	book := filepath.Join("testdata", "book.md")
	if err := os.MkdirAll("testdata", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(book, []byte(testBookMarkdown), 0o644); err != nil {
		t.Fatal(err)
	}
	var out, stderr bytes.Buffer
	if code := run([]string{"parse", book, "--format", "json", "--section", "第二章"}, &out, &stderr); code != 0 {
		t.Fatalf("parse exit=%d", code)
	}
	if !strings.Contains(stderr.String(), "jq") {
		t.Fatalf("expect jq hint on stderr:\n%s", stderr.String())
	}
	var doc map[string]any
	if err := json.Unmarshal(out.Bytes(), &doc); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !strings.Contains(out.String(), "三章内容") {
		t.Fatal("json output should be unfiltered")
	}
}

// testBookMarkdown 是章节过滤测试输入。
const testBookMarkdown = `# 使用手册

一章内容概述。

## 第二章 操作步骤

二章内容详情。

### 第二章.1 高级操作

二章高级内容。

## 第三章 附录

三章内容。
`

// mustBuildFormulaXLSX 构造含公式单元格(带表头行)的 xlsx 字节流:
// A1=合计 B1=SUM(A2:A3) A2=1 A3=2。
func mustBuildFormulaXLSX(t *testing.T) []byte {
	t.Helper()
	file := excelize.NewFile()
	t.Cleanup(func() { _ = file.Close() })
	if err := file.SetCellValue("Sheet1", "A1", "合计"); err != nil {
		t.Fatal(err)
	}
	if err := file.SetCellFormula("Sheet1", "B1", "SUM(A2:A3)"); err != nil {
		t.Fatal(err)
	}
	for cell, value := range map[string]int{"A2": 1, "A3": 2} {
		if err := file.SetCellValue("Sheet1", cell, value); err != nil {
			t.Fatal(err)
		}
	}
	var buf bytes.Buffer
	if err := file.Write(&buf); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

package docling

import (
	"strings"
	"testing"
)

func TestPostprocessTableMarkdown_RemovesEmptyRowsAndColumns(t *testing.T) {
	input := `| 姓名 | 年龄 |  | 部门 |
|---|---|---|---|
| 张三 | 20 |  | 研发 |
|  |  |  |  |
| 李四 | 25 |  | 产品 |`

	got := postprocessTableMarkdown(input)
	if strings.Contains(got, "|  |  |  |") {
		t.Errorf("expected empty row removed, got:\n%s", got)
	}
	if strings.Contains(got, "|  |") {
		t.Errorf("expected empty column removed, got:\n%s", got)
	}
	if !strings.Contains(got, "张三") || !strings.Contains(got, "李四") {
		t.Errorf("expected keep data rows, got:\n%s", got)
	}
}

func TestPostprocessTableMarkdown_MergesDuplicateAdjacentColumns(t *testing.T) {
	input := `| 指标 | 指标 | 数值 | 数值 |
|---|---|---|---|
| 长度 | 长度 | 100 | 100 |
| 宽度 | 宽度 | 50 | 50 |`

	got := postprocessTableMarkdown(input)
	lines := strings.Split(got, "\n")
	if len(lines) < 2 {
		t.Fatalf("expected at least header and separator, got:\n%s", got)
	}
	cols := strings.Split(lines[0], "|")
	// 去除首尾空串
	var nonEmpty []string
	for _, c := range cols {
		if strings.TrimSpace(c) != "" {
			nonEmpty = append(nonEmpty, strings.TrimSpace(c))
		}
	}
	if len(nonEmpty) != 2 {
		t.Errorf("expected 2 unique columns, got %d: %v", len(nonEmpty), nonEmpty)
	}
}

func TestPostprocessTableMarkdown_DeduplicatesMergedCells(t *testing.T) {
	input := `| 项目 | 阶段 |
|---|---|
| 项目A | 设计 |
| 项目A | 开发 |
| 项目A | 测试 |`

	got := postprocessTableMarkdown(input)
	t.Logf("got:\n%s", got)
	lines := strings.Split(got, "\n")
	if len(lines) < 4 {
		t.Fatalf("expected header + separator + 3 data rows, got:\n%s", got)
	}
	// 第二行数据（索引 3，表头+分隔行之后）的项目列应该为空（被去重）
	cols := strings.Split(lines[3], "|")
	if len(cols) < 3 {
		t.Fatalf("unexpected column count: %v", cols)
	}
	if strings.TrimSpace(cols[1]) != "" {
		t.Errorf("expected duplicate merged cell text to be empty, got %q", cols[1])
	}
}

func TestPostprocessTableMarkdown_AddsSeparatorIfMissing(t *testing.T) {
	input := `| 姓名 | 年龄 |
| 张三 | 20 |`

	got := postprocessTableMarkdown(input)
	lines := strings.Split(got, "\n")
	if len(lines) != 3 {
		t.Fatalf("expected header + separator + data row, got %d lines:\n%s", len(lines), got)
	}
	if !isTableSeparatorLine(lines[1]) {
		t.Errorf("expected second line to be separator, got %q", lines[1])
	}
}

func TestPostprocessTableMarkdown_KeepsNormalTable(t *testing.T) {
	input := `| 姓名 | 年龄 | 部门 |
|---|---|---|
| 张三 | 20 | 研发 |
| 李四 | 25 | 产品 |`

	got := postprocessTableMarkdown(input)
	if !strings.Contains(got, "张三") || !strings.Contains(got, "李四") {
		t.Errorf("expected keep normal table, got:\n%s", got)
	}
	if strings.Count(got, "\n") != 3 {
		t.Errorf("expected 4 lines, got %d lines:\n%s", strings.Count(got, "\n")+1, got)
	}
}

// TestParseMarkdownTableStrict_EscapedPipe 验证 goldmark 严格解析能还原单元格中的转义管道符：
// a\|b 应作为单个单元格内容 a|b，而不是被切分成两列。分隔行不进入数据数组。
func TestParseMarkdownTableStrict_EscapedPipe(t *testing.T) {
	rows := ParseMarkdownTable("| a | b |\n| --- | --- |\n| x\\|y | z |")
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows (separator excluded), got %d: %v", len(rows), rows)
	}
	if len(rows[1]) != 2 {
		t.Fatalf("expected 2 columns in data row, got %d: %v", len(rows[1]), rows[1])
	}
	if rows[1][0] != "x|y" {
		t.Fatalf("escaped pipe not restored, got %q, want %q", rows[1][0], "x|y")
	}
}

// TestParseMarkdownTableStrict_EscapedPipeInInlineCode 验证行内代码中的转义管道符处理：
// 按 GFM 规范，单元格内管道必须转义，转义后即使在行内代码里也应保持单列。
func TestParseMarkdownTableStrict_EscapedPipeInInlineCode(t *testing.T) {
	rows := ParseMarkdownTable("| code | out |\n| --- | --- |\n| `a\\|b` | c |")
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows (separator excluded), got %d: %v", len(rows), rows)
	}
	if len(rows[1]) != 2 {
		t.Fatalf("expected 2 columns, got %d: %v", len(rows[1]), rows[1])
	}
	if rows[1][0] != "a|b" {
		t.Fatalf("inline code cell = %q, want %q", rows[1][0], "a|b")
	}
}

// TestRenderMarkdownTable_EscapesPipe 验证渲染时对单元格内的管道符转义、换行压平，
// 保证输出仍是合法 GFM 表格。
func TestRenderMarkdownTable_EscapesPipe(t *testing.T) {
	got := RenderMarkdownTable([][]string{{"a|b", "c"}, {"x", "多\n行"}})
	lines := strings.Split(got, "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines (no cell line break), got %d:\n%s", len(lines), got)
	}
	if !strings.Contains(lines[0], `a\|b`) {
		t.Fatalf("pipe not escaped in render: %q", lines[0])
	}
	if strings.Contains(lines[1], "\n") {
		t.Fatalf("cell line break not flattened: %q", lines[1])
	}
}

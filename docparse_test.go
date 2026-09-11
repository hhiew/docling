package docparse

import (
	"strings"
	"testing"
)

func TestSanitizeText(t *testing.T) {
	got := sanitizeText("a\x00b\x01c\nd\te")
	if got != "abc\nd\te" {
		t.Fatalf("sanitize = %q", got)
	}
}

func TestItemHeadingPath(t *testing.T) {
	if got := ItemHeadingPath(Item{SectionPath: []string{"部署指南", "环境要求"}}); got != "部署指南 > 环境要求" {
		t.Fatalf("heading path = %q", got)
	}
	if ItemHeadingPath(Item{}) != "" {
		t.Fatal("empty section path should render empty")
	}
}

func TestJoinItemTexts(t *testing.T) {
	got := JoinItemTexts([]Item{
		{Type: ItemTypeText, Text: "A"},
		{Type: ItemTypeTable, TableBody: "| a |"},
		{Type: ItemTypeText, Text: ""},
	})
	if got != "A\n| a |" {
		t.Fatalf("joined = %q", got)
	}
}

func TestParseText(t *testing.T) {
	pdoc, err := ParseText([]byte("  hello \x01world  "))
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	items := ToContentList(pdoc, SourceGolight)
	if len(items) != 1 || items[0].Text != "hello world" {
		t.Fatalf("unexpected items: %+v", items)
	}
	// ParseText 产出经过 sanitizeText：控制字符被清理、首尾空白被裁剪
	if text := pdoc.Text(); strings.TrimSpace(text) != "hello world" {
		t.Fatalf("text = %q", text)
	}
	if empty, _ := ParseText(nil); len(empty.Texts) != 0 {
		t.Fatalf("empty input should return empty doc")
	}
}

// TestParseTextBOM 验证 UTF-8 BOM 不会进入首个文本元素。
func TestParseTextBOM(t *testing.T) {
	doc, err := ParseText([]byte("\xef\xbb\xbf正文"))
	if err != nil {
		t.Fatalf("ParseText: %v", err)
	}
	if len(doc.Texts) != 1 || doc.Texts[0].Text != "正文" {
		t.Fatalf("BOM should be stripped: %+v", doc.Texts)
	}
}

func TestEscapeMarkdownTableCell(t *testing.T) {
	if got := escapeMarkdownTableCell("a|b\nc"); got != `a\|b c` {
		t.Fatalf("escape = %q", got)
	}
}

func TestParseMarkdownTableLooseFallback(t *testing.T) {
	// 缺分隔行时回退宽松解析（goldmark 无法识别该格式）
	rows := ParseMarkdownTable("| 姓名 | 年龄 |\n| 张三 | 20 |")
	if len(rows) != 2 || rows[0][0] != "姓名" || rows[1][0] != "张三" {
		t.Fatalf("rows = %v", rows)
	}
	if isTableSeparatorLine("|---|---|") != true {
		t.Fatal("separator line not detected")
	}
}

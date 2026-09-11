// export_html_test.go 验证导出能力：ExportHTML（与 ExportMarkdown 同构的
// 元素覆盖面：标题/段落/列表/表格/图片/代码/公式）、ToHTML/ToMarkdown 方法
// 形式入口与 ParseByExtToMarkdown 一步导出。
package docling

import (
	"strings"
	"testing"
)

// buildHTMLSampleDoc 构造覆盖全部导出元素的样例文档。
func buildHTMLSampleDoc() *DoclingDocument {
	doc := NewDoclingDocument("样例文档")
	doc.Meta = &DocMeta{Title: "样例文档"}
	doc.AddTitle("样例文档", nil, nil)
	doc.AddHeading(2, "概述", nil, nil)
	doc.AddText(LabelText, "第一段内容", nil, nil)
	listGroup := doc.AddListGroup("list", nil)
	doc.AddListItem(listGroup, "项一", false, "", nil)
	doc.AddListItem(listGroup, "项二", true, "1.", nil)
	doc.AddTable([]DoclingTableCell{
		{Text: "列A", StartRowOffsetIdx: 0, EndRowOffsetIdx: 1, StartColOffsetIdx: 0, EndColOffsetIdx: 1, ColumnHeader: true},
		{Text: "列B", StartRowOffsetIdx: 0, EndRowOffsetIdx: 1, StartColOffsetIdx: 1, EndColOffsetIdx: 2, ColumnHeader: true},
		{Text: "值1", StartRowOffsetIdx: 1, EndRowOffsetIdx: 2, StartColOffsetIdx: 0, EndColOffsetIdx: 1},
		{Text: "值2", StartRowOffsetIdx: 1, EndRowOffsetIdx: 2, StartColOffsetIdx: 1, EndColOffsetIdx: 2},
	}, 2, 2, nil, nil)
	doc.AddPicture(&ImageRef{Mimetype: "image/png", URI: "data:image/png;base64,AAAA"}, nil, nil)
	doc.AddCode("println(\"hi\")", "go", nil, nil)
	doc.AddFormula("E=mc^2", nil, nil)
	return doc
}

// assertContains 断言 s 包含全部子串。
func assertContains(t *testing.T, name, s string, subs ...string) {
	t.Helper()
	for _, sub := range subs {
		if !strings.Contains(s, sub) {
			t.Fatalf("%s missing %q:\n%s", name, sub, s)
		}
	}
}

// TestExportHTMLFullCoverage 验证 ExportHTML 覆盖全部元素类型且输出完整
// HTML 文档（DOCTYPE/head/body、title 取 Meta.Title、文本经 HTML 转义）。
func TestExportHTMLFullCoverage(t *testing.T) {
	out := ExportHTML(buildHTMLSampleDoc())
	assertContains(t, "html", out,
		"<!DOCTYPE html>",
		"<title>样例文档</title>",
		"<h1>样例文档</h1>",
		"<h2>概述</h2>",
		"<p>第一段内容</p>",
		"<ol>", "<li>项二</li>",
		"<table>", "<thead>", "<th>列A</th>", "<td>值1</td>", "</tbody>",
		`<img src="data:image/png;base64,AAAA"`,
		`<pre><code data-lang="go">`,
		`<span class="formula">E=mc^2</span>`,
		"</body>", "</html>",
	)
	if !strings.HasPrefix(out, "<!DOCTYPE html>") || strings.HasSuffix(out, "\n\n") {
		t.Fatalf("html document shape wrong: %q", out[:50])
	}
	// HTML 转义：代码中的引号不得破坏文档结构
	if strings.Contains(out, `println("hi")`) {
		t.Fatalf("code text should be escaped: %s", out)
	}
}

// TestExportHTMLEmptyAndFallbacks 验证空文档骨架、无 URI 图片占位与
// title 回退文档名。
func TestExportHTMLEmptyAndFallbacks(t *testing.T) {
	out := ExportHTML(nil)
	assertContains(t, "empty html", out, "<!DOCTYPE html>", "<body>", "</body>")
	doc := NewDoclingDocument("回退名")
	doc.AddPicture(&ImageRef{}, nil, nil)
	out2 := ExportHTML(doc)
	assertContains(t, "fallback html", out2,
		"<title>回退名</title>",
		`<div class="image-placeholder">[图片]</div>`)
}

// TestToHTMLAndToMarkdownMethods 验证方法形式入口与函数形式等价。
func TestToHTMLAndToMarkdownMethods(t *testing.T) {
	doc := buildHTMLSampleDoc()
	if doc.ToHTML() != ExportHTML(doc) {
		t.Fatal("ToHTML should equal ExportHTML")
	}
	if doc.ToMarkdown() != ExportMarkdown(doc) {
		t.Fatal("ToMarkdown should equal ExportMarkdown")
	}
}

// TestParseByExtToMarkdown 验证一步导出：Markdown 输入按原文还原、docx 输入
// 经解析还原、未知扩展名返回错误。
func TestParseByExtToMarkdown(t *testing.T) {
	out, err := ParseByExtToMarkdown("样例.md", []byte("# 标题一\n\n正文内容\n"))
	if err != nil {
		t.Fatalf("md: %v", err)
	}
	assertContains(t, "md out", out, "# 标题一", "正文内容")

	docxData := mustZipDocx(t, buildStructuredDocxXML(
		`<w:p><w:pPr><w:pStyle w:val="Heading1"/></w:pPr><w:r><w:t>部署指南</w:t></w:r></w:p>`+
			`<w:p><w:r><w:t>准备 Docker 环境。</w:t></w:r></w:p>`))
	out, err = ParseByExtToMarkdown("部署.docx", docxData)
	if err != nil {
		t.Fatalf("docx: %v", err)
	}
	assertContains(t, "docx out", out, "# 部署指南", "准备 Docker 环境。")

	if _, err := ParseByExtToMarkdown("unknown.xyz", []byte("x")); err == nil {
		t.Fatal("unknown ext should return error")
	}
}

package docling

import (
	"strings"
	"testing"
)

// TestParseHTML_HeadingTreeAndSectionPath 验证标题按官方平铺，content_list
// 仍根据 level 推导完整章节路径。
func TestParseHTML_HeadingTreeAndSectionPath(t *testing.T) {
	htmlSrc := `<!DOCTYPE html>
<html>
<head><title>页面标题</title></head>
<body>
<h1>部署指南</h1>
<p>准备开始。</p>
<h2>环境要求</h2>
<p>需要 Docker。</p>
<h3>系统要求</h3>
<p>64 位系统。</p>
</body>
</html>`
	doc, err := ParseHTML([]byte(htmlSrc))
	if err != nil {
		t.Fatalf("parse html: %v", err)
	}
	items := ToContentList(doc, SourceGolight)
	type expect struct {
		text string
		path string
		lv   int64
	}
	expects := []expect{
		{"部署指南", "部署指南", 1},
		{"准备开始。", "部署指南", 0},
		{"环境要求", "部署指南>环境要求", 2},
		{"需要 Docker。", "部署指南>环境要求", 0},
		{"系统要求", "部署指南>环境要求>系统要求", 3},
		{"64 位系统。", "部署指南>环境要求>系统要求", 0},
	}
	if len(items) != len(expects) {
		t.Fatalf("expected %d items, got %d: %+v", len(expects), len(items), items)
	}
	for i, want := range expects {
		if items[i].Text != want.text {
			t.Fatalf("item %d text = %q, want %q", i, items[i].Text, want.text)
		}
		if strings.Join(items[i].SectionPath, ">") != want.path {
			t.Fatalf("item %q section path = %v, want %v", want.text, items[i].SectionPath, want.path)
		}
		if items[i].TextLevel != want.lv {
			t.Fatalf("item %q text level = %d, want %d", want.text, items[i].TextLevel, want.lv)
		}
	}
	// 详细结构中 h1 → title、h2/h3 → section_header level 1/2。
	var h1, h2, h3 *TextItem
	for i := range doc.Texts {
		switch doc.Texts[i].Text {
		case "部署指南":
			h1 = &doc.Texts[i]
		case "环境要求":
			h2 = &doc.Texts[i]
		case "系统要求":
			h3 = &doc.Texts[i]
		}
	}
	if h1 == nil || h1.Label != LabelTitle {
		t.Fatalf("h1 missing or invalid: %+v", h1)
	}
	if h2 == nil || h2.Label != LabelSectionHeader || h2.TextLevel != 1 {
		t.Fatalf("h2 heading not recognized: %+v", h2)
	}
	if h3 == nil || h3.Label != LabelSectionHeader || h3.TextLevel != 2 {
		t.Fatalf("h3 heading not recognized: %+v", h3)
	}
	// 标题全部直接挂 body。
	if h2.Parent == nil || h2.Parent.Kind != refBody {
		t.Fatalf("h2 parent = %v, want body", h2.Parent)
	}
	if h3.Parent == nil || h3.Parent.Kind != refBody {
		t.Fatalf("h3 parent = %v, want body", h3.Parent)
	}
	// <title> 属于文档元数据，不产出正文
	if strings.Contains(doc.Text(), "页面标题") {
		t.Fatalf("<title> leaked into body text: %q", doc.Text())
	}
}

// TestParseHTML_NestedList 验证 ul/ol 列表分组与列表项产出：嵌套列表挂上一
// 列表项、ol 产出有序编号标记、ul 无编号。
func TestParseHTML_NestedList(t *testing.T) {
	htmlSrc := `<html><body>
<ul><li>外层一</li><li>外层二<ul><li>内层一</li></ul></li></ul>
<ol><li>第一步</li><li>第二步</li></ol>
</body></html>`
	doc, err := ParseHTML([]byte(htmlSrc))
	if err != nil {
		t.Fatalf("parse html: %v", err)
	}
	// 两组列表均挂 body
	if len(doc.Body.Children) != 2 || doc.Body.Children[0].Kind != refGroups || doc.Body.Children[1].Kind != refGroups {
		t.Fatalf("body children = %v, want two list group refs", doc.Body.Children)
	}
	if len(doc.Groups) != 3 {
		t.Fatalf("groups = %d, want 3 (ul + nested ul + ol)", len(doc.Groups))
	}
	outer := doc.Groups[0]
	if outer.Label != GroupLabelList || outer.Name != "list" {
		t.Fatalf("ul group = %+v, want list group", outer)
	}
	// 外层列表项：外层一、外层二（嵌套列表不并入项文本）
	if len(doc.Texts) != 5 {
		t.Fatalf("texts = %d, want 5", len(doc.Texts))
	}
	if doc.Texts[0].Text != "外层一" || doc.Texts[1].Text != "外层二" || doc.Texts[2].Text != "内层一" {
		t.Fatalf("ul item texts = [%s %s %s]", doc.Texts[0].Text, doc.Texts[1].Text, doc.Texts[2].Text)
	}
	if doc.Texts[0].Enumerated == nil || *doc.Texts[0].Enumerated {
		t.Fatalf("ul item should be unordered: %+v", doc.Texts[0])
	}
	if doc.Texts[0].Marker != "" || doc.Texts[1].Marker != "" {
		t.Fatalf("ul markers = [%s %s], want empty", doc.Texts[0].Marker, doc.Texts[1].Marker)
	}
	// 嵌套列表分组挂在外层二（texts:1）下，内层项挂在嵌套分组下
	nested := doc.Groups[1]
	if nested.Parent == nil || nested.Parent.Kind != refTexts || nested.Parent.Idx != 1 {
		t.Fatalf("nested group parent = %v, want texts:1", nested.Parent)
	}
	if doc.Texts[2].Parent == nil || doc.Texts[2].Parent.Kind != refGroups || doc.Texts[2].Parent.Idx != 1 {
		t.Fatalf("nested item parent = %v, want groups:1", doc.Texts[2].Parent)
	}
	// ol 有序编号
	olGroup := doc.Groups[2]
	if olGroup.Name != "ordered list" {
		t.Fatalf("ol group name = %q, want ordered list", olGroup.Name)
	}
	if doc.Texts[3].Text != "第一步" || doc.Texts[3].Marker != "1." || doc.Texts[3].Enumerated == nil || !*doc.Texts[3].Enumerated {
		t.Fatalf("ol item 1 = %+v, want enumerated with marker 1.", doc.Texts[3])
	}
	if doc.Texts[4].Marker != "2." {
		t.Fatalf("ol item 2 marker = %q, want 2.", doc.Texts[4].Marker)
	}
}

// TestParseHTML_PreCodeLanguage 验证 pre 代码块保留换行并从 class 前缀提取语言。
func TestParseHTML_PreCodeLanguage(t *testing.T) {
	htmlSrc := `<html><body>
<pre class="language-go highlight">for i := 0; i &lt; 3; i++ {
    fmt.Println(i)
}</pre>
</body></html>`
	doc, err := ParseHTML([]byte(htmlSrc))
	if err != nil {
		t.Fatalf("parse html: %v", err)
	}
	if len(doc.Texts) != 1 {
		t.Fatalf("texts = %d, want 1: %+v", len(doc.Texts), doc.Texts)
	}
	code := doc.Texts[0]
	if code.Label != LabelCode {
		t.Fatalf("label = %s, want code", code.Label)
	}
	if code.CodeLanguage != "go" {
		t.Fatalf("code language = %q, want go", code.CodeLanguage)
	}
	if !strings.Contains(code.Text, "\n") || !strings.Contains(code.Text, "fmt.Println(i)") || !strings.Contains(code.Text, "i < 3") {
		t.Fatalf("code text = %q, want multiline with decoded entities", code.Text)
	}
}

// TestParseHTML_TableSpansAndHeaders 验证表格：th 表头行 column_header、
// 数据行内 th 单元格 row_header、colspan/rowspan 跨度（start 闭 end 开）与
// 已占格跳过。
func TestParseHTML_TableSpansAndHeaders(t *testing.T) {
	htmlSrc := `<html><body>
<table>
<tr><th>名称</th><th>说明</th></tr>
<tr><td>alpha</td><td>第一个</td></tr>
<tr><td>x</td><th>行头</th></tr>
<tr><td rowspan="2">跨行</td><td>a1</td></tr>
<tr><td>a2</td></tr>
<tr><td colspan="2">合并说明</td></tr>
</table>
</body></html>`
	doc, err := ParseHTML([]byte(htmlSrc))
	if err != nil {
		t.Fatalf("parse html: %v", err)
	}
	if len(doc.Tables) != 1 {
		t.Fatalf("tables = %d, want 1", len(doc.Tables))
	}
	data := doc.Tables[0].Data
	if data.NumRows != 6 || data.NumCols != 2 {
		t.Fatalf("table size = %dx%d, want 6x2", data.NumRows, data.NumCols)
	}
	// 表头行：全 th → column_header=true
	for _, col := range []int64{0, 1} {
		c := findHTMLCell(t, data, 0, col)
		if !c.ColumnHeader || c.RowHeader {
			t.Fatalf("header cell (0,%d) column_header=%v row_header=%v", col, c.ColumnHeader, c.RowHeader)
		}
	}
	// 数据行：td 单元格无表头语义
	c := findHTMLCell(t, data, 1, 0)
	if c.ColumnHeader || c.RowHeader || c.Text != "alpha" {
		t.Fatalf("data cell (1,0) = %+v", c)
	}
	// 数据行中的 th 单元格 → row_header=true
	th := findHTMLCell(t, data, 2, 1)
	if th.ColumnHeader || !th.RowHeader || th.Text != "行头" {
		t.Fatalf("th-in-body cell (2,1) = %+v, want row_header", th)
	}
	// rowspan：跨行单元格占两行一列（start 闭 end 开）
	span := findHTMLCell(t, data, 3, 0)
	if span.RowSpan != 2 || span.StartRowOffsetIdx != 3 || span.EndRowOffsetIdx != 5 {
		t.Fatalf("rowspan cell = %+v, want row_span 2 rows 3..5", span)
	}
	// 下一行的 a2 被 rowspan 占位挤到第二列
	if c := findHTMLCell(t, data, 4, 1); c.Text != "a2" || c.StartColOffsetIdx != 1 {
		t.Fatalf("occupied-cell skip failed: %+v", c)
	}
	// colspan：合并单元格占两列
	merged := findHTMLCell(t, data, 5, 0)
	if merged.ColSpan != 2 || merged.StartColOffsetIdx != 0 || merged.EndColOffsetIdx != 2 || merged.Text != "合并说明" {
		t.Fatalf("colspan cell = %+v", merged)
	}
}

// TestParseHTMLRichTableCell 验证富表格单元格通过 ref 关联保留格式与链接的
// TextItem，而表格网格文本仍可独立渲染。
func TestParseHTMLRichTableCell(t *testing.T) {
	doc, err := ParseHTML([]byte(`<table><tr><th>名称</th></tr><tr><td><strong><a href="https://example.com/a">设备 A</a></strong></td></tr></table>`))
	if err != nil {
		t.Fatalf("ParseHTML: %v", err)
	}
	cell := findHTMLCell(t, doc.Tables[0].Data, 1, 0)
	if cell.Ref == nil || cell.Ref.Kind != refTexts {
		t.Fatalf("rich cell ref missing: %+v", cell)
	}
	text := doc.Texts[cell.Ref.Idx]
	if text.Text != "设备 A" || text.Hyperlink != "https://example.com/a" || text.Formatting == nil || !text.Formatting.Bold {
		t.Fatalf("rich cell metadata wrong: %+v", text)
	}
}

// findHTMLCell 按起始行列查找表格单元格。
func findHTMLCell(t *testing.T, data *TableData, row, col int64) *DoclingTableCell {
	t.Helper()
	for i := range data.TableCells {
		c := &data.TableCells[i]
		if c.StartRowOffsetIdx == row && c.StartColOffsetIdx == col {
			return c
		}
	}
	t.Fatalf("cell (%d,%d) not found in %d cells", row, col, len(data.TableCells))
	return nil
}

// TestParseHTML_BreakSplitting 验证 <br> 哨兵语义：单个为段内换行、
// 连续两个以上为分段。
func TestParseHTML_BreakSplitting(t *testing.T) {
	htmlSrc := `<html><body>
<p>第一行<br>第二行</p>
<p>段一<br><br>段二</p>
</body></html>`
	doc, err := ParseHTML([]byte(htmlSrc))
	if err != nil {
		t.Fatalf("parse html: %v", err)
	}
	if len(doc.Texts) != 3 {
		t.Fatalf("texts = %d, want 3: %+v", len(doc.Texts), doc.Texts)
	}
	want := []string{"第一行\n第二行", "段一", "段二"}
	for i, w := range want {
		if doc.Texts[i].Text != w {
			t.Fatalf("text %d = %q, want %q", i, doc.Texts[i].Text, w)
		}
		if doc.Texts[i].Label != LabelText {
			t.Fatalf("text %d label = %s, want text", i, doc.Texts[i].Label)
		}
	}
}

// TestParseHTML_ScriptHiddenIgnored 验证 script/style/hidden 元素不产出正文。
func TestParseHTML_ScriptHiddenIgnored(t *testing.T) {
	htmlSrc := `<html><body>
<script>var secret = 1;</script>
<style>.secret { color: red; }</style>
<noscript>需要启用脚本</noscript>
<p hidden>隐藏文本</p>
<p style="display:none">样式隐藏</p>
<div aria-hidden="true">辅助隐藏</div>
<p>可见文本</p>
</body></html>`
	doc, err := ParseHTML([]byte(htmlSrc))
	if err != nil {
		t.Fatalf("parse html: %v", err)
	}
	items := ToContentList(doc, SourceGolight)
	if len(items) != 1 || items[0].Text != "可见文本" {
		t.Fatalf("items = %+v, want only 可见文本", items)
	}
	for _, banned := range []string{"secret", "隐藏文本", "样式隐藏", "辅助隐藏", "需要启用脚本"} {
		if strings.Contains(doc.Text(), banned) {
			t.Fatalf("suppressed content %q leaked into text: %q", banned, doc.Text())
		}
	}
}

// TestParseHTML_ImageAlt 验证 img 始终产出 PictureItem，alt/figcaption 通过
// captions 引用保存，figure 内 figcaption 优先于 alt。
func TestParseHTML_ImageAlt(t *testing.T) {
	htmlSrc := `<html><body>
<img src="a.png" alt="架构图">
<img src="b.png">
<figure><img src="c.png" alt="alt文本"><figcaption>图注文本</figcaption></figure>
</body></html>`
	doc, err := ParseHTML([]byte(htmlSrc))
	if err != nil {
		t.Fatalf("parse html: %v", err)
	}
	if len(doc.Pictures) != 3 {
		t.Fatalf("pictures = %d, want 3: %+v", len(doc.Pictures), doc.Pictures)
	}
	if doc.Pictures[0].Image == nil || doc.Pictures[0].Image.URI != "a.png" || len(doc.Pictures[0].Captions) != 1 {
		t.Fatalf("img placeholder/caption = %+v", doc.Pictures[0])
	}
	if len(doc.Pictures[1].Captions) != 0 {
		t.Fatalf("image without alt should have no caption: %+v", doc.Pictures[1])
	}
	captionRef := doc.Pictures[2].Captions[0]
	if doc.Texts[captionRef.Idx].Text != "图注文本" {
		t.Fatalf("figcaption should win over alt: %+v", doc.Texts[captionRef.Idx])
	}
}

// TestParseHTML_InlineHyperlink 验证段落内链接文本保留且 href 写入 Hyperlink。
func TestParseHTML_InlineHyperlink(t *testing.T) {
	htmlSrc := `<html><body>
<p>参见 <a href="https://example.com/doc">官方文档</a> 获取详情。</p>
</body></html>`
	doc, err := ParseHTML([]byte(htmlSrc))
	if err != nil {
		t.Fatalf("parse html: %v", err)
	}
	if len(doc.Texts) != 1 {
		t.Fatalf("texts = %d, want 1: %+v", len(doc.Texts), doc.Texts)
	}
	if doc.Texts[0].Text != "参见 官方文档 获取详情。" {
		t.Fatalf("paragraph text = %q", doc.Texts[0].Text)
	}
	if doc.Texts[0].Hyperlink != "https://example.com/doc" {
		t.Fatalf("hyperlink = %q", doc.Texts[0].Hyperlink)
	}
}

// TestParseHTMLTitleFurnitureAndFormatting 验证 head title 与首个正文标题前
// 内容进入 furniture，行内样式写入 formatting。
func TestParseHTMLTitleFurnitureAndFormatting(t *testing.T) {
	withoutH1, err := ParseHTML([]byte(`<html><head><title>页面标题</title></head><body><p>正文</p></body></html>`))
	if err != nil {
		t.Fatalf("ParseHTML title: %v", err)
	}
	if len(withoutH1.Texts) < 2 || withoutH1.Texts[0].Label != LabelTitle || withoutH1.Texts[0].Text != "页面标题" ||
		withoutH1.Texts[0].ContentLayer != LayerFurniture || len(withoutH1.Furniture.Children) != 1 {
		t.Fatalf("head title missing: %+v", withoutH1.Texts)
	}

	doc, err := ParseHTML([]byte(`<html><body><nav><b>导航</b></nav><h1>正文标题</h1><p><strong>重要正文</strong></p></body></html>`))
	if err != nil {
		t.Fatalf("ParseHTML furniture: %v", err)
	}
	if len(doc.Furniture.Children) != 1 {
		t.Fatalf("leading furniture missing: %+v", doc.Furniture.Children)
	}
	leading := doc.Texts[doc.Furniture.Children[0].Idx]
	if leading.Text != "导航" || leading.ContentLayer != LayerFurniture {
		t.Fatalf("leading furniture wrong: %+v", leading)
	}
	var important *TextItem
	for i := range doc.Texts {
		if doc.Texts[i].Text == "重要正文" {
			important = &doc.Texts[i]
		}
	}
	if important == nil || important.Formatting == nil || !important.Formatting.Bold {
		t.Fatalf("inline formatting missing: %+v", important)
	}
}

// TestParseHTML_EmptyInput 验证空输入与无有效内容输入返回空文档不报错。
func TestParseHTML_EmptyInput(t *testing.T) {
	cases := [][]byte{
		nil,
		[]byte(""),
		[]byte("   \n\t "),
		[]byte("<div></div>"),
		[]byte("<html><body><script>x=1</script></body></html>"),
	}
	for i, data := range cases {
		doc, err := ParseHTML(data)
		if err != nil {
			t.Fatalf("case %d: unexpected error: %v", i, err)
		}
		if items := ToContentList(doc, SourceGolight); len(items) != 0 {
			t.Fatalf("case %d: expected empty result, got %d items", i, len(items))
		}
	}
}

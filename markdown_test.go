package docling

import (
	"strings"
	"testing"
)

// TestParseMarkdownContentItems_FencedCodeBlockIntegrity 验证围栏代码块独立成块：
// 代码内的 # 注释行不会被当作标题切断（旧版纯文本分块会把该行误判为标题）。
func TestParseMarkdownContentItems_FencedCodeBlockIntegrity(t *testing.T) {
	md := "## 使用说明\n\n```bash\n# 这是 bash 注释\necho hello\n```\n\n以上内容来自脚本示例。"
	doc, err := ParseMarkdown([]byte(md))
	if err != nil {
		t.Fatalf("parse markdown: %v", err)
	}
	items := ToContentList(doc, SourceGolight)
	text := strings.TrimSpace(doc.Text())
	if len(items) < 3 {
		t.Fatalf("expected at least 3 items, got %d: %+v", len(items), items)
	}
	var codeItem *Item
	for i := range items {
		if strings.Contains(items[i].Text, "echo hello") {
			codeItem = &items[i]
			break
		}
	}
	if codeItem == nil {
		t.Fatalf("code block content missing from items: %+v", items)
	}
	if !strings.Contains(codeItem.Text, "# 这是 bash 注释") {
		t.Fatalf("code block content incomplete: %q", codeItem.Text)
	}
	// 代码块注释必须与代码同块，不能被拆成独立标题块
	for _, item := range items {
		if strings.TrimSpace(item.Text) == "# 这是 bash 注释" {
			t.Fatalf("code comment wrongly split as standalone heading: %+v", items)
		}
	}
	if !strings.Contains(text, "echo hello") {
		t.Fatalf("aggregated text missing code content: %q", text)
	}
}

// TestParseMarkdownContentItems_GfmTableExtracted 验证 GFM 表格被提取为独立的 table 元素。
func TestParseMarkdownContentItems_GfmTableExtracted(t *testing.T) {
	md := "## 参数说明\n\n| 参数 | 说明 |\n| --- | --- |\n| name | 设备名称 |\n| code | 设备编码 |\n\n上述参数必填。"
	doc, err := ParseMarkdown([]byte(md))
	if err != nil {
		t.Fatalf("parse markdown: %v", err)
	}
	items := ToContentList(doc, SourceGolight)
	text := strings.TrimSpace(doc.Text())
	var tableItem *Item
	for i := range items {
		if items[i].Type == ItemTypeTable {
			tableItem = &items[i]
			break
		}
	}
	if tableItem == nil {
		t.Fatalf("table item missing from items: %+v", items)
	}
	if !strings.Contains(tableItem.TableBody, "设备名称") || !strings.Contains(tableItem.TableBody, "设备编码") {
		t.Fatalf("table body incomplete: %q", tableItem.TableBody)
	}
	// 清洗链应补出标准分隔行
	if !strings.Contains(tableItem.TableBody, "| --- |") && !strings.Contains(tableItem.TableBody, "| ---") {
		t.Fatalf("table body missing separator row: %q", tableItem.TableBody)
	}
	if strings.Join(tableItem.SectionPath, ">") != "参数说明" {
		t.Fatalf("table section path = %v, want [参数说明]", tableItem.SectionPath)
	}
	if !strings.Contains(text, "上述参数必填。") {
		t.Fatalf("aggregated text missing paragraph: %q", text)
	}
}

// TestParseMarkdownContentItems_SetextHeading 验证 setext 标题（下划线式）被正确识别为标题。
func TestParseMarkdownContentItems_SetextHeading(t *testing.T) {
	md := "概述\n====\n\n这是概述内容。\n\n子标题\n----\n\n子内容。"
	doc, err := ParseMarkdown([]byte(md))
	if err != nil {
		t.Fatalf("parse markdown: %v", err)
	}
	items := ToContentList(doc, SourceGolight)
	// setext 下划线是标题的一部分，不产生独立块：概述(H1)+正文+子标题(H2)+正文 = 4 块
	if len(items) != 4 {
		t.Fatalf("expected 4 items, got %d: %+v", len(items), items)
	}
	if items[0].Text != "概述" || items[0].TextLevel != 1 {
		t.Fatalf("setext h1 not recognized: %+v", items[0])
	}
	if strings.Contains(items[0].Text, "===") {
		t.Fatalf("setext underline leaked into heading text: %q", items[0].Text)
	}
	// Docling JSON 把 H2 归一为 level=1，content_list 继续输出展示层级 2。
	found := false
	for _, item := range items {
		if item.Text == "子标题" {
			found = true
			if item.TextLevel != 2 {
				t.Fatalf("setext h2 level = %d, want 2", item.TextLevel)
			}
		}
	}
	if !found {
		t.Fatalf("setext h2 missing from items: %+v", items)
	}
}

// TestParseMarkdownContentItems_SectionPathHierarchy 验证多级标题生成完整章节路径。
func TestParseMarkdownContentItems_SectionPathHierarchy(t *testing.T) {
	md := "# 部署指南\n\n## 环境要求\n\n需要 Docker。\n\n## 安装步骤\n\n执行安装脚本。"
	doc, err := ParseMarkdown([]byte(md))
	if err != nil {
		t.Fatalf("parse markdown: %v", err)
	}
	items := ToContentList(doc, SourceGolight)
	type expect struct {
		text    string
		path    string
		isTitle bool
	}
	expects := []expect{
		{"部署指南", "部署指南", true},
		{"环境要求", "部署指南>环境要求", true},
		{"需要 Docker。", "部署指南>环境要求", false},
		{"安装步骤", "部署指南>安装步骤", true},
		{"执行安装脚本。", "部署指南>安装步骤", false},
	}
	idx := 0
	for _, want := range expects {
		found := false
		for ; idx < len(items); idx++ {
			item := items[idx]
			if strings.TrimSpace(item.Text) != want.text {
				continue
			}
			if want.isTitle && item.TextLevel == 0 {
				t.Fatalf("item %q should be heading, got level 0", want.text)
			}
			if strings.Join(item.SectionPath, ">") != want.path {
				t.Fatalf("item %q section path = %v, want %v", want.text, item.SectionPath, want.path)
			}
			found = true
			idx++
			break
		}
		if !found {
			t.Fatalf("expected item %q not found in order, items: %+v", want.text, items)
		}
	}
}

// TestParseMarkdownContentItems_ListAndQuoteInSection 验证列表与引用归入所在章节且内容不丢失。
func TestParseMarkdownContentItems_ListAndQuoteInSection(t *testing.T) {
	md := "## 操作步骤\n\n1. 登录平台\n2. 创建产品\n\n> 注意保存密钥\n"
	doc, err := ParseMarkdown([]byte(md))
	if err != nil {
		t.Fatalf("parse markdown: %v", err)
	}
	items := ToContentList(doc, SourceGolight)
	text := strings.TrimSpace(doc.Text())
	joined := JoinItemTexts(items)
	for _, keyword := range []string{"登录平台", "创建产品", "注意保存密钥"} {
		if !strings.Contains(joined, keyword) {
			t.Fatalf("list/quote content %q missing from items: %+v", keyword, items)
		}
	}
	for _, item := range items {
		if strings.Contains(item.Text, "登录平台") && strings.Join(item.SectionPath, ">") != "操作步骤" {
			t.Fatalf("list item section path = %v, want [操作步骤]", item.SectionPath)
		}
	}
	if !strings.Contains(text, "创建产品") {
		t.Fatalf("aggregated text missing list content: %q", text)
	}
}

// TestParseMarkdownContentItems_HtmlBlockDelegated 验证原始 HTML 块委托给
// HTML 后端解析，保留其文本、链接和格式语义。
func TestParseMarkdownContentItems_HtmlBlockDelegated(t *testing.T) {
	md := "# 标题\n\n<div class=\"x\">raw html content</div>\n\n正文段落。"
	doc, err := ParseMarkdown([]byte(md))
	if err != nil {
		t.Fatalf("parse markdown: %v", err)
	}
	items := ToContentList(doc, SourceGolight)
	text := strings.TrimSpace(doc.Text())
	if !strings.Contains(text, "raw html content") {
		t.Fatalf("delegated html block missing: %q items=%+v", text, items)
	}
}

// TestParseMarkdownContentItems_EmptyAndFallback 验证空输入与全无效内容返回空结果且不报错，
// 由调用方回退纯文本逻辑。
func TestParseMarkdownContentItems_EmptyAndFallback(t *testing.T) {
	cases := [][]byte{
		nil,
		[]byte(""),
		[]byte("   \n\n  \t\n"),
		[]byte("<div></div>\n"),
	}
	for i, data := range cases {
		pdoc, err := ParseMarkdown(data)
		if err != nil {
			t.Fatalf("case %d: unexpected error: %v", i, err)
		}
		items := ToContentList(pdoc, SourceGolight)
		text := strings.TrimSpace(pdoc.Text())
		if len(items) != 0 || strings.TrimSpace(text) != "" {
			t.Fatalf("case %d: expected empty result, got %d items text=%q", i, len(items), text)
		}
	}
}

// TestParseMarkdownOfficialHeadingsImagesAndInlineMeta 验证标题按官方平铺，
// H2/H3 的 level 分别为 1/2；图片产出 PictureItem；行内样式和链接写入文本元数据。
func TestParseMarkdownOfficialHeadingsImagesAndInlineMeta(t *testing.T) {
	src := `# 文档标题

## 第一节

**粗体正文**

[官方文档](https://example.com/doc)

![架构图](images/arch.png "系统架构")

### 子节

子节正文。`
	doc, err := ParseMarkdown([]byte(src))
	if err != nil {
		t.Fatalf("ParseMarkdown: %v", err)
	}
	if len(doc.Texts) < 7 {
		t.Fatalf("texts missing: %+v", doc.Texts)
	}
	if doc.Texts[1].Label != LabelSectionHeader || doc.Texts[1].TextLevel != 1 ||
		doc.Texts[1].Parent == nil || doc.Texts[1].Parent.Kind != refBody {
		t.Fatalf("H2 not canonical/flat: %+v", doc.Texts[1])
	}
	if doc.Texts[5].Label != LabelSectionHeader || doc.Texts[5].TextLevel != 2 ||
		doc.Texts[5].Parent == nil || doc.Texts[5].Parent.Kind != refBody {
		t.Fatalf("H3 not canonical/flat: %+v", doc.Texts[5])
	}
	if doc.Texts[2].Formatting == nil || !doc.Texts[2].Formatting.Bold || doc.Texts[2].Text != "粗体正文" {
		t.Fatalf("bold formatting missing: %+v", doc.Texts[2])
	}
	if doc.Texts[3].Hyperlink != "https://example.com/doc" || doc.Texts[3].Text != "官方文档" {
		t.Fatalf("hyperlink missing: %+v", doc.Texts[3])
	}
	if len(doc.Pictures) != 1 || doc.Pictures[0].Image == nil || doc.Pictures[0].Image.URI != "images/arch.png" {
		t.Fatalf("picture placeholder missing: %+v", doc.Pictures)
	}
	if len(doc.Pictures[0].Captions) != 1 {
		t.Fatalf("picture caption ref missing: %+v", doc.Pictures[0])
	}
	items := ToContentList(doc, SourceGolight)
	for _, item := range items {
		if item.Text == "子节正文。" && strings.Join(item.SectionPath, ">") != "文档标题>第一节>子节" {
			t.Fatalf("flat heading path not derived: %+v", item)
		}
	}
}

// TestParseMarkdownContentItems_PlainTextAggregate 验证聚合纯文本与 content_list 拼接结果一致。
func TestParseMarkdownContentItems_PlainTextAggregate(t *testing.T) {
	md := "# 设备管理\n\n支持设备注册。\n\n| 字段 | 值 |\n| --- | --- |\n| id | 1 |\n\n```go\nfmt.Println(\"ok\")\n```"
	doc, err := ParseMarkdown([]byte(md))
	if err != nil {
		t.Fatalf("parse markdown: %v", err)
	}
	items := ToContentList(doc, SourceGolight)
	text := strings.TrimSpace(doc.Text())
	want := strings.TrimSpace(JoinItemTexts(items))
	if text != want {
		t.Fatalf("aggregated text mismatch:\n got: %q\nwant: %q", text, want)
	}
}

// TestParseMarkdownContentItems_EscapedPipeInTable 验证表格单元格中的转义管道符被正确还原，
// 不再切错列（旧版按裸管道切分会把 a\|b 拆成两列）。
func TestParseMarkdownContentItems_EscapedPipeInTable(t *testing.T) {
	md := "| 表达式 | 含义 |\n| --- | --- |\n| a\\|b | 逻辑或 |"
	doc, err := ParseMarkdown([]byte(md))
	if err != nil {
		t.Fatalf("parse markdown: %v", err)
	}
	items := ToContentList(doc, SourceGolight)
	var tableItem *Item
	for i := range items {
		if items[i].Type == ItemTypeTable {
			tableItem = &items[i]
			break
		}
	}
	if tableItem == nil {
		t.Fatalf("table item missing: %+v", items)
	}
	// 渲染时 a|b 应重新转义为 a\|b，保持表格列数正确
	if !strings.Contains(tableItem.TableBody, "a\\|b") {
		t.Fatalf("escaped pipe lost after re-render: %q", tableItem.TableBody)
	}
	// 数据行必须仍为 2 列：| a\|b | 逻辑或 |
	if !strings.Contains(tableItem.TableBody, "| a\\|b | 逻辑或 |") {
		t.Fatalf("table column layout broken: %q", tableItem.TableBody)
	}
}

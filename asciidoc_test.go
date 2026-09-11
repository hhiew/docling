package docparse

import (
	"strings"
	"testing"
)

// TestParseAsciiDoc_HeadingTreeAndSectionPath 验证 =/==/=== 标题层级树：
// == 级挂在 title 下、=== 挂在上一层标题下，简化后 _section_path 为祖先标题链。
func TestParseAsciiDoc_HeadingTreeAndSectionPath(t *testing.T) {
	src := `= 部署指南
== 环境要求

需要 Docker。

=== 硬件要求

64 位系统。

== 安装步骤

执行安装脚本。
`
	doc, err := ParseAsciiDoc([]byte(src))
	if err != nil {
		t.Fatalf("parse asciidoc: %v", err)
	}
	items := ToContentList(doc, SourceGolight)
	type expect struct {
		text string
		path string
		lv   int64
	}
	expects := []expect{
		{"部署指南", "部署指南", 1},
		{"环境要求", "部署指南>环境要求", 1},
		{"需要 Docker。", "部署指南>环境要求", 0},
		{"硬件要求", "部署指南>环境要求>硬件要求", 2},
		{"64 位系统。", "部署指南>环境要求>硬件要求", 0},
		{"安装步骤", "部署指南>安装步骤", 1},
		{"执行安装脚本。", "部署指南>安装步骤", 0},
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
	// 详细树：= → title；== → section_header level 1 且父为 title（对齐 docling
	// 的 parents[level-1] 挂接）；=== → level 2 且父为 == 标题
	if doc.Texts[0].Label != LabelTitle {
		t.Fatalf("= label = %s, want title", doc.Texts[0].Label)
	}
	if doc.Texts[1].Label != LabelSectionHeader || doc.Texts[1].TextLevel != 1 {
		t.Fatalf("== heading = %+v", doc.Texts[1])
	}
	if doc.Texts[1].Parent == nil || doc.Texts[1].Parent.Idx != 0 {
		t.Fatalf("== parent = %v, want title", doc.Texts[1].Parent)
	}
	if doc.Texts[3].TextLevel != 2 || doc.Texts[3].Parent == nil || doc.Texts[3].Parent.Idx != 1 {
		t.Fatalf("=== heading = %+v, want level 2 under ==", doc.Texts[3])
	}
}

// TestParseAsciiDoc_ListNesting 验证缩进驱动的列表嵌套与编号 marker：
// 嵌套子分组挂在上一层列表分组下（对齐 docling asciidoc 后端的分组链），
// 编号型 marker 保留、符号 marker 不保留。
func TestParseAsciiDoc_ListNesting(t *testing.T) {
	src := `== 操作步骤

* 登录平台
* 创建产品
  * 选择类型

1. 第一步
2. 第二步
`
	doc, err := ParseAsciiDoc([]byte(src))
	if err != nil {
		t.Fatalf("parse asciidoc: %v", err)
	}
	var items []*TextItem
	for i := range doc.Texts {
		if doc.Texts[i].Label == LabelListItem {
			items = append(items, &doc.Texts[i])
		}
	}
	if len(items) != 5 {
		t.Fatalf("list items = %d, want 5: %+v", len(items), doc.Texts)
	}
	wantTexts := []string{"登录平台", "创建产品", "选择类型", "第一步", "第二步"}
	for i, want := range wantTexts {
		if items[i].Text != want {
			t.Fatalf("item %d text = %q, want %q", i, items[i].Text, want)
		}
	}
	// 无序项 marker 为空、有序项保留编号 marker
	if items[0].Marker != "" || items[2].Marker != "" {
		t.Fatalf("bullet markers = [%s %s %s], want empty", items[0].Marker, items[1].Marker, items[2].Marker)
	}
	if items[3].Marker != "1." || items[4].Marker != "2." {
		t.Fatalf("ordered markers = [%s %s], want [1. 2.]", items[3].Marker, items[4].Marker)
	}
	if items[3].Enumerated == nil || !*items[3].Enumerated {
		t.Fatalf("ordered item should be enumerated: %+v", items[3])
	}
	// 嵌套分组链：子列表分组挂在第一个列表分组下（缩进+2 深一级），
	// "选择类型" 挂在嵌套分组下
	var listGroups []int
	for i := range doc.Groups {
		if doc.Groups[i].Label == GroupLabelList {
			listGroups = append(listGroups, i)
		}
	}
	// docling 行为：列表类型切换（* → 1.）且缩进不变时不新开分组，
	// 编号项直接并入回退后的外层分组，共 2 个分组
	if len(listGroups) != 2 {
		t.Fatalf("list groups = %d, want 2 (外层*组、嵌套*组)", len(listGroups))
	}
	nested := doc.Groups[listGroups[1]]
	if nested.Parent == nil || nested.Parent.Kind != refGroups || nested.Parent.Idx != int64(listGroups[0]) {
		t.Fatalf("nested group parent = %v, want outer list group", nested.Parent)
	}
	if items[2].Parent == nil || items[2].Parent.Kind != refGroups || items[2].Parent.Idx != int64(listGroups[1]) {
		t.Fatalf("nested item parent = %v, want nested list group", items[2].Parent)
	}
	// 缩进回落：编号项并入回退后的外层分组
	if items[3].Parent == nil || items[3].Parent.Kind != refGroups || items[3].Parent.Idx != int64(listGroups[0]) {
		t.Fatalf("ordered item parent = %v, want outer list group", items[3].Parent)
	}
}

// TestParseAsciiDoc_LiteralBlock 验证 `....` 字面块产出保留换行的 code 元素。
func TestParseAsciiDoc_LiteralBlock(t *testing.T) {
	doc, err := ParseAsciiDoc([]byte("前置说明\n\n....\nverbatim line\n保持  原样\n....\n"))
	if err != nil {
		t.Fatalf("parse asciidoc: %v", err)
	}
	var codes []*TextItem
	for i := range doc.Texts {
		if doc.Texts[i].Label == LabelCode {
			codes = append(codes, &doc.Texts[i])
		}
	}
	if len(codes) != 1 {
		t.Fatalf("code items = %d, want 1: %+v", len(codes), doc.Texts)
	}
	if codes[0].Text != "verbatim line\n保持  原样" {
		t.Fatalf("literal block text = %q, want raw multiline", codes[0].Text)
	}
}

// TestParseAsciiDoc_SourceBlock 验证 [source,lang] + ---- 代码块产出
// 带语言标记的 code 元素。
func TestParseAsciiDoc_SourceBlock(t *testing.T) {
	src := "[source,go]\n----\nfmt.Println(\"ok\")\nfmt.Println(\"done\")\n----\n"
	doc, err := ParseAsciiDoc([]byte(src))
	if err != nil {
		t.Fatalf("parse asciidoc: %v", err)
	}
	if len(doc.Texts) != 1 {
		t.Fatalf("texts = %d, want 1: %+v", len(doc.Texts), doc.Texts)
	}
	code := doc.Texts[0]
	if code.Label != LabelCode || code.CodeLanguage != "go" {
		t.Fatalf("source block = %+v, want code with language go", code)
	}
	if code.Text != "fmt.Println(\"ok\")\nfmt.Println(\"done\")" {
		t.Fatalf("source block text = %q", code.Text)
	}
}

// TestParseAsciiDoc_TableWithCellSpec 验证 |=== 表格：cell 装饰剥离、h 装饰
// 表头标记、首行 column_header、行数不齐时按最大列数补齐。
func TestParseAsciiDoc_TableWithCellSpec(t *testing.T) {
	src := `.参数表
|===
^.^h|名称|^.^h|说明
| alpha | 第一个
| beta |
|===
`
	doc, err := ParseAsciiDoc([]byte(src))
	if err != nil {
		t.Fatalf("parse asciidoc: %v", err)
	}
	if len(doc.Tables) != 1 {
		t.Fatalf("tables = %d, want 1", len(doc.Tables))
	}
	data := doc.Tables[0].Data
	if data.NumRows != 3 || data.NumCols != 2 {
		t.Fatalf("table size = %dx%d, want 3x2", data.NumRows, data.NumCols)
	}
	// 首行：h 装饰 cell 文本剥离装饰且 column_header=true
	first := doc.Tables[0].Data.TableCells
	if len(first) != 6 {
		t.Fatalf("cells = %d, want 6 (2+2+2)", len(first))
	}
	if first[0].Text != "名称" || first[1].Text != "说明" {
		t.Fatalf("header cells = [%q %q], want 装饰已剥离", first[0].Text, first[1].Text)
	}
	if !first[0].ColumnHeader || !first[1].ColumnHeader {
		t.Fatalf("h cells column_header = [%v %v], want true", first[0].ColumnHeader, first[1].ColumnHeader)
	}
	if first[0].StartColOffsetIdx != 0 || first[1].StartColOffsetIdx != 1 {
		t.Fatalf("header cell columns = [%d %d], want [0 1]", first[0].StartColOffsetIdx, first[1].StartColOffsetIdx)
	}
	// 数据行无 h 装饰：column_header=false（首行语义仅作用于 row 0）
	if first[2].Text != "alpha" || first[2].ColumnHeader {
		t.Fatalf("data cell = %+v, want non-header", first[2])
	}
	// 行数不齐补齐语义：末行 "| beta |" 产出 beta 与行尾空 cell（对齐
	// split("|")[1:]），列缺口由行列规模（num_cols）表达
	for _, c := range data.TableCells {
		if c.StartRowOffsetIdx == 2 && c.StartColOffsetIdx == 0 && c.Text != "beta" {
			t.Fatalf("row 2 first cell text = %q, want beta", c.Text)
		}
	}
	// 前导 "." 行产出 caption
	foundCaption := false
	for i := range doc.Texts {
		if doc.Texts[i].Label == LabelCaption && doc.Texts[i].Text == "参数表" {
			foundCaption = true
		}
	}
	if !foundCaption {
		t.Fatalf("caption 参数表 missing: %+v", doc.Texts)
	}
}

// TestParseAsciiDoc_ParagraphFlush 验证多行段落按空行冲刷为独立 paragraph 元素，
// 行间以空格合并。
func TestParseAsciiDoc_ParagraphFlush(t *testing.T) {
	src := `这是第一段，
跨行继续。

这是第二段。
`
	doc, err := ParseAsciiDoc([]byte(src))
	if err != nil {
		t.Fatalf("parse asciidoc: %v", err)
	}
	if len(doc.Texts) != 2 {
		t.Fatalf("texts = %d, want 2: %+v", len(doc.Texts), doc.Texts)
	}
	if doc.Texts[0].Label != LabelParagraph || doc.Texts[0].Text != "这是第一段， 跨行继续。" {
		t.Fatalf("paragraph 1 = %+v", doc.Texts[0])
	}
	if doc.Texts[1].Label != LabelParagraph || doc.Texts[1].Text != "这是第二段。" {
		t.Fatalf("paragraph 2 = %+v", doc.Texts[1])
	}
}

// TestParseAsciiDoc_ImageWithCaption 验证 image:: 宏产出 PictureItem，前导
// 块标题优先作为 captions 引用；无 alt 的图片也保留占位。
func TestParseAsciiDoc_ImageWithCaption(t *testing.T) {
	src := `.整体架构
image::images/arch.png[整体架构图, width=600]
image::images/blank.png[]
`
	doc, err := ParseAsciiDoc([]byte(src))
	if err != nil {
		t.Fatalf("parse asciidoc: %v", err)
	}
	if len(doc.Pictures) != 2 {
		t.Fatalf("pictures = %d, want 2: %+v", len(doc.Pictures), doc.Pictures)
	}
	if doc.Pictures[0].Image == nil || doc.Pictures[0].Image.URI != "images/arch.png" || len(doc.Pictures[0].Captions) != 1 {
		t.Fatalf("first picture = %+v", doc.Pictures[0])
	}
	caption := doc.Texts[doc.Pictures[0].Captions[0].Idx]
	if caption.Label != LabelCaption || caption.Text != "整体架构" {
		t.Fatalf("caption = %+v", caption)
	}
	if doc.Pictures[1].Image == nil || doc.Pictures[1].Image.URI != "images/blank.png" || len(doc.Pictures[1].Captions) != 0 {
		t.Fatalf("blank picture = %+v", doc.Pictures[1])
	}
}

// TestParseAsciiDoc_EmptyInput 验证空输入与无有效内容输入返回空文档不报错。
func TestParseAsciiDoc_EmptyInput(t *testing.T) {
	cases := [][]byte{
		nil,
		[]byte(""),
		[]byte("   \n\n\t\n"),
		[]byte("|===\n|===\n"), // 空表格：不产出任何内容
		[]byte("\uFEFF\n"),
	}
	for i, data := range cases {
		doc, err := ParseAsciiDoc(data)
		if err != nil {
			t.Fatalf("case %d: unexpected error: %v", i, err)
		}
		if items := ToContentList(doc, SourceGolight); len(items) != 0 {
			t.Fatalf("case %d: expected empty result, got %d items", i, len(items))
		}
	}
}

// TestParseAsciiDoc_BomStripped 验证 UTF-8 BOM 被剥离：BOM 保留时
// "= 标题" 不再被识别为文档标题（对齐 docling 的 utf-8-sig 解码行为）。
func TestParseAsciiDoc_BomStripped(t *testing.T) {
	doc, err := ParseAsciiDoc([]byte("\uFEFF= 标题\n\n正文。\n"))
	if err != nil {
		t.Fatalf("parse asciidoc: %v", err)
	}
	items := ToContentList(doc, SourceGolight)
	if len(items) != 2 {
		t.Fatalf("items = %d, want 2: %+v", len(items), items)
	}
	if items[0].Text != "标题" || items[0].TextLevel != 1 {
		t.Fatalf("title after BOM = %+v", items[0])
	}
}

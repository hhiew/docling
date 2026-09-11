package docparse

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// newZipWriterForTest 创建测试用 zip 写入器。
func newZipWriterForTest(t *testing.T, buf *bytes.Buffer) *zip.Writer {
	t.Helper()
	return zip.NewWriter(buf)
}

// writeZipEntryForTest 向 zip 写入单个文件条目。
func writeZipEntryForTest(t *testing.T, zw *zip.Writer, name, content string) {
	t.Helper()
	f, err := zw.Create(name)
	if err != nil {
		t.Fatalf("create zip entry %s: %v", name, err)
	}
	if _, err := f.Write([]byte(content)); err != nil {
		t.Fatalf("write zip entry %s: %v", name, err)
	}
}

// closeZipWriterForTest 关闭 zip 写入器。
func closeZipWriterForTest(t *testing.T, zw *zip.Writer) {
	t.Helper()
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
}

// buildStructuredDocxXML 构造带标题样式与表格的 document.xml body。
// 声明覆盖 w/w14/m 命名空间，供编号复选框与公式用例复用。
func buildStructuredDocxXML(paragraphs string) string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"` +
		` xmlns:w14="http://schemas.microsoft.com/office/word/2010/wordml"` +
		` xmlns:m="http://schemas.openxmlformats.org/officeDocument/2006/math">` +
		`<w:body>` + paragraphs + `</w:body></w:document>`
}

// mustZipDocx 把 document.xml 内容打包为 docx（zip）字节流。
func mustZipDocx(t *testing.T, documentXML string) []byte {
	t.Helper()
	return mustZipDocxParts(t, documentXML, "", "")
}

// mustZipDocxParts 打包 document/styles/numbering 为 docx 字节流；
// styles 与 numbering 传空串时跳过对应条目。
func mustZipDocxParts(t *testing.T, documentXML, stylesXML, numberingXML string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := newZipWriterForTest(t, &buf)
	writeZipEntryForTest(t, zw, "word/document.xml", documentXML)
	if stylesXML != "" {
		writeZipEntryForTest(t, zw, "word/styles.xml", stylesXML)
	}
	if numberingXML != "" {
		writeZipEntryForTest(t, zw, "word/numbering.xml", numberingXML)
	}
	closeZipWriterForTest(t, zw)
	return buf.Bytes()
}

// wordStylesXMLRoot styles.xml 外壳，包裹样式定义片段。
func wordStylesXMLRoot(styleDefs string) string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<w:styles xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">` +
		styleDefs + `</w:styles>`
}

// wordNumberingXMLRoot numbering.xml 外壳，包裹编号定义片段。
func wordNumberingXMLRoot(numDefs string) string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<w:numbering xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">` +
		numDefs + `</w:numbering>`
}

// TestParseDocxFootnotesLinksBodyReference 验证 DOCX 脚注和尾注正文转为官方
// footnote TextItem，并从引用所在正文建立 footnotes 与 parent 双向关系。
func TestParseDocxFootnotesLinksBodyReference(t *testing.T) {
	documentXML := buildStructuredDocxXML(
		`<w:p><w:r><w:t>正文</w:t></w:r><w:r><w:footnoteReference w:id="1"/><w:endnoteReference w:id="2"/></w:r></w:p>`,
	)
	footnotesXML := `<?xml version="1.0"?><w:footnotes xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" xmlns:m="http://schemas.openxmlformats.org/officeDocument/2006/math">` +
		`<w:footnote w:id="-1"><w:p><w:r><w:t>分隔线</w:t></w:r></w:p></w:footnote>` +
		`<w:footnote w:id="1"><w:p><w:r><w:t>脚注内容 </w:t></w:r><m:oMath><m:sSup><m:e><m:r><m:t>x</m:t></m:r></m:e><m:sup><m:r><m:t>2</m:t></m:r></m:sup></m:sSup></m:oMath></w:p></w:footnote>` +
		`</w:footnotes>`
	endnotesXML := `<?xml version="1.0"?><w:endnotes xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">` +
		`<w:endnote w:id="2"><w:p><w:r><w:t>尾注内容</w:t></w:r></w:p></w:endnote>` +
		`</w:endnotes>`
	var buffer bytes.Buffer
	writer := newZipWriterForTest(t, &buffer)
	writeZipEntryForTest(t, writer, "word/document.xml", documentXML)
	writeZipEntryForTest(t, writer, "word/footnotes.xml", footnotesXML)
	writeZipEntryForTest(t, writer, "word/endnotes.xml", endnotesXML)
	closeZipWriterForTest(t, writer)

	doc, err := ParseDocx(buffer.Bytes())
	if err != nil {
		t.Fatalf("parse footnote docx: %v", err)
	}
	if len(doc.Texts) != 3 || doc.Texts[0].Text != "正文" || len(doc.Texts[0].Footnotes) != 2 {
		t.Fatalf("body footnote link=%+v", doc.Texts)
	}
	footnoteRef := doc.Texts[0].Footnotes[0]
	if footnoteRef.Idx != 1 || doc.Texts[1].Label != LabelFootnote ||
		doc.Texts[1].Text != `脚注内容 {x}^{2}` || doc.Texts[1].Parent == nil || *doc.Texts[1].Parent != (RefItem{Kind: refTexts, Idx: 0}) {
		t.Fatalf("footnote item=%+v", doc.Texts[1])
	}
	if doc.Texts[2].Label != LabelFootnote || doc.Texts[2].Text != "尾注内容" ||
		doc.Texts[2].Parent == nil || doc.Texts[0].Footnotes[1].Idx != 2 {
		t.Fatalf("endnote item=%+v", doc.Texts[2])
	}
	validateDocumentWithDoclingCore110ForTest(t, "docx-footnote", doc)
}

// TestParseDocxPreservesTrackedChangesAndFields 验证接受视图保留插入和域结果、
// 排除删除文本，同时把修订审计信息与域指令写入节点扩展元数据。
func TestParseDocxPreservesTrackedChangesAndFields(t *testing.T) {
	body := `<w:p>` +
		`<w:ins w:id="1" w:author="Alice" w:date="2026-09-09T08:00:00Z"><w:r><w:t>新内容</w:t></w:r></w:ins>` +
		`<w:del w:id="2" w:author="Bob" w:date="2026-09-08T08:00:00Z"><w:r><w:delText>旧内容</w:delText></w:r></w:del>` +
		`<w:fldSimple w:instr=" REF target "><w:r><w:t>第1章</w:t></w:r></w:fldSimple>` +
		`<w:r><w:fldChar w:fldCharType="begin"/></w:r><w:r><w:instrText> PAGE </w:instrText></w:r>` +
		`<w:r><w:fldChar w:fldCharType="separate"/></w:r><w:r><w:t>1</w:t></w:r><w:r><w:fldChar w:fldCharType="end"/></w:r>` +
		`</w:p>` +
		`<w:p><w:del w:id="3" w:author="Carol"><w:r><w:delText>整段删除</w:delText></w:r></w:del></w:p>` +
		`<w:tbl><w:tblGrid><w:gridCol/><w:gridCol/></w:tblGrid><w:tr>` +
		`<w:tc><w:p><w:ins w:id="4"><w:r><w:t>表格新</w:t></w:r></w:ins>` +
		`<w:del w:id="5"><w:r><w:delText>表格旧</w:delText></w:r></w:del>` +
		`<w:fldSimple w:instr=" DATE "><w:r><w:t>日期</w:t></w:r></w:fldSimple></w:p></w:tc>` +
		`<w:tc><w:p><w:r><w:t>值</w:t></w:r></w:p></w:tc>` +
		`</w:tr></w:tbl>`
	doc, err := ParseDocx(mustZipDocx(t, buildStructuredDocxXML(body)))
	if err != nil {
		t.Fatalf("parse revisions docx: %v", err)
	}
	if len(doc.Texts) != 3 || doc.Texts[0].Text != "新内容第1章1" || doc.Texts[1].Text != "整段删除" ||
		doc.Texts[1].ContentLayer != LayerNotes {
		t.Fatalf("revision visible view=%+v", doc.Texts)
	}
	var revisions []wordRevision
	if err := json.Unmarshal(doc.Texts[0].Meta[wordRevisionsMetaKey], &revisions); err != nil || len(revisions) != 2 ||
		revisions[0].Kind != "insert" || revisions[0].Text != "新内容" || revisions[1].Kind != "delete" || revisions[1].Text != "旧内容" {
		t.Fatalf("revision metadata=%+v err=%v", revisions, err)
	}
	var fields []string
	if err := json.Unmarshal(doc.Texts[0].Meta[wordFieldsMetaKey], &fields); err != nil || len(fields) != 2 ||
		fields[0] != "REF target" || fields[1] != "PAGE" {
		t.Fatalf("field metadata=%+v err=%v", fields, err)
	}
	if len(doc.Tables) != 1 || doc.Tables[0].Data.TableCells[0].Ref == nil || doc.Tables[0].Data.TableCells[0].Ref.Idx != 2 {
		t.Fatalf("tracked table cell ref=%+v", doc.Tables)
	}
	var tableRevisions []wordRevision
	if err := json.Unmarshal(doc.Texts[2].Meta[wordRevisionsMetaKey], &tableRevisions); err != nil || len(tableRevisions) != 2 ||
		tableRevisions[0].Text != "表格新" || tableRevisions[1].Text != "表格旧" || doc.Texts[2].Text != "表格新日期" {
		t.Fatalf("tracked table cell=%+v revisions=%+v err=%v", doc.Texts[2], tableRevisions, err)
	}
	validateDocumentWithDoclingCore110ForTest(t, "docx-revisions-fields", doc)
}

// TestParseKnowledgeFileStructuresDocx 验证 go_light 解析 docx 时：
// 1. Heading 样式段落产出 TextLevel，并维护 SectionPath 章节栈；
// 2. 表格独立产出 table 元素（Markdown 格式），不再混入正文文本流。
func TestParseKnowledgeFileStructuresDocx(t *testing.T) {
	body := `<w:p><w:pPr><w:pStyle w:val="Heading1"/></w:pPr><w:r><w:t>部署指南</w:t></w:r></w:p>` +
		`<w:p><w:r><w:t>准备 Docker 环境。</w:t></w:r></w:p>` +
		`<w:p><w:pPr><w:pStyle w:val="Heading2"/></w:pPr><w:r><w:t>环境要求</w:t></w:r></w:p>` +
		`<w:tbl><w:tr>` +
		`<w:tc><w:p><w:r><w:t>参数</w:t></w:r></w:p></w:tc>` +
		`<w:tc><w:p><w:r><w:t>说明</w:t></w:r></w:p></w:tc>` +
		`</w:tr><w:tr>` +
		`<w:tc><w:p><w:r><w:t>timeout</w:t></w:r></w:p></w:tc>` +
		`<w:tc><w:p><w:r><w:t>超时时间</w:t></w:r></w:p></w:tc>` +
		`</w:tr></w:tbl>` +
		`<w:p><w:r><w:t>满足以上条件后开始部署。</w:t></w:r></w:p>`
	data := mustZipDocx(t, buildStructuredDocxXML(body))

	doc, err := ParseDocx(data)
	if err != nil {
		t.Fatalf("parse docx: %v", err)
	}
	items := ToContentList(doc, SourceGolight)
	text := JoinItemTexts(items)
	if strings.TrimSpace(text) == "" {
		t.Fatal("aggregated text is empty")
	}

	var headings, paragraphs, tables int
	sawBodyInSection := false
	sawTableInSection := false
	for _, item := range items {
		switch {
		case item.TextLevel > 0:
			headings++
		case item.Type == ItemTypeTable:
			tables++
			if strings.Join(item.SectionPath, ">") == "部署指南>环境要求" &&
				strings.Contains(item.TableBody, "timeout") && strings.Contains(item.TableBody, "超时时间") {
				sawTableInSection = true
			}
		default:
			paragraphs++
			if strings.Contains(item.Text, "满足以上条件后开始部署") &&
				strings.Join(item.SectionPath, ">") == "部署指南>环境要求" {
				sawBodyInSection = true
			}
		}
	}
	if headings != 2 {
		t.Fatalf("expected 2 heading items, got %d: %+v", headings, items)
	}
	if tables != 1 {
		t.Fatalf("expected 1 table item, got %d: %+v", tables, items)
	}
	if paragraphs < 2 {
		t.Fatalf("expected at least 2 paragraph items, got %d: %+v", paragraphs, items)
	}
	if !sawTableInSection {
		t.Fatalf("table item missing or wrong section path: %+v", items)
	}
	if !sawBodyInSection {
		t.Fatalf("paragraph after table missing section path: %+v", items)
	}

	// 首个元素为一级标题
	if items[0].Text != "部署指南" || items[0].TextLevel != 1 {
		t.Fatalf("unexpected first item: %+v", items[0])
	}
	// 聚合文本仍包含全部内容
	for _, keyword := range []string{"部署指南", "准备 Docker 环境", "timeout", "满足以上条件后开始部署"} {
		if !strings.Contains(text, keyword) {
			t.Fatalf("aggregated text missing %q: %q", keyword, text)
		}
	}
}

// TestParseKnowledgeFileStructuresDocxTitleStyle 验证 Title 样式按一级标题处理。
func TestParseKnowledgeFileStructuresDocxTitleStyle(t *testing.T) {
	body := `<w:p><w:pPr><w:pStyle w:val="Title"/></w:pPr><w:r><w:t>平台手册</w:t></w:r></w:p>` +
		`<w:p><w:r><w:t>手册正文。</w:t></w:r></w:p>`
	data := mustZipDocx(t, buildStructuredDocxXML(body))

	doc, err := ParseDocx(data)
	if err != nil {
		t.Fatalf("parse docx: %v", err)
	}
	items := ToContentList(doc, SourceGolight)
	if len(items) < 2 {
		t.Fatalf("expected at least 2 items, got %+v", items)
	}
	if items[0].TextLevel != 1 || items[0].Text != "平台手册" {
		t.Fatalf("Title style not treated as level-1 heading: %+v", items[0])
	}
	if strings.Join(items[1].SectionPath, ">") != "平台手册" {
		t.Fatalf("body section path = %v, want [平台手册]", items[1].SectionPath)
	}
}

// TestParseKnowledgeFileDocxTextAggregationIsJSONSafe 验证段落文本中的
// 特殊字符不会破坏聚合结果的 JSON 序列化（入向量库 keywords 等场景）。
func TestParseKnowledgeFileDocxTextAggregationIsJSONSafe(t *testing.T) {
	body := `<w:p><w:r><w:t>配置示例 {\"key\": \"value\"}</w:t></w:r></w:p>`
	data := mustZipDocx(t, buildStructuredDocxXML(body))

	doc, err := ParseDocx(data)
	if err != nil {
		t.Fatalf("parse docx: %v", err)
	}
	text := doc.Text()
	if _, jsonErr := json.Marshal(text); jsonErr != nil {
		t.Fatalf("aggregated text not json marshalable: %v", jsonErr)
	}
}

// TestParseDocxHeadingOutlineLvlPreferred 验证标题层级判定：
// outlineLvl 优先于样式名中的 "Heading N"；非 heading 命名样式仅凭
// outlineLvl 也按标题处理（对齐 _get_label_and_level）。
func TestParseDocxHeadingOutlineLvlPreferred(t *testing.T) {
	body := `<w:p><w:pPr><w:pStyle w:val="CustomHead"/></w:pPr><w:r><w:t>第一部分</w:t></w:r></w:p>` +
		`<w:p><w:pPr><w:pStyle w:val="ChapterLook"/></w:pPr><w:r><w:t>背景说明</w:t></w:r></w:p>`
	stylesXML := wordStylesXMLRoot(
		`<w:style w:type="paragraph" w:styleId="CustomHead">` +
			`<w:name w:val="My Heading 5"/><w:pPr><w:outlineLvl w:val="1"/></w:pPr></w:style>` +
			`<w:style w:type="paragraph" w:styleId="ChapterLook">` +
			`<w:name w:val="Chapter Style"/><w:pPr><w:outlineLvl w:val="2"/></w:pPr></w:style>`)
	data := mustZipDocxParts(t, buildStructuredDocxXML(body), stylesXML, "")

	doc, err := ParseDocx(data)
	if err != nil {
		t.Fatalf("parse docx: %v", err)
	}
	if len(doc.Texts) != 2 {
		t.Fatalf("expected 2 texts, got %+v", doc.Texts)
	}
	// outlineLvl=1 → level 2，覆盖样式名中的 5
	if doc.Texts[0].Label != LabelSectionHeader || doc.Texts[0].TextLevel != 2 {
		t.Fatalf("outlineLvl should win over style name: %+v", doc.Texts[0])
	}
	// 样式名不含 heading 时 outlineLvl 单独生效：outlineLvl=2 → level 3
	if doc.Texts[1].Label != LabelSectionHeader || doc.Texts[1].TextLevel != 3 {
		t.Fatalf("outlineLvl-only style should be heading: %+v", doc.Texts[1])
	}
}

// TestParseDocxTitleStyleLabel 验证 Title 样式产出 label=title 且挂 body 顶层
// （对齐 msword_backend 的 Title 分支）。
func TestParseDocxTitleStyleLabel(t *testing.T) {
	body := `<w:p><w:pPr><w:pStyle w:val="Title"/></w:pPr><w:r><w:t>平台手册</w:t></w:r></w:p>`
	data := mustZipDocx(t, buildStructuredDocxXML(body))

	doc, err := ParseDocx(data)
	if err != nil {
		t.Fatalf("parse docx: %v", err)
	}
	if len(doc.Texts) != 1 {
		t.Fatalf("expected 1 text, got %+v", doc.Texts)
	}
	title := doc.Texts[0]
	if title.Label != LabelTitle || title.Text != "平台手册" {
		t.Fatalf("Title style should emit label=title: %+v", title)
	}
	if title.Parent == nil || title.Parent.Kind != refBody {
		t.Fatalf("title should hang on body: %+v", title.Parent)
	}
}

// TestParseDocxListNestingAndNumIdSwitch 验证列表结构：同 numId 的 ilvl 嵌套
// 子分组挂上一级列表项；numId 切换新建分组挂 body；有序 marker 组内自增、
// 无序（bullet）marker 为空（对齐 _manage_list_structure 简化语义）。
func TestParseDocxListNestingAndNumIdSwitch(t *testing.T) {
	body := `<w:p><w:pPr><w:numPr><w:ilvl w:val="0"/><w:numId w:val="1"/></w:numPr></w:pPr><w:r><w:t>第一项</w:t></w:r></w:p>` +
		`<w:p><w:pPr><w:numPr><w:ilvl w:val="1"/><w:numId w:val="1"/></w:numPr></w:pPr><w:r><w:t>子项</w:t></w:r></w:p>` +
		`<w:p><w:pPr><w:numPr><w:ilvl w:val="0"/><w:numId w:val="1"/></w:numPr></w:pPr><w:r><w:t>第二项</w:t></w:r></w:p>` +
		`<w:p><w:pPr><w:numPr><w:ilvl w:val="0"/><w:numId w:val="2"/></w:numPr></w:pPr><w:r><w:t>要点</w:t></w:r></w:p>`
	numberingXML := wordNumberingXMLRoot(
		`<w:abstractNum w:abstractNumId="10">` +
			`<w:lvl w:ilvl="0"><w:numFmt w:val="decimal"/></w:lvl>` +
			`<w:lvl w:ilvl="1"><w:numFmt w:val="decimal"/></w:lvl></w:abstractNum>` +
			`<w:abstractNum w:abstractNumId="11">` +
			`<w:lvl w:ilvl="0"><w:numFmt w:val="bullet"/></w:lvl></w:abstractNum>` +
			`<w:num w:numId="1"><w:abstractNumId w:val="10"/></w:num>` +
			`<w:num w:numId="2"><w:abstractNumId w:val="11"/></w:num>`)
	data := mustZipDocxParts(t, buildStructuredDocxXML(body), "", numberingXML)

	doc, err := ParseDocx(data)
	if err != nil {
		t.Fatalf("parse docx: %v", err)
	}
	if len(doc.Texts) != 4 {
		t.Fatalf("expected 4 list items, got %+v", doc.Texts)
	}
	if len(doc.Groups) != 3 {
		t.Fatalf("expected 3 list groups, got %+v", doc.Groups)
	}
	for i, g := range doc.Groups {
		if g.Label != GroupLabelList {
			t.Fatalf("group %d should be list group: %+v", i, g)
		}
	}
	assertRef := func(name string, got *RefItem, want RefItem) {
		t.Helper()
		if got == nil || *got != want {
			t.Fatalf("%s parent = %+v, want %s", name, got, want)
		}
	}
	groupRef := func(idx int) RefItem { return RefItem{Kind: refGroups, Idx: int64(idx)} }
	textRef := func(idx int) RefItem { return RefItem{Kind: refTexts, Idx: int64(idx)} }
	bodyRef := RefItem{Kind: refBody}
	// 第一项：组内计数 1，挂第一个列表组
	first := doc.Texts[0]
	if first.Marker != "1." || first.Enumerated == nil || !*first.Enumerated {
		t.Fatalf("first item should be ordered with marker 1.: %+v", first)
	}
	assertRef("first item", first.Parent, groupRef(0))
	// 子项：新建嵌套子分组，挂第一项下，计数独立从 1 开始
	child := doc.Texts[1]
	if child.Marker != "1." {
		t.Fatalf("nested item marker should restart at 1.: %+v", child)
	}
	assertRef("nested item", child.Parent, groupRef(1))
	assertRef("nested group", doc.Groups[1].Parent, textRef(0))
	// 第二项：回到第一组，计数延续为 2
	second := doc.Texts[2]
	if second.Marker != "2." {
		t.Fatalf("third item marker should continue to 2.: %+v", second)
	}
	assertRef("third item", second.Parent, groupRef(0))
	// numId 切换：新建无序分组挂 body，marker 为空
	bullet := doc.Texts[3]
	if bullet.Marker != "" || bullet.Enumerated == nil || *bullet.Enumerated {
		t.Fatalf("bullet item should be unordered: %+v", bullet)
	}
	assertRef("bullet item", bullet.Parent, groupRef(2))
	assertRef("bullet group", doc.Groups[2].Parent, bodyRef)
	assertRef("top group", doc.Groups[0].Parent, bodyRef)
	// body 顶层只挂两个列表分组（嵌套子分组挂在列表项下）
	if len(doc.Body.Children) != 2 {
		t.Fatalf("body should hold 2 list groups, got %+v", doc.Body.Children)
	}
}

// TestParseDocxTableSpansAndMerge 验证表格合并单元格：gridSpan 映射 ColSpan；
// vMerge restart 开启合并、无 val 的 vMerge（continue）扩展锚单元格的
// EndRowOffsetIdx 与 RowSpan（对齐 _handle_tables）。
func TestParseDocxTableSpansAndMerge(t *testing.T) {
	body := `<w:tbl>` +
		`<w:tblGrid><w:gridCol/><w:gridCol/><w:gridCol/></w:tblGrid>` +
		`<w:tr>` +
		`<w:tc><w:tcPr><w:gridSpan w:val="2"/></w:tcPr><w:p><w:r><w:t>跨度两列</w:t></w:r></w:p></w:tc>` +
		`<w:tc><w:p><w:r><w:t>B1</w:t></w:r></w:p></w:tc>` +
		`</w:tr><w:tr>` +
		`<w:tc><w:p><w:r><w:t>A2</w:t></w:r></w:p></w:tc>` +
		`<w:tc><w:tcPr><w:vMerge w:val="restart"/></w:tcPr><w:p><w:r><w:t>纵向合并</w:t></w:r></w:p></w:tc>` +
		`<w:tc><w:p><w:r><w:t>C2</w:t></w:r></w:p></w:tc>` +
		`</w:tr><w:tr>` +
		`<w:tc><w:p><w:r><w:t>A3</w:t></w:r></w:p></w:tc>` +
		`<w:tc><w:tcPr><w:vMerge/></w:tcPr><w:p><w:r><w:t>延续格</w:t></w:r></w:p></w:tc>` +
		`<w:tc><w:p><w:r><w:t>C3</w:t></w:r></w:p></w:tc>` +
		`</w:tr></w:tbl>`
	data := mustZipDocx(t, buildStructuredDocxXML(body))

	doc, err := ParseDocx(data)
	if err != nil {
		t.Fatalf("parse docx: %v", err)
	}
	if len(doc.Tables) != 1 {
		t.Fatalf("expected 1 table, got %+v", doc.Tables)
	}
	td := doc.Tables[0].Data
	if td.NumRows != 3 || td.NumCols != 3 {
		t.Fatalf("table grid = %dx%d, want 3x3", td.NumRows, td.NumCols)
	}
	findCell := func(text string) *DoclingTableCell {
		t.Helper()
		for i := range td.TableCells {
			if td.TableCells[i].Text == text {
				return &td.TableCells[i]
			}
		}
		t.Fatalf("cell %q not found in %+v", text, td.TableCells)
		return nil
	}
	// gridSpan=2：ColSpan=2，列偏移 [0,2)，首行 column_header
	spanCell := findCell("跨度两列")
	if spanCell.ColSpan != 2 || spanCell.StartColOffsetIdx != 0 || spanCell.EndColOffsetIdx != 2 ||
		spanCell.StartRowOffsetIdx != 0 || spanCell.EndRowOffsetIdx != 1 || !spanCell.ColumnHeader {
		t.Fatalf("gridSpan cell wrong: %+v", spanCell)
	}
	// vMerge restart 锚格被 continue 扩展：EndRowOffsetIdx=3、RowSpan=2
	mergeCell := findCell("纵向合并")
	if mergeCell.StartRowOffsetIdx != 1 || mergeCell.EndRowOffsetIdx != 3 ||
		mergeCell.RowSpan != 2 || mergeCell.StartColOffsetIdx != 1 || mergeCell.ColumnHeader {
		t.Fatalf("vMerge anchor cell wrong: %+v", mergeCell)
	}
	// continue 格不产出独立单元格
	for _, c := range td.TableCells {
		if c.Text == "延续格" {
			t.Fatalf("vMerge continue cell should not emit its own cell: %+v", td.TableCells)
		}
	}
	// 普通格按网格位置铺设，末行不再标记 column_header
	a3 := findCell("A3")
	if a3.StartRowOffsetIdx != 2 || a3.StartColOffsetIdx != 0 || a3.ColSpan != 1 || a3.ColumnHeader {
		t.Fatalf("plain cell wrong: %+v", a3)
	}
}

// TestParseDocxTableGridBefore 验证 w:gridBefore 行首列偏移：单元格从偏移列
// 开始铺设（对齐 _handle_tables 的 grid_cols_before 处理）。
func TestParseDocxTableGridBefore(t *testing.T) {
	body := `<w:tbl>` +
		`<w:tblGrid><w:gridCol/><w:gridCol/></w:tblGrid>` +
		`<w:tr>` +
		`<w:tc><w:p><w:r><w:t>H1</w:t></w:r></w:p></w:tc>` +
		`<w:tc><w:p><w:r><w:t>H2</w:t></w:r></w:p></w:tc>` +
		`</w:tr><w:tr>` +
		`<w:trPr><w:gridBefore w:val="1"/></w:trPr>` +
		`<w:tc><w:p><w:r><w:t>缩进格</w:t></w:r></w:p></w:tc>` +
		`</w:tr></w:tbl>`
	data := mustZipDocx(t, buildStructuredDocxXML(body))

	doc, err := ParseDocx(data)
	if err != nil {
		t.Fatalf("parse docx: %v", err)
	}
	if len(doc.Tables) != 1 {
		t.Fatalf("expected 1 table, got %+v", doc.Tables)
	}
	td := doc.Tables[0].Data
	var indented *DoclingTableCell
	for i := range td.TableCells {
		if td.TableCells[i].Text == "缩进格" {
			indented = &td.TableCells[i]
		}
	}
	if indented == nil {
		t.Fatalf("indented cell not found: %+v", td.TableCells)
	}
	if indented.StartRowOffsetIdx != 1 || indented.StartColOffsetIdx != 1 || indented.EndColOffsetIdx != 2 {
		t.Fatalf("gridBefore offset not applied: %+v", indented)
	}
}

// TestParseDocxSingleCellTableDegrades 验证 1x1 表降级为版式容器：
// 单元格内容按普通正文处理，不产出 table 元素。
func TestParseDocxSingleCellTableDegrades(t *testing.T) {
	body := `<w:tbl><w:tblGrid><w:gridCol/></w:tblGrid>` +
		`<w:tr><w:tc>` +
		`<w:p><w:r><w:t>盒子内文一</w:t></w:r></w:p>` +
		`<w:p><w:r><w:t>盒子内文二</w:t></w:r></w:p>` +
		`</w:tc></w:tr></w:tbl>` +
		`<w:p><w:r><w:t>盒子外段落</w:t></w:r></w:p>`
	data := mustZipDocx(t, buildStructuredDocxXML(body))

	doc, err := ParseDocx(data)
	if err != nil {
		t.Fatalf("parse docx: %v", err)
	}
	if len(doc.Tables) != 0 {
		t.Fatalf("1x1 table should degrade, got tables: %+v", doc.Tables)
	}
	if len(doc.Texts) != 3 {
		t.Fatalf("expected 3 texts from degraded cell, got %+v", doc.Texts)
	}
	for _, want := range []string{"盒子内文一", "盒子内文二", "盒子外段落"} {
		found := false
		for _, txt := range doc.Texts {
			if txt.Text == want {
				found = true
			}
		}
		if !found {
			t.Fatalf("text %q missing: %+v", want, doc.Texts)
		}
	}
}

// TestParseDocxCodeMergeAndBreak 验证 Code 样式段落：连续段合并为同一 code
// 元素（\n 连接，空段缓冲一个换行）；普通段落打断合并链。
func TestParseDocxCodeMergeAndBreak(t *testing.T) {
	body := `<w:p><w:pPr><w:pStyle w:val="Code"/></w:pPr><w:r><w:t>fmt.Println("a")</w:t></w:r></w:p>` +
		`<w:p><w:pPr><w:pStyle w:val="Code"/></w:pPr></w:p>` +
		`<w:p><w:pPr><w:pStyle w:val="Code"/></w:pPr><w:r><w:t>fmt.Println("b")</w:t></w:r></w:p>` +
		`<w:p><w:r><w:t>中间插入的说明</w:t></w:r></w:p>` +
		`<w:p><w:pPr><w:pStyle w:val="Code"/></w:pPr><w:r><w:t>fmt.Println("c")</w:t></w:r></w:p>`
	stylesXML := wordStylesXMLRoot(
		`<w:style w:type="paragraph" w:styleId="Code"><w:name w:val="Code"/></w:style>`)
	data := mustZipDocxParts(t, buildStructuredDocxXML(body), stylesXML, "")

	doc, err := ParseDocx(data)
	if err != nil {
		t.Fatalf("parse docx: %v", err)
	}
	if len(doc.Texts) != 3 {
		t.Fatalf("expected 3 texts, got %+v", doc.Texts)
	}
	if doc.Texts[0].Label != LabelCode {
		t.Fatalf("first text should be code: %+v", doc.Texts[0])
	}
	// 连续 code 合并，空 code 段缓冲为换行
	want := "fmt.Println(\"a\")\n\nfmt.Println(\"b\")"
	if doc.Texts[0].Text != want {
		t.Fatalf("merged code = %q, want %q", doc.Texts[0].Text, want)
	}
	if doc.Texts[1].Label != LabelText || doc.Texts[1].Text != "中间插入的说明" {
		t.Fatalf("plain paragraph should break code chain: %+v", doc.Texts[1])
	}
	if doc.Texts[2].Label != LabelCode || doc.Texts[2].Text != "fmt.Println(\"c\")" {
		t.Fatalf("code after break should start new block: %+v", doc.Texts[2])
	}
}

// TestParseDocxCheckbox 验证 w14:checkbox 段落：按 w14:checked 判定选中态
// label，并清除文本中的 ☐ 前缀符号。
func TestParseDocxCheckbox(t *testing.T) {
	body := `<w:p><w:r><w14:checkbox><w14:checked w14:val="1"/></w14:checkbox><w:t>已完成事项</w:t></w:r></w:p>` +
		`<w:p><w:r><w14:checkbox><w14:checked w14:val="0"/></w14:checkbox><w:t>☐待办事项</w:t></w:r></w:p>`
	data := mustZipDocx(t, buildStructuredDocxXML(body))

	doc, err := ParseDocx(data)
	if err != nil {
		t.Fatalf("parse docx: %v", err)
	}
	if len(doc.Texts) != 2 {
		t.Fatalf("expected 2 checkbox texts, got %+v", doc.Texts)
	}
	if doc.Texts[0].Label != LabelCheckboxSelected || doc.Texts[0].Text != "已完成事项" {
		t.Fatalf("checked box wrong: %+v", doc.Texts[0])
	}
	if doc.Texts[1].Label != LabelCheckboxUnselected {
		t.Fatalf("unchecked box label wrong: %+v", doc.Texts[1])
	}
	if strings.Contains(doc.Texts[1].Text, "☐") || doc.Texts[1].Text != "待办事项" {
		t.Fatalf("checkbox symbol should be stripped: %+v", doc.Texts[1])
	}
}

// TestParseDocxFormulaStandaloneAndInline 验证 m:oMath：独立公式段落产出
// formula 元素（text 取 oMath 内文本），行内公式文本并入段落文本。
func TestParseDocxFormulaStandaloneAndInline(t *testing.T) {
	body := `<w:p><m:oMath><m:t>E=mc^2</m:t></m:oMath></w:p>` +
		`<w:p><w:r><w:t>速度公式 </w:t></w:r><m:oMath><m:t>v=d/t</m:t></m:oMath></w:p>`
	data := mustZipDocx(t, buildStructuredDocxXML(body))

	doc, err := ParseDocx(data)
	if err != nil {
		t.Fatalf("parse docx: %v", err)
	}
	if len(doc.Texts) != 2 {
		t.Fatalf("expected 2 texts, got %+v", doc.Texts)
	}
	if doc.Texts[0].Label != LabelFormula || doc.Texts[0].Text != "E=mc^2" {
		t.Fatalf("standalone formula wrong: %+v", doc.Texts[0])
	}
	if doc.Texts[1].Label != LabelText || doc.Texts[1].Text != "速度公式 v=d/t" {
		t.Fatalf("inline formula should merge into paragraph text: %+v", doc.Texts[1])
	}
}

// TestParseDocxOMMLToLaTeX 验证常见 OMML 分式、根式、上下标和大型运算符
// 转换为可检索、可渲染的 LaTeX，而不是简单拼接 m:t 字符。
func TestParseDocxOMMLToLaTeX(t *testing.T) {
	body := `<w:p><m:oMath><m:f><m:num><m:r><m:t>a</m:t></m:r></m:num><m:den><m:r><m:t>b</m:t></m:r></m:den></m:f></m:oMath></w:p>` +
		`<w:p><m:oMath><m:rad><m:deg><m:r><m:t>3</m:t></m:r></m:deg><m:e><m:sSup><m:e><m:r><m:t>x</m:t></m:r></m:e><m:sup><m:r><m:t>2</m:t></m:r></m:sup></m:sSup></m:e></m:rad></m:oMath></w:p>` +
		`<w:p><m:oMath><m:nary><m:naryPr><m:chr m:val="∑"/></m:naryPr><m:sub><m:r><m:t>i=1</m:t></m:r></m:sub><m:sup><m:r><m:t>n</m:t></m:r></m:sup><m:e><m:r><m:t>x_i</m:t></m:r></m:e></m:nary></m:oMath></w:p>`
	doc, err := ParseDocx(mustZipDocx(t, buildStructuredDocxXML(body)))
	if err != nil {
		t.Fatalf("parse docx: %v", err)
	}
	want := []string{`\frac{a}{b}`, `\sqrt[3]{{x}^{2}}`, `\sum_{i=1}^{n}{x\_i}`}
	if len(doc.Texts) != len(want) {
		t.Fatalf("texts=%+v", doc.Texts)
	}
	for i := range want {
		if doc.Texts[i].Label != LabelFormula || doc.Texts[i].Text != want[i] {
			t.Fatalf("texts[%d]=%+v want=%q", i, doc.Texts[i], want[i])
		}
	}
}

// TestConvertOMMLMatrixDelimiterAndUnknownFallback 验证矩阵、定界符和未知
// 包装标签均保持可用 LaTeX 或可见内容。
func TestConvertOMMLMatrixDelimiterAndUnknownFallback(t *testing.T) {
	raw := []byte(`<m:oMath xmlns:m="http://schemas.openxmlformats.org/officeDocument/2006/math"><m:d><m:dPr><m:begChr m:val="["/><m:endChr m:val="]"/></m:dPr><m:e><m:m><m:mr><m:e><m:r><m:t>a</m:t></m:r></m:e><m:e><m:r><m:t>b</m:t></m:r></m:e></m:mr><m:mr><m:e><m:r><m:t>c</m:t></m:r></m:e><m:e><m:r><m:t>d</m:t></m:r></m:e></m:mr></m:m></m:e></m:d><m:futureNode><m:e><m:r><m:t>z</m:t></m:r></m:e></m:futureNode></m:oMath>`)
	want := `\left[\begin{matrix}a & b \\ c & d\end{matrix}\right]z`
	if got := convertOMMLToLaTeX(raw); got != want {
		t.Fatalf("latex=%q want=%q", got, want)
	}
}

// TestParseDocxSdtContentWalked 验证 w:sdt 内容控件递归：sdtContent 内段落
// 照常解析（对齐 _walk_linear 的 sdt 分支）。
func TestParseDocxSdtContentWalked(t *testing.T) {
	body := `<w:sdt><w:sdtPr><w:id w:val="1"/></w:sdtPr>` +
		`<w:sdtContent><w:p><w:r><w:t>目录条目一</w:t></w:r></w:p></w:sdtContent></w:sdt>` +
		`<w:p><w:r><w:t>正文段落</w:t></w:r></w:p>`
	data := mustZipDocx(t, buildStructuredDocxXML(body))

	doc, err := ParseDocx(data)
	if err != nil {
		t.Fatalf("parse docx: %v", err)
	}
	if len(doc.Texts) != 2 {
		t.Fatalf("expected 2 texts, got %+v", doc.Texts)
	}
	if doc.Texts[0].Text != "目录条目一" || doc.Texts[1].Text != "正文段落" {
		t.Fatalf("sdt content not walked in order: %+v", doc.Texts)
	}
}

// TestParseDocxEmptyBodyReturnsEmptyDoc 验证空 body 返回元素为空的文档
// （不视为错误），以及畸形 zip 返回 error。
func TestParseDocxEmptyBodyReturnsEmptyDoc(t *testing.T) {
	data := mustZipDocx(t, buildStructuredDocxXML(""))
	doc, err := ParseDocx(data)
	if err != nil {
		t.Fatalf("empty body should not be an error: %v", err)
	}
	if len(doc.Texts) != 0 || len(doc.Tables) != 0 || len(doc.Groups) != 0 {
		t.Fatalf("expected empty document, got texts=%d tables=%d groups=%d",
			len(doc.Texts), len(doc.Tables), len(doc.Groups))
	}
	if _, err := ParseDocx([]byte("not a zip")); err == nil {
		t.Fatal("malformed zip should return error")
	}
}

// TestParseDocxUnstyledHeadingFallback 验证无样式标题 fallback（超出 Docling
// msword 后端的增强）：全文档无 heading/title 样式时，手工加粗短段改标
// section_header（TextLevel=1）并线性挂接（后续标题挂前一标题下）；冒号结尾
// 加粗段、加粗列表项、显式 w:b val="0" 段与仅 w:bCs 段不误判；正文文本不变。
func TestParseDocxUnstyledHeadingFallback(t *testing.T) {
	body := `<w:p><w:r><w:rPr><w:b/></w:rPr><w:t>第一章 总则</w:t></w:r></w:p>` +
		`<w:p><w:r><w:t>本规范适用于平台全部产品线的研发与运维团队，请各团队仔细阅读并严格遵照执行，如有疑问请联系平台组。</w:t></w:r></w:p>` +
		`<w:p><w:r><w:rPr><w:b/></w:rPr><w:t>适用范围：</w:t></w:r></w:p>` +
		`<w:p><w:r><w:rPr><w:b w:val="0"/></w:rPr><w:t>显式关闭加粗的短句</w:t></w:r></w:p>` +
		`<w:p><w:r><w:rPr><w:bCs/></w:rPr><w:t>仅复杂文种加粗的短句</w:t></w:r></w:p>` +
		`<w:p><w:pPr><w:numPr><w:ilvl w:val="0"/><w:numId w:val="1"/></w:numPr></w:pPr><w:r><w:rPr><w:b/></w:rPr><w:t>部署要点</w:t></w:r></w:p>` +
		`<w:p><w:r><w:rPr><w:b/></w:rPr><w:t>第二章 附则</w:t></w:r></w:p>` +
		`<w:p><w:r><w:t>本规范自发布之日起开始施行，由平台组负责解释与维护。</w:t></w:r></w:p>`
	numberingXML := wordNumberingXMLRoot(
		`<w:abstractNum w:abstractNumId="10"><w:lvl w:ilvl="0"><w:numFmt w:val="decimal"/></w:lvl></w:abstractNum>` +
			`<w:num w:numId="1"><w:abstractNumId w:val="10"/></w:num>`)
	data := mustZipDocxParts(t, buildStructuredDocxXML(body), "", numberingXML)

	doc, err := ParseDocx(data)
	if err != nil {
		t.Fatalf("parse docx: %v", err)
	}
	findTextIdx := func(text string) int64 {
		t.Helper()
		for i := range doc.Texts {
			if doc.Texts[i].Text == text {
				return int64(i)
			}
		}
		t.Fatalf("text %q not found: %+v", text, doc.Texts)
		return -1
	}
	// fallback 生效：加粗短段 → section_header + TextLevel=1
	first := findTextIdx("第一章 总则")
	if doc.Texts[first].Label != LabelSectionHeader || doc.Texts[first].TextLevel != 1 {
		t.Fatalf("bold short paragraph should become level-1 heading: %+v", doc.Texts[first])
	}
	second := findTextIdx("第二章 附则")
	if doc.Texts[second].Label != LabelSectionHeader || doc.Texts[second].TextLevel != 1 {
		t.Fatalf("second bold heading should be level-1: %+v", doc.Texts[second])
	}
	// 线性挂接：相邻 fallback 标题平级挂前一标题下，body 顶层仅剩首个标题
	if doc.Texts[second].Parent == nil || doc.Texts[second].Parent.Kind != refTexts ||
		doc.Texts[second].Parent.Idx != first {
		t.Fatalf("second heading should hang under the first: %+v", doc.Texts[second].Parent)
	}
	if len(doc.Body.Children) != 1 || doc.Body.Children[0].Idx != first {
		t.Fatalf("body should hold only the first heading: %+v", doc.Body.Children)
	}
	// 不误判：冒号结尾加粗段、显式关闭加粗段、仅 bCs 段保持普通文本
	for _, text := range []string{"适用范围：", "显式关闭加粗的短句", "仅复杂文种加粗的短句"} {
		item := doc.Texts[findTextIdx(text)]
		if item.Label != LabelText || item.TextLevel != 0 {
			t.Fatalf("%q should stay plain text: %+v", text, item)
		}
	}
	// 加粗列表项保持 list_item
	deploy := doc.Texts[findTextIdx("部署要点")]
	if deploy.Label != LabelListItem {
		t.Fatalf("bold list item should stay list_item: %+v", deploy)
	}
	// 正文文本内容不变
	for _, body1 := range []string{
		"本规范适用于平台全部产品线的研发与运维团队，请各团队仔细阅读并严格遵照执行，如有疑问请联系平台组。",
		"本规范自发布之日起开始施行，由平台组负责解释与维护。",
	} {
		if item := doc.Texts[findTextIdx(body1)]; item.Orig != body1 || item.Text != body1 {
			t.Fatalf("body text changed: %+v", item)
		}
	}
	// 简化器视角：标题 TextLevel=1，正文携带标题章节路径
	items := ToContentList(doc, SourceGolight)
	var sawHeading, sawBody bool
	for _, item := range items {
		if item.Text == "第一章 总则" {
			if item.TextLevel != 1 {
				t.Fatalf("simplified heading level = %d, want 1: %+v", item.TextLevel, item)
			}
			sawHeading = true
		}
		if strings.HasPrefix(item.Text, "本规范适用于") {
			if strings.Join(item.SectionPath, ">") != "第一章 总则" {
				t.Fatalf("body section path = %v, want [第一章 总则]: %+v", item.SectionPath, item)
			}
			sawBody = true
		}
	}
	if !sawHeading || !sawBody {
		t.Fatalf("missing heading/body in content list: %+v", items)
	}
}

// TestParseDocxStyledHeadingSkipsFallback 验证含标准 Heading 样式的文档不触发
// fallback（行为与现在一致，不双跑）：手工加粗短段保持普通文本，挂样式标题下。
func TestParseDocxStyledHeadingSkipsFallback(t *testing.T) {
	body := `<w:p><w:pPr><w:pStyle w:val="Heading1"/></w:pPr><w:r><w:t>第一章 总则</w:t></w:r></w:p>` +
		`<w:p><w:r><w:t>本规范适用于平台全部产品线。</w:t></w:r></w:p>` +
		`<w:p><w:r><w:rPr><w:b/></w:rPr><w:t>第二章 附则</w:t></w:r></w:p>`
	data := mustZipDocx(t, buildStructuredDocxXML(body))

	doc, err := ParseDocx(data)
	if err != nil {
		t.Fatalf("parse docx: %v", err)
	}
	if len(doc.Texts) != 3 {
		t.Fatalf("expected 3 texts, got %+v", doc.Texts)
	}
	heading := doc.Texts[0]
	if heading.Label != LabelSectionHeader || heading.TextLevel != 1 || heading.Text != "第一章 总则" {
		t.Fatalf("Heading1 style should still work: %+v", heading)
	}
	// fallback 未触发：加粗短段保持 text，不双跑改标
	bold := doc.Texts[2]
	if bold.Label != LabelText || bold.TextLevel != 0 {
		t.Fatalf("bold paragraph should stay plain when styled headings exist: %+v", bold)
	}
	// 挂接维持样式树口径：加粗段挂 Heading1 下
	if bold.Parent == nil || bold.Parent.Kind != refTexts || bold.Parent.Idx != 0 {
		t.Fatalf("bold paragraph should hang under Heading1: %+v", bold.Parent)
	}
}

// TestParseDocxFontSizeFallback 验证字号路径的 fallback：无加粗但 w:sz 显著
// 大于正文众数（半磅差 ≥2 即 +1pt）的短段判标题；仅大 1 半磅（0.5pt）与等大
// 段落不判（等大文档字号条件退化为仅加粗）。
func TestParseDocxFontSizeFallback(t *testing.T) {
	body := `<w:p><w:r><w:rPr><w:sz w:val="32"/></w:rPr><w:t>安装准备</w:t></w:r></w:p>` +
		`<w:p><w:r><w:rPr><w:sz w:val="21"/></w:rPr><w:t>开始安装前请检查系统版本与磁盘剩余空间，并关闭杀毒软件以避免安装中断。</w:t></w:r></w:p>` +
		`<w:p><w:r><w:rPr><w:sz w:val="21"/></w:rPr><w:t>安装完成后请重启系统使配置生效，并按章节顺序逐项核对初始化结果。</w:t></w:r></w:p>` +
		`<w:p><w:r><w:rPr><w:sz w:val="22"/></w:rPr><w:t>只大半磅不升级</w:t></w:r></w:p>` +
		`<w:p><w:r><w:rPr><w:sz w:val="21"/></w:rPr><w:t>与正文等大的普通结尾段</w:t></w:r></w:p>`
	data := mustZipDocx(t, buildStructuredDocxXML(body))

	doc, err := ParseDocx(data)
	if err != nil {
		t.Fatalf("parse docx: %v", err)
	}
	findTextIdx := func(text string) int64 {
		t.Helper()
		for i := range doc.Texts {
			if doc.Texts[i].Text == text {
				return int64(i)
			}
		}
		t.Fatalf("text %q not found: %+v", text, doc.Texts)
		return -1
	}
	// 16pt（sz=32）短段显著大于正文众数 10.5pt（sz=21，差 11 半磅）→ 标题
	big := findTextIdx("安装准备")
	if doc.Texts[big].Label != LabelSectionHeader || doc.Texts[big].TextLevel != 1 {
		t.Fatalf("oversized short paragraph should become level-1 heading: %+v", doc.Texts[big])
	}
	// 仅大 1 半磅（sz=22，差 1 < 2）与等大段（sz=21）不判标题
	for _, text := range []string{"只大半磅不升级", "与正文等大的普通结尾段"} {
		item := doc.Texts[findTextIdx(text)]
		if item.Label != LabelText || item.TextLevel != 0 {
			t.Fatalf("%q should stay plain text: %+v", text, item)
		}
	}
	// 字号标题同样线性重挂：首个 fallback 标题挂 body
	if len(doc.Body.Children) != 1 || doc.Body.Children[0].Idx != big {
		t.Fatalf("body should hold only the oversized heading: %+v", doc.Body.Children)
	}
}

// TestParseDocxHeaderFooterFurniture 验证页眉页脚关系部件被提取到 furniture
// 根节点，并分别使用 page_header/page_footer 标签与 furniture 内容层。
func TestParseDocxHeaderFooterFurniture(t *testing.T) {
	documentXML := `<?xml version="1.0" encoding="UTF-8"?>` +
		`<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" ` +
		`xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">` +
		`<w:body><w:p><w:r><w:t>正文</w:t></w:r></w:p><w:sectPr>` +
		`<w:headerReference w:type="default" r:id="rIdHeader"/>` +
		`<w:footerReference w:type="default" r:id="rIdFooter"/>` +
		`</w:sectPr></w:body></w:document>`
	relsXML := `<?xml version="1.0" encoding="UTF-8"?>` +
		`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
		`<Relationship Id="rIdHeader" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/header" Target="header1.xml"/>` +
		`<Relationship Id="rIdFooter" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/footer" Target="footer1.xml"/>` +
		`</Relationships>`
	data := mustZipEntriesForTest(t, map[string]string{
		"word/document.xml":            documentXML,
		"word/_rels/document.xml.rels": relsXML,
		"word/header1.xml": `<w:hdr xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">` +
			`<w:p><w:r><w:t>平台页眉</w:t></w:r></w:p></w:hdr>`,
		"word/footer1.xml": `<w:ftr xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">` +
			`<w:p><w:r><w:t>第 1 页</w:t></w:r></w:p></w:ftr>`,
	})

	doc, err := ParseDocx(data)
	if err != nil {
		t.Fatalf("parse docx: %v", err)
	}
	if len(doc.Texts) != 3 || len(doc.Furniture.Children) != 2 {
		t.Fatalf("unexpected texts/furniture: texts=%+v furniture=%+v", doc.Texts, doc.Furniture.Children)
	}
	for _, tc := range []struct {
		idx   int
		label DocItemLabel
		text  string
	}{{1, LabelPageHeader, "平台页眉"}, {2, LabelPageFooter, "第 1 页"}} {
		item := doc.Texts[tc.idx]
		if item.Label != tc.label || item.Text != tc.text || item.ContentLayer != LayerFurniture ||
			item.Parent == nil || item.Parent.Kind != refFurniture {
			t.Fatalf("furniture item wrong: %+v", item)
		}
	}
}

// TestParseDocxCommentsAsNotes 验证 comments.xml 批注建立 comment_section 分组，
// 批注正文使用 notes 内容层，并由被批注正文的 comments 细粒度引用关联。
func TestParseDocxCommentsAsNotes(t *testing.T) {
	documentXML := `<?xml version="1.0" encoding="UTF-8"?>` +
		`<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">` +
		`<w:body><w:p><w:commentRangeStart w:id="7"/><w:r><w:t>需要确认</w:t></w:r>` +
		`<w:commentRangeEnd w:id="7"/><w:r><w:commentReference w:id="7"/></w:r></w:p>` +
		`</w:body></w:document>`
	commentsXML := `<?xml version="1.0" encoding="UTF-8"?>` +
		`<w:comments xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">` +
		`<w:comment w:id="7" w:author="审核员"><w:p><w:r><w:t>请补充依据</w:t></w:r></w:p></w:comment>` +
		`</w:comments>`
	data := mustZipEntriesForTest(t, map[string]string{
		"word/document.xml": documentXML,
		"word/comments.xml": commentsXML,
	})

	doc, err := ParseDocx(data)
	if err != nil {
		t.Fatalf("parse docx: %v", err)
	}
	if len(doc.Groups) != 1 || doc.Groups[0].Label != GroupLabelCommentSection ||
		doc.Groups[0].ContentLayer != LayerNotes {
		t.Fatalf("comment group wrong: %+v", doc.Groups)
	}
	if len(doc.Texts) != 2 || doc.Texts[1].Text != "请补充依据" || doc.Texts[1].ContentLayer != LayerNotes {
		t.Fatalf("comment note wrong: %+v", doc.Texts)
	}
	if len(doc.Texts[0].Comments) != 1 || doc.Texts[0].Comments[0].Kind != refGroups ||
		doc.Texts[0].Comments[0].Idx != 0 {
		t.Fatalf("body comment reference missing: %+v", doc.Texts[0])
	}
	wantRange := [2]int64{0, 4}
	if doc.Texts[0].Comments[0].Range == nil || *doc.Texts[0].Comments[0].Range != wantRange {
		t.Fatalf("comment range = %+v, want %+v", doc.Texts[0].Comments[0].Range, wantRange)
	}
}

// TestParseDocxTextBox 验证 DrawingML/VML 文本框中的 txbxContent 作为独立正文
// 输出，且不会重复并入锚点段落文本。
func TestParseDocxTextBox(t *testing.T) {
	body := `<w:p><w:r><w:t>锚点前文</w:t></w:r>` +
		`<w:r><w:drawing><wps:wsp><wps:txbx><w:txbxContent>` +
		`<w:p><w:r><w:t>文本框内容</w:t></w:r></w:p>` +
		`</w:txbxContent></wps:txbx></wps:wsp></w:drawing></w:r></w:p>` +
		`<w:p><w:r><w:t>后续正文</w:t></w:r></w:p>`
	documentXML := `<?xml version="1.0" encoding="UTF-8"?>` +
		`<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" ` +
		`xmlns:wps="http://schemas.microsoft.com/office/word/2010/wordprocessingShape">` +
		`<w:body>` + body + `</w:body></w:document>`

	doc, err := ParseDocx(mustZipDocx(t, documentXML))
	if err != nil {
		t.Fatalf("parse docx: %v", err)
	}
	if len(doc.Texts) != 3 {
		t.Fatalf("expected anchor, textbox and following text, got %+v", doc.Texts)
	}
	for i, want := range []string{"锚点前文", "文本框内容", "后续正文"} {
		if doc.Texts[i].Text != want {
			t.Fatalf("text order[%d] = %q, want %q; all=%+v", i, doc.Texts[i].Text, want, doc.Texts)
		}
	}
}

// TestParseDocxInlineFormattingAndHyperlink 验证 run 样式统一汇总到 Formatting，
// 外部超链接关系写入 Hyperlink，并保留上下标脚本信息。
func TestParseDocxInlineFormattingAndHyperlink(t *testing.T) {
	documentXML := `<?xml version="1.0" encoding="UTF-8"?>` +
		`<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" ` +
		`xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">` +
		`<w:body><w:p><w:hyperlink r:id="rIdLink"><w:r><w:rPr><w:b/><w:i/><w:u w:val="single"/>` +
		`<w:strike/><w:vertAlign w:val="superscript"/></w:rPr><w:t>官方文档</w:t></w:r></w:hyperlink>` +
		`</w:p></w:body></w:document>`
	relsXML := `<?xml version="1.0" encoding="UTF-8"?>` +
		`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
		`<Relationship Id="rIdLink" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/hyperlink" ` +
		`Target="https://example.com/doc" TargetMode="External"/></Relationships>`
	data := mustZipEntriesForTest(t, map[string]string{
		"word/document.xml":            documentXML,
		"word/_rels/document.xml.rels": relsXML,
	})

	doc, err := ParseDocx(data)
	if err != nil {
		t.Fatalf("parse docx: %v", err)
	}
	item := doc.Texts[0]
	if item.Hyperlink != "https://example.com/doc" || item.Formatting == nil ||
		!item.Formatting.Bold || !item.Formatting.Italic || !item.Formatting.Underline ||
		!item.Formatting.Strikethrough || item.Formatting.Script != ScriptSuper {
		t.Fatalf("inline metadata wrong: %+v", item)
	}
}

// TestParseDocxPictureAnchorOrderAndMetadata 验证图片按文档锚点顺序进入 body，
// 并补齐 Docling ImageRef 要求的 MIME、DPI 与像素尺寸。
func TestParseDocxPictureAnchorOrderAndMetadata(t *testing.T) {
	body := `<w:p><w:r><w:t>图前</w:t></w:r></w:p>` +
		`<w:p><w:drawing><a:blip r:embed="rId1"/></w:drawing></w:p>` +
		`<w:p><w:r><w:t>图间</w:t></w:r></w:p>` +
		`<w:p><w:drawing><a:blip r:embed="rId2"/></w:drawing></w:p>`
	relsXML := `<?xml version="1.0" encoding="UTF-8"?>` +
		`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
		`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="media/image1.png"/>` +
		`<Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="media/image2.png"/>` +
		`</Relationships>`
	data := mustZipEntriesForTest(t, map[string]string{
		"word/document.xml":            docxImageDocumentXMLRoot(body),
		"word/_rels/document.xml.rels": relsXML,
		"word/media/image1.png":        string(minimalPNG),
		"word/media/image2.png":        string(minimalPNG),
	})

	doc, err := ParseDocx(data)
	if err != nil {
		t.Fatalf("parse docx: %v", err)
	}
	if len(doc.Body.Children) != 4 {
		t.Fatalf("body order missing: %+v", doc.Body.Children)
	}
	wantKinds := []docRefKind{refTexts, refPictures, refTexts, refPictures}
	for i, want := range wantKinds {
		if doc.Body.Children[i].Kind != want {
			t.Fatalf("body child[%d] kind=%s want=%s: %+v", i, doc.Body.Children[i].Kind, want, doc.Body.Children)
		}
	}
	for _, picture := range doc.Pictures {
		if picture.Image == nil || picture.Image.Mimetype != "image/png" || picture.Image.Dpi != 72 ||
			picture.Image.Size == nil || picture.Image.Size.Width != 1 || picture.Image.Size.Height != 1 {
			t.Fatalf("picture metadata incomplete: %+v", picture.Image)
		}
	}
}

// TestParseDocxRichTableCell 验证富表格单元格通过 ref 关联带格式与链接的
// TextItem，网格本身仍保留完整文本与跨度语义。
func TestParseDocxRichTableCell(t *testing.T) {
	body := `<w:tbl><w:tblGrid><w:gridCol/></w:tblGrid>` +
		`<w:tr><w:tc><w:p><w:r><w:t>名称</w:t></w:r></w:p></w:tc></w:tr>` +
		`<w:tr><w:tc><w:p><w:hyperlink r:id="rIdLink"><w:r><w:rPr><w:b/></w:rPr>` +
		`<w:t>设备 A</w:t></w:r></w:hyperlink></w:p></w:tc></w:tr></w:tbl>`
	documentXML := `<?xml version="1.0" encoding="UTF-8"?>` +
		`<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" ` +
		`xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><w:body>` + body + `</w:body></w:document>`
	relsXML := `<?xml version="1.0" encoding="UTF-8"?>` +
		`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
		`<Relationship Id="rIdLink" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/hyperlink" ` +
		`Target="https://example.com/device-a" TargetMode="External"/></Relationships>`
	data := mustZipEntriesForTest(t, map[string]string{
		"word/document.xml":            documentXML,
		"word/_rels/document.xml.rels": relsXML,
	})

	doc, err := ParseDocx(data)
	if err != nil {
		t.Fatalf("parse docx: %v", err)
	}
	cell := doc.Tables[0].Data.TableCells[1]
	if cell.Text != "设备 A" || cell.Ref == nil || cell.Ref.Kind != refTexts {
		t.Fatalf("rich table cell ref missing: %+v", cell)
	}
	text := doc.Texts[cell.Ref.Idx]
	if text.Hyperlink != "https://example.com/device-a" || text.Formatting == nil || !text.Formatting.Bold ||
		text.Parent == nil || text.Parent.Kind != refTables {
		t.Fatalf("rich table cell metadata wrong: %+v", text)
	}
}

// TestParseDocxChartReference 验证 c:chart 关系委托共享 OOXML 图表解析器，
// 输出 label=picture 且携带 classification 与 tabular_chart 数据。
func TestParseDocxChartReference(t *testing.T) {
	documentXML := `<?xml version="1.0" encoding="UTF-8"?>` +
		`<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" ` +
		`xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" ` +
		`xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart">` +
		`<w:body><w:p><w:r><w:drawing><c:chart r:id="rIdChart"/></w:drawing></w:r></w:p>` +
		`</w:body></w:document>`
	relsXML := `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
		`<Relationship Id="rIdChart" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/chart" ` +
		`Target="charts/chart1.xml"/></Relationships>`
	chartXML := `<c:chartSpace xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart">` +
		`<c:chart><c:title><c:tx><c:rich><a:p xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">` +
		`<a:r><a:t>季度销量</a:t></a:r></a:p></c:rich></c:tx></c:title><c:plotArea><c:barChart><c:ser>` +
		`<c:tx><c:strRef><c:f>Sheet1!$B$1</c:f></c:strRef></c:tx>` +
		`<c:cat><c:strRef><c:f>Sheet1!$A$2:$A$3</c:f></c:strRef></c:cat>` +
		`<c:val><c:numRef><c:f>Sheet1!$B$2:$B$3</c:f></c:numRef></c:val>` +
		`</c:ser></c:barChart></c:plotArea></c:chart></c:chartSpace>`
	chartRelsXML := `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
		`<Relationship Id="rIdWorkbook" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/package" ` +
		`Target="../embeddings/Microsoft_Excel_Worksheet1.xlsx"/></Relationships>`
	data := mustZipEntriesForTest(t, map[string]string{
		"word/document.xml":                               documentXML,
		"word/_rels/document.xml.rels":                    relsXML,
		"word/charts/chart1.xml":                          chartXML,
		"word/charts/_rels/chart1.xml.rels":               chartRelsXML,
		"word/embeddings/Microsoft_Excel_Worksheet1.xlsx": string(buildOOXMLChartWorkbookFixture(t)),
	})

	doc, err := ParseDocx(data)
	if err != nil {
		t.Fatalf("parse docx: %v", err)
	}
	if len(doc.Pictures) != 1 || doc.Pictures[0].Meta == nil || doc.Pictures[0].Meta.Classification == nil ||
		doc.Pictures[0].Meta.TabularChart == nil || doc.Pictures[0].Meta.TabularChart.ChartData == nil {
		t.Fatalf("chart picture metadata missing: %+v", doc.Pictures)
	}
	if doc.Pictures[0].Meta.Classification.Predictions[0].ClassName != "bar_chart" ||
		doc.Pictures[0].Meta.TabularChart.Title != "季度销量" ||
		doc.Pictures[0].Meta.TabularChart.ChartData.NumRows != 3 || doc.Pictures[0].Image == nil {
		t.Fatalf("chart metadata wrong: %+v", doc.Pictures[0].Meta)
	}
	assertChartTable(t, doc.Pictures[0].Meta.TabularChart.ChartData, [][]string{
		{"类别", "销量"}, {"1月", "12"}, {"2月", "18"},
	})
	validateDocumentWithDoclingCore110ForTest(t, "docx-chart-formula", doc)
}

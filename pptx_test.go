package docparse

import (
	"archive/zip"
	"bytes"
	"fmt"
	"strings"
	"testing"
)

// 测试 fixture 的 XML 部件模板：覆盖 title 占位符、普通文本形状、
// 带层级列表的文本形状、含 gridSpan 的 2x2 表格、空 slide 与备注页。

// pptxTestXMLNS 测试部件共用的命名空间声明。
const pptxTestXMLNS = `xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" ` +
	`xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" ` +
	`xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"`

// buildTestPPTX 用 zip.Writer 手写最小 pptx fixture：
//   - slide1：title 占位符 + 两个视觉同行（y 差 5420 EMU < 容差）的文本形状
//     （XML 顺序 B 在 A 前，验证视觉排序；B 含层级列表与有序列表）+ 2x2 表格（gridSpan）；
//   - slide2：空页；
//   - notesSlide1：body 占位符备注文本。
func buildTestPPTX(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	write := func(name, content string) {
		t.Helper()
		f, err := w.Create(name)
		if err != nil {
			t.Fatalf("zip create %s: %v", name, err)
		}
		if _, err := f.Write([]byte(content)); err != nil {
			t.Fatalf("zip write %s: %v", name, err)
		}
	}

	write("[Content_Types].xml", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
<Default Extension="xml" ContentType="application/xml"/>
<Override PartName="/ppt/presentation.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.presentation.main+xml"/>
<Override PartName="/ppt/slides/slide1.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slide+xml"/>
<Override PartName="/ppt/slides/slide2.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slide+xml"/>
<Override PartName="/ppt/notesSlides/notesSlide1.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.notesSlide+xml"/>
</Types>`)

	write("_rels/.rels", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="ppt/presentation.xml"/>
</Relationships>`)

	write("ppt/presentation.xml", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<p:presentation `+pptxTestXMLNS+`>
<p:sldMasterIdLst><p:sldMasterId id="2147483648" r:id="rId100"/></p:sldMasterIdLst>
<p:sldIdLst><p:sldId id="256" r:id="rId1"/><p:sldId id="257" r:id="rId2"/></p:sldIdLst>
<p:sldSz cx="9144000" cy="6858000"/>
</p:presentation>`)

	// slideMaster 关系不应被误认成 slide 关系（Type 后缀精确匹配）
	write("ppt/_rels/presentation.xml.rels", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
<Relationship Id="rId100" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideMaster" Target="slideMasters/slideMaster1.xml"/>
<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide1.xml"/>
<Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide2.xml"/>
</Relationships>`)

	write("ppt/slides/slide1.xml", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<p:sld `+pptxTestXMLNS+`>
<p:cSld><p:spTree>
<p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr>
<p:grpSpPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="0" cy="0"/></a:xfrm></p:grpSpPr>
<p:sp><p:nvSpPr><p:cNvPr id="2" name="Title 1"/><p:cNvSpPr/><p:nvPr><p:ph type="title"/></p:nvPr></p:nvSpPr>
<p:spPr><a:xfrm><a:off x="838200" y="1825625"/><a:ext cx="7467600" cy="1325563"/></a:xfrm></p:spPr>
<p:txBody><a:bodyPr/><a:p><a:r><a:t>演示文稿标题</a:t></a:r></a:p></p:txBody></p:sp>
<p:sp><p:nvSpPr><p:cNvPr id="3" name="TextBox 2"/><p:cNvSpPr txBox="1"/><p:nvPr/></p:nvSpPr>
<p:spPr><a:xfrm><a:off x="4000000" y="3505420"/><a:ext cx="3000000" cy="1500000"/></a:xfrm></p:spPr>
<p:txBody><a:bodyPr/>
<a:p><a:pPr lvl="0"><a:buChar char="•"/></a:pPr><a:r><a:t>要点一</a:t></a:r></a:p>
<a:p><a:pPr lvl="1"><a:buChar char="•"/></a:pPr><a:r><a:t>子要点</a:t></a:r></a:p>
<a:p><a:pPr lvl="0"><a:buChar char="•"/></a:pPr><a:r><a:t>要点二</a:t></a:r></a:p>
<a:p><a:pPr><a:buNone/></a:pPr><a:r><a:t>普通段落文本</a:t></a:r></a:p>
<a:p><a:pPr><a:buAutoNum type="arabicPeriod"/></a:pPr><a:r><a:t>有序一</a:t></a:r></a:p>
<a:p><a:pPr><a:buAutoNum type="arabicPeriod"/></a:pPr><a:r><a:t>有序二</a:t></a:r></a:p>
</p:txBody></p:sp>
<p:sp><p:nvSpPr><p:cNvPr id="4" name="TextBox 3"/><p:cNvSpPr txBox="1"/><p:nvPr/></p:nvSpPr>
<p:spPr><a:xfrm><a:off x="838200" y="3500000"/><a:ext cx="3000000" cy="1000000"/></a:xfrm></p:spPr>
<p:txBody><a:bodyPr/><a:p><a:r><a:t>左侧文本</a:t></a:r></a:p></p:txBody></p:sp>
<p:graphicFrame><p:nvGraphicFramePr><p:cNvPr id="5" name="Table 4"/><p:cNvGraphicFramePr/><p:nvPr/></p:nvGraphicFramePr>
<p:xfrm><a:off x="838200" y="5000000"/><a:ext cx="7467600" cy="1000000"/></p:xfrm>
<a:graphic><a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/table">
<a:tbl><a:tblPr firstRow="1"/><a:tblGrid><a:gridCol w="3733800"/><a:gridCol w="3733800"/></a:tblGrid>
<a:tr h="500000">
<a:tc><a:txBody><a:bodyPr/><a:p><a:r><a:t>表头A</a:t></a:r></a:p></a:txBody><a:tcPr/></a:tc>
<a:tc><a:txBody><a:bodyPr/><a:p><a:r><a:t></a:t></a:r></a:p></a:txBody><a:tcPr/></a:tc>
</a:tr>
<a:tr h="500000">
<a:tc gridSpan="2"><a:txBody><a:bodyPr/><a:p><a:r><a:t>跨列内容</a:t></a:r></a:p></a:txBody><a:tcPr/></a:tc>
</a:tr>
</a:tbl>
</a:graphicData></a:graphic></p:graphicFrame>
</p:spTree></p:cSld></p:sld>`)

	// slideLayout 关系存在但文件缺失，验证后缀匹配不误判且缺失文件被跳过
	write("ppt/slides/_rels/slide1.xml.rels", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideLayout" Target="../slideLayouts/slideLayout1.xml"/>
<Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/notesSlide" Target="../notesSlides/notesSlide1.xml"/>
</Relationships>`)

	// 空页：仅 spTree 骨架，无形状
	write("ppt/slides/slide2.xml", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<p:sld `+pptxTestXMLNS+`>
<p:cSld><p:spTree>
<p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr>
<p:grpSpPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="0" cy="0"/></a:xfrm></p:grpSpPr>
</p:spTree></p:cSld></p:sld>`)

	write("ppt/notesSlides/notesSlide1.xml", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<p:notes `+pptxTestXMLNS+`>
<p:cSld><p:spTree>
<p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr>
<p:grpSpPr/>
<p:sp><p:nvSpPr><p:cNvPr id="2" name="Notes Placeholder 2"/><p:cNvSpPr/><p:nvPr><p:ph type="body" idx="1"/></p:nvPr></p:nvSpPr>
<p:spPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="0" cy="0"/></a:xfrm></p:spPr>
<p:txBody><a:bodyPr/><a:p><a:r><a:t>这是备注内容</a:t></a:r></a:p></p:txBody></p:sp>
</p:spTree></p:cSld></p:notes>`)

	if err := w.Close(); err != nil {
		t.Fatalf("zip close: %v", err)
	}
	return buf.Bytes()
}

// buildAdvancedTestPPTX 构造覆盖布局/母版回退、批注与组合变换的最小 PPTX。
func buildAdvancedTestPPTX(t *testing.T) []byte {
	t.Helper()
	parts := map[string]string{
		"ppt/presentation.xml":            `<p:presentation ` + pptxTestXMLNS + `><p:sldIdLst><p:sldId id="256" r:id="rId1"/></p:sldIdLst><p:sldSz cx="1000" cy="800"/></p:presentation>`,
		"ppt/_rels/presentation.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide1.xml"/></Relationships>`,
		"ppt/slides/slide1.xml": `<p:sld ` + pptxTestXMLNS + `><p:cSld><p:spTree>
<p:sp><p:nvSpPr><p:cNvPr id="2" name="Body"/><p:cNvSpPr/><p:nvPr><p:ph idx="5"/></p:nvPr></p:nvSpPr><p:spPr/><p:txBody><a:bodyPr/><a:p><a:r><a:t>继承的项目</a:t></a:r></a:p></p:txBody></p:sp>
<p:sp><p:nvSpPr><p:cNvPr id="3" name="Outside"/><p:cNvSpPr/><p:nvPr/></p:nvSpPr><p:spPr><a:xfrm><a:off x="220" y="300"/><a:ext cx="80" cy="30"/></a:xfrm></p:spPr><p:txBody><a:bodyPr/><a:p><a:r><a:t>外部形状</a:t></a:r></a:p></p:txBody></p:sp>
<p:grpSp><p:nvGrpSpPr/><p:grpSpPr><a:xfrm><a:off x="300" y="200"/><a:ext cx="400" cy="200"/><a:chOff x="0" y="0"/><a:chExt cx="200" cy="100"/></a:xfrm></p:grpSpPr>
<p:sp><p:nvSpPr><p:cNvPr id="4" name="Right"/><p:cNvSpPr/><p:nvPr/></p:nvSpPr><p:spPr><a:xfrm><a:off x="100" y="50"/><a:ext cx="40" cy="20"/></a:xfrm></p:spPr><p:txBody><a:bodyPr/><a:p><a:r><a:t>组合右</a:t></a:r></a:p></p:txBody></p:sp>
<p:sp><p:nvSpPr><p:cNvPr id="5" name="Left"/><p:cNvSpPr/><p:nvPr/></p:nvSpPr><p:spPr><a:xfrm><a:off x="0" y="50"/><a:ext cx="40" cy="20"/></a:xfrm></p:spPr><p:txBody><a:bodyPr/><a:p><a:r><a:t>组合左</a:t></a:r></a:p></p:txBody></p:sp>
</p:grpSp>
<p:graphicFrame><p:nvGraphicFramePr/><p:xfrm><a:off x="10" y="700"/><a:ext cx="100" cy="50"/></p:xfrm><a:graphic><a:graphicData><c:chart xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart" r:id="rIdChart"/></a:graphicData></a:graphic></p:graphicFrame>
</p:spTree></p:cSld></p:sld>`,
		"ppt/slides/_rels/slide1.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
<Relationship Id="rIdLayout" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideLayout" Target="../slideLayouts/slideLayout1.xml"/>
<Relationship Id="rIdComments" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/comments" Target="../comments/comment1.xml"/>
<Relationship Id="rIdChart" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/chart" Target="../charts/chart1.xml"/>
</Relationships>`,
		"ppt/slideLayouts/slideLayout1.xml":            `<p:sldLayout ` + pptxTestXMLNS + `><p:cSld><p:spTree><p:sp><p:nvSpPr><p:cNvPr id="2" name="Body"/><p:cNvSpPr/><p:nvPr><p:ph type="body" idx="5"/></p:nvPr></p:nvSpPr><p:spPr><a:xfrm><a:off x="20" y="100"/><a:ext cx="180" cy="80"/></a:xfrm></p:spPr><p:txBody><a:bodyPr/><a:p/></p:txBody></p:sp></p:spTree></p:cSld></p:sldLayout>`,
		"ppt/slideLayouts/_rels/slideLayout1.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rIdMaster" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideMaster" Target="../slideMasters/slideMaster1.xml"/></Relationships>`,
		"ppt/slideMasters/slideMaster1.xml":            `<p:sldMaster ` + pptxTestXMLNS + `><p:txStyles><p:bodyStyle><a:lvl1pPr><a:buChar char="•"/></a:lvl1pPr></p:bodyStyle></p:txStyles></p:sldMaster>`,
		"ppt/comments/comment1.xml":                    `<p:cmLst ` + pptxTestXMLNS + `><p:cm authorId="0" idx="1"><p:pos x="230" y="310"/><p:text>请核对此处</p:text></p:cm></p:cmLst>`,
		"ppt/charts/chart1.xml": `<c:chartSpace xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart"><c:chart><c:plotArea><c:barChart><c:ser>` +
			`<c:tx><c:strRef><c:f>Sheet1!$B$1</c:f></c:strRef></c:tx>` +
			`<c:cat><c:strRef><c:f>Sheet1!$A$2:$A$3</c:f></c:strRef></c:cat>` +
			`<c:val><c:numRef><c:f>Sheet1!$B$2:$B$3</c:f></c:numRef></c:val>` +
			`</c:ser></c:barChart></c:plotArea></c:chart></c:chartSpace>`,
		"ppt/charts/_rels/chart1.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
			`<Relationship Id="rIdWorkbook" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/package" ` +
			`Target="../embeddings/Microsoft_Excel_Worksheet1.xlsx"/></Relationships>`,
		"ppt/embeddings/Microsoft_Excel_Worksheet1.xlsx": string(buildOOXMLChartWorkbookFixture(t)),
	}
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for name, content := range parts {
		f, err := w.Create(name)
		if err != nil {
			t.Fatalf("zip create %s: %v", name, err)
		}
		if _, err := fmt.Fprint(f, content); err != nil {
			t.Fatalf("zip write %s: %v", name, err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("zip close: %v", err)
	}
	return buf.Bytes()
}

// TestParsePPTX_LayoutMasterFallback 验证占位符类型和几何从布局回退，
// 项目符号样式再从母版 bodyStyle 回退。
func TestParsePPTX_LayoutMasterFallback(t *testing.T) {
	doc, err := ParsePPTX(buildAdvancedTestPPTX(t))
	if err != nil {
		t.Fatalf("parse advanced pptx: %v", err)
	}
	var inherited *TextItem
	for i := range doc.Texts {
		if doc.Texts[i].Text == "继承的项目" {
			inherited = &doc.Texts[i]
			break
		}
	}
	if inherited == nil || inherited.Label != LabelListItem {
		t.Fatalf("master bullet fallback missing: %+v", inherited)
	}
	if len(inherited.Prov) != 1 || inherited.Prov[0].BBox == nil ||
		inherited.Prov[0].BBox.L != 20 || inherited.Prov[0].BBox.T != 100 {
		t.Fatalf("layout geometry fallback missing: %+v", inherited)
	}
}

// TestParsePPTX_NotesAndCommentsLayer 验证演讲者备注与幻灯片批注均进入 notes 层，
// 且批注引用挂到最接近批注坐标的正文元素。
func TestParsePPTX_NotesAndCommentsLayer(t *testing.T) {
	doc, err := ParsePPTX(buildAdvancedTestPPTX(t))
	if err != nil {
		t.Fatalf("parse advanced pptx: %v", err)
	}
	var comment *TextItem
	var outside *TextItem
	for i := range doc.Texts {
		switch doc.Texts[i].Text {
		case "请核对此处":
			comment = &doc.Texts[i]
		case "外部形状":
			outside = &doc.Texts[i]
		}
	}
	if comment == nil || comment.ContentLayer != LayerNotes {
		t.Fatalf("comment layer = %+v, want notes", comment)
	}
	if outside == nil || len(outside.Comments) != 1 || outside.Comments[0].RefItem.String() != comment.SelfRef {
		t.Fatalf("comment target link missing: outside=%+v comment=%+v", outside, comment)
	}

	base := parseTestPPTX(t)
	for i := range base.Texts {
		if base.Texts[i].Text == "这是备注内容" && base.Texts[i].ContentLayer != LayerNotes {
			t.Fatalf("speaker notes layer = %s, want notes", base.Texts[i].ContentLayer)
		}
	}
}

// TestParsePPTX_GroupTransformOrder 验证组合内子形状应用组合坐标变换后，
// 与外部形状共同按绝对视觉位置排序，且组合内仍按左到右排列。
func TestParsePPTX_GroupTransformOrder(t *testing.T) {
	doc, err := ParsePPTX(buildAdvancedTestPPTX(t))
	if err != nil {
		t.Fatalf("parse advanced pptx: %v", err)
	}
	items := ToContentList(doc, SourceGolight)
	outside := findItemText(items, "外部形状")
	left := findItemText(items, "组合左")
	right := findItemText(items, "组合右")
	if outside < 0 || left < 0 || right < 0 || outside >= left || left >= right {
		t.Fatalf("group visual order = outside:%d left:%d right:%d, items=%+v", outside, left, right, items)
	}
	var leftItem *TextItem
	for i := range doc.Texts {
		if doc.Texts[i].Text == "组合左" {
			leftItem = &doc.Texts[i]
		}
	}
	if leftItem == nil || leftItem.Prov[0].BBox.L != 300 || leftItem.Prov[0].BBox.T != 300 {
		t.Fatalf("group transform not applied: %+v", leftItem)
	}
}

// TestParsePPTX_ChartReferenceHook 验证图表 frame 保留关系 ID，供共享图表解析器接入。
func TestParsePPTX_ChartReferenceHook(t *testing.T) {
	data := buildAdvancedTestPPTX(t)
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("open pptx: %v", err)
	}
	slide, ok := readPptxZipFile(reader, "ppt/slides/slide1.xml")
	if !ok {
		t.Fatal("slide missing")
	}
	shapes := parsePptxSlideShapes(slide)
	found := false
	for _, shape := range shapes {
		if shape.kind == pptxShapeFrame && shape.chartRID == "rIdChart" {
			found = true
		}
	}
	if !found {
		t.Fatalf("chart relationship hook missing: %+v", shapes)
	}
	doc, err := ParsePPTX(data)
	if err != nil {
		t.Fatalf("parse chart pptx: %v", err)
	}
	found = false
	for i := range doc.Pictures {
		meta := doc.Pictures[i].Meta
		if meta != nil && meta.TabularChart != nil && meta.Classification != nil &&
			len(meta.Classification.Predictions) == 1 && meta.Classification.Predictions[0].ClassName == "bar_chart" {
			assertChartTable(t, meta.TabularChart.ChartData, [][]string{
				{"类别", "销量"}, {"1月", "12"}, {"2月", "18"},
			})
			if doc.Pictures[i].Image == nil {
				t.Fatalf("chart SVG preview missing: %+v", doc.Pictures[i])
			}
			found = true
		}
	}
	if !found {
		t.Fatalf("chart helper integration missing: %+v", doc.Pictures)
	}
	validateDocumentWithDoclingCore110ForTest(t, "pptx-chart-formula", doc)
}

// parseTestPPTX 解析 fixture 并断言成功。
func parseTestPPTX(t *testing.T) *DoclingDocument {
	t.Helper()
	doc, err := ParsePPTX(buildTestPPTX(t))
	if err != nil {
		t.Fatalf("parse pptx: %v", err)
	}
	return doc
}

// findItemText 按文本查找 content_list 元素下标。
func findItemText(items []Item, text string) int {
	for i := range items {
		if items[i].Text == text {
			return i
		}
	}
	return -1
}

// TestParsePPTX_DocumentStructure 验证 slide 分组、页面尺寸（EMU）、
// title 占位符识别与普通段落 label、层级列表组织与有序 marker 自增、备注文本。
func TestParsePPTX_DocumentStructure(t *testing.T) {
	doc := parseTestPPTX(t)

	// 每页登记 sldSz 的 EMU 尺寸，页号字符串化为键
	if len(doc.Pages) != 2 {
		t.Fatalf("expected 2 pages, got %d: %+v", len(doc.Pages), doc.Pages)
	}
	page1 := doc.Pages["1"]
	if page1.Size == nil || page1.Size.Width != 9144000 || page1.Size.Height != 6858000 {
		t.Fatalf("page 1 size = %+v, want EMU 9144000x6858000", page1.Size)
	}
	if _, ok := doc.Pages["2"]; !ok {
		t.Fatalf("empty slide 2 should still register a page: %+v", doc.Pages)
	}

	// 每页一个 slide-N 分组（slideInd 0 起，对齐源码 f"slide-{slide_ind}"）；
	// Groups 中还混有列表分组，只统计 section 类
	var slideGroups []GroupItem
	for _, g := range doc.Groups {
		if g.Label == GroupLabelSection {
			slideGroups = append(slideGroups, g)
		}
	}
	if len(slideGroups) != 2 || slideGroups[0].Name != "slide-0" || slideGroups[1].Name != "slide-1" {
		t.Fatalf("slide groups = %+v", doc.Groups)
	}

	// title 占位符识别
	var titleItem *TextItem
	for i := range doc.Texts {
		if doc.Texts[i].Label == LabelTitle {
			titleItem = &doc.Texts[i]
			break
		}
	}
	if titleItem == nil || titleItem.Text != "演示文稿标题" {
		t.Fatalf("title placeholder not recognized: %+v", doc.Texts)
	}

	// 普通文本形状逐段落产出 label=paragraph
	for _, want := range []string{"左侧文本", "普通段落文本"} {
		found := false
		for i := range doc.Texts {
			if doc.Texts[i].Label == LabelParagraph && doc.Texts[i].Text == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("paragraph %q missing or mislabeled: %+v", want, doc.Texts)
		}
	}

	// 列表项顺序与 marker：无序（同组共享+lvl 嵌套+中断重建）+有序自增
	listTexts := []string{"要点一", "子要点", "要点二", "有序一", "有序二"}
	pos := 0
	for _, want := range listTexts {
		found := false
		for ; pos < len(doc.Texts); pos++ {
			if doc.Texts[pos].Label == LabelListItem && doc.Texts[pos].Text == want {
				found = true
				pos++
				break
			}
		}
		if !found {
			t.Fatalf("list item %q missing or out of order: %+v", want, doc.Texts)
		}
	}
	markers := map[string]string{"有序一": "1.", "有序二": "2."}
	for i := range doc.Texts {
		item := &doc.Texts[i]
		if item.Label != LabelListItem {
			continue
		}
		if want, ok := markers[item.Text]; ok {
			if item.Marker != want || item.Enumerated == nil || !*item.Enumerated {
				t.Fatalf("ordered item %q marker=%q enumerated=%v, want %q/true",
					item.Text, item.Marker, item.Enumerated, want)
			}
		} else if item.Marker != "" || (item.Enumerated != nil && *item.Enumerated) {
			t.Fatalf("unordered item %q marker=%q enumerated=%v, want empty/false",
				item.Text, item.Marker, item.Enumerated)
		}
	}

	// lvl=1 段落的子列表分组挂在上一列表项（"要点一"）下
	var yaoDian *TextItem
	for i := range doc.Texts {
		if doc.Texts[i].Text == "要点一" && doc.Texts[i].Label == LabelListItem {
			yaoDian = &doc.Texts[i]
			break
		}
	}
	if yaoDian == nil || len(yaoDian.Children) != 1 || yaoDian.Children[0].Kind != refGroups {
		t.Fatalf("nested list group should attach to first list item: %+v", yaoDian)
	}
	subGroup := doc.Groups[yaoDian.Children[0].Idx]
	if len(subGroup.Children) != 1 || doc.Texts[subGroup.Children[0].Idx].Text != "子要点" {
		t.Fatalf("nested group children = %+v, want [子要点]", subGroup.Children)
	}

	// 备注文本产出（body 占位符）
	foundNotes := false
	for i := range doc.Texts {
		if doc.Texts[i].Text == "这是备注内容" && doc.Texts[i].Label == LabelText {
			foundNotes = true
			break
		}
	}
	if !foundNotes {
		t.Fatalf("notes text missing: %+v", doc.Texts)
	}
}

// TestParsePPTX_Table 验证表格：gridSpan → ColSpan/EndColOffsetIdx、
// 空文本 cell 丢弃、首行 column_header、num_rows/num_cols 按全尺寸声明。
func TestParsePPTX_Table(t *testing.T) {
	doc := parseTestPPTX(t)
	if len(doc.Tables) != 1 {
		t.Fatalf("expected 1 table, got %d", len(doc.Tables))
	}
	data := doc.Tables[0].Data
	if data.NumRows != 2 || data.NumCols != 2 {
		t.Fatalf("table size = %dx%d, want 2x2", data.NumRows, data.NumCols)
	}
	// 空文本 cell（行 0 列 1）被丢弃，只产出 2 个 cell
	if len(data.TableCells) != 2 {
		t.Fatalf("expected 2 cells (empty cell dropped), got %+v", data.TableCells)
	}
	byText := map[string]*DoclingTableCell{}
	for i := range data.TableCells {
		byText[data.TableCells[i].Text] = &data.TableCells[i]
	}
	header := byText["表头A"]
	if header == nil || !header.ColumnHeader || header.StartColOffsetIdx != 0 ||
		header.EndColOffsetIdx != 1 || header.StartRowOffsetIdx != 0 || header.EndRowOffsetIdx != 1 {
		t.Fatalf("header cell wrong: %+v", header)
	}
	merged := byText["跨列内容"]
	if merged == nil || merged.ColumnHeader {
		t.Fatalf("merged cell wrong or marked as header: %+v", merged)
	}
	if merged.ColSpan != 2 || merged.StartColOffsetIdx != 0 || merged.EndColOffsetIdx != 2 {
		t.Fatalf("gridSpan mapping wrong: %+v", merged)
	}
	if merged.StartRowOffsetIdx != 1 || merged.EndRowOffsetIdx != 2 || merged.RowSpan != 1 {
		t.Fatalf("merged cell row offsets wrong: %+v", merged)
	}
}

// TestParsePPTX_Provenance 验证 prov：bbox 为 EMU 原始值且 CoordOrigin=BOTTOMLEFT、
// page_no 为 slide 序号、charspan=[0,len]；无几何形状回退整页。
func TestParsePPTX_Provenance(t *testing.T) {
	doc := parseTestPPTX(t)

	var titleItem *TextItem
	for i := range doc.Texts {
		if doc.Texts[i].Label == LabelTitle {
			titleItem = &doc.Texts[i]
			break
		}
	}
	if titleItem == nil || len(titleItem.Prov) != 1 {
		t.Fatalf("title prov missing: %+v", titleItem)
	}
	prov := titleItem.Prov[0]
	if prov.PageNo != 1 {
		t.Fatalf("title page_no = %d, want 1", prov.PageNo)
	}
	// bbox = [l, t, l+w, t+h]，EMU 原始值，BOTTOMLEFT 坐标原点
	bbox := prov.BBox
	if bbox == nil || bbox.CoordOrigin != CoordOriginBottomLeft {
		t.Fatalf("title bbox origin wrong: %+v", bbox)
	}
	if bbox.L != 838200 || bbox.T != 1825625 || bbox.R != 8305800 || bbox.B != 3151188 {
		t.Fatalf("title bbox = %+v, want EMU [838200,1825625,8305800,3151188]", bbox)
	}
	if prov.CharSpan != [2]int64{0, 6} {
		t.Fatalf("title charspan = %v, want [0 6]", prov.CharSpan)
	}

	// 表格 charspan=[0,0]、页号为 slide 序号
	if len(doc.Tables) != 1 || len(doc.Tables[0].Prov) != 1 {
		t.Fatalf("table prov missing: %+v", doc.Tables)
	}
	tProv := doc.Tables[0].Prov[0]
	if tProv.PageNo != 1 || tProv.CharSpan != [2]int64{0, 0} {
		t.Fatalf("table prov = %+v, want page 1 charspan [0 0]", tProv)
	}

	// 备注 prov：bbox 全 0（源码行为）、页号对齐
	var notesItem *TextItem
	for i := range doc.Texts {
		if doc.Texts[i].Text == "这是备注内容" {
			notesItem = &doc.Texts[i]
			break
		}
	}
	if notesItem == nil || len(notesItem.Prov) != 1 || notesItem.Prov[0].PageNo != 1 ||
		notesItem.Prov[0].BBox == nil || *notesItem.Prov[0].BBox != (DoclingBBox{}) {
		t.Fatalf("notes prov wrong: %+v", notesItem)
	}

	// 无几何形状的 prov 回退整页 (0,0,slideW,slideH)
	fallback := makePptxProv(false, 100, 200, 300, 400, 0, 3, 9144000, 6858000)[0]
	if fallback.BBox.L != 0 || fallback.BBox.T != 0 ||
		fallback.BBox.R != 9144000 || fallback.BBox.B != 6858000 ||
		fallback.BBox.CoordOrigin != CoordOriginBottomLeft || fallback.PageNo != 1 {
		t.Fatalf("fallback prov wrong: %+v", fallback)
	}
}

// TestParsePPTX_VisualOrder 验证形状按视觉位置排序：
// y 相近的两个形状按行内 x 升序输出（XML 中右侧形状先出现，仍应排后），
// 标题（页面最上）最先、表格（页面最下）最后。
func TestParsePPTX_VisualOrder(t *testing.T) {
	doc := parseTestPPTX(t)
	items := ToContentList(doc, SourceGolight)
	titleIdx := findItemText(items, "演示文稿标题")
	leftIdx := findItemText(items, "左侧文本")
	rightIdx := findItemText(items, "普通段落文本")
	tableIdx := -1
	for i := range items {
		if items[i].Type == ItemTypeTable {
			tableIdx = i
			break
		}
	}
	if titleIdx < 0 || leftIdx < 0 || rightIdx < 0 || tableIdx < 0 {
		t.Fatalf("expected items missing: %+v", items)
	}
	// y 相近：左侧文本 y=3500000 与普通段落文本 y=3505420 同行，x 小者在前
	if leftIdx >= rightIdx {
		t.Fatalf("visual order wrong: left text idx=%d should precede right text idx=%d", leftIdx, rightIdx)
	}
	if titleIdx > leftIdx {
		t.Fatalf("title (top of slide) should come first: title=%d left=%d", titleIdx, leftIdx)
	}
	if tableIdx < rightIdx {
		t.Fatalf("table (bottom of slide) should come last: table=%d right=%d", tableIdx, rightIdx)
	}
}

// TestParsePPTX_ToContentListPageIdx 验证简化后 PageIdx=slide-1（0 起页基准）。
func TestParsePPTX_ToContentListPageIdx(t *testing.T) {
	doc := parseTestPPTX(t)
	items := ToContentList(doc, SourceGolight)
	if len(items) == 0 {
		t.Fatalf("content list empty: %+v", items)
	}
	for _, item := range items {
		if item.PageIdx != 0 {
			t.Fatalf("item %q page_idx = %d, want 0 (slide 1 -> 0)", item.Text, item.PageIdx)
		}
	}
}

// TestParsePPTX_Errors 验证错误路径：非 zip 数据报错、缺 presentation.xml 报错、
// 空 slide 集合返回空文档不报错。
func TestParsePPTX_Errors(t *testing.T) {
	// 非 zip 数据
	if _, err := ParsePPTX([]byte("this is not a zip file")); err == nil {
		t.Fatalf("non-zip input should return error")
	}
	// zip 但缺 presentation.xml（如误传 docx）
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	f, _ := w.Create("word/document.xml")
	f.Write([]byte("<w:document/>"))
	w.Close()
	_, err := ParsePPTX(buf.Bytes())
	if err == nil {
		t.Fatalf("zip without presentation.xml should return error")
	}
	if !strings.Contains(err.Error(), "presentation.xml") {
		t.Fatalf("error should mention presentation.xml, got %q", err.Error())
	}
	// 合法 zip 但 slide 集合为空：返回空文档不报错
	var emptyBuf bytes.Buffer
	ew := zip.NewWriter(&emptyBuf)
	ef, _ := ew.Create("ppt/presentation.xml")
	ef.Write([]byte(`<?xml version="1.0"?>
<p:presentation xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">
<p:sldSz cx="9144000" cy="6858000"/>
</p:presentation>`))
	ew.Close()
	doc, err := ParsePPTX(emptyBuf.Bytes())
	if err != nil {
		t.Fatalf("empty slide list should not error: %v", err)
	}
	if doc == nil || len(doc.Pages) != 0 || len(doc.Texts) != 0 || len(doc.Tables) != 0 {
		t.Fatalf("empty slide list should yield empty document: %+v", doc)
	}
	if items := ToContentList(doc, SourceGolight); len(items) != 0 {
		t.Fatalf("empty document should simplify to empty list: %+v", items)
	}
}

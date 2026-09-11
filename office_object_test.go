// office_object_test.go 验证 OOXML 复杂对象统一映射到官方 PictureItem。
package docparse

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

// TestParseDocxComplexOfficeObjects 验证 SmartArt、艺术字和 OLE 对象保留
// 可检索语义、关系目标及对象分类，嵌入二进制不会被执行。
func TestParseDocxComplexOfficeObjects(t *testing.T) {
	documentXML := `<?xml version="1.0"?><w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:dgm="http://schemas.openxmlformats.org/drawingml/2006/diagram" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:wps="http://schemas.microsoft.com/office/word/2010/wordprocessingShape" xmlns:o="urn:schemas-microsoft-com:office:office"><w:body>` +
		`<w:p><w:r><w:drawing><dgm:relIds r:dm="rIdDiagram"/></w:drawing></w:r></w:p>` +
		`<w:p><w:r><w:drawing><wps:wsp><wps:spPr><a:prstTxWarp prst="textArchUp"/></wps:spPr><wps:txbx><a:p><a:r><a:t>年度规划</a:t></a:r></a:p></wps:txbx></wps:wsp></w:drawing></w:r></w:p>` +
		`<w:p><w:r><w:object><o:OLEObject r:id="rIdOle" ProgID="Excel.Sheet.12"/></w:object></w:r></w:p>` +
		`</w:body></w:document>`
	relsXML := `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
		`<Relationship Id="rIdDiagram" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/diagramData" Target="diagrams/data1.xml"/>` +
		`<Relationship Id="rIdOle" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/oleObject" Target="embeddings/sheet1.xlsx"/>` +
		`</Relationships>`
	diagramXML := `<dgm:dataModel xmlns:dgm="http://schemas.openxmlformats.org/drawingml/2006/diagram" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><dgm:ptLst><dgm:pt><dgm:t><a:p><a:r><a:t>开始</a:t></a:r></a:p></dgm:t></dgm:pt><dgm:pt><dgm:t><a:p><a:r><a:t>审批</a:t></a:r></a:p></dgm:t></dgm:pt></dgm:ptLst></dgm:dataModel>`
	data := mustZipEntriesForTest(t, map[string]string{
		"word/document.xml": documentXML, "word/_rels/document.xml.rels": relsXML,
		"word/diagrams/data1.xml": diagramXML, "word/embeddings/sheet1.xlsx": "opaque",
	})

	doc, err := ParseDocx(data)
	if err != nil {
		t.Fatalf("parse docx: %v", err)
	}
	if len(doc.Pictures) != 3 {
		t.Fatalf("pictures=%d want=3: %+v", len(doc.Pictures), doc.Pictures)
	}
	wantClasses := []string{"smartart", "wordart", "embedded_object"}
	wantCaptions := []string{"开始\n审批", "年度规划", "Excel.Sheet.12"}
	for index := range wantClasses {
		picture := doc.Pictures[index]
		if picture.Meta == nil || picture.Meta.Classification == nil || picture.Meta.Classification.Predictions[0].ClassName != wantClasses[index] {
			t.Fatalf("picture[%d] class wrong: %+v", index, picture.Meta)
		}
		if len(picture.Captions) != 1 || doc.Texts[picture.Captions[0].Idx].Text != wantCaptions[index] {
			t.Fatalf("picture[%d] caption wrong: %+v", index, picture.Captions)
		}
		if picture.Image == nil || picture.Image.Mimetype != "image/svg+xml" || picture.Image.Size == nil {
			t.Fatalf("picture[%d] semantic preview missing: %+v", index, picture.Image)
		}
	}
	var target string
	if err := json.Unmarshal(doc.Pictures[2].Meta.Extra["docparse__office_object_target"], &target); err != nil || target != "word/embeddings/sheet1.xlsx" {
		t.Fatalf("OLE target=%q err=%v", target, err)
	}
	validateDocumentWithDoclingCore110ForTest(t, "docx-complex-office-objects", doc)
}

// TestParsePptxComplexShapeRecords 验证幻灯片状态机识别 SmartArt、艺术字和 OLE。
func TestParsePptxComplexShapeRecords(t *testing.T) {
	slide := []byte(`<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:dgm="http://schemas.openxmlformats.org/drawingml/2006/diagram"><p:cSld><p:spTree>` +
		`<p:graphicFrame><p:nvGraphicFramePr><p:cNvPr id="1" name="流程"/></p:nvGraphicFramePr><a:graphic><a:graphicData><dgm:relIds r:dm="rIdDm"/></a:graphicData></a:graphic></p:graphicFrame>` +
		`<p:sp><p:nvSpPr><p:cNvPr id="2" name="艺术字"/></p:nvSpPr><p:spPr/><p:txBody><a:bodyPr><a:prstTxWarp prst="textArchUp"/></a:bodyPr><a:p><a:r><a:t>销售冠军</a:t></a:r></a:p></p:txBody></p:sp>` +
		`<p:graphicFrame><p:nvGraphicFramePr><p:cNvPr id="3" name="嵌入表格"/></p:nvGraphicFramePr><a:graphic><a:graphicData><p:oleObj r:id="rIdOle" progId="Excel.Sheet.12"/></a:graphicData></a:graphic></p:graphicFrame>` +
		`</p:spTree></p:cSld></p:sld>`)
	shapes := parsePptxSlideShapes(slide)
	if len(shapes) != 3 {
		t.Fatalf("shapes=%d want=3: %+v", len(shapes), shapes)
	}
	want := []string{"smartart", "wordart", "embedded_object"}
	for index, className := range want {
		if shapes[index].officeObject == nil || officeObjectClassName(shapes[index].officeObject.kind) != className {
			t.Fatalf("shape[%d] object wrong: %+v", index, shapes[index].officeObject)
		}
	}
	if got := pptxAllParaText(shapes[1]); got != "销售冠军" {
		t.Fatalf("wordart text=%q", got)
	}
}

// TestParsePPTXComplexOfficeObjectsIntegration 验证 PPTX 入口读取 SmartArt
// 数据部件、艺术字文本和 OLE 关系，并生成语义预览。
func TestParsePPTXComplexOfficeObjectsIntegration(t *testing.T) {
	slide := `<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:dgm="http://schemas.openxmlformats.org/drawingml/2006/diagram"><p:cSld><p:spTree>` +
		`<p:graphicFrame><p:nvGraphicFramePr><p:cNvPr id="1" name="流程"/></p:nvGraphicFramePr><p:xfrm><a:off x="10" y="10"/><a:ext cx="200" cy="100"/></p:xfrm><a:graphic><a:graphicData><dgm:relIds r:dm="rIdDm"/></a:graphicData></a:graphic></p:graphicFrame>` +
		`<p:sp><p:nvSpPr><p:cNvPr id="2" name="艺术字"/></p:nvSpPr><p:spPr><a:xfrm><a:off x="10" y="150"/><a:ext cx="200" cy="100"/></a:xfrm></p:spPr><p:txBody><a:bodyPr><a:prstTxWarp prst="textArchUp"/></a:bodyPr><a:p><a:r><a:t>销售冠军</a:t></a:r></a:p></p:txBody></p:sp>` +
		`<p:graphicFrame><p:nvGraphicFramePr><p:cNvPr id="3" name="嵌入表格"/></p:nvGraphicFramePr><p:xfrm><a:off x="10" y="300"/><a:ext cx="200" cy="100"/></p:xfrm><a:graphic><a:graphicData><p:oleObj r:id="rIdOle" progId="Excel.Sheet.12"/></a:graphicData></a:graphic></p:graphicFrame>` +
		`</p:spTree></p:cSld></p:sld>`
	rels := `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rIdDm" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/diagramData" Target="../diagrams/data1.xml"/><Relationship Id="rIdOle" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/oleObject" Target="../embeddings/object1.bin"/></Relationships>`
	data := rewriteOOXMLFixture(t, buildTestPPTX(t), map[string]string{
		"ppt/slides/slide1.xml":            slide,
		"ppt/slides/_rels/slide1.xml.rels": rels,
		"ppt/diagrams/data1.xml":           `<dgm:dataModel xmlns:dgm="http://schemas.openxmlformats.org/drawingml/2006/diagram" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><dgm:ptLst><dgm:pt><dgm:t><a:p><a:r><a:t>提交</a:t></a:r></a:p></dgm:t></dgm:pt><dgm:pt><dgm:t><a:p><a:r><a:t>通过</a:t></a:r></a:p></dgm:t></dgm:pt></dgm:ptLst></dgm:dataModel>`,
		"ppt/embeddings/object1.bin":       "opaque",
	}, nil)
	doc, err := ParsePPTX(data)
	if err != nil {
		t.Fatalf("parse pptx: %v", err)
	}
	classes := map[string]string{}
	for _, picture := range doc.Pictures {
		if picture.Meta == nil || picture.Meta.Classification == nil || len(picture.Meta.Classification.Predictions) == 0 || len(picture.Captions) == 0 {
			continue
		}
		classes[picture.Meta.Classification.Predictions[0].ClassName] = doc.Texts[picture.Captions[0].Idx].Text
	}
	if classes["smartart"] != "提交\n通过" || classes["wordart"] != "销售冠军" || classes["embedded_object"] != "嵌入表格" {
		t.Fatalf("PPTX complex objects wrong: %+v", classes)
	}
	validateDocumentWithDoclingCore110ForTest(t, "pptx-complex-office-objects", doc)
}

// TestParseXLSXDrawingAndOLEObjects 验证工作表 DrawingML 形状、SmartArt
// 及 worksheet OLE 引用的纯 XML 提取。
func TestParseXLSXDrawingAndOLEObjects(t *testing.T) {
	drawing := []byte(`<xdr:wsDr xmlns:xdr="http://schemas.openxmlformats.org/drawingml/2006/spreadsheetDrawing" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:dgm="http://schemas.openxmlformats.org/drawingml/2006/diagram"><xdr:twoCellAnchor><xdr:from><xdr:col>2</xdr:col><xdr:row>3</xdr:row></xdr:from><xdr:sp><xdr:nvSpPr><xdr:cNvPr id="1" name="提示框"/></xdr:nvSpPr><xdr:spPr><a:prstGeom prst="roundRect"/></xdr:spPr><xdr:txBody><a:p><a:r><a:t>重要提示</a:t></a:r></a:p></xdr:txBody></xdr:sp></xdr:twoCellAnchor><xdr:oneCellAnchor><xdr:from><xdr:col>5</xdr:col><xdr:row>6</xdr:row></xdr:from><xdr:graphicFrame><a:graphic><a:graphicData><dgm:relIds r:dm="rIdDm"/></a:graphicData></a:graphic></xdr:graphicFrame></xdr:oneCellAnchor></xdr:wsDr>`)
	objects := parseXLSXDrawingObjects(drawing)
	if len(objects) != 2 || objects[0].row != 3 || objects[0].col != 2 || objects[0].record.text != "重要提示" || objects[0].record.geometry != "roundRect" {
		t.Fatalf("drawing objects wrong: %+v", objects)
	}
	if objects[1].record.relationship != "rIdDm" || officeObjectClassName(objects[1].record.kind) != "smartart" {
		t.Fatalf("smartart wrong: %+v", objects[1])
	}
	ole := parseXLSXOLEObjects([]byte(`<worksheet xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><oleObjects><oleObject r:id="rIdOle" progId="AcroExch.Document" shapeId="1025"/></oleObjects></worksheet>`))
	if len(ole) != 1 || ole[0].program != "AcroExch.Document" || ole[0].relationship != "rIdOle" {
		t.Fatalf("ole objects wrong: %+v", ole)
	}
}

// TestParseXLSXComplexOfficeObjectsIntegration 验证工作簿入口实际装配 drawing
// 形状、SmartArt 数据部件与 OLE 关系。
func TestParseXLSXComplexOfficeObjectsIntegration(t *testing.T) {
	workbook := excelize.NewFile()
	if err := workbook.SetCellValue("Sheet1", "A1", "正文"); err != nil {
		t.Fatalf("set cell: %v", err)
	}
	base, err := workbook.WriteToBuffer()
	if err != nil {
		t.Fatalf("write xlsx: %v", err)
	}
	_ = workbook.Close()
	drawing := `<xdr:wsDr xmlns:xdr="http://schemas.openxmlformats.org/drawingml/2006/spreadsheetDrawing" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:dgm="http://schemas.openxmlformats.org/drawingml/2006/diagram"><xdr:twoCellAnchor><xdr:from><xdr:col>1</xdr:col><xdr:row>1</xdr:row></xdr:from><xdr:sp><xdr:nvSpPr><xdr:cNvPr id="1" name="说明框"/></xdr:nvSpPr><xdr:txBody><a:p><a:r><a:t>工作表说明</a:t></a:r></a:p></xdr:txBody></xdr:sp></xdr:twoCellAnchor><xdr:oneCellAnchor><xdr:from><xdr:col>3</xdr:col><xdr:row>2</xdr:row></xdr:from><xdr:graphicFrame><a:graphic><a:graphicData><dgm:relIds r:dm="rIdDm"/></a:graphicData></a:graphic></xdr:graphicFrame></xdr:oneCellAnchor></xdr:wsDr>`
	rels := `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rIdDrawing" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/drawing" Target="../drawings/drawing1.xml"/><Relationship Id="rIdOle" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/oleObject" Target="../embeddings/object1.bin"/></Relationships>`
	drawingRels := `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rIdDm" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/diagramData" Target="../diagrams/data1.xml"/></Relationships>`
	data := rewriteOOXMLFixture(t, base.Bytes(), map[string]string{
		"xl/worksheets/_rels/sheet1.xml.rels": rels,
		"xl/drawings/drawing1.xml":            drawing,
		"xl/drawings/_rels/drawing1.xml.rels": drawingRels,
		"xl/diagrams/data1.xml":               `<dgm:dataModel xmlns:dgm="http://schemas.openxmlformats.org/drawingml/2006/diagram" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><dgm:ptLst><dgm:pt><dgm:t><a:p><a:r><a:t>节点甲</a:t></a:r></a:p></dgm:t></dgm:pt></dgm:ptLst></dgm:dataModel>`,
		"xl/embeddings/object1.bin":           "opaque",
	}, func(name, content string) string {
		if name != "xl/worksheets/sheet1.xml" {
			return content
		}
		if !strings.Contains(content, `xmlns:r=`) {
			content = strings.Replace(content, `<worksheet`, `<worksheet xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"`, 1)
		}
		return strings.Replace(content, `</worksheet>`, `<drawing r:id="rIdDrawing"/><oleObjects><oleObject r:id="rIdOle" progId="Package" shapeId="7"/></oleObjects></worksheet>`, 1)
	})
	doc, err := ParseXLSX(data)
	if err != nil {
		t.Fatalf("parse xlsx: %v", err)
	}
	classes := map[string]bool{}
	for _, picture := range doc.Pictures {
		if picture.Meta != nil && picture.Meta.Classification != nil && len(picture.Meta.Classification.Predictions) > 0 {
			classes[picture.Meta.Classification.Predictions[0].ClassName] = true
		}
	}
	for _, className := range []string{"shape", "smartart", "embedded_object"} {
		if !classes[className] {
			t.Fatalf("missing %s: pictures=%+v", className, doc.Pictures)
		}
	}
	validateDocumentWithDoclingCore110ForTest(t, "xlsx-complex-office-objects", doc)
}

// TestOfficeObjectCaptionRoundTrip 验证扩展元数据与 caption 可官方化序列化。
func TestOfficeObjectCaptionRoundTrip(t *testing.T) {
	doc := NewDoclingDocument("office")
	addOfficeObjectPicture(doc, officeObjectRecord{kind: "shape", name: "说明", text: "流程说明"}, nil, nil, LayerBody)
	payload, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded DoclingDocument
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(decoded.Pictures) != 1 || len(decoded.Pictures[0].Captions) != 1 || decoded.Texts[0].Text != "流程说明" {
		t.Fatalf("round trip wrong: %+v", decoded)
	}
}

// rewriteOOXMLFixture 改写/追加测试 OOXML zip 条目。
func rewriteOOXMLFixture(t *testing.T, source []byte, replacements map[string]string, transform func(string, string) string) []byte {
	t.Helper()
	reader, err := zip.NewReader(bytes.NewReader(source), int64(len(source)))
	if err != nil {
		t.Fatalf("open fixture zip: %v", err)
	}
	var output bytes.Buffer
	writer := zip.NewWriter(&output)
	written := map[string]bool{}
	for _, file := range reader.File {
		rc, err := file.Open()
		if err != nil {
			t.Fatalf("open %s: %v", file.Name, err)
		}
		content, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			t.Fatalf("read %s: %v", file.Name, err)
		}
		value := string(content)
		if replacement, ok := replacements[file.Name]; ok {
			value = replacement
			written[file.Name] = true
		}
		if transform != nil {
			value = transform(file.Name, value)
		}
		entry, _ := writer.Create(file.Name)
		_, _ = entry.Write([]byte(value))
	}
	for name, value := range replacements {
		if written[name] {
			continue
		}
		entry, _ := writer.Create(name)
		_, _ = entry.Write([]byte(value))
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close fixture zip: %v", err)
	}
	return output.Bytes()
}

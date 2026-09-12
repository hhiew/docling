// examples_rich_test.go 生成 examples/{docx,pptx}/rich-* 复杂对象样例:
// DOCX 含 SmartArt、艺术字、OMML 公式与 OLE 嵌入对象,PPTX 含原生图表与
// 演讲者备注。构造逻辑复用测试 helper,期望输出与常规样例一起走
// TestExamplesGolden 严格逐字回归。
package docling

import (
	"bytes"
	"testing"

	"github.com/xuri/excelize/v2"
)

// buildRichExampleDocx 构造复杂 DOCX 样例:标题、正文、OMML 公式、
// SmartArt 流程、艺术字与 OLE 嵌入工作簿。
func buildRichExampleDocx(t *testing.T) []byte {
	t.Helper()
	documentXML := `<?xml version="1.0"?><w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:dgm="http://schemas.openxmlformats.org/drawingml/2006/diagram" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:wps="http://schemas.microsoft.com/office/word/2010/wordprocessingShape" xmlns:m="http://schemas.openxmlformats.org/officeDocument/2006/math" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:o="urn:schemas-microsoft-com:office:office"><w:body>` +
		`<w:p><w:pPr><w:pStyle w:val="Heading1"/></w:pPr><w:r><w:t>产品评审报告</w:t></w:r></w:p>` +
		`<w:p><w:r><w:t>本报告汇总评审结论、关键公式与流程图,供后续验收引用。</w:t></w:r></w:p>` +
		`<w:p><w:r><w:t>能量换算关系:</w:t></w:r><m:oMath><m:sSup><m:e><m:r><m:t>E</m:t></m:r></m:e><m:sup><m:r><m:t>2</m:t></m:r></m:sup></m:sSup><m:r><m:t>=</m:t></m:r><m:r><m:t>mc</m:t></m:r></m:oMath></w:p>` +
		`<w:p><w:r><w:drawing><dgm:relIds r:dm="rIdDiagram"/></w:drawing></w:r></w:p>` +
		`<w:p><w:r><w:drawing><wps:wsp><wps:spPr><a:prstTxWarp prst="textArchUp"/></wps:spPr><wps:txbx><a:p><a:r><a:t>年度规划</a:t></a:r></a:p></wps:txbx></wps:wsp></w:drawing></w:r></w:p>` +
		`<w:p><w:r><w:object><o:OLEObject r:id="rIdOle" ProgID="Excel.Sheet.12"/></w:object></w:r></w:p>` +
		`</w:body></w:document>`
	relsXML := `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
		`<Relationship Id="rIdDiagram" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/diagramData" Target="diagrams/data1.xml"/>` +
		`<Relationship Id="rIdOle" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/oleObject" Target="embeddings/sheet1.xlsx"/>` +
		`</Relationships>`
	diagramXML := `<dgm:dataModel xmlns:dgm="http://schemas.openxmlformats.org/drawingml/2006/diagram" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><dgm:ptLst>` +
		`<dgm:pt><dgm:t><a:p><a:r><a:t>提交申请</a:t></a:r></a:p></dgm:t></dgm:pt>` +
		`<dgm:pt><dgm:t><a:p><a:r><a:t>技术评审</a:t></a:r></a:p></dgm:t></dgm:pt>` +
		`<dgm:pt><dgm:t><a:p><a:r><a:t>发布上线</a:t></a:r></a:p></dgm:t></dgm:pt>` +
		`</dgm:ptLst></dgm:dataModel>`
	return mustZipEntriesForTest(t, map[string]string{
		"word/document.xml":            documentXML,
		"word/_rels/document.xml.rels": relsXML,
		"word/diagrams/data1.xml":      diagramXML,
		"word/embeddings/sheet1.xlsx":  "opaque",
	})
}

// buildRichExamplePPTX 复用高级 PPTX 构造器:含母版继承、组合形状、
// 原生柱状图(带缓存数据)与演讲者备注。
func buildRichExamplePPTX(t *testing.T) []byte {
	t.Helper()
	return buildAdvancedTestPPTX(t)
}

// buildRichExampleXLSX 构造含原生折线图的工作簿样例:图表系列引用工作
// 簿单元格区域,验证图表→SVG 语义预览与可检索数据表格的转换。
func buildRichExampleXLSX(t *testing.T) []byte {
	t.Helper()
	book := excelize.NewFile()
	defer func() { _ = book.Close() }()
	for cell, value := range map[string]any{
		"A1": "月份", "B1": "销量",
		"A2": "1月", "B2": 120,
		"A3": "2月", "B3": 186,
		"A4": "3月", "B4": 154,
	} {
		if err := book.SetCellValue("Sheet1", cell, value); err != nil {
			t.Fatalf("set cell %s: %v", cell, err)
		}
	}
	if err := book.AddChart("Sheet1", "D2", &excelize.Chart{
		Type: excelize.Line,
		Series: []excelize.ChartSeries{{
			Name:       "Sheet1!$B$1",
			Categories: "Sheet1!$A$2:$A$4",
			Values:     "Sheet1!$B$2:$B$4",
		}},
	}); err != nil {
		t.Fatalf("add chart: %v", err)
	}
	var buf bytes.Buffer
	if _, err := book.WriteTo(&buf); err != nil {
		t.Fatalf("write xlsx: %v", err)
	}
	return buf.Bytes()
}

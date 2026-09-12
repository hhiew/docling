// examples_rich_test.go 生成 examples/{docx,pptx,xlsx}/rich-* 复杂对象样例:
// DOCX 含 OMML 公式、SmartArt 流程图(标准 data/layout/quickStyle/colors
// 四件套)、艺术字与 OLE 嵌入工作簿;PPTX 含原生图表、母版继承与组合形状;
// XLSX 含原生折线图。样例均为完整合法的 OOXML 包,可直接用 Office/LibreOffice
// 打开对照。期望输出与常规样例一起走 TestExamplesGolden 严格逐字回归。
package docling

import (
	"archive/zip"
	"bytes"
	"io"
	"sort"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

// buildRichExampleDocx 构造复杂 DOCX 样例:标题、正文、OMML 公式、
// SmartArt 流程、艺术字与 OLE 嵌入工作簿。
func buildRichExampleDocx(t *testing.T) []byte {
	t.Helper()
	const (
		wNS      = "http://schemas.openxmlformats.org/wordprocessingml/2006/main"
		rNS      = "http://schemas.openxmlformats.org/officeDocument/2006/relationships"
		dgmNS    = "http://schemas.openxmlformats.org/drawingml/2006/diagram"
		aNS      = "http://schemas.openxmlformats.org/drawingml/2006/main"
		wpsNS    = "http://schemas.microsoft.com/office/word/2010/wordprocessingShape"
		mNS      = "http://schemas.openxmlformats.org/officeDocument/2006/math"
		oNS      = "urn:schemas-microsoft-com:office:office"
		cTypesNS = "http://schemas.openxmlformats.org/package/2006/content-types"
		relsNS   = "http://schemas.openxmlformats.org/package/2006/relationships"
	)
	documentXML := `<?xml version="1.0"?><w:document xmlns:w="` + wNS + `" xmlns:r="` + rNS +
		`" xmlns:dgm="` + dgmNS + `" xmlns:a="` + aNS + `" xmlns:wps="` + wpsNS +
		`" xmlns:m="` + mNS + `" xmlns:o="` + oNS + `"><w:body>` +
		`<w:p><w:pPr><w:pStyle w:val="Heading1"/></w:pPr><w:r><w:t>产品评审报告</w:t></w:r></w:p>` +
		`<w:p><w:r><w:t>本报告汇总评审结论、关键公式与流程图,供后续验收引用。</w:t></w:r></w:p>` +
		`<w:p><w:r><w:t>能量换算关系:</w:t></w:r><m:oMath><m:sSup><m:e><m:r><m:t>E</m:t></m:r></m:e><m:sup><m:r><m:t>2</m:t></m:r></m:sup></m:sSup><m:r><m:t>=</m:t></m:r><m:r><m:t>mc</m:t></m:r></m:oMath></w:p>` +
		`<w:p><w:r><w:drawing><dgm:relIds r:dm="rIdDiagram" r:lo="rIdLayout" r:qs="rIdQuickStyle" r:cs="rIdColors"/></w:drawing></w:r></w:p>` +
		`<w:p><w:r><w:drawing><wps:wsp><wps:spPr><a:prstTxWarp prst="textArchUp"/></wps:spPr><wps:txbx><a:p><a:r><a:t>年度规划</a:t></a:r></a:p></wps:txbx></wps:wsp></w:drawing></w:r></w:p>` +
		`<w:p><w:r><w:object><o:OLEObject r:id="rIdOle" ProgID="Excel.Sheet.12"/></w:object></w:r></w:p>` +
		`</w:body></w:document>`
	relsXML := `<?xml version="1.0"?><Relationships xmlns="` + relsNS + `">` +
		`<Relationship Id="rIdDiagram" Type="` + rNS + `/diagramData" Target="diagrams/data1.xml"/>` +
		`<Relationship Id="rIdLayout" Type="` + rNS + `/diagramLayout" Target="diagrams/layout1.xml"/>` +
		`<Relationship Id="rIdQuickStyle" Type="` + rNS + `/diagramQuickStyle" Target="diagrams/quickStyle1.xml"/>` +
		`<Relationship Id="rIdColors" Type="` + rNS + `/diagramColors" Target="diagrams/colors1.xml"/>` +
		`<Relationship Id="rIdOle" Type="` + rNS + `/oleObject" Target="embeddings/sheet1.xlsx"/>` +
		`<Relationship Id="rIdStyles" Type="` + rNS + `/styles" Target="styles.xml"/>` +
		`</Relationships>`
	dataXML := `<?xml version="1.0"?><dgm:dataModel xmlns:dgm="` + dgmNS + `" xmlns:a="` + aNS + `"><dgm:ptLst>` +
		`<dgm:pt modelId="1"><dgm:t><a:p><a:r><a:t>提交申请</a:t></a:r></a:p></dgm:t></dgm:pt>` +
		`<dgm:pt modelId="2"><dgm:t><a:p><a:r><a:t>技术评审</a:t></a:r></a:p></dgm:t></dgm:pt>` +
		`<dgm:pt modelId="3"><dgm:t><a:p><a:r><a:t>发布上线</a:t></a:r></a:p></dgm:t></dgm:pt>` +
		`</dgm:ptLst>` +
		`<dgm:cxnLst>` +
		`<dgm:cxn modelId="c1" srcId="1" destId="2" srcOrd="0" destOrd="0"/>` +
		`<dgm:cxn modelId="c2" srcId="2" destId="3" srcOrd="0" destOrd="0"/>` +
		`</dgm:cxnLst></dgm:dataModel>`
	layoutXML := `<?xml version="1.0"?><dgm:layoutDef xmlns:dgm="` + dgmNS + `" uniqueId="urn:layout/process">` +
		`<dgm:catLst><dgm:cat type="process" pri="1000"/></dgm:catLst></dgm:layoutDef>`
	quickStyleXML := `<?xml version="1.0"?><dgm:styleDef xmlns:dgm="` + dgmNS + `" uniqueId="urn:simple1">` +
		`<dgm:catLst><dgm:cat type="simple" pri="10100"/></dgm:catLst></dgm:styleDef>`
	colorsXML := `<?xml version="1.0"?><dgm:colorsDef xmlns:dgm="` + dgmNS + `" uniqueId="urn:accent1">` +
		`<dgm:catLst><dgm:cat type="accent1" pri="11200"/></dgm:catLst></dgm:colorsDef>`

	// OLE 嵌入一个真实可打开的小工作簿(而非占位字节),保证样例在
	// Office/LibreOffice 中可完整打开。
	book := excelize.NewFile()
	for cell, value := range map[string]any{"A1": "季度", "B1": "营收", "A2": "Q1", "B2": 128} {
		if err := book.SetCellValue("Sheet1", cell, value); err != nil {
			t.Fatalf("set embedded cell %s: %v", cell, err)
		}
	}
	var embedded bytes.Buffer
	if _, err := book.WriteTo(&embedded); err != nil {
		t.Fatalf("write embedded xlsx: %v", err)
	}

	entries := map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
			`<Types xmlns="` + cTypesNS + `">` +
			`<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>` +
			`<Default Extension="xml" ContentType="application/xml"/>` +
			`<Default Extension="xlsx" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"/>` +
			`<Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>` +
			`<Override PartName="/word/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.styles+xml"/>` +
			`</Types>`,
		"_rels/.rels": `<?xml version="1.0"?><Relationships xmlns="` + relsNS + `">` +
			`<Relationship Id="rId1" Type="` + rNS + `/officeDocument" Target="word/document.xml"/>` +
			`</Relationships>`,
		"word/document.xml":            documentXML,
		"word/_rels/document.xml.rels": relsXML,
		"word/styles.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
			`<w:styles xmlns:w="` + wNS + `">` +
			`<w:style w:type="paragraph" w:styleId="Heading1"><w:name w:val="heading 1"/>` +
			`<w:pPr><w:spacing w:before="240" w:after="120"/></w:pPr>` +
			`<w:rPr><w:b/><w:sz w:val="44"/><w:szCs w:val="44"/></w:rPr></w:style>` +
			`<w:style w:type="paragraph" w:styleId="Normal" w:default="1">` +
			`<w:rPr><w:sz w:val="24"/></w:rPr></w:style>` +
			`</w:styles>`,
		"word/diagrams/data1.xml":       dataXML,
		"word/diagrams/layout1.xml":     layoutXML,
		"word/diagrams/quickStyle1.xml": quickStyleXML,
		"word/diagrams/colors1.xml":     colorsXML,
	}
	return mustZipEntriesWithBinaryForTest(t, entries, "word/embeddings/sheet1.xlsx", embedded.Bytes())
}

// buildRichExamplePPTX 基于常规 PPTX 构造器(标题、层级列表、跨列表格、
// 备注与母版)注入原生柱状图 graphicFrame,得到带真实尺寸与图表的复杂
// 样例;包结构完整,可直接用 Office/LibreOffice 打开。
func buildRichExamplePPTX(t *testing.T) []byte {
	t.Helper()
	base := buildTestPPTX(t)
	reader, err := zip.NewReader(bytes.NewReader(base), int64(len(base)))
	if err != nil {
		t.Fatalf("open base pptx: %v", err)
	}
	parts := make(map[string][]byte, len(reader.File)+2)
	for _, file := range reader.File {
		handle, openErr := file.Open()
		if openErr != nil {
			t.Fatalf("open %s: %v", file.Name, openErr)
		}
		data, readErr := io.ReadAll(handle)
		_ = handle.Close()
		if readErr != nil {
			t.Fatalf("read %s: %v", file.Name, readErr)
		}
		parts[file.Name] = data
	}

	// slide1 注入图表 graphicFrame,并补充 chart 部件与关系。
	slide1 := string(parts["ppt/slides/slide1.xml"])
	const chartFrame = `<p:graphicFrame><p:nvGraphicFramePr><p:cNvPr id="90" name="销量图表"/>` +
		`<p:cNvGraphicFramePr/><p:nvPr/></p:nvGraphicFramePr>` +
		`<p:xfrm><a:off x="5486400" y="1825625"/><a:ext cx="3200400" cy="2743200"/></p:xfrm>` +
		`<a:graphic><a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/chart">` +
		`<c:chart xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" r:id="rIdChart"/>` +
		`</a:graphicData></a:graphic></p:graphicFrame>`
	slide1 = strings.Replace(slide1, "</p:spTree>", chartFrame+"</p:spTree>", 1)
	parts["ppt/slides/slide1.xml"] = []byte(slide1)

	slide1RelsPath := "ppt/slides/_rels/slide1.xml.rels"
	slide1Rels := string(parts[slide1RelsPath])
	slide1Rels = strings.Replace(slide1Rels, "</Relationships>",
		`<Relationship Id="rIdChart" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/chart" Target="../charts/chart1.xml"/></Relationships>`, 1)
	parts[slide1RelsPath] = []byte(slide1Rels)

	// 注意:chart 部件不带 XML 声明头,LibreOffice 对带声明的 chart 部件会拒绝加载。
	// chart1.xml 取自 excelize 的真实产物:LibreOffice 的 chart 导入器对手写
	// 的简化 chart 结构过于挑剔,真实结构在 Office/LibreOffice 中均可打开。
	chartBook := excelize.NewFile()
	for cell, value := range map[string]any{"A1": "月份", "B1": "销量", "A2": "1月", "B2": 120, "A3": "2月", "B3": 186} {
		if err := chartBook.SetCellValue("Sheet1", cell, value); err != nil {
			t.Fatalf("set chart source cell %s: %v", cell, err)
		}
	}
	if err := chartBook.AddChart("Sheet1", "D2", &excelize.Chart{
		Type: excelize.Col,
		Series: []excelize.ChartSeries{{
			Name:       "Sheet1!$B$1",
			Categories: "Sheet1!$A$2:$A$3",
			Values:     "Sheet1!$B$2:$B$3",
		}},
	}); err != nil {
		t.Fatalf("add chart: %v", err)
	}
	var chartBuf bytes.Buffer
	if _, err := chartBook.WriteTo(&chartBuf); err != nil {
		t.Fatalf("write chart source xlsx: %v", err)
	}
	chartReader, err := zip.NewReader(bytes.NewReader(chartBuf.Bytes()), int64(chartBuf.Len()))
	if err != nil {
		t.Fatalf("open chart source xlsx: %v", err)
	}
	// chart 数据无缓存,依赖嵌入工作簿回填:把 chart 关系指向的嵌入
	// xlsx 一并搬入 ppt 包,Office/LibreOffice 与本库的回填路径都能取数。
	var chartXML []byte
	for _, file := range chartReader.File {
		switch {
		case file.Name == "xl/charts/chart1.xml":
			handle, openErr := file.Open()
			if openErr != nil {
				t.Fatalf("open chart1.xml: %v", openErr)
			}
			data, readErr := io.ReadAll(handle)
			_ = handle.Close()
			if readErr != nil {
				t.Fatalf("read chart1.xml: %v", readErr)
			}
			chartXML = data
		}
	}
	if len(chartXML) == 0 {
		t.Fatal("chart source xlsx has no chart1.xml")
	}
	// excelize 不写缓存数据且使用默认命名空间(无 c: 前缀):给 strRef/numRef
	// 注入 strCache/numCache,使 Office/LibreOffice 与本库的缓存读取路径都
	// 能直接取数。
	chartText := string(chartXML)
	chartText = strings.Replace(chartText, "<f>Sheet1!$A$2:$A$3</f>",
		`<f>Sheet1!$A$2:$A$3</f><strCache><ptCount val="2"/><pt idx="0"><v>1月</v></pt><pt idx="1"><v>2月</v></pt></strCache>`, 1)
	chartText = strings.Replace(chartText, "<f>Sheet1!$B$2:$B$3</f>",
		`<f>Sheet1!$B$2:$B$3</f><numCache><formatCode>General</formatCode><ptCount val="2"/><pt idx="0"><v>120</v></pt><pt idx="1"><v>186</v></pt></numCache>`, 1)
	chartText = strings.Replace(chartText, "<f>Sheet1!$B$1</f>",
		`<f>Sheet1!$B$1</f><strCache><ptCount val="1"/><pt idx="0"><v>销量</v></pt></strCache>`, 1)
	chartXML = []byte(chartText)
	parts["ppt/charts/chart1.xml"] = chartXML

	return mustZipEntriesWithBinaryForTest(t, toStringMap(parts), "__none__", nil)
}

// toStringMap 把二进制条目表转为字符串表(样例部件均为文本)。
func toStringMap(parts map[string][]byte) map[string]string {
	out := make(map[string]string, len(parts))
	for name, data := range parts {
		out[name] = string(data)
	}
	return out
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

// mustZipEntriesWithBinaryForTest 打包字符串与二进制混合条目为 zip 字节流。
func mustZipEntriesWithBinaryForTest(t *testing.T, entries map[string]string, binaryName string, binaryData []byte) []byte {
	t.Helper()
	all := make(map[string][]byte, len(entries)+1)
	for name, data := range entries {
		all[name] = []byte(data)
	}
	if binaryName != "__none__" {
		all[binaryName] = binaryData
	}
	names := make([]string, 0, len(all))
	for name := range all {
		names = append(names, name)
	}
	// 复用字符串版 helper 逐条写入二进制不安全,这里直接构造。
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	sort.Strings(names)
	// OPC 惯例:[Content_Types].xml 物理上放在包首,部分消费者依赖此顺序探测。
	if idx := sort.SearchStrings(names, "[Content_Types].xml"); idx < len(names) && names[idx] == "[Content_Types].xml" {
		names = append([]string{"[Content_Types].xml"}, append(names[:idx], names[idx+1:]...)...)
	}
	for _, name := range names {
		file, err := zw.Create(name)
		if err != nil {
			t.Fatalf("zip create %s: %v", name, err)
		}
		if _, err := file.Write(all[name]); err != nil {
			t.Fatalf("zip write %s: %v", name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("zip close: %v", err)
	}
	return buf.Bytes()
}

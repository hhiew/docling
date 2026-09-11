// ooxml_chart_test.go 验证 Office Open XML 图表缓存到官方 PictureItem
// 元数据的转换，覆盖柱状图、折线图和饼图的分类、标题与表格化数据。
package docling

import (
	"bytes"
	"encoding/base64"
	"regexp"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

// ooxmlChartRadiusPattern 提取 SVG 圆点半径，用于断言气泡按大小缩放。
var ooxmlChartRadiusPattern = regexp.MustCompile(`r="([0-9.]+)"`)

// buildOOXMLChartWorkbookFixture 构造只有单元格数据、由图表公式引用的最小
// 工作簿，用于验证缓存缺失时的纯 Go 数据回填。
func buildOOXMLChartWorkbookFixture(t *testing.T) []byte {
	t.Helper()
	book := excelize.NewFile()
	defer func() { _ = book.Close() }()
	for cell, value := range map[string]any{
		"A1": "月份", "B1": "销量", "A2": "1月", "B2": 12, "A3": "2月", "B3": 18,
	} {
		if err := book.SetCellValue("Sheet1", cell, value); err != nil {
			t.Fatalf("set chart workbook cell %s: %v", cell, err)
		}
	}
	var buf bytes.Buffer
	if _, err := book.WriteTo(&buf); err != nil {
		t.Fatalf("write chart workbook: %v", err)
	}
	return buf.Bytes()
}

// TestAddOOXMLChartPictureBar 验证柱状图生成 label=picture，并把分类、标题、
// 分类轴和多系列数值写入 meta.tabular_chart.chart_data。
func TestAddOOXMLChartPictureBar(t *testing.T) {
	doc := NewDoclingDocument("chart")
	chart := []byte(`<c:chartSpace xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
<c:chart><c:title><c:tx><c:rich><a:p><a:r><a:t>季度销量</a:t></a:r></a:p></c:rich></c:tx></c:title><c:plotArea><c:barChart>
<c:ser><c:tx><c:strRef><c:strCache><c:pt idx="0"><c:v>华东</c:v></c:pt></c:strCache></c:strRef></c:tx><c:cat><c:strRef><c:strCache><c:pt idx="0"><c:v>一季度</c:v></c:pt><c:pt idx="1"><c:v>二季度</c:v></c:pt></c:strCache></c:strRef></c:cat><c:val><c:numRef><c:numCache><c:pt idx="0"><c:v>12</c:v></c:pt><c:pt idx="1"><c:v>18</c:v></c:pt></c:numCache></c:numRef></c:val></c:ser>
<c:ser><c:tx><c:v>华南</c:v></c:tx><c:cat><c:strRef><c:strCache><c:pt idx="0"><c:v>一季度</c:v></c:pt><c:pt idx="1"><c:v>二季度</c:v></c:pt></c:strCache></c:strRef></c:cat><c:val><c:numRef><c:numCache><c:pt idx="0"><c:v>9</c:v></c:pt><c:pt idx="1"><c:v>16</c:v></c:pt></c:numCache></c:numRef></c:val></c:ser>
</c:barChart></c:plotArea></c:chart></c:chartSpace>`)

	ref, ok := addOOXMLChartPicture(doc, chart, nil, nil)
	if !ok || ref.String() != "#/pictures/0" || len(doc.Pictures) != 1 {
		t.Fatalf("add chart picture failed: ok=%v ref=%s pictures=%d", ok, ref.String(), len(doc.Pictures))
	}
	picture := doc.Pictures[0]
	if picture.Label != LabelPicture || picture.Meta == nil || picture.Meta.Classification == nil ||
		len(picture.Meta.Classification.Predictions) != 1 ||
		picture.Meta.Classification.Predictions[0].ClassName != "bar_chart" {
		t.Fatalf("chart classification wrong: %+v", picture)
	}
	chartMeta := picture.Meta.TabularChart
	if chartMeta == nil || chartMeta.Title != "季度销量" || chartMeta.ChartData == nil {
		t.Fatalf("tabular chart meta wrong: %+v", chartMeta)
	}
	assertChartTable(t, chartMeta.ChartData, [][]string{
		{"类别", "华东", "华南"},
		{"一季度", "12", "9"},
		{"二季度", "18", "16"},
	})
	if picture.Image == nil || picture.Image.Mimetype != "image/svg+xml" || picture.Image.Dpi != 96 ||
		picture.Image.Size == nil || picture.Image.Size.Width != 960 || picture.Image.Size.Height != 540 ||
		!strings.HasPrefix(picture.Image.URI, "data:image/svg+xml;base64,") {
		t.Fatalf("chart SVG preview missing: %+v", picture.Image)
	}
	validateDocumentWithDoclingCore110ForTest(t, "chart-svg", doc)
}

// TestParseOOXMLChartLineAndPie 验证折线图与饼图使用各自官方分类，并兼容
// strLit/numLit 和稀疏缓存点索引。
func TestParseOOXMLChartLineAndPie(t *testing.T) {
	tests := []struct {
		name      string
		xml       string
		className string
		want      [][]string
	}{
		{
			name:      "line",
			xml:       `<c:chartSpace xmlns:c="urn:chart"><c:chart><c:plotArea><c:lineChart><c:ser><c:tx><c:v>趋势</c:v></c:tx><c:cat><c:strLit><c:pt idx="0"><c:v>1月</c:v></c:pt><c:pt idx="2"><c:v>3月</c:v></c:pt></c:strLit></c:cat><c:val><c:numLit><c:pt idx="0"><c:v>2</c:v></c:pt><c:pt idx="2"><c:v>8</c:v></c:pt></c:numLit></c:val></c:ser></c:lineChart></c:plotArea></c:chart></c:chartSpace>`,
			className: "line_chart",
			want:      [][]string{{"类别", "趋势"}, {"1月", "2"}, {"", ""}, {"3月", "8"}},
		},
		{
			name:      "pie",
			xml:       `<c:chartSpace xmlns:c="urn:chart"><c:chart><c:plotArea><c:pieChart><c:ser><c:tx><c:v>占比</c:v></c:tx><c:cat><c:strLit><c:pt idx="0"><c:v>A</c:v></c:pt><c:pt idx="1"><c:v>B</c:v></c:pt></c:strLit></c:cat><c:val><c:numLit><c:pt idx="0"><c:v>40</c:v></c:pt><c:pt idx="1"><c:v>60</c:v></c:pt></c:numLit></c:val></c:ser></c:pieChart></c:plotArea></c:chart></c:chartSpace>`,
			className: "pie_chart",
			want:      [][]string{{"类别", "占比"}, {"A", "40"}, {"B", "60"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			meta, ok := parseOOXMLChart([]byte(tt.xml))
			if !ok || meta.Classification == nil || len(meta.Classification.Predictions) != 1 {
				t.Fatalf("parse chart failed: ok=%v meta=%+v", ok, meta)
			}
			if got := meta.Classification.Predictions[0].ClassName; got != tt.className {
				t.Fatalf("class = %q, want %q", got, tt.className)
			}
			assertChartTable(t, meta.TabularChart.ChartData, tt.want)
		})
	}
}

// TestParseOOXMLChartCombo 验证组合图不会只保留第一个图表容器，结构化数据
// 包含全部系列，并在语义 SVG 中同时绘制柱形和折线。
func TestParseOOXMLChartCombo(t *testing.T) {
	chart := []byte(`<c:chartSpace xmlns:c="urn:chart"><c:chart><c:plotArea>` +
		`<c:barChart><c:ser><c:tx><c:v>销量</c:v></c:tx><c:cat><c:strLit>` +
		`<c:pt idx="0"><c:v>一月</c:v></c:pt><c:pt idx="1"><c:v>二月</c:v></c:pt>` +
		`</c:strLit></c:cat><c:val><c:numLit><c:pt idx="0"><c:v>10</c:v></c:pt>` +
		`<c:pt idx="1"><c:v>20</c:v></c:pt></c:numLit></c:val></c:ser></c:barChart>` +
		`<c:lineChart><c:ser><c:tx><c:v>利润</c:v></c:tx><c:cat><c:strLit>` +
		`<c:pt idx="0"><c:v>一月</c:v></c:pt><c:pt idx="1"><c:v>二月</c:v></c:pt>` +
		`</c:strLit></c:cat><c:val><c:numLit><c:pt idx="0"><c:v>3</c:v></c:pt>` +
		`<c:pt idx="1"><c:v>7</c:v></c:pt></c:numLit></c:val></c:ser></c:lineChart>` +
		`</c:plotArea></c:chart></c:chartSpace>`)
	meta, ok := parseOOXMLChart(chart)
	if !ok || meta.Classification == nil || len(meta.Classification.Predictions) != 1 ||
		meta.Classification.Predictions[0].ClassName != "combo_chart" {
		t.Fatalf("combo classification = %+v, ok=%v", meta, ok)
	}
	assertChartTable(t, meta.TabularChart.ChartData, [][]string{
		{"类别", "销量", "利润"}, {"一月", "10", "3"}, {"二月", "20", "7"},
	})
	if got := string(meta.Extra["docling__chart_series_types"]); got != `["bar_chart","line_chart"]` {
		t.Fatalf("combo series types = %s", got)
	}
	image := renderOOXMLChartSVG(meta)
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(image.URI, "data:image/svg+xml;base64,"))
	if err != nil || !bytes.Contains(raw, []byte(`<polyline`)) || !bytes.Contains(raw, []byte(`rx="2" fill="#2563eb"`)) {
		t.Fatalf("combo SVG missing bars/line: err=%v svg=%q", err, raw)
	}
}

// TestParseOOXMLChartBubble 验证气泡图的 X/Y 之外还保留第三维气泡大小：
// 结构化表格为每个系列追加“·气泡大小”列，预览 SVG 按大小绘制半径不同的
// 圆点且不连线。
func TestParseOOXMLChartBubble(t *testing.T) {
	chart := []byte(`<c:chartSpace xmlns:c="urn:chart"><c:chart><c:plotArea><c:bubbleChart>` +
		`<c:ser><c:tx><c:v>样本</c:v></c:tx>` +
		`<c:xVal><c:numLit><c:pt idx="0"><c:v>1</c:v></c:pt><c:pt idx="1"><c:v>2</c:v></c:pt></c:numLit></c:xVal>` +
		`<c:yVal><c:numLit><c:pt idx="0"><c:v>10</c:v></c:pt><c:pt idx="1"><c:v>20</c:v></c:pt></c:numLit></c:yVal>` +
		`<c:bubbleSize><c:numLit><c:pt idx="0"><c:v>5</c:v></c:pt><c:pt idx="1"><c:v>45</c:v></c:pt></c:numLit></c:bubbleSize>` +
		`</c:ser></c:bubbleChart></c:plotArea></c:chart></c:chartSpace>`)
	meta, ok := parseOOXMLChart(chart)
	if !ok || meta.Classification == nil || len(meta.Classification.Predictions) != 1 ||
		meta.Classification.Predictions[0].ClassName != "bubble_chart" {
		t.Fatalf("bubble classification = %+v, ok=%v", meta, ok)
	}
	assertChartTable(t, meta.TabularChart.ChartData, [][]string{
		{"X", "样本", "样本·气泡大小"},
		{"1", "10", "5"},
		{"2", "20", "45"},
	})
	image := renderOOXMLChartSVG(meta)
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(image.URI, "data:image/svg+xml;base64,"))
	if err != nil {
		t.Fatalf("decode bubble svg: %v", err)
	}
	svg := string(raw)
	if strings.Contains(svg, "<polyline") {
		t.Fatalf("bubble svg should not connect points: %s", svg)
	}
	radii := map[string]bool{}
	for _, match := range ooxmlChartRadiusPattern.FindAllStringSubmatch(svg, -1) {
		radii[match[1]] = true
	}
	if len(radii) < 2 {
		t.Fatalf("bubble radii should differ by size: %s", svg)
	}
}

// TestParseOOXMLChartComboScatter 验证组合图中的散点系列仍按 xVal/yVal
// 读取数据：X 值占独立的“·X”列，不因整体分类不是 scatter_chart 而丢失。
func TestParseOOXMLChartComboScatter(t *testing.T) {
	chart := []byte(`<c:chartSpace xmlns:c="urn:chart"><c:chart><c:plotArea>` +
		`<c:barChart><c:ser><c:tx><c:v>销量</c:v></c:tx><c:cat><c:strLit>` +
		`<c:pt idx="0"><c:v>一月</c:v></c:pt><c:pt idx="1"><c:v>二月</c:v></c:pt>` +
		`</c:strLit></c:cat><c:val><c:numLit><c:pt idx="0"><c:v>10</c:v></c:pt>` +
		`<c:pt idx="1"><c:v>20</c:v></c:pt></c:numLit></c:val></c:ser></c:barChart>` +
		`<c:scatterChart><c:ser><c:tx><c:v>实验点</c:v></c:tx>` +
		`<c:xVal><c:numLit><c:pt idx="0"><c:v>4</c:v></c:pt><c:pt idx="1"><c:v>8</c:v></c:pt></c:numLit></c:xVal>` +
		`<c:yVal><c:numLit><c:pt idx="0"><c:v>3</c:v></c:pt><c:pt idx="1"><c:v>7</c:v></c:pt></c:numLit></c:yVal>` +
		`</c:ser></c:scatterChart>` +
		`</c:plotArea></c:chart></c:chartSpace>`)
	meta, ok := parseOOXMLChart(chart)
	if !ok || meta.Classification == nil || len(meta.Classification.Predictions) != 1 ||
		meta.Classification.Predictions[0].ClassName != "combo_chart" {
		t.Fatalf("combo classification = %+v, ok=%v", meta, ok)
	}
	assertChartTable(t, meta.TabularChart.ChartData, [][]string{
		{"类别", "销量", "实验点·X", "实验点"},
		{"一月", "10", "4", "3"},
		{"二月", "20", "8", "7"},
	})
	if got := string(meta.Extra["docling__chart_series_types"]); got != `["bar_chart","scatter_chart"]` {
		t.Fatalf("combo series types = %s", got)
	}
}

// TestExcelizeChartFormulaResolverRanges 验证公式读取支持带空格/引号的工作表、
// 外部工作簿前缀、绝对引用、反向范围及工作簿/工作表命名区域，并拒绝
// 多区域表达式。
func TestExcelizeChartFormulaResolverRanges(t *testing.T) {
	book := excelize.NewFile()
	defer func() { _ = book.Close() }()
	if _, err := book.NewSheet("数据 表"); err != nil {
		t.Fatalf("new sheet: %v", err)
	}
	for cell, value := range map[string]any{"A1": "甲", "A2": "乙", "B1": 10, "B2": 20} {
		if err := book.SetCellValue("数据 表", cell, value); err != nil {
			t.Fatalf("set cell %s: %v", cell, err)
		}
	}
	if err := book.SetDefinedName(&excelize.DefinedName{
		Name: "命名区域", RefersTo: "'数据 表'!$A$1:$A$2",
	}); err != nil {
		t.Fatalf("set workbook defined name: %v", err)
	}
	if err := book.SetDefinedName(&excelize.DefinedName{
		Name: "局部区域", RefersTo: "'数据 表'!$B$1:$B$2", Scope: "数据 表",
	}); err != nil {
		t.Fatalf("set worksheet defined name: %v", err)
	}
	resolve := newExcelizeChartFormulaResolver(book)
	points := resolve("'[book.xlsx]数据 表'!$B$2:$A$1")
	if len(points) != 4 || points[0].Value != "甲" || points[1].Value != "10" ||
		points[2].Value != "乙" || points[3].Value != "20" {
		t.Fatalf("formula range values = %+v", points)
	}
	if points := resolve("命名区域"); len(points) != 2 || points[0].Value != "甲" || points[1].Value != "乙" {
		t.Fatalf("workbook defined name values = %+v", points)
	}
	if points := resolve("'数据 表'!局部区域"); len(points) != 2 || points[0].Value != "10" || points[1].Value != "20" {
		t.Fatalf("worksheet defined name values = %+v", points)
	}
	if points := resolve("'数据 表'!A1,A2"); points != nil {
		t.Fatalf("union range should be unsupported: %+v", points)
	}
	if points := resolve("'数据 表'!A1:A100001"); points != nil {
		t.Fatalf("oversized range should be rejected, got %d points", len(points))
	}
}

// TestExcelizeChartFormulaResolverStructuredReference 验证普通表格列结构化引用
// 只返回数据正文，不混入表头；带工作表限定符时同样可读取。
func TestExcelizeChartFormulaResolverStructuredReference(t *testing.T) {
	book := excelize.NewFile()
	defer func() { _ = book.Close() }()
	if _, err := book.NewSheet("明细"); err != nil {
		t.Fatalf("new sheet: %v", err)
	}
	for cell, value := range map[string]any{
		"C1": "月份", "D1": "销量", "C2": "1月", "D2": 12, "C3": "2月", "D3": 18,
		"C4": "合计", "D4": 30,
	} {
		if err := book.SetCellValue("明细", cell, value); err != nil {
			t.Fatalf("set cell %s: %v", cell, err)
		}
	}
	if err := book.AddTable("明细", &excelize.Table{Range: "C1:D4", Name: "销售表"}); err != nil {
		t.Fatalf("add table: %v", err)
	}
	part, loaded := book.Pkg.Load("xl/tables/table1.xml")
	if !loaded {
		t.Fatal("table definition not found")
	}
	partXML, typeOK := part.([]byte)
	if !typeOK {
		t.Fatalf("table definition type = %T", part)
	}
	partXML = bytes.Replace(partXML, []byte("<table "), []byte(`<table totalsRowCount="1" totalsRowShown="1" `), 1)
	book.Pkg.Store("xl/tables/table1.xml", partXML)

	resolve := newExcelizeChartFormulaResolver(book)
	if points := resolve("销售表[月份]"); len(points) != 2 || points[0].Value != "1月" || points[1].Value != "2月" {
		t.Fatalf("structured category values = %+v", points)
	}
	if points := resolve("'明细'!销售表[销量]"); len(points) != 2 || points[0].Value != "12" || points[1].Value != "18" {
		t.Fatalf("qualified structured values = %+v", points)
	}
	if points := resolve("销售表[[#All],[月份]]"); points != nil {
		t.Fatalf("special-item structured reference should be unsupported: %+v", points)
	}
}

// TestParseOOXMLChartCachePrecedesFormula 验证文档自带缓存优先于工作簿，避免
// 外部工作簿过期或公式无法计算时覆盖 Office 保存时的可见数据。
func TestParseOOXMLChartCachePrecedesFormula(t *testing.T) {
	chart := []byte(`<c:chartSpace xmlns:c="urn:chart"><c:chart><c:plotArea><c:lineChart><c:ser>` +
		`<c:tx><c:strRef><c:f>Sheet1!A1</c:f><c:strCache><c:pt idx="0"><c:v>缓存系列</c:v></c:pt></c:strCache></c:strRef></c:tx>` +
		`<c:cat><c:strRef><c:f>Sheet1!A2</c:f><c:strCache><c:pt idx="0"><c:v>缓存分类</c:v></c:pt></c:strCache></c:strRef></c:cat>` +
		`<c:val><c:numRef><c:f>Sheet1!B2</c:f><c:numCache><c:pt idx="0"><c:v>7</c:v></c:pt></c:numCache></c:numRef></c:val>` +
		`</c:ser></c:lineChart></c:plotArea></c:chart></c:chartSpace>`)
	resolverCalls := 0
	meta, ok := parseOOXMLChartWithResolver(chart, func(string) []ooxmlChartPoint {
		resolverCalls++
		return []ooxmlChartPoint{{Index: 0, Value: "错误回填"}}
	})
	if !ok {
		t.Fatal("parse cached chart failed")
	}
	assertChartTable(t, meta.TabularChart.ChartData, [][]string{{"类别", "缓存系列"}, {"缓存分类", "7"}})
	if resolverCalls != 0 {
		t.Fatalf("resolver called %d times for cached chart", resolverCalls)
	}
}

// TestRenderOOXMLChartSVGCommonTypes 验证常见图表家族与无数据情况都生成
// 可解码、转义安全的 SVG，确保预览不依赖桌面 Office 渲染器。
func TestRenderOOXMLChartSVGCommonTypes(t *testing.T) {
	for _, className := range []string{"bar_chart", "line_chart", "area_chart", "pie_chart", "doughnut_chart", "scatter_chart", "radar_chart", "unknown_chart"} {
		t.Run(className, func(t *testing.T) {
			meta := &PictureMeta{
				Classification: &PictureClassificationMetaField{Predictions: []PictureClassificationPrediction{{ClassName: className}}},
				TabularChart: &TabularChartMetaField{Title: `<季度&趋势>`, ChartData: &TableData{
					NumRows: 3, NumCols: 2, Orientation: TableOrientation0,
					TableCells: []DoclingTableCell{
						{StartRowOffsetIdx: 0, StartColOffsetIdx: 0, Text: "类别"},
						{StartRowOffsetIdx: 0, StartColOffsetIdx: 1, Text: "销量"},
						{StartRowOffsetIdx: 1, StartColOffsetIdx: 0, Text: "1"},
						{StartRowOffsetIdx: 1, StartColOffsetIdx: 1, Text: "-2"},
						{StartRowOffsetIdx: 2, StartColOffsetIdx: 0, Text: "2"},
						{StartRowOffsetIdx: 2, StartColOffsetIdx: 1, Text: "8"},
					},
				}},
			}
			image := renderOOXMLChartSVG(meta)
			if image == nil || !strings.HasPrefix(image.URI, "data:image/svg+xml;base64,") {
				t.Fatalf("render %s failed: %+v", className, image)
			}
			raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(image.URI, "data:image/svg+xml;base64,"))
			if err != nil || !strings.Contains(string(raw), "&lt;季度&amp;趋势&gt;") || !strings.HasSuffix(string(raw), "</svg>") {
				t.Fatalf("invalid SVG for %s: err=%v svg=%q", className, err, raw)
			}
		})
	}
	empty := renderOOXMLChartSVG(&PictureMeta{TabularChart: &TabularChartMetaField{ChartData: &TableData{}}})
	if empty == nil {
		t.Fatal("empty chart should retain preview placeholder")
	}
}

// assertChartTable 按行列断言表格化图表数据。
func assertChartTable(t *testing.T, data *TableData, want [][]string) {
	t.Helper()
	if data == nil || data.NumRows != int64(len(want)) || data.NumCols != int64(len(want[0])) {
		t.Fatalf("chart table size wrong: %+v", data)
	}
	got := make([][]string, data.NumRows)
	for row := range got {
		got[row] = make([]string, data.NumCols)
	}
	for _, cell := range data.TableCells {
		got[cell.StartRowOffsetIdx][cell.StartColOffsetIdx] = cell.Text
		if cell.StartRowOffsetIdx == 0 && !cell.ColumnHeader {
			t.Fatalf("first row cell should be column header: %+v", cell)
		}
	}
	for row := range want {
		for col := range want[row] {
			if got[row][col] != want[row][col] {
				t.Fatalf("cell[%d,%d] = %q, want %q (all=%v)", row, col, got[row][col], want[row][col], got)
			}
		}
	}
}

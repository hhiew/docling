// ooxml_chart.go 实现 DOCX、PPTX、XLSX 共用的 OOXML 图表解析：从
// chart*.xml 的内嵌缓存读取图表类型、标题、分类轴与系列数值，转换为官方
// label=picture 的 PictureMeta.classification 和 tabular_chart.chart_data。
//
// 本实现只读取 OOXML 包内已有 XML/缓存，不调用 LibreOffice 或其他外部程序；
// 公式引用没有缓存值时，由调用方提供的工作簿解析器回填数据。
package docparse

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

// ooxmlChartSeriesTypesMetaKey 保存组合图中与结构化数据列一一对应的原始
// 系列图表类型，供纯 Go 语义预览恢复柱形/折线差异。
const ooxmlChartSeriesTypesMetaKey = "docparse__chart_series_types"

// ooxmlChartClassByElement 把 DrawingML 图表元素映射为图片分类名称。
var ooxmlChartClassByElement = map[string]string{
	"areaChart":      "area_chart",
	"area3DChart":    "area_chart",
	"barChart":       "bar_chart",
	"bar3DChart":     "bar_chart",
	"bubbleChart":    "bubble_chart",
	"doughnutChart":  "doughnut_chart",
	"lineChart":      "line_chart",
	"line3DChart":    "line_chart",
	"ofPieChart":     "pie_chart",
	"pieChart":       "pie_chart",
	"pie3DChart":     "pie_chart",
	"radarChart":     "radar_chart",
	"scatterChart":   "scatter_chart",
	"stockChart":     "stock_chart",
	"surfaceChart":   "surface_chart",
	"surface3DChart": "surface_chart",
}

// ooxmlChartPoint 表示缓存中的一个带显式索引的数据点。
type ooxmlChartPoint struct {
	Index int    `xml:"idx,attr"`
	Value string `xml:"v"`
}

// ooxmlChartCache 表示 strCache/numCache 或 strLit/numLit 数据点集合。
type ooxmlChartCache struct {
	Points []ooxmlChartPoint `xml:"pt"`
}

// ooxmlChartRef 表示公式引用及其随文档保存的缓存数据。
type ooxmlChartRef struct {
	Formula     string          `xml:"f"`
	StringCache ooxmlChartCache `xml:"strCache"`
	NumberCache ooxmlChartCache `xml:"numCache"`
}

// ooxmlChartFormulaResolver 根据图表公式读取工作簿单元格，返回按公式区域
// 行优先排列的数据点；解析失败时返回 nil 并保留现有缓存降级语义。
type ooxmlChartFormulaResolver func(formula string) []ooxmlChartPoint

// ooxmlChartValues 表示分类轴或数值轴的四种缓存承载方式。
type ooxmlChartValues struct {
	StringRef     ooxmlChartRef   `xml:"strRef"`
	NumberRef     ooxmlChartRef   `xml:"numRef"`
	StringLiteral ooxmlChartCache `xml:"strLit"`
	NumberLiteral ooxmlChartCache `xml:"numLit"`
}

// points 返回轴字段中第一组非空缓存点。
func (v ooxmlChartValues) points(resolver ooxmlChartFormulaResolver) []ooxmlChartPoint {
	for _, points := range [][]ooxmlChartPoint{
		v.StringRef.StringCache.Points,
		v.StringRef.NumberCache.Points,
		v.NumberRef.StringCache.Points,
		v.NumberRef.NumberCache.Points,
		v.StringLiteral.Points,
		v.NumberLiteral.Points,
	} {
		if len(points) > 0 {
			return points
		}
	}
	if resolver != nil {
		for _, formula := range []string{v.StringRef.Formula, v.NumberRef.Formula} {
			if points := resolver(strings.TrimSpace(formula)); len(points) > 0 {
				return points
			}
		}
	}
	return nil
}

// ooxmlChartSeriesTitle 表示系列名称的直接文本或引用缓存。
type ooxmlChartSeriesTitle struct {
	Value     string        `xml:"v"`
	StringRef ooxmlChartRef `xml:"strRef"`
}

// text 返回系列名称，优先使用直接值，其次取缓存中索引最小的数据点。
func (t ooxmlChartSeriesTitle) text(resolver ooxmlChartFormulaResolver) string {
	if value := strings.TrimSpace(t.Value); value != "" {
		return value
	}
	points := t.StringRef.StringCache.Points
	if len(points) == 0 {
		points = t.StringRef.NumberCache.Points
	}
	bestIndex := int(^uint(0) >> 1)
	value := ""
	for _, point := range points {
		if point.Index < bestIndex {
			bestIndex = point.Index
			value = point.Value
		}
	}
	if value == "" && resolver != nil {
		if resolved := resolver(strings.TrimSpace(t.StringRef.Formula)); len(resolved) > 0 {
			value = resolved[0].Value
		}
	}
	return strings.TrimSpace(value)
}

// ooxmlChartSeries 表示常规图表与散点/气泡图共用的系列字段。气泡系列的
// BubbleSize 承载第三维数据，缺省时与散点系列等价。
type ooxmlChartSeries struct {
	Title      ooxmlChartSeriesTitle `xml:"tx"`
	Category   ooxmlChartValues      `xml:"cat"`
	Value      ooxmlChartValues      `xml:"val"`
	XValue     ooxmlChartValues      `xml:"xVal"`
	YValue     ooxmlChartValues      `xml:"yVal"`
	BubbleSize ooxmlChartValues      `xml:"bubbleSize"`
}

// parseOOXMLChart 解析单个 chart*.xml，并构造官方 PictureMeta。识别到图表
// 元素即返回 true；XML 损坏或没有受支持图表时返回 false。
func parseOOXMLChart(data []byte) (*PictureMeta, bool) {
	return parseOOXMLChartWithResolver(data, nil)
}

// parseOOXMLChartWithResolver 解析单个 chart*.xml；resolver 用于在图表未保存
// strCache/numCache 时按 c:f 公式从所属工作簿恢复数据。
func parseOOXMLChartWithResolver(data []byte, resolver ooxmlChartFormulaResolver) (*PictureMeta, bool) {
	if len(data) == 0 {
		return nil, false
	}
	title := findOOXMLChartTitle(data)
	dec := xml.NewDecoder(bytes.NewReader(data))
	var classes []string
	var series []ooxmlChartSeries
	var seriesTypes []string
	for {
		token, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, false
		}
		start, ok := token.(xml.StartElement)
		if !ok {
			continue
		}
		className, supported := ooxmlChartClassByElement[start.Name.Local]
		if !supported {
			continue
		}
		raw, err := collectSubTree(dec, start)
		if err != nil {
			return nil, false
		}
		classes = append(classes, className)
		parsedSeries := parseOOXMLChartSeries(raw)
		series = append(series, parsedSeries...)
		for range parsedSeries {
			seriesTypes = append(seriesTypes, className)
		}
	}
	if len(classes) == 0 {
		return nil, false
	}
	className := classes[0]
	combo := false
	for _, candidate := range classes[1:] {
		if candidate != className {
			combo = true
			break
		}
	}
	if combo {
		className = "combo_chart"
	}
	confidence := 1.0
	meta := &PictureMeta{
		Classification: &PictureClassificationMetaField{Predictions: []PictureClassificationPrediction{{
			PredictionMeta: PredictionMeta{Confidence: &confidence, CreatedBy: "docparse-ooxml"},
			ClassName:      className,
		}}},
		TabularChart: &TabularChartMetaField{
			PredictionMeta: PredictionMeta{Confidence: &confidence, CreatedBy: "docparse-ooxml"},
			Title:          title,
			ChartData:      buildOOXMLChartTable(series, seriesTypes, resolver),
		},
	}
	if combo {
		if encoded, err := json.Marshal(seriesTypes); err == nil {
			meta.Extra = map[string]json.RawMessage{ooxmlChartSeriesTypesMetaKey: encoded}
		}
	}
	return meta, true
}

// addOOXMLChartPicture 把 chart XML 直接追加为 PictureItem；prov 与 parent
// 由各 Office 后端根据其锚点语义提供。
func addOOXMLChartPicture(doc *DoclingDocument, data []byte, prov []ProvenanceItem, parent *RefItem) (RefItem, bool) {
	return addOOXMLChartPictureWithResolver(doc, data, nil, prov, parent)
}

// addOOXMLChartPictureWithResolver 追加图表 PictureItem，并生成纯 Go SVG 预览。
// resolver 仅在 OOXML 缓存缺失时参与公式数据回填。
func addOOXMLChartPictureWithResolver(doc *DoclingDocument, data []byte, resolver ooxmlChartFormulaResolver, prov []ProvenanceItem, parent *RefItem) (RefItem, bool) {
	meta, ok := parseOOXMLChartWithResolver(data, resolver)
	if !ok {
		return RefItem{}, false
	}
	ref := doc.AddPicture(nil, prov, parent)
	doc.Pictures[ref.Idx].Meta = meta
	doc.Pictures[ref.Idx].Image = renderOOXMLChartSVG(meta)
	return ref, true
}

// findOOXMLChartTitle 读取第一个图表标题子树内的 DrawingML 文本。
func findOOXMLChartTitle(data []byte) string {
	dec := xml.NewDecoder(bytes.NewReader(data))
	for {
		token, err := dec.Token()
		if err != nil {
			return ""
		}
		start, ok := token.(xml.StartElement)
		if !ok || start.Name.Local != "title" {
			continue
		}
		raw, err := collectSubTree(dec, start)
		if err != nil {
			return ""
		}
		return collectOOXMLChartText(raw)
	}
}

// collectOOXMLChartText 拼接标题子树中的 a:t/c:v 文本节点。
func collectOOXMLChartText(data []byte) string {
	dec := xml.NewDecoder(bytes.NewReader(data))
	var parts []string
	collect := false
	for {
		token, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return ""
		}
		switch value := token.(type) {
		case xml.StartElement:
			collect = value.Name.Local == "t" || value.Name.Local == "v"
		case xml.EndElement:
			if value.Name.Local == "t" || value.Name.Local == "v" {
				collect = false
			}
		case xml.CharData:
			if collect && strings.TrimSpace(string(value)) != "" {
				parts = append(parts, string(value))
			}
		}
	}
	return strings.TrimSpace(strings.Join(parts, ""))
}

// parseOOXMLChartSeries 解析图表容器中按 XML 顺序排列的系列。
func parseOOXMLChartSeries(data []byte) []ooxmlChartSeries {
	dec := xml.NewDecoder(bytes.NewReader(data))
	var series []ooxmlChartSeries
	for {
		token, err := dec.Token()
		if err != nil {
			return series
		}
		start, ok := token.(xml.StartElement)
		if !ok || start.Name.Local != "ser" {
			continue
		}
		raw, err := collectSubTree(dec, start)
		if err != nil {
			return series
		}
		var item ooxmlChartSeries
		if err := xml.Unmarshal(raw, &item); err == nil {
			series = append(series, item)
		}
	}
}

// buildOOXMLChartTable 把系列缓存对齐为“分类/X + 系列”矩形表格。缓存点
// 索引允许稀疏，缺失位置保留空字符串以保持坐标稳定。
//
// seriesTypes 与 series 一一对应并决定读取方式：散点/气泡系列读 xVal/yVal，
// 其余读 cat/val。首个系列为散点/气泡时第一列视为数值 X；混合模式下散点/
// 气泡系列追加“·X”列；气泡系列始终追加“·气泡大小”列，保证第三维数据
// 不因表格形状而丢失。
func buildOOXMLChartTable(series []ooxmlChartSeries, seriesTypes []string, resolver ooxmlChartFormulaResolver) *TableData {
	if len(series) == 0 {
		return &TableData{TableCells: []DoclingTableCell{}, Orientation: TableOrientation0}
	}
	seriesType := func(index int) string {
		if index < len(seriesTypes) {
			return seriesTypes[index]
		}
		if len(seriesTypes) > 0 {
			return seriesTypes[0]
		}
		return ""
	}
	xySeries := func(index int) bool {
		switch seriesType(index) {
		case "scatter_chart", "bubble_chart":
			return true
		default:
			return false
		}
	}
	bubbleSeries := func(index int) bool { return seriesType(index) == "bubble_chart" }
	firstXY := xySeries(0)

	// 展开列布局：混合模式下的散点/气泡系列先占“·X”列，气泡系列末尾
	// 追加“·气泡大小”列；kind 标记该列的数据来源。
	type tableColumn struct {
		seriesIndex int
		kind        string // "x" | "value" | "size"
	}
	var columns []tableColumn
	for index := range series {
		if xySeries(index) && !firstXY {
			columns = append(columns, tableColumn{index, "x"})
		}
		columns = append(columns, tableColumn{index, "value"})
		if bubbleSeries(index) {
			columns = append(columns, tableColumn{index, "size"})
		}
	}

	categories := map[int]string{}
	values := make([]map[int]string, len(series))
	xValues := make([]map[int]string, len(series))
	sizes := make([]map[int]string, len(series))
	maxIndex := -1
	for seriesIndex, item := range series {
		categoryPoints := item.Category.points(resolver)
		valuePoints := item.Value.points(resolver)
		if xySeries(seriesIndex) {
			categoryPoints = item.XValue.points(resolver)
			valuePoints = item.YValue.points(resolver)
		}
		for _, point := range categoryPoints {
			if _, exists := categories[point.Index]; !exists {
				categories[point.Index] = point.Value
			}
			if point.Index > maxIndex {
				maxIndex = point.Index
			}
		}
		values[seriesIndex] = make(map[int]string, len(valuePoints))
		for _, point := range valuePoints {
			values[seriesIndex][point.Index] = point.Value
			if point.Index > maxIndex {
				maxIndex = point.Index
			}
		}
		if !firstXY && xySeries(seriesIndex) {
			// 混合模式下散点/气泡的数值 X 有独立列，不再挤占分类轴。
			xValues[seriesIndex] = map[int]string{}
			for _, point := range categoryPoints {
				xValues[seriesIndex][point.Index] = point.Value
			}
		}
		if bubbleSeries(seriesIndex) {
			sizes[seriesIndex] = map[int]string{}
			for _, point := range item.BubbleSize.points(resolver) {
				sizes[seriesIndex][point.Index] = point.Value
				if point.Index > maxIndex {
					maxIndex = point.Index
				}
			}
		}
	}
	firstColumn := "类别"
	if firstXY {
		firstColumn = "X"
	}
	numRows := int64(maxIndex + 2)
	numCols := int64(len(columns) + 1)
	cells := make([]DoclingTableCell, 0, numRows*numCols)
	for row := int64(0); row < numRows; row++ {
		cell := DoclingTableCell{
			RowSpan: 1, ColSpan: 1,
			StartRowOffsetIdx: row, EndRowOffsetIdx: row + 1,
			StartColOffsetIdx: 0, EndColOffsetIdx: 1,
			Text: firstColumn, ColumnHeader: row == 0,
		}
		if row > 0 {
			cell.Text = categories[int(row-1)]
		}
		cells = append(cells, cell)
		for columnIndex, column := range columns {
			title := series[column.seriesIndex].Title.text(resolver)
			if title == "" {
				title = fmt.Sprintf("系列 %d", column.seriesIndex+1)
			}
			text := ""
			if row == 0 {
				switch column.kind {
				case "x":
					text = title + "·X"
				case "size":
					text = title + "·气泡大小"
				default:
					text = title
				}
			} else {
				switch column.kind {
				case "x":
					text = xValues[column.seriesIndex][int(row-1)]
				case "size":
					text = sizes[column.seriesIndex][int(row-1)]
				default:
					text = values[column.seriesIndex][int(row-1)]
				}
			}
			col := int64(columnIndex + 1)
			cells = append(cells, DoclingTableCell{
				RowSpan: 1, ColSpan: 1,
				StartRowOffsetIdx: row, EndRowOffsetIdx: row + 1,
				StartColOffsetIdx: col, EndColOffsetIdx: col + 1,
				Text: text, ColumnHeader: row == 0,
			})
		}
	}
	return &TableData{TableCells: cells, NumRows: numRows, NumCols: numCols, Orientation: TableOrientation0}
}

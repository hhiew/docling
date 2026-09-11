// ooxml_chart_render.go 把结构化 OOXML 图表数据渲染为内嵌 SVG ImageRef。
// 渲染器覆盖柱状、折线、面积、饼图、圆环、散点和雷达等常见系列；未知
// 类型输出带标题与数据摘要的稳定占位图。实现仅使用 Go 标准库。
package docparse

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html"
	"math"
	"strconv"
	"strings"
)

const (
	ooxmlChartSVGWidth  = 960
	ooxmlChartSVGHeight = 540
)

// ooxmlChartSVGPalette 是图表系列的稳定配色。
var ooxmlChartSVGPalette = []string{"#2563eb", "#16a34a", "#ea580c", "#9333ea", "#0891b2", "#dc2626", "#65a30d", "#4f46e5"}

// ooxmlChartRenderSeries 是 SVG 渲染阶段使用的数值系列。xValues/sizes 承载
// 从“·X”“·气泡大小”辅助列并回的散点数值 X 与气泡大小；nil 表示缺失，
// 渲染按类目索引或固定半径安全降级。
type ooxmlChartRenderSeries struct {
	name       string
	values     []float64
	valid      []bool
	xValues    []float64
	xValid     []bool
	sizes      []float64
	sizesValid []bool
}

// renderOOXMLChartSVG 将 PictureMeta 转为官方 ImageRef。即使 chart_data
// 为空也输出可识别占位预览，避免图表在无 Office 渲染器环境中完全不可见。
func renderOOXMLChartSVG(meta *PictureMeta) *ImageRef {
	if meta == nil || meta.TabularChart == nil {
		return nil
	}
	className := "chart"
	if meta.Classification != nil && len(meta.Classification.Predictions) > 0 {
		className = meta.Classification.Predictions[0].ClassName
	}
	categories, series := ooxmlChartRenderData(meta.TabularChart.ChartData)
	series = ooxmlChartAttachAuxiliary(series)
	var body strings.Builder
	ooxmlChartSVGFrame(&body, meta.TabularChart.Title)
	switch className {
	case "bar_chart":
		ooxmlChartSVGBar(&body, categories, series)
	case "line_chart", "stock_chart", "surface_chart":
		ooxmlChartSVGLine(&body, categories, series, false)
	case "area_chart":
		ooxmlChartSVGLine(&body, categories, series, true)
	case "scatter_chart", "bubble_chart":
		ooxmlChartSVGScatter(&body, categories, series, className == "bubble_chart")
	case "pie_chart", "doughnut_chart":
		ooxmlChartSVGPie(&body, categories, series, className == "doughnut_chart")
	case "radar_chart":
		ooxmlChartSVGRadar(&body, categories, series)
	case "combo_chart":
		ooxmlChartSVGCombo(&body, categories, series, ooxmlChartSeriesTypes(meta))
	default:
		ooxmlChartSVGEmpty(&body, "图表预览")
	}
	body.WriteString(`</svg>`)
	svg := body.String()
	return &ImageRef{
		Mimetype: "image/svg+xml",
		Dpi:      96,
		Size:     &ImageSize{Width: ooxmlChartSVGWidth, Height: ooxmlChartSVGHeight},
		URI:      "data:image/svg+xml;base64," + base64.StdEncoding.EncodeToString([]byte(svg)),
	}
}

// ooxmlChartSeriesTypes 读取组合图的逐系列类型；元数据损坏时返回空切片，
// 渲染器会把全部系列按折线安全展示。
func ooxmlChartSeriesTypes(meta *PictureMeta) []string {
	if meta == nil || meta.Extra == nil {
		return nil
	}
	var types []string
	if err := json.Unmarshal(meta.Extra[ooxmlChartSeriesTypesMetaKey], &types); err != nil {
		return nil
	}
	return types
}

// ooxmlChartSVGFrame 写入 SVG 根节点、背景和标题。
func ooxmlChartSVGFrame(body *strings.Builder, title string) {
	body.WriteString(`<svg xmlns="http://www.w3.org/2000/svg" width="960" height="540" viewBox="0 0 960 540" role="img">`)
	body.WriteString(`<rect width="960" height="540" fill="#ffffff"/>`)
	if strings.TrimSpace(title) == "" {
		title = "图表"
	}
	fmt.Fprintf(body, `<text x="480" y="38" text-anchor="middle" font-family="sans-serif" font-size="22" font-weight="600" fill="#111827">%s</text>`, html.EscapeString(title))
}

// ooxmlChartRenderData 把 TableData 转为分类标签与数值系列。
func ooxmlChartRenderData(data *TableData) ([]string, []ooxmlChartRenderSeries) {
	if data == nil || data.NumRows < 2 || data.NumCols < 2 {
		return nil, nil
	}
	grid := make([][]string, data.NumRows)
	for row := range grid {
		grid[row] = make([]string, data.NumCols)
	}
	for _, cell := range data.TableCells {
		if cell.StartRowOffsetIdx >= 0 && cell.StartRowOffsetIdx < data.NumRows &&
			cell.StartColOffsetIdx >= 0 && cell.StartColOffsetIdx < data.NumCols {
			grid[cell.StartRowOffsetIdx][cell.StartColOffsetIdx] = cell.Text
		}
	}
	categories := make([]string, data.NumRows-1)
	for row := int64(1); row < data.NumRows; row++ {
		categories[row-1] = grid[row][0]
	}
	series := make([]ooxmlChartRenderSeries, 0, data.NumCols-1)
	for col := int64(1); col < data.NumCols; col++ {
		item := ooxmlChartRenderSeries{name: grid[0][col], values: make([]float64, data.NumRows-1), valid: make([]bool, data.NumRows-1)}
		for row := int64(1); row < data.NumRows; row++ {
			item.values[row-1], item.valid[row-1] = parseOOXMLChartNumber(grid[row][col])
		}
		series = append(series, item)
	}
	return categories, series
}

// parseOOXMLChartNumber 解析常见 Office 数值文本，千位分隔符与百分号不会
// 阻断渲染；百分数保持显示值比例，不影响系列内部相对关系。
func parseOOXMLChartNumber(value string) (float64, bool) {
	value = strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(value, ",", ""), "，", ""))
	value = strings.TrimSuffix(value, "%")
	if value == "" {
		return 0, false
	}
	number, err := strconv.ParseFloat(value, 64)
	return number, err == nil && !math.IsNaN(number) && !math.IsInf(number, 0)
}

// ooxmlChartSVGBar 渲染分组柱状图，正负值共享零基线。
func ooxmlChartSVGBar(body *strings.Builder, categories []string, series []ooxmlChartRenderSeries) {
	if len(categories) == 0 || len(series) == 0 {
		ooxmlChartSVGEmpty(body, "暂无可渲染数据")
		return
	}
	minValue, maxValue, ok := ooxmlChartValueRange(series)
	if !ok {
		ooxmlChartSVGEmpty(body, "暂无可渲染数据")
		return
	}
	minValue = math.Min(minValue, 0)
	maxValue = math.Max(maxValue, 0)
	if minValue == maxValue {
		maxValue = minValue + 1
	}
	left, top, width, height := 76.0, 74.0, 820.0, 380.0
	zeroY := top + (maxValue/(maxValue-minValue))*height
	ooxmlChartSVGAxes(body, left, top, width, height, zeroY)
	groupWidth := width / float64(len(categories))
	barWidth := math.Max(2, math.Min(42, groupWidth*0.76/float64(len(series))))
	for categoryIndex, category := range categories {
		center := left + groupWidth*(float64(categoryIndex)+0.5)
		for seriesIndex, item := range series {
			if categoryIndex >= len(item.values) || !item.valid[categoryIndex] {
				continue
			}
			valueY := top + (maxValue-item.values[categoryIndex])/(maxValue-minValue)*height
			y := math.Min(zeroY, valueY)
			h := math.Max(1, math.Abs(zeroY-valueY))
			x := center - float64(len(series))*barWidth/2 + float64(seriesIndex)*barWidth
			fmt.Fprintf(body, `<rect x="%.2f" y="%.2f" width="%.2f" height="%.2f" rx="2" fill="%s"/>`, x, y, barWidth-2, h, ooxmlChartSVGPalette[seriesIndex%len(ooxmlChartSVGPalette)])
		}
		ooxmlChartSVGLabel(body, center, top+height+22, category, "middle")
	}
	ooxmlChartSVGLegend(body, series)
}

// ooxmlChartSVGLine 渲染折线或面积图，正负值共享零基线。
func ooxmlChartSVGLine(body *strings.Builder, categories []string, series []ooxmlChartRenderSeries, area bool) {
	if len(categories) == 0 || len(series) == 0 {
		ooxmlChartSVGEmpty(body, "暂无可渲染数据")
		return
	}
	minValue, maxValue, ok := ooxmlChartValueRange(series)
	if !ok {
		ooxmlChartSVGEmpty(body, "暂无可渲染数据")
		return
	}
	if minValue == maxValue {
		minValue--
		maxValue++
	}
	left, top, width, height := 76.0, 74.0, 820.0, 380.0
	ooxmlChartSVGAxes(body, left, top, width, height, top+height)
	denominator := math.Max(1, float64(len(categories)-1))
	for seriesIndex, item := range series {
		var points []string
		for index, value := range item.values {
			if index >= len(categories) || !item.valid[index] {
				continue
			}
			x := left + width*float64(index)/denominator
			y := top + (maxValue-value)/(maxValue-minValue)*height
			points = append(points, fmt.Sprintf("%.2f,%.2f", x, y))
			fmt.Fprintf(body, `<circle cx="%.2f" cy="%.2f" r="4" fill="%s"/>`, x, y, ooxmlChartSVGPalette[seriesIndex%len(ooxmlChartSVGPalette)])
		}
		if len(points) >= 2 {
			color := ooxmlChartSVGPalette[seriesIndex%len(ooxmlChartSVGPalette)]
			if area {
				fmt.Fprintf(body, `<polygon points="%.2f,%.2f %s %.2f,%.2f" fill="%s" fill-opacity="0.18"/>`, left, top+height, strings.Join(points, " "), left+width, top+height, color)
			}
			fmt.Fprintf(body, `<polyline points="%s" fill="none" stroke="%s" stroke-width="3" stroke-linejoin="round"/>`, strings.Join(points, " "), color)
		}
	}
	for index, category := range categories {
		x := left + width*float64(index)/denominator
		ooxmlChartSVGLabel(body, x, top+height+22, category, "middle")
	}
	ooxmlChartSVGLegend(body, series)
}

// ooxmlChartAttachAuxiliary 把表格中的散点/气泡辅助列（“·X”“·气泡大小”
// 后缀）并回前一系列的数值 X 与气泡大小字段；辅助列不再作为独立系列渲染。
// 列名不符合后缀规则的表格保持原样，不影响其他图表类型。
func ooxmlChartAttachAuxiliary(series []ooxmlChartRenderSeries) []ooxmlChartRenderSeries {
	merged := make([]ooxmlChartRenderSeries, 0, len(series))
	for _, item := range series {
		switch {
		case strings.HasSuffix(item.name, "·X") && len(merged) > 0:
			merged[len(merged)-1].xValues = item.values
			merged[len(merged)-1].xValid = item.valid
		case strings.HasSuffix(item.name, "·气泡大小") && len(merged) > 0:
			merged[len(merged)-1].sizes = item.values
			merged[len(merged)-1].sizesValid = item.valid
		default:
			merged = append(merged, item)
		}
	}
	return merged
}

// ooxmlChartSVGScatter 渲染散点/气泡图：只绘制数据点不连线。X 优先使用
// “·X”辅助列的数值，缺失时退回分类列文本解析；气泡图按大小的平方根比例
// 把半径缩放到 3..20，无有效大小时保持固定半径。
func ooxmlChartSVGScatter(body *strings.Builder, categories []string, series []ooxmlChartRenderSeries, bubble bool) {
	if len(series) == 0 {
		ooxmlChartSVGEmpty(body, "暂无可渲染数据")
		return
	}
	minValue, maxValue, ok := ooxmlChartValueRange(series)
	if !ok {
		ooxmlChartSVGEmpty(body, "暂无可渲染数据")
		return
	}
	if minValue == maxValue {
		minValue--
		maxValue++
	}
	maxSize := 0.0
	for _, item := range series {
		for index, size := range item.sizes {
			if index < len(item.sizesValid) && item.sizesValid[index] && size > maxSize {
				maxSize = size
			}
		}
	}
	xMin, xMax, xFound := 0.0, 0.0, false
	for _, item := range series {
		for index, value := range item.xValues {
			if index < len(item.xValid) && !item.xValid[index] {
				continue
			}
			if !xFound || value < xMin {
				xMin = value
			}
			if !xFound || value > xMax {
				xMax = value
			}
			xFound = true
		}
	}
	left, top, width, height := 76.0, 74.0, 820.0, 380.0
	ooxmlChartSVGAxes(body, left, top, width, height, top+height)
	for seriesIndex, item := range series {
		color := ooxmlChartSVGPalette[seriesIndex%len(ooxmlChartSVGPalette)]
		for index, value := range item.values {
			if !item.valid[index] {
				continue
			}
			ratio := 0.5
			switch {
			case index < len(item.xValues) && index < len(item.xValid) && item.xValid[index]:
				if xFound && xMax > xMin {
					ratio = (item.xValues[index] - xMin) / (xMax - xMin)
				}
			case index < len(categories):
				if parsed, valid := parseOOXMLChartNumber(categories[index]); valid {
					ratio = ooxmlChartNormalizeX(categories, parsed)
				}
			}
			x := left + width*ratio
			y := top + (maxValue-value)/(maxValue-minValue)*height
			radius := 4.0
			if bubble && index < len(item.sizes) && index < len(item.sizesValid) && item.sizesValid[index] && maxSize > 0 {
				radius = 3 + 17*math.Sqrt(math.Max(0, item.sizes[index])/maxSize)
			}
			fmt.Fprintf(body, `<circle cx="%.2f" cy="%.2f" r="%.2f" fill="%s"/>`, x, y, radius, color)
		}
	}
	denominator := math.Max(1, float64(len(categories)-1))
	for index, category := range categories {
		x := left + width*float64(index)/denominator
		ooxmlChartSVGLabel(body, x, top+height+22, category, "middle")
	}
	ooxmlChartSVGLegend(body, series)
}

// ooxmlChartSVGCombo 在同一坐标系中渲染组合图的柱形与折线/面积系列。
// OOXML 次坐标轴、三维效果和主题样式不在语义预览中复刻，结构化表格仍是事实来源。
func ooxmlChartSVGCombo(body *strings.Builder, categories []string, series []ooxmlChartRenderSeries, seriesTypes []string) {
	if len(categories) == 0 || len(series) == 0 {
		ooxmlChartSVGEmpty(body, "暂无可渲染数据")
		return
	}
	minValue, maxValue, ok := ooxmlChartValueRange(series)
	if !ok {
		ooxmlChartSVGEmpty(body, "暂无可渲染数据")
		return
	}
	minValue = math.Min(minValue, 0)
	maxValue = math.Max(maxValue, 0)
	if minValue == maxValue {
		maxValue = minValue + 1
	}
	left, top, width, height := 76.0, 74.0, 820.0, 380.0
	zeroY := top + (maxValue/(maxValue-minValue))*height
	ooxmlChartSVGAxes(body, left, top, width, height, zeroY)
	groupWidth := width / float64(len(categories))
	barCount := 0
	for index := range series {
		if index < len(seriesTypes) && seriesTypes[index] == "bar_chart" {
			barCount++
		}
	}
	barWidth := math.Max(2, math.Min(42, groupWidth*0.7/math.Max(1, float64(barCount))))
	barIndex := 0
	for seriesIndex, item := range series {
		seriesType := "line_chart"
		if seriesIndex < len(seriesTypes) {
			seriesType = seriesTypes[seriesIndex]
		}
		if seriesType != "bar_chart" {
			continue
		}
		for categoryIndex, value := range item.values {
			if categoryIndex >= len(categories) || !item.valid[categoryIndex] {
				continue
			}
			center := left + groupWidth*(float64(categoryIndex)+0.5)
			valueY := top + (maxValue-value)/(maxValue-minValue)*height
			x := center - float64(barCount)*barWidth/2 + float64(barIndex)*barWidth
			fmt.Fprintf(body, `<rect x="%.2f" y="%.2f" width="%.2f" height="%.2f" rx="2" fill="%s"/>`, x, math.Min(zeroY, valueY), barWidth-2, math.Max(1, math.Abs(zeroY-valueY)), ooxmlChartSVGPalette[seriesIndex%len(ooxmlChartSVGPalette)])
		}
		barIndex++
	}
	denominator := math.Max(1, float64(len(categories)-1))
	for seriesIndex, item := range series {
		seriesType := "line_chart"
		if seriesIndex < len(seriesTypes) {
			seriesType = seriesTypes[seriesIndex]
		}
		if seriesType == "bar_chart" {
			continue
		}
		var points []string
		scatterSeries := seriesType == "scatter_chart" || seriesType == "bubble_chart"
		for index, value := range item.values {
			if index >= len(categories) || !item.valid[index] {
				continue
			}
			x := left + width*float64(index)/denominator
			if scatterSeries && index < len(item.xValues) && index < len(item.xValid) && item.xValid[index] {
				// 组合图中的散点系列按其数值 X 定位，保持与表格一致。
				xMin, xMax, xFound := ooxmlChartSeriesXRange(series)
				if xFound && xMax > xMin {
					x = left + width*(item.xValues[index]-xMin)/(xMax-xMin)
				}
			}
			y := top + (maxValue-value)/(maxValue-minValue)*height
			if scatterSeries {
				// 散点系列只绘制数据点，不参与折线连接。
				fmt.Fprintf(body, `<circle cx="%.2f" cy="%.2f" r="4" fill="%s"/>`, x, y, ooxmlChartSVGPalette[seriesIndex%len(ooxmlChartSVGPalette)])
				continue
			}
			points = append(points, fmt.Sprintf("%.2f,%.2f", x, y))
			fmt.Fprintf(body, `<circle cx="%.2f" cy="%.2f" r="4" fill="%s"/>`, x, y, ooxmlChartSVGPalette[seriesIndex%len(ooxmlChartSVGPalette)])
		}
		if len(points) >= 2 {
			color := ooxmlChartSVGPalette[seriesIndex%len(ooxmlChartSVGPalette)]
			if seriesType == "area_chart" {
				fmt.Fprintf(body, `<polygon points="%.2f,%.2f %s %.2f,%.2f" fill="%s" fill-opacity="0.18"/>`, left, zeroY, strings.Join(points, " "), left+width, zeroY, color)
			}
			fmt.Fprintf(body, `<polyline points="%s" fill="none" stroke="%s" stroke-width="3" stroke-linejoin="round"/>`, strings.Join(points, " "), color)
		}
	}
	for index, category := range categories {
		center := left + groupWidth*(float64(index)+0.5)
		ooxmlChartSVGLabel(body, center, top+height+22, category, "middle")
	}
	ooxmlChartSVGLegend(body, series)
}

// ooxmlChartSeriesXRange 汇总各系列“·X”辅助列的数值范围，供组合图中散点
// 系列的数值 X 定位使用。
func ooxmlChartSeriesXRange(series []ooxmlChartRenderSeries) (minValue, maxValue float64, found bool) {
	for _, item := range series {
		for index, value := range item.xValues {
			if index < len(item.xValid) && !item.xValid[index] {
				continue
			}
			if !found || value < minValue {
				minValue = value
			}
			if !found || value > maxValue {
				maxValue = value
			}
			found = true
		}
	}
	return minValue, maxValue, found
}

// ooxmlChartSVGPie 渲染首个有效系列的饼图或圆环图。
func ooxmlChartSVGPie(body *strings.Builder, categories []string, series []ooxmlChartRenderSeries, doughnut bool) {
	if len(series) == 0 {
		ooxmlChartSVGEmpty(body, "暂无可渲染数据")
		return
	}
	values := series[0]
	total := 0.0
	for index, value := range values.values {
		if index < len(values.valid) && values.valid[index] && value > 0 {
			total += value
		}
	}
	if total <= 0 {
		ooxmlChartSVGEmpty(body, "暂无可渲染数据")
		return
	}
	cx, cy, radius, angle := 370.0, 280.0, 175.0, -math.Pi/2
	for index, value := range values.values {
		if index >= len(values.valid) || !values.valid[index] || value <= 0 {
			continue
		}
		next := angle + 2*math.Pi*value/total
		color := ooxmlChartSVGPalette[index%len(ooxmlChartSVGPalette)]
		if value == total {
			fmt.Fprintf(body, `<circle cx="%.2f" cy="%.2f" r="%.2f" fill="%s"/>`, cx, cy, radius, color)
		} else {
			x1, y1 := cx+radius*math.Cos(angle), cy+radius*math.Sin(angle)
			x2, y2 := cx+radius*math.Cos(next), cy+radius*math.Sin(next)
			largeArc := 0
			if next-angle > math.Pi {
				largeArc = 1
			}
			fmt.Fprintf(body, `<path d="M %.2f %.2f L %.2f %.2f A %.2f %.2f 0 %d 1 %.2f %.2f Z" fill="%s"/>`, cx, cy, x1, y1, radius, radius, largeArc, x2, y2, color)
		}
		label := fmt.Sprintf("项目 %d", index+1)
		if index < len(categories) && strings.TrimSpace(categories[index]) != "" {
			label = categories[index]
		}
		fmt.Fprintf(body, `<rect x="610" y="%.2f" width="14" height="14" rx="2" fill="%s"/>`, 105+float64(index)*28, color)
		ooxmlChartSVGLabel(body, 634, 117+float64(index)*28, label, "start")
		angle = next
	}
	if doughnut {
		fmt.Fprintf(body, `<circle cx="%.2f" cy="%.2f" r="82" fill="#ffffff"/>`, cx, cy)
	}
}

// ooxmlChartSVGRadar 渲染雷达图的多边形系列。
func ooxmlChartSVGRadar(body *strings.Builder, categories []string, series []ooxmlChartRenderSeries) {
	if len(categories) < 3 || len(series) == 0 {
		ooxmlChartSVGLine(body, categories, series, false)
		return
	}
	_, maxValue, ok := ooxmlChartValueRange(series)
	if !ok || maxValue <= 0 {
		ooxmlChartSVGEmpty(body, "暂无可渲染数据")
		return
	}
	cx, cy, radius := 430.0, 275.0, 175.0
	for ring := 1; ring <= 4; ring++ {
		var points []string
		for index := range categories {
			angle := -math.Pi/2 + 2*math.Pi*float64(index)/float64(len(categories))
			r := radius * float64(ring) / 4
			points = append(points, fmt.Sprintf("%.2f,%.2f", cx+r*math.Cos(angle), cy+r*math.Sin(angle)))
		}
		fmt.Fprintf(body, `<polygon points="%s" fill="none" stroke="#d1d5db"/>`, strings.Join(points, " "))
	}
	for seriesIndex, item := range series {
		var points []string
		for index := range categories {
			value := 0.0
			if index < len(item.values) && item.valid[index] {
				value = math.Max(0, item.values[index])
			}
			angle := -math.Pi/2 + 2*math.Pi*float64(index)/float64(len(categories))
			r := radius * value / maxValue
			points = append(points, fmt.Sprintf("%.2f,%.2f", cx+r*math.Cos(angle), cy+r*math.Sin(angle)))
		}
		color := ooxmlChartSVGPalette[seriesIndex%len(ooxmlChartSVGPalette)]
		fmt.Fprintf(body, `<polygon points="%s" fill="%s" fill-opacity="0.15" stroke="%s" stroke-width="2"/>`, strings.Join(points, " "), color, color)
	}
	ooxmlChartSVGLegend(body, series)
}

// ooxmlChartValueRange 返回全部有效系列的最小值与最大值。
func ooxmlChartValueRange(series []ooxmlChartRenderSeries) (float64, float64, bool) {
	minValue, maxValue, found := 0.0, 0.0, false
	for _, item := range series {
		for index, value := range item.values {
			if index >= len(item.valid) || !item.valid[index] {
				continue
			}
			if !found || value < minValue {
				minValue = value
			}
			if !found || value > maxValue {
				maxValue = value
			}
			found = true
		}
	}
	return minValue, maxValue, found
}

// ooxmlChartNormalizeX 把散点图分类列中的数值映射到 0..1。
func ooxmlChartNormalizeX(categories []string, value float64) float64 {
	minValue, maxValue, found := 0.0, 0.0, false
	for _, category := range categories {
		parsed, valid := parseOOXMLChartNumber(category)
		if !valid {
			continue
		}
		if !found || parsed < minValue {
			minValue = parsed
		}
		if !found || parsed > maxValue {
			maxValue = parsed
		}
		found = true
	}
	if !found || minValue == maxValue {
		return 0.5
	}
	return (value - minValue) / (maxValue - minValue)
}

// ooxmlChartSVGAxes 绘制浅色网格、横轴和纵轴。
func ooxmlChartSVGAxes(body *strings.Builder, left, top, width, height, zeroY float64) {
	for index := 0; index <= 4; index++ {
		y := top + height*float64(index)/4
		fmt.Fprintf(body, `<line x1="%.2f" y1="%.2f" x2="%.2f" y2="%.2f" stroke="#e5e7eb"/>`, left, y, left+width, y)
	}
	fmt.Fprintf(body, `<line x1="%.2f" y1="%.2f" x2="%.2f" y2="%.2f" stroke="#6b7280" stroke-width="1.5"/>`, left, top, left, top+height)
	fmt.Fprintf(body, `<line x1="%.2f" y1="%.2f" x2="%.2f" y2="%.2f" stroke="#6b7280" stroke-width="1.5"/>`, left, zeroY, left+width, zeroY)
}

// ooxmlChartSVGLegend 在底部输出系列名称。
func ooxmlChartSVGLegend(body *strings.Builder, series []ooxmlChartRenderSeries) {
	for index, item := range series {
		x := 78.0 + float64(index%5)*168
		y := 510.0 + float64(index/5)*18
		fmt.Fprintf(body, `<rect x="%.2f" y="%.2f" width="12" height="12" rx="2" fill="%s"/>`, x, y-10, ooxmlChartSVGPalette[index%len(ooxmlChartSVGPalette)])
		name := item.name
		if strings.TrimSpace(name) == "" {
			name = fmt.Sprintf("系列 %d", index+1)
		}
		ooxmlChartSVGLabel(body, x+18, y, name, "start")
	}
}

// ooxmlChartSVGLabel 输出受长度保护的 SVG 文本。
func ooxmlChartSVGLabel(body *strings.Builder, x, y float64, value, anchor string) {
	runes := []rune(strings.TrimSpace(value))
	if len(runes) > 14 {
		value = string(runes[:13]) + "…"
	}
	fmt.Fprintf(body, `<text x="%.2f" y="%.2f" text-anchor="%s" font-family="sans-serif" font-size="12" fill="#374151">%s</text>`, x, y, anchor, html.EscapeString(value))
}

// ooxmlChartSVGEmpty 输出无结构化数据或未知类型时的可见占位。
func ooxmlChartSVGEmpty(body *strings.Builder, message string) {
	body.WriteString(`<rect x="120" y="110" width="720" height="330" rx="12" fill="#f9fafb" stroke="#d1d5db" stroke-dasharray="8 6"/>`)
	fmt.Fprintf(body, `<text x="480" y="282" text-anchor="middle" font-family="sans-serif" font-size="18" fill="#6b7280">%s</text>`, html.EscapeString(message))
}

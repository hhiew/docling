// pdf_layout.go 基于行级边界框实现保守的 PDF XY-cut 阅读顺序恢复。
// 算法只在页面存在稳定主空白带时切分左右栏；跨越空白带的宽行作为标题、
// 说明等横向锚点，先按锚点划分上下区域，再在各区域内按左到右递归读取。
package docling

import (
	"math"
	"sort"
)

const (
	// pdfLayoutMinColumnLines 要求空白带两侧至少各有两行，避免单个缩进行误触发切栏。
	pdfLayoutMinColumnLines = 2
	// pdfLayoutMinGapPoints 是主空白带的绝对最小宽度（PDF point）。
	pdfLayoutMinGapPoints = 18.0
	// pdfLayoutMinGapRatio 是主空白带相对当前区域宽度的最小比例。
	pdfLayoutMinGapRatio = 0.04
	// pdfLayoutMinColumnWidthRatio 是左右栏相对当前区域宽度的最小包络比例。
	pdfLayoutMinColumnWidthRatio = 0.12
	// pdfLayoutMinVerticalOverlapRatio 要求两栏垂直范围具有足够重叠，防止顺序段落被误判为并列栏。
	pdfLayoutMinVerticalOverlapRatio = 0.20
	// pdfLayoutMaxDepth 限制异常坐标输入下的递归深度。
	pdfLayoutMaxDepth = 8
)

// pdfLayoutCut 表示当前区域内可用于左右递归切分的主垂直空白带。
type pdfLayoutCut struct {
	left    []pdfLine
	right   []pdfLine
	anchors []pdfLine
	gap     float64
	score   float64
}

// sortPDFLinesByPageXYCut 按页号升序组织全文行，并对每页应用保守 XY-cut 阅读顺序。
// 同一单栏页维持传入顺序；多栏页按跨栏锚点、左栏、右栏的语义重新排列。
func sortPDFLinesByPageXYCut(lines []pdfLine) []pdfLine {
	if len(lines) < 2 {
		return append([]pdfLine(nil), lines...)
	}

	pages := make(map[int64][]pdfLine)
	pageNumbers := make([]int64, 0)
	for _, line := range lines {
		if _, ok := pages[line.PageIdx]; !ok {
			pageNumbers = append(pageNumbers, line.PageIdx)
		}
		pages[line.PageIdx] = append(pages[line.PageIdx], line)
	}
	sort.Slice(pageNumbers, func(i, j int) bool { return pageNumbers[i] < pageNumbers[j] })

	ordered := make([]pdfLine, 0, len(lines))
	for _, pageNo := range pageNumbers {
		ordered = append(ordered, sortPDFPageLinesXYCut(pages[pageNo])...)
	}
	return ordered
}

// sortPDFPageLinesXYCut 恢复单页多栏阅读顺序；坐标无效或缺少稳定空白带时原样返回。
// pdfLine 使用 BOTTOMLEFT 坐标，因此行内阅读次序按 MaxY 从大到小判断。
func sortPDFPageLinesXYCut(lines []pdfLine) []pdfLine {
	cloned := append([]pdfLine(nil), lines...)
	if len(cloned) < pdfLayoutMinColumnLines*2 || !pdfLayoutLinesHaveBounds(cloned) {
		return cloned
	}
	return layoutPDFRegionXYCut(cloned, 0)
}

// layoutPDFRegionXYCut 递归寻找当前区域的主垂直空白带，并处理跨栏横向锚点。
func layoutPDFRegionXYCut(lines []pdfLine, depth int) []pdfLine {
	if len(lines) < pdfLayoutMinColumnLines*2 || depth >= pdfLayoutMaxDepth {
		return append([]pdfLine(nil), lines...)
	}
	cut, ok := findPDFLayoutMainCut(lines)
	if !ok {
		return append([]pdfLine(nil), lines...)
	}

	left := pdfLayoutSortTopDown(cut.left)
	right := pdfLayoutSortTopDown(cut.right)
	if len(cut.anchors) == 0 {
		ordered := layoutPDFRegionXYCut(left, depth+1)
		return append(ordered, layoutPDFRegionXYCut(right, depth+1)...)
	}

	anchors := pdfLayoutSortTopDown(cut.anchors)
	ordered := make([]pdfLine, 0, len(lines))
	remainingLeft, remainingRight := left, right
	for _, anchor := range anchors {
		anchorY := pdfLayoutLineCenterY(anchor)
		leftAbove, leftBelow := pdfLayoutSplitAbove(remainingLeft, anchorY)
		rightAbove, rightBelow := pdfLayoutSplitAbove(remainingRight, anchorY)
		ordered = append(ordered, layoutPDFRegionXYCut(leftAbove, depth+1)...)
		ordered = append(ordered, layoutPDFRegionXYCut(rightAbove, depth+1)...)
		ordered = append(ordered, anchor)
		remainingLeft, remainingRight = leftBelow, rightBelow
	}
	ordered = append(ordered, layoutPDFRegionXYCut(remainingLeft, depth+1)...)
	ordered = append(ordered, layoutPDFRegionXYCut(remainingRight, depth+1)...)
	return ordered
}

// findPDFLayoutMainCut 从全部 X 边界之间选择得分最高的稳定主空白带。
func findPDFLayoutMainCut(lines []pdfLine) (pdfLayoutCut, bool) {
	minX, maxX := lines[0].MinX, lines[0].MaxX
	fontSizes := make([]float64, 0, len(lines))
	edges := make([]float64, 0, len(lines)*2)
	for _, line := range lines {
		minX = math.Min(minX, line.MinX)
		maxX = math.Max(maxX, line.MaxX)
		edges = append(edges, line.MinX, line.MaxX)
		if line.MaxFontSize > 0 {
			fontSizes = append(fontSizes, line.MaxFontSize)
		}
	}
	sort.Float64s(edges)
	regionWidth := maxX - minX
	medianFont := pdfLayoutMedian(fontSizes, 10)
	minGap := math.Max(pdfLayoutMinGapPoints, math.Max(regionWidth*pdfLayoutMinGapRatio, medianFont*1.5))

	best := pdfLayoutCut{}
	found := false
	for i := 1; i < len(edges); i++ {
		if edges[i]-edges[i-1] < minGap {
			continue
		}
		cutX := (edges[i] + edges[i-1]) / 2
		candidate, ok := evaluatePDFLayoutCut(lines, cutX, regionWidth, minGap, medianFont)
		if !ok || found && candidate.score <= best.score {
			continue
		}
		best, found = candidate, true
	}
	return best, found
}

// evaluatePDFLayoutCut 校验一个候选切点是否形成双侧多行、垂直重叠且宽度充足的空白带。
func evaluatePDFLayoutCut(lines []pdfLine, cutX, regionWidth, minGap, medianFont float64) (pdfLayoutCut, bool) {
	left := make([]pdfLine, 0, len(lines))
	right := make([]pdfLine, 0, len(lines))
	crossing := make([]pdfLine, 0)
	for _, line := range lines {
		switch {
		case line.MaxX <= cutX:
			left = append(left, line)
		case line.MinX >= cutX:
			right = append(right, line)
		default:
			crossing = append(crossing, line)
		}
	}
	if len(left) < pdfLayoutMinColumnLines || len(right) < pdfLayoutMinColumnLines {
		return pdfLayoutCut{}, false
	}

	leftMin, leftMax, leftBottom, leftTop := pdfLayoutBounds(left)
	rightMin, rightMax, rightBottom, rightTop := pdfLayoutBounds(right)
	gap := rightMin - leftMax
	minColumnWidth := math.Max(36, regionWidth*pdfLayoutMinColumnWidthRatio)
	if gap < minGap || leftMax-leftMin < minColumnWidth || rightMax-rightMin < minColumnWidth {
		return pdfLayoutCut{}, false
	}

	leftSpan := leftTop - leftBottom
	rightSpan := rightTop - rightBottom
	overlap := math.Min(leftTop, rightTop) - math.Max(leftBottom, rightBottom)
	minSpan := math.Min(leftSpan, rightSpan)
	if overlap <= 0 || minSpan <= 0 || overlap < minSpan*pdfLayoutMinVerticalOverlapRatio {
		return pdfLayoutCut{}, false
	}
	pairTolerance := math.Max(medianFont*4, minSpan*0.12)
	if pdfLayoutPairedRows(left, right, pairTolerance) < pdfLayoutMinColumnLines {
		return pdfLayoutCut{}, false
	}

	anchors := make([]pdfLine, 0, len(crossing))
	for _, line := range crossing {
		if !pdfLayoutIsSpanningAnchor(line, leftMin, leftMax, rightMin, rightMax) {
			return pdfLayoutCut{}, false
		}
		anchors = append(anchors, line)
	}
	balance := float64(min(len(left), len(right))) / float64(max(len(left), len(right)))
	overlapRatio := math.Min(1, overlap/minSpan)
	return pdfLayoutCut{
		left:    left,
		right:   right,
		anchors: anchors,
		gap:     gap,
		score:   gap * (0.5 + 0.5*balance) * (0.5 + 0.5*overlapRatio),
	}, true
}

// pdfLayoutIsSpanningAnchor 判断穿过栏间空白带的行是否同时覆盖左右栏主体宽度。
func pdfLayoutIsSpanningAnchor(line pdfLine, leftMin, leftMax, rightMin, rightMax float64) bool {
	leftReach := leftMin + (leftMax-leftMin)*0.25
	rightReach := rightMax - (rightMax-rightMin)*0.25
	return line.MinX <= leftReach && line.MaxX >= rightReach
}

// pdfLayoutPairedRows 统计左右栏在垂直方向可一一配对的行数，用于排除上下顺序段落。
func pdfLayoutPairedRows(left, right []pdfLine, tolerance float64) int {
	leftY := pdfLayoutSortedCenters(left)
	rightY := pdfLayoutSortedCenters(right)
	paired := 0
	for i, j := 0, 0; i < len(leftY) && j < len(rightY); {
		delta := leftY[i] - rightY[j]
		switch {
		case math.Abs(delta) <= tolerance:
			paired++
			i++
			j++
		case delta > 0:
			i++
		default:
			j++
		}
	}
	return paired
}

// pdfLayoutSortedCenters 返回按页面从上到下排列的行中心 Y 坐标。
func pdfLayoutSortedCenters(lines []pdfLine) []float64 {
	centers := make([]float64, 0, len(lines))
	for _, line := range lines {
		centers = append(centers, pdfLayoutLineCenterY(line))
	}
	sort.Sort(sort.Reverse(sort.Float64Slice(centers)))
	return centers
}

// pdfLayoutBounds 返回一组行的左右下上包络边界。
func pdfLayoutBounds(lines []pdfLine) (minX, maxX, bottom, top float64) {
	minX, maxX = lines[0].MinX, lines[0].MaxX
	bottom, top = lines[0].MinY, lines[0].MaxY
	for _, line := range lines[1:] {
		minX = math.Min(minX, line.MinX)
		maxX = math.Max(maxX, line.MaxX)
		bottom = math.Min(bottom, line.MinY)
		top = math.Max(top, line.MaxY)
	}
	return minX, maxX, bottom, top
}

// pdfLayoutLinesHaveBounds 判断行集合是否都具有可参与水平切分的有效 X 边界。
func pdfLayoutLinesHaveBounds(lines []pdfLine) bool {
	for _, line := range lines {
		if line.MaxX <= line.MinX || math.IsNaN(line.MinX) || math.IsNaN(line.MaxX) ||
			math.IsNaN(line.MinY) || math.IsNaN(line.MaxY) {
			return false
		}
	}
	return true
}

// pdfLayoutSortTopDown 稳定地按 BOTTOMLEFT 坐标中的 MaxY 降序排列行。
func pdfLayoutSortTopDown(lines []pdfLine) []pdfLine {
	ordered := append([]pdfLine(nil), lines...)
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].MaxY == ordered[j].MaxY {
			return ordered[i].MinX < ordered[j].MinX
		}
		return ordered[i].MaxY > ordered[j].MaxY
	})
	return ordered
}

// pdfLayoutSplitAbove 按行中心把已从上到下排序的栏分成锚点上方与其余部分。
func pdfLayoutSplitAbove(lines []pdfLine, anchorY float64) (above, below []pdfLine) {
	for _, line := range lines {
		if pdfLayoutLineCenterY(line) > anchorY {
			above = append(above, line)
		} else {
			below = append(below, line)
		}
	}
	return above, below
}

// pdfLayoutLineCenterY 返回 BOTTOMLEFT 坐标中的行垂直中心。
func pdfLayoutLineCenterY(line pdfLine) float64 {
	return (line.MinY + line.MaxY) / 2
}

// pdfLayoutMedian 返回浮点集合中位数；空集合使用 fallback。
func pdfLayoutMedian(values []float64, fallback float64) float64 {
	if len(values) == 0 {
		return fallback
	}
	ordered := append([]float64(nil), values...)
	sort.Float64s(ordered)
	middle := len(ordered) / 2
	if len(ordered)%2 == 0 {
		return (ordered[middle-1] + ordered[middle]) / 2
	}
	return ordered[middle]
}

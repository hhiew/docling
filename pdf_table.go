// pdf_table.go 基于 PDF 词级坐标恢复保守的矩形表格网格，并从后续正文流移除
// 已消费的表格行。识别只接受至少两行两列且列起点稳定的连续区域，避免把普通
// 双栏长正文当成表格。
package docparse

import (
	"math"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	// pdfTableMinColumnGapPt 是切分相邻表格单元格的最小水平留白（pt）。
	pdfTableMinColumnGapPt = 10.0
	// pdfTableAnchorTolerancePt 是不同表格行列起点允许的最小漂移（pt）。
	pdfTableAnchorTolerancePt = 7.0
	// pdfTableMinAnchorDistancePt 是相邻列起点的最小距离，排除编号列表的
	// “序号 + 短文本”结构。
	pdfTableMinAnchorDistancePt = 36.0
	// pdfTableTwoColumnMaxRunes 限制两列表格单元格长度，避免稳定双栏正文误判。
	pdfTableTwoColumnMaxRunes = 32
	// pdfContinuationBottomRatio 要求前一段表格进入页面底部区域。
	pdfContinuationBottomRatio = 0.30
	// pdfContinuationTopRatio 要求后一段表格从下一页顶部区域开始。
	pdfContinuationTopRatio = 0.70
	// pdfContinuationAnchorTolerancePt 是跨页表格左右边界允许的基础漂移。
	pdfContinuationAnchorTolerancePt = 12.0
)

// pdfDetectedTable 表示一个从连续 PDF 行中恢复出的矩形表格区域。
// StartLine/EndLine 是原始 lines 的半开区间，便于调用方在版面排序后原位插入表格。
type pdfDetectedTable struct {
	PageIdx   int64            // 表格所在页（从 0 开始）
	Data      TableData        // 官方表格网格数据
	BBox      *DoclingBBox     // 全表 BOTTOMLEFT 边界框
	Lines     []pdfLine        // 被表格消费的原始行
	Prov      []ProvenanceItem // Prov 保存跨页续表每一页的独立来源框。
	StartLine int              // 原始行半开区间起点
	EndLine   int              // 原始行半开区间终点
}

// mergePDFContinuationTables 合并翻页边界两侧具有相同表头和列锚的表格。
// 仅允许家具行位于两段之间；重复表头从后一段结构中移除，但仍保留其页面
// provenance。规则保持保守，避免把相邻页面上两个独立同构表格误合并。
func mergePDFContinuationTables(lines []pdfLine, pages map[string]PageItem) []pdfLine {
	if len(lines) < 2 {
		return lines
	}
	result := make([]pdfLine, 0, len(lines))
	for index := 0; index < len(lines); {
		line := lines[index]
		if line.Table == nil {
			result = append(result, line)
			index++
			continue
		}

		mergedLine := line
		mergedTable := clonePDFDetectedTable(line.Table)
		mergedLine.Table = &mergedTable
		tail := line.Table
		cursor := index + 1
		var furniture []pdfLine
		mergedAny := false
		for {
			candidateIndex := cursor
			for candidateIndex < len(lines) && lines[candidateIndex].Furniture != "" {
				candidateIndex++
			}
			if candidateIndex >= len(lines) || lines[candidateIndex].Table == nil ||
				!canMergePDFContinuationTables(tail, lines[candidateIndex].Table, pages) {
				break
			}
			furniture = append(furniture, lines[cursor:candidateIndex]...)
			mergePDFTableContinuation(&mergedTable, lines[candidateIndex].Table)
			tail = lines[candidateIndex].Table
			cursor = candidateIndex + 1
			mergedAny = true
		}
		result = append(result, mergedLine)
		if mergedAny {
			result = append(result, furniture...)
			index = cursor
			continue
		}
		index++
	}
	return result
}

// clonePDFDetectedTable 复制可变网格切片，避免续表合并污染调用方输入。
func clonePDFDetectedTable(table *pdfDetectedTable) pdfDetectedTable {
	if table == nil {
		return pdfDetectedTable{}
	}
	clone := *table
	clone.Data.TableCells = append([]DoclingTableCell(nil), table.Data.TableCells...)
	clone.Lines = append([]pdfLine(nil), table.Lines...)
	clone.Prov = append([]ProvenanceItem(nil), table.Prov...)
	return clone
}

// canMergePDFContinuationTables 校验页码、翻页位置、列数、方向、水平锚点
// 和重复表头；任一证据不足即保持两张独立表格。
func canMergePDFContinuationTables(first, second *pdfDetectedTable, pages map[string]PageItem) bool {
	if first == nil || second == nil || first.BBox == nil || second.BBox == nil ||
		second.PageIdx != first.PageIdx+1 || first.Data.NumCols < 2 ||
		first.Data.NumCols != second.Data.NumCols || first.Data.Orientation != second.Data.Orientation {
		return false
	}
	firstPage, firstOK := pages[strconv.FormatInt(first.PageIdx+1, 10)]
	secondPage, secondOK := pages[strconv.FormatInt(second.PageIdx+1, 10)]
	if !firstOK || !secondOK || firstPage.Size == nil || secondPage.Size == nil ||
		firstPage.Size.Height <= 0 || secondPage.Size.Height <= 0 {
		return false
	}
	if first.BBox.B > firstPage.Size.Height*pdfContinuationBottomRatio ||
		second.BBox.T < secondPage.Size.Height*pdfContinuationTopRatio {
		return false
	}
	width := math.Max(first.BBox.R-first.BBox.L, second.BBox.R-second.BBox.L)
	tolerance := math.Max(pdfContinuationAnchorTolerancePt, width*0.03)
	if math.Abs(first.BBox.L-second.BBox.L) > tolerance || math.Abs(first.BBox.R-second.BBox.R) > tolerance {
		return false
	}
	firstHeader, firstOK := pdfTableHeaderTexts(first.Data)
	secondHeader, secondOK := pdfTableHeaderTexts(second.Data)
	if !firstOK || !secondOK || len(firstHeader) != len(secondHeader) {
		return false
	}
	for index := range firstHeader {
		if firstHeader[index] != secondHeader[index] {
			return false
		}
	}
	return true
}

// pdfTableHeaderTexts 返回首行按网格展开并归一化后的表头；跨数据行的复杂
// 表头不参与自动续表，交由结构化视觉模型处理。
func pdfTableHeaderTexts(data TableData) ([]string, bool) {
	if data.NumRows < 2 || data.NumCols < 2 {
		return nil, false
	}
	header := make([]string, data.NumCols)
	nonEmpty := false
	for _, cell := range data.TableCells {
		if !cell.ColumnHeader || cell.StartRowOffsetIdx != 0 || cell.EndRowOffsetIdx != 1 {
			continue
		}
		text := strings.ToLower(strings.Join(strings.Fields(cell.Text), ""))
		for column := cell.StartColOffsetIdx; column < cell.EndColOffsetIdx && column < data.NumCols; column++ {
			if column < 0 || header[column] != "" {
				return nil, false
			}
			header[column] = text
		}
		if text != "" {
			nonEmpty = true
		}
	}
	if !nonEmpty {
		return nil, false
	}
	for _, text := range header {
		if text == "" {
			return nil, false
		}
	}
	return header, true
}

// mergePDFTableContinuation 把后一页数据行接到首段之后，并保存两段的独立
// 页面来源；第二段首行是已经校验相同的重复表头，因此不重复写入网格。
func mergePDFTableContinuation(first *pdfDetectedTable, second *pdfDetectedTable) {
	if first == nil || second == nil {
		return
	}
	if len(first.Prov) == 0 {
		first.Prov = pdfDetectedTableProvenance(first)
	}
	first.Prov = append(first.Prov, pdfDetectedTableProvenance(second)...)
	rowShift := first.Data.NumRows - 1
	for _, cell := range second.Data.TableCells {
		if cell.EndRowOffsetIdx <= 1 {
			continue
		}
		cell.StartRowOffsetIdx += rowShift
		cell.EndRowOffsetIdx += rowShift
		cell.ColumnHeader = false
		first.Data.TableCells = append(first.Data.TableCells, cell)
	}
	first.Data.NumRows += second.Data.NumRows - 1
	first.Lines = append(first.Lines, second.Lines...)
}

// pdfDetectedTableProvenance 按页聚合表格原始行的来源框；视觉表格没有原始
// 行时回退到表格 bbox 和 PageIdx。
func pdfDetectedTableProvenance(table *pdfDetectedTable) []ProvenanceItem {
	if table == nil {
		return nil
	}
	if len(table.Prov) > 0 {
		return append([]ProvenanceItem(nil), table.Prov...)
	}
	byPage := map[int64]*DoclingBBox{}
	pageOrder := make([]int64, 0)
	for _, line := range table.Lines {
		bbox := &DoclingBBox{L: line.MinX, B: line.MinY, R: line.MaxX, T: line.MaxY, CoordOrigin: CoordOriginBottomLeft}
		if current, ok := byPage[line.PageIdx]; ok {
			byPage[line.PageIdx] = unionPDFTableBBox(current, bbox)
			continue
		}
		byPage[line.PageIdx] = bbox
		pageOrder = append(pageOrder, line.PageIdx)
	}
	if len(pageOrder) == 0 && table.BBox != nil {
		byPage[table.PageIdx] = table.BBox
		pageOrder = append(pageOrder, table.PageIdx)
	}
	provenance := make([]ProvenanceItem, 0, len(pageOrder))
	for _, pageIdx := range pageOrder {
		provenance = append(provenance, ProvenanceItem{
			PageNo: pageIdx + 1, BBox: byPage[pageIdx], CharSpan: [2]int64{0, 0},
		})
	}
	return provenance
}

// pdfTableCellCandidate 表示单行内由大水平留白切分出的候选单元格。
type pdfTableCellCandidate struct {
	Text     string  // 单元格纯文本
	MinX     float64 // 左边缘（pt）
	MaxX     float64 // 右边缘（pt）
	MinY     float64 // BOTTOMLEFT 下边缘（pt）
	MaxY     float64 // BOTTOMLEFT 上边缘（pt）
	FontSize float64 // 单元格内最大字号（pt）
}

// pdfTableRowCandidate 表示列数和各列起点待与相邻行校验的候选表格行。
type pdfTableRowCandidate struct {
	lineIndex int                     // 原始行下标
	line      pdfLine                 // 原始坐标行
	cells     []pdfTableCellCandidate // 按大留白切分的单元格
}

// pdfTableAnchorCluster 保存稀疏表格列起点聚类。
type pdfTableAnchorCluster struct {
	x     float64
	count int
}

// detectPDFTables 识别连续、同页、列起点稳定的矩形表格区域，并返回未被表格
// 消费的正文行。返回表格顺序与输入顺序一致，remaining 也保持原始相对顺序。
func detectPDFTables(lines []pdfLine) ([]pdfDetectedTable, []pdfLine) {
	if len(lines) == 0 {
		return nil, nil
	}

	tables := make([]pdfDetectedTable, 0)
	consumed := make([]bool, len(lines))
	var run []pdfTableRowCandidate

	flush := func() {
		if table, ok := buildPDFDetectedTable(run); ok {
			tables = append(tables, table)
			for _, row := range run {
				consumed[row.lineIndex] = true
			}
		}
		run = nil
	}

	for index, line := range lines {
		cells := splitPDFTableRow(line)
		if len(cells) < 2 {
			flush()
			continue
		}
		row := pdfTableRowCandidate{lineIndex: index, line: line, cells: cells}
		if len(run) > 0 && !pdfTableRowsCompatible(run[len(run)-1], row) {
			flush()
		}
		run = append(run, row)
	}
	flush()

	remaining := make([]pdfLine, 0, len(lines))
	for index, line := range lines {
		if !consumed[index] {
			remaining = append(remaining, line)
		}
	}
	return tables, remaining
}

// detectPDFSparseTables 恢复列锚稳定但部分行缺少单元格的无边框表格。
// 该路径只接受至少三行且至少三列的区域，作为严格矩形检测失败后的补充，
// 避免把普通双栏正文误判为表格。
func detectPDFSparseTables(lines []pdfLine) ([]pdfDetectedTable, []pdfLine) {
	if len(lines) < 3 {
		return nil, append([]pdfLine(nil), lines...)
	}
	var tables []pdfDetectedTable
	consumed := make([]bool, len(lines))
	var run []pdfTableRowCandidate
	flush := func() {
		if table, ok := buildPDFSparseTable(run); ok {
			tables = append(tables, table)
			for _, row := range run {
				consumed[row.lineIndex] = true
			}
		}
		run = nil
	}
	for index, line := range lines {
		cells := splitPDFTableRow(line)
		if len(cells) < 2 {
			flush()
			continue
		}
		row := pdfTableRowCandidate{lineIndex: index, line: line, cells: cells}
		if len(run) > 0 {
			previous := run[len(run)-1].line
			fontSize := math.Max(previous.MaxFontSize, line.MaxFontSize)
			if previous.PageIdx != line.PageIdx || previous.MinY-line.MaxY > math.Max(36, fontSize*4) {
				flush()
			}
		}
		run = append(run, row)
	}
	flush()
	remaining := make([]pdfLine, 0, len(lines))
	for index, line := range lines {
		if !consumed[index] {
			remaining = append(remaining, line)
		}
	}
	return tables, remaining
}

// buildPDFSparseTable 基于全区域 X 起点聚类生成允许空单元格的矩形网格。
func buildPDFSparseTable(rows []pdfTableRowCandidate) (pdfDetectedTable, bool) {
	if len(rows) < 3 || !pdfSparseTableIsCompact(rows) {
		return pdfDetectedTable{}, false
	}
	fontSize := 0.0
	for _, row := range rows {
		fontSize = math.Max(fontSize, row.line.MaxFontSize)
	}
	tolerance := math.Max(pdfTableAnchorTolerancePt, fontSize*0.8)
	var clusters []pdfTableAnchorCluster
	for _, row := range rows {
		for _, cell := range row.cells {
			best := -1
			bestDistance := math.MaxFloat64
			for index := range clusters {
				distance := math.Abs(clusters[index].x - cell.MinX)
				if distance <= tolerance && distance < bestDistance {
					best, bestDistance = index, distance
				}
			}
			if best < 0 {
				clusters = append(clusters, pdfTableAnchorCluster{x: cell.MinX, count: 1})
				continue
			}
			cluster := &clusters[best]
			cluster.x = (cluster.x*float64(cluster.count) + cell.MinX) / float64(cluster.count+1)
			cluster.count++
		}
	}
	stable := clusters[:0]
	for _, cluster := range clusters {
		if cluster.count >= 2 {
			stable = append(stable, cluster)
		}
	}
	clusters = stable
	sort.Slice(clusters, func(i, j int) bool { return clusters[i].x < clusters[j].x })
	if len(clusters) < 3 {
		return pdfDetectedTable{}, false
	}
	for index := 1; index < len(clusters); index++ {
		if clusters[index].x-clusters[index-1].x < math.Max(pdfTableMinAnchorDistancePt, fontSize*4) {
			return pdfDetectedTable{}, false
		}
	}

	header := inferPDFSparseTableHeader(rows, clusters, tolerance)
	var cells []DoclingTableCell
	var bbox *DoclingBBox
	for rowIndex, row := range rows {
		used := map[int]bool{}
		for _, cell := range row.cells {
			column := nearestPDFTableAnchor(cell.MinX, clusters, tolerance)
			if column < 0 || used[column] {
				return pdfDetectedTable{}, false
			}
			used[column] = true
			cellBBox := &DoclingBBox{L: cell.MinX, T: cell.MaxY, R: cell.MaxX, B: cell.MinY, CoordOrigin: CoordOriginBottomLeft}
			bbox = unionPDFTableBBox(bbox, cellBBox)
			cells = append(cells, DoclingTableCell{
				BBox: cellBBox, RowSpan: 1, ColSpan: 1,
				StartRowOffsetIdx: int64(rowIndex), EndRowOffsetIdx: int64(rowIndex + 1),
				StartColOffsetIdx: int64(column), EndColOffsetIdx: int64(column + 1),
				Text: cell.Text, ColumnHeader: header && rowIndex == 0,
			})
		}
	}
	return pdfDetectedTable{
		PageIdx: rows[0].line.PageIdx,
		Data:    TableData{TableCells: cells, NumRows: int64(len(rows)), NumCols: int64(len(clusters)), Orientation: TableOrientation0},
		BBox:    bbox, Lines: func() []pdfLine {
			result := make([]pdfLine, 0, len(rows))
			for _, row := range rows {
				result = append(result, row.line)
			}
			return result
		}(),
		StartLine: rows[0].lineIndex, EndLine: rows[len(rows)-1].lineIndex + 1,
	}, true
}

// pdfSparseTableIsCompact 对稀疏表采用紧凑文本约束，降低多栏正文误判率。
func pdfSparseTableIsCompact(rows []pdfTableRowCandidate) bool {
	total, count := 0, 0
	for _, row := range rows {
		for _, cell := range row.cells {
			length := utf8.RuneCountInString(strings.TrimSpace(cell.Text))
			if length == 0 || length > 48 {
				return false
			}
			total += length
			count++
		}
	}
	return count > 0 && float64(total)/float64(count) <= 20
}

// nearestPDFTableAnchor 返回单元格匹配的最近稳定列锚。
func nearestPDFTableAnchor(x float64, clusters []pdfTableAnchorCluster, tolerance float64) int {
	best, distance := -1, math.MaxFloat64
	for index, cluster := range clusters {
		current := math.Abs(cluster.x - x)
		if current <= tolerance && current < distance {
			best, distance = index, current
		}
	}
	return best
}

// inferPDFSparseTableHeader 判断稀疏表首行是否为文本表头。
func inferPDFSparseTableHeader(rows []pdfTableRowCandidate, clusters []pdfTableAnchorCluster, tolerance float64) bool {
	if len(rows) < 2 {
		return false
	}
	for _, cell := range rows[0].cells {
		if nearestPDFTableAnchor(cell.MinX, clusters, tolerance) < 0 || pdfTableTextHasDigit(cell.Text) {
			return false
		}
	}
	for _, row := range rows[1:] {
		for _, cell := range row.cells {
			if pdfTableTextHasDigit(cell.Text) {
				return true
			}
		}
	}
	return false
}

// splitPDFTableRow 根据词间显著水平留白生成候选单元格。页眉页脚、目录、OCR
// 透传块和无词坐标兜底行不参与表格识别，避免破坏既有特殊语义。
func splitPDFTableRow(line pdfLine) []pdfTableCellCandidate {
	if line.Fallback || line.OCRSub != nil || line.Furniture != "" || line.InTOC || len(line.Words) < 2 {
		return nil
	}

	words := make([]pdfWord, 0, len(line.Words))
	for _, word := range line.Words {
		if strings.TrimSpace(word.Text) != "" {
			words = append(words, word)
		}
	}
	if len(words) < 2 {
		return nil
	}
	sort.SliceStable(words, func(i, j int) bool { return words[i].MinX < words[j].MinX })
	fontSize := medianPDFWordFontSize(words, line.MaxFontSize)
	gapThreshold := math.Max(pdfTableMinColumnGapPt, fontSize*1.2)

	var cells []pdfTableCellCandidate
	current := newPDFTableCellCandidate(words[0])
	for _, word := range words[1:] {
		if strings.TrimSpace(word.Text) == "" {
			continue
		}
		if word.MinX-current.MaxX >= gapThreshold {
			cells = append(cells, current)
			current = newPDFTableCellCandidate(word)
			continue
		}
		current.Text = joinPDFTableCellText(current.Text, word.Text)
		current.MinX = math.Min(current.MinX, word.MinX)
		current.MaxX = math.Max(current.MaxX, word.MaxX)
		current.MinY = math.Min(current.MinY, word.MinY)
		current.MaxY = math.Max(current.MaxY, word.MaxY)
		current.FontSize = math.Max(current.FontSize, word.FontSize)
	}
	cells = append(cells, current)
	return cells
}

// newPDFTableCellCandidate 从首个词初始化候选单元格边界。
func newPDFTableCellCandidate(word pdfWord) pdfTableCellCandidate {
	return pdfTableCellCandidate{
		Text: strings.TrimSpace(word.Text), MinX: word.MinX, MaxX: word.MaxX,
		MinY: word.MinY, MaxY: word.MaxY, FontSize: word.FontSize,
	}
}

// joinPDFTableCellText 合并单元格内相邻词，并避免对空白片段产生多余空格。
func joinPDFTableCellText(left, right string) string {
	left = strings.TrimSpace(left)
	right = strings.TrimSpace(right)
	if left == "" {
		return right
	}
	if right == "" {
		return left
	}
	return left + " " + right
}

// medianPDFWordFontSize 返回行内有效词字号中位数；词字号缺失时回退行字号，
// 再缺失时使用常见 10pt，保证空白阈值稳定。
func medianPDFWordFontSize(words []pdfWord, fallback float64) float64 {
	fonts := make([]float64, 0, len(words))
	for _, word := range words {
		if word.FontSize > 0 {
			fonts = append(fonts, word.FontSize)
		}
	}
	if len(fonts) == 0 {
		if fallback > 0 {
			return fallback
		}
		return 10
	}
	sort.Float64s(fonts)
	mid := len(fonts) / 2
	if len(fonts)%2 == 0 {
		return (fonts[mid-1] + fonts[mid]) / 2
	}
	return fonts[mid]
}

// pdfTableRowsCompatible 判断相邻候选行能否属于同一矩形网格：页号、列数、
// 列起点与垂直距离均须稳定。列锚容差随字号略微放宽。
func pdfTableRowsCompatible(previous, current pdfTableRowCandidate) bool {
	if previous.line.PageIdx != current.line.PageIdx || len(previous.cells) != len(current.cells) {
		return false
	}
	fontSize := math.Max(previous.line.MaxFontSize, current.line.MaxFontSize)
	anchorTolerance := math.Max(pdfTableAnchorTolerancePt, fontSize*0.8)
	for column := range previous.cells {
		if math.Abs(previous.cells[column].MinX-current.cells[column].MinX) > anchorTolerance {
			return false
		}
	}

	// BOTTOMLEFT 坐标中前一阅读行通常位于当前行上方。间距过大时视为不同区域；
	// 对输入中偶发同 Y 或轻微乱序不在此强制拒绝，后续版面排序仍可校正。
	verticalGap := previous.line.MinY - current.line.MaxY
	maxGap := math.Max(36, fontSize*4)
	return verticalGap <= maxGap
}

// buildPDFDetectedTable 把连续候选行转换为官方 TableData。两列区域额外要求
// 单元格文本紧凑，以排除最常见的双栏正文误判。
func buildPDFDetectedTable(rows []pdfTableRowCandidate) (pdfDetectedTable, bool) {
	if len(rows) < 2 || len(rows[0].cells) < 2 || !pdfTableRunHasStableAnchors(rows) {
		return pdfDetectedTable{}, false
	}
	columnCount := len(rows[0].cells)
	if columnCount == 2 && !pdfTableTwoColumnRunIsCompact(rows) {
		return pdfDetectedTable{}, false
	}

	header := inferPDFTableHeader(rows)
	cells := make([]DoclingTableCell, 0, len(rows)*columnCount)
	var bbox *DoclingBBox
	for rowIndex, row := range rows {
		for columnIndex, cell := range row.cells {
			cellBBox := &DoclingBBox{
				L: cell.MinX, T: cell.MaxY, R: cell.MaxX, B: cell.MinY,
				CoordOrigin: CoordOriginBottomLeft,
			}
			bbox = unionPDFTableBBox(bbox, cellBBox)
			cells = append(cells, DoclingTableCell{
				BBox: cellBBox, RowSpan: 1, ColSpan: 1,
				StartRowOffsetIdx: int64(rowIndex), EndRowOffsetIdx: int64(rowIndex + 1),
				StartColOffsetIdx: int64(columnIndex), EndColOffsetIdx: int64(columnIndex + 1),
				Text: cell.Text, ColumnHeader: header && rowIndex == 0,
			})
		}
	}

	lines := make([]pdfLine, 0, len(rows))
	for _, row := range rows {
		lines = append(lines, row.line)
	}
	return pdfDetectedTable{
		PageIdx: rows[0].line.PageIdx,
		Data: TableData{
			TableCells: cells, NumRows: int64(len(rows)), NumCols: int64(columnCount),
			Orientation: TableOrientation0,
		},
		BBox: bbox, Lines: lines,
		StartLine: rows[0].lineIndex, EndLine: rows[len(rows)-1].lineIndex + 1,
	}, true
}

// pdfTableRunHasStableAnchors 用整段首行复核所有列起点，防止逐行小幅漂移累积后
// 形成实际不稳定的伪表格。
func pdfTableRunHasStableAnchors(rows []pdfTableRowCandidate) bool {
	base := rows[0]
	fontSize := base.line.MaxFontSize
	for _, row := range rows[1:] {
		fontSize = math.Max(fontSize, row.line.MaxFontSize)
	}
	tolerance := math.Max(pdfTableAnchorTolerancePt, fontSize*0.8)
	minAnchorDistance := math.Max(pdfTableMinAnchorDistancePt, fontSize*4)
	for column := 1; column < len(base.cells); column++ {
		if base.cells[column].MinX-base.cells[column-1].MinX < minAnchorDistance {
			return false
		}
	}
	for _, row := range rows[1:] {
		if len(row.cells) != len(base.cells) {
			return false
		}
		for column := range base.cells {
			if math.Abs(base.cells[column].MinX-row.cells[column].MinX) > tolerance {
				return false
			}
		}
	}
	return true
}

// pdfTableTwoColumnRunIsCompact 对两列表格采用更严格的文本长度规则；三列及以上
// 已具备更强网格信号，不套用该限制。
func pdfTableTwoColumnRunIsCompact(rows []pdfTableRowCandidate) bool {
	totalRunes := 0
	cellCount := 0
	for _, row := range rows {
		for _, cell := range row.cells {
			length := utf8.RuneCountInString(strings.TrimSpace(cell.Text))
			if length == 0 || length > pdfTableTwoColumnMaxRunes {
				return false
			}
			totalRunes += length
			cellCount++
		}
	}
	return cellCount > 0 && float64(totalRunes)/float64(cellCount) <= 20
}

// inferPDFTableHeader 在首行均为非数字文本、而后续行出现数字值时把首行标记为
// 列表头；无法可靠判断时保持普通单元格，避免臆造结构语义。
func inferPDFTableHeader(rows []pdfTableRowCandidate) bool {
	for _, cell := range rows[0].cells {
		if pdfTableTextHasDigit(cell.Text) {
			return false
		}
	}
	for _, row := range rows[1:] {
		for _, cell := range row.cells {
			if pdfTableTextHasDigit(cell.Text) {
				return true
			}
		}
	}
	return false
}

// pdfTableTextHasDigit 判断文本是否包含任意 Unicode 数字。
func pdfTableTextHasDigit(text string) bool {
	for _, r := range text {
		if unicode.IsDigit(r) {
			return true
		}
	}
	return false
}

// unionPDFTableBBox 返回两个 BOTTOMLEFT Docling 边界框的并集。
func unionPDFTableBBox(current, next *DoclingBBox) *DoclingBBox {
	if next == nil {
		return current
	}
	if current == nil {
		copyBBox := *next
		return &copyBBox
	}
	current.L = math.Min(current.L, next.L)
	current.R = math.Max(current.R, next.R)
	current.T = math.Max(current.T, next.T)
	current.B = math.Min(current.B, next.B)
	return current
}

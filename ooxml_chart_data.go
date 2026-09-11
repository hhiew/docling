// ooxml_chart_data.go 实现 OOXML 图表公式的数据回填。XLSX 图表直接读取
// 当前工作簿，DOCX/PPTX 图表读取 chart*.xml.rels 指向的嵌入 XLSX；全部
// 使用 archive/zip 与 excelize 的纯 Go 路径，不依赖 Office 或 LibreOffice。
package docparse

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"path"
	"sort"
	"strings"

	"github.com/xuri/excelize/v2"
)

// maxOOXMLChartFormulaPoints 限制单个图表公式可读取的单元格数量，避免损坏
// 或恶意工作簿通过整表引用触发超大分配和长时间逐格读取。
const maxOOXMLChartFormulaPoints = 100_000

// addOOXMLChartPictureFromPart 从 Office 包中的 chart 部件创建 PictureItem。
// fallback 用于 XLSX 当前工作簿；DOCX/PPTX 若存在嵌入工作簿则优先使用它。
func addOOXMLChartPictureFromPart(doc *DoclingDocument, reader *zip.Reader, chartPath string, fallback ooxmlChartFormulaResolver, prov []ProvenanceItem, parent *RefItem) (RefItem, bool) {
	chartXML, err := readZipFileBytes(reader, chartPath)
	if err != nil || len(chartXML) == 0 {
		return RefItem{}, false
	}
	resolver := fallback
	if workbook := openOOXMLEmbeddedChartWorkbook(reader, chartPath); workbook != nil {
		defer func() { _ = workbook.Close() }()
		resolver = newExcelizeChartFormulaResolver(workbook)
	}
	return addOOXMLChartPictureWithResolver(doc, chartXML, resolver, prov, parent)
}

// openOOXMLEmbeddedChartWorkbook 打开图表关系中的第一个有效嵌入 XLSX。
// 关系 ID 排序后尝试，确保多个 package 关系时结果稳定。
func openOOXMLEmbeddedChartWorkbook(reader *zip.Reader, chartPath string) *excelize.File {
	if reader == nil || chartPath == "" {
		return nil
	}
	relsPath := path.Join(path.Dir(chartPath), "_rels", path.Base(chartPath)+".rels")
	relsXML, err := readZipFileBytes(reader, relsPath)
	if err != nil || len(relsXML) == 0 {
		return nil
	}
	targets := parseOOXMLRelationships(relsXML, "/package", path.Dir(chartPath))
	ids := make([]string, 0, len(targets))
	for id := range targets {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		data, readErr := readZipFileBytes(reader, targets[id])
		if readErr != nil || len(data) == 0 {
			continue
		}
		workbook, openErr := excelize.OpenReader(bytes.NewReader(data))
		if openErr == nil {
			return workbook
		}
	}
	return nil
}

// newExcelizeChartFormulaResolver 创建工作簿公式读取器。它支持常见的单格、
// 横向/纵向矩形范围、带引号工作表名、绝对引用及最终指向单一 A1 区域的
// 工作簿/工作表命名范围和普通表格列结构化引用；动态公式与多区域公式由
// 图表自身缓存继续兜底。
func newExcelizeChartFormulaResolver(workbook *excelize.File) ooxmlChartFormulaResolver {
	if workbook == nil {
		return nil
	}
	return func(formula string) []ooxmlChartPoint {
		sheet, startCell, endCell, ok := parseOOXMLChartCellRange(formula)
		startCol, startRow, startErr := excelize.CellNameToCoordinates(startCell)
		endCol, endRow, endErr := excelize.CellNameToCoordinates(endCell)
		if !ok || startErr != nil || endErr != nil {
			sheet, startCell, endCell, ok = resolveOOXMLChartDefinedName(workbook, formula)
			if !ok {
				sheet, startCell, endCell, ok = resolveOOXMLChartStructuredReference(workbook, formula)
				if !ok {
					return nil
				}
			}
			startCol, startRow, startErr = excelize.CellNameToCoordinates(startCell)
			endCol, endRow, endErr = excelize.CellNameToCoordinates(endCell)
			if startErr != nil || endErr != nil {
				return nil
			}
		}
		if startCol > endCol {
			startCol, endCol = endCol, startCol
		}
		if startRow > endRow {
			startRow, endRow = endRow, startRow
		}
		pointCount := int64(endRow-startRow+1) * int64(endCol-startCol+1)
		if pointCount <= 0 || pointCount > maxOOXMLChartFormulaPoints {
			return nil
		}
		points := make([]ooxmlChartPoint, 0, int(pointCount))
		index := 0
		for row := startRow; row <= endRow; row++ {
			for col := startCol; col <= endCol; col++ {
				cell, coordErr := excelize.CoordinatesToCellName(col, row)
				if coordErr != nil {
					return nil
				}
				value, valueErr := workbook.GetCellValue(sheet, cell)
				if valueErr != nil {
					return nil
				}
				points = append(points, ooxmlChartPoint{Index: index, Value: value})
				index++
			}
		}
		return points
	}
}

// resolveOOXMLChartDefinedName 将图表中的工作簿级或工作表级命名范围解析为
// 单一 A1 区域。显式工作表作用域优先于工作簿作用域；不执行 OFFSET 等动态
// 公式，也不展开联合区域，防止静态解析得到与 Office 可见结果不一致的数据。
func resolveOOXMLChartDefinedName(workbook *excelize.File, formula string) (sheet, startCell, endCell string, ok bool) {
	if workbook == nil {
		return "", "", "", false
	}
	expression := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(formula), "="))
	if expression == "" || strings.ContainsAny(expression, ",;():") {
		return "", "", "", false
	}
	name := expression
	scope := ""
	if parsedSheet, parsedName, parsedEnd, parsed := parseOOXMLChartCellRange(expression); parsed && parsedName == parsedEnd {
		scope = parsedSheet
		name = parsedName
	}
	name = strings.TrimSpace(name)
	if name == "" || strings.ContainsAny(name, "!$[]'") {
		return "", "", "", false
	}

	var workbookName, worksheetName *excelize.DefinedName
	for _, definedName := range workbook.GetDefinedName() {
		if !strings.EqualFold(strings.TrimSpace(definedName.Name), name) {
			continue
		}
		candidate := definedName
		if strings.EqualFold(definedName.Scope, "Workbook") {
			workbookName = &candidate
		}
		if scope != "" && strings.EqualFold(definedName.Scope, scope) {
			worksheetName = &candidate
		}
	}
	selected := worksheetName
	if selected == nil {
		selected = workbookName
	}
	if selected == nil {
		return "", "", "", false
	}
	return parseOOXMLChartCellRange(selected.RefersTo)
}

// ooxmlChartTableDefinition 保存解析结构化引用所需的最小表格定义。指针字段
// 用于区分 OOXML 属性缺省值与显式零值。
type ooxmlChartTableDefinition struct {
	Name            string `xml:"name,attr"`
	DisplayName     string `xml:"displayName,attr"`
	HeaderRowCount  *int   `xml:"headerRowCount,attr"`
	TotalsRowCount  int    `xml:"totalsRowCount,attr"`
	TotalsRowShown  *bool  `xml:"totalsRowShown,attr"`
	TableColumnList struct {
		Columns []struct {
			Name string `xml:"name,attr"`
		} `xml:"tableColumn"`
	} `xml:"tableColumns"`
}

// resolveOOXMLChartStructuredReference 将 TableName[Column] 解析为表格数据正文
// 的单列 A1 区域。带 #All/#Headers/#Totals、当前行或多列选择器的表达式暂不
// 静态展开，避免把表头、汇总行或隐式交集错误混入图表系列。
func resolveOOXMLChartStructuredReference(workbook *excelize.File, formula string) (sheet, startCell, endCell string, ok bool) {
	if workbook == nil {
		return "", "", "", false
	}
	expression := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(formula), "="))
	if expression == "" || len(expression) > 1024 || !strings.HasSuffix(expression, "]") {
		return "", "", "", false
	}
	open := strings.Index(expression, "[")
	if open <= 0 {
		return "", "", "", false
	}
	prefix := strings.TrimSpace(expression[:open])
	columnName := strings.TrimSpace(expression[open+1 : len(expression)-1])
	if prefix == "" || columnName == "" || strings.ContainsAny(columnName, "[]#,:@") {
		return "", "", "", false
	}
	tableName := prefix
	scope := ""
	if separator := strings.LastIndex(prefix, "!"); separator >= 0 {
		if separator == 0 || separator == len(prefix)-1 {
			return "", "", "", false
		}
		parsedScope, _, _, parsed := parseOOXMLChartCellRange(prefix[:separator] + "!A1")
		if !parsed {
			return "", "", "", false
		}
		scope = parsedScope
		tableName = strings.TrimSpace(prefix[separator+1:])
	}
	if tableName == "" || strings.ContainsAny(tableName, "$[]'") {
		return "", "", "", false
	}

	for _, sheetName := range workbook.GetSheetList() {
		if scope != "" && !strings.EqualFold(scope, sheetName) {
			continue
		}
		tables, err := workbook.GetTables(sheetName)
		if err != nil {
			continue
		}
		for _, table := range tables {
			if !strings.EqualFold(table.Name, tableName) {
				continue
			}
			definition, found := findOOXMLChartTableDefinition(workbook, tableName)
			if !found {
				return "", "", "", false
			}
			return resolveOOXMLChartTableColumn(sheetName, table.Range, definition, columnName)
		}
	}
	return "", "", "", false
}

// findOOXMLChartTableDefinition 从 excelize 已加载的 OOXML 包中查找表格定义，
// 读取列名以及表头、汇总行计数，不依赖未导出的 excelize 内部字段。
func findOOXMLChartTableDefinition(workbook *excelize.File, tableName string) (definition ooxmlChartTableDefinition, ok bool) {
	if workbook == nil || tableName == "" {
		return definition, false
	}
	workbook.Pkg.Range(func(key, value any) bool {
		partName, nameOK := key.(string)
		data, dataOK := value.([]byte)
		if !nameOK || !dataOK || !strings.HasPrefix(partName, "xl/tables/") || !strings.HasSuffix(partName, ".xml") || len(data) > 4<<20 {
			return true
		}
		var candidate ooxmlChartTableDefinition
		if err := xml.Unmarshal(data, &candidate); err != nil {
			return true
		}
		if strings.EqualFold(candidate.Name, tableName) || strings.EqualFold(candidate.DisplayName, tableName) {
			definition = candidate
			ok = true
			return false
		}
		return true
	})
	return definition, ok
}

// resolveOOXMLChartTableColumn 根据表定义把普通列引用换算为数据正文坐标；
// 默认表头为一行，并从尾部扣除显式汇总行。
func resolveOOXMLChartTableColumn(sheet, tableRange string, definition ooxmlChartTableDefinition, columnName string) (resolvedSheet, startCell, endCell string, ok bool) {
	parts := strings.Split(tableRange, ":")
	if len(parts) == 0 || len(parts) > 2 {
		return "", "", "", false
	}
	startCol, startRow, startErr := excelize.CellNameToCoordinates(strings.ReplaceAll(strings.TrimSpace(parts[0]), "$", ""))
	endCol, endRow := startCol, startRow
	var endErr error
	if len(parts) == 2 {
		endCol, endRow, endErr = excelize.CellNameToCoordinates(strings.ReplaceAll(strings.TrimSpace(parts[1]), "$", ""))
	}
	if startErr != nil || endErr != nil || startCol > endCol || startRow > endRow {
		return "", "", "", false
	}
	columnOffset := -1
	for index, column := range definition.TableColumnList.Columns {
		if strings.EqualFold(strings.TrimSpace(column.Name), columnName) {
			columnOffset = index
			break
		}
	}
	if columnOffset < 0 || startCol+columnOffset > endCol {
		return "", "", "", false
	}
	headerRows := 1
	if definition.HeaderRowCount != nil {
		headerRows = *definition.HeaderRowCount
	}
	totalsRows := definition.TotalsRowCount
	if totalsRows == 0 && definition.TotalsRowShown != nil && *definition.TotalsRowShown {
		totalsRows = 1
	}
	dataStartRow := startRow + headerRows
	dataEndRow := endRow - totalsRows
	if headerRows < 0 || totalsRows < 0 || dataStartRow > dataEndRow {
		return "", "", "", false
	}
	column := startCol + columnOffset
	startCell, startErr = excelize.CoordinatesToCellName(column, dataStartRow)
	endCell, endErr = excelize.CoordinatesToCellName(column, dataEndRow)
	if startErr != nil || endErr != nil {
		return "", "", "", false
	}
	return sheet, startCell, endCell, true
}

// parseOOXMLChartCellRange 把 Sheet1!$A$1:$B$3 或
// '数据 表'!$A$1 解析为 excelize 可识别的工作表与坐标。
func parseOOXMLChartCellRange(formula string) (sheet, startCell, endCell string, ok bool) {
	formula = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(formula), "="))
	separator := strings.LastIndex(formula, "!")
	if separator <= 0 || separator == len(formula)-1 {
		return "", "", "", false
	}
	sheet = strings.TrimSpace(formula[:separator])
	cellRange := strings.TrimSpace(formula[separator+1:])
	if strings.ContainsAny(cellRange, ",;()") {
		return "", "", "", false
	}
	if strings.HasPrefix(sheet, "'") && strings.HasSuffix(sheet, "'") && len(sheet) >= 2 {
		sheet = strings.ReplaceAll(sheet[1:len(sheet)-1], "''", "'")
	}
	if closeBracket := strings.LastIndex(sheet, "]"); closeBracket >= 0 {
		sheet = sheet[closeBracket+1:]
	}
	parts := strings.Split(cellRange, ":")
	if len(parts) > 2 || len(parts) == 0 {
		return "", "", "", false
	}
	startCell = strings.ReplaceAll(strings.TrimSpace(parts[0]), "$", "")
	endCell = startCell
	if len(parts) == 2 {
		endCell = strings.ReplaceAll(strings.TrimSpace(parts[1]), "$", "")
	}
	if sheet == "" || startCell == "" || endCell == "" {
		return "", "", "", false
	}
	return sheet, startCell, endCell, true
}

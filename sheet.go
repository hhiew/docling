// sheet.go 实现 xlsx 与 csv 的纯规则表格解析后端，产出 DoclingDocument
// （详细 JSON 协议见 docling.go），逐条复刻 Docling 对应后端的规则行为。
//
// xlsx 复刻 docling/backend/msexcel_backend.py：
//  1. 每 sheet 产出一个页面（页号按工作簿顺序自增，尺寸=内容包络，
//     对齐 _find_page_size 取该页 prov bbox 的最大 r/b）与一个 sheet 分组
//     （AddSectionGroup，name=sheet 名）；
//  2. 表格检测为 BFS flood fill 连通区域（_find_data_tables/_find_table_bounds，
//     gap_tolerance=0：4 邻域相邻即连通），连通载体为"值非空 cell + 合并区域
//     全部位置"，一个连通区域产出一个 table_item，包围盒内空洞补空 cell
//     保持矩形；
//  3. 合并单元格（GetMergeCells）：锚点承载行列跨度（end 开区间），影子位置
//     不产独立 cell；
//  4. 表内首行 column_header=true；
//  5. 表首行为"跨全列合并的单一标题"时拆出为 TEXT 元素（挂 sheet 分组下），
//     表格锚点行下移（_split_leading_section_label）；
//  6. prov 以 0-based 单元格索引坐标表达：bbox={l:起始列, t:起始行,
//     r:起始列+列数, b:起始行+行数}，charspan=[0,0]；
//  7. 隐藏 sheet 保留并标记 invisible，批注标记 notes，图片与图表保留锚点；
//  8. 各元素按单元格/对象左上角坐标稳定排序，图表数据复用共享 OOXML helper；
//  9. 值取 excelize 缓存结果（公式取缓存值，对齐 data_only 语义）。
//
// csv 复刻 docling/backend/csv_backend.py：
//  1. 分隔符嗅探：候选 ,;\t|: 逐个试解析，取各行字段数一致且列数最多者，
//     全部失败回退逗号；
//  2. 整文件产出一个 table：首行 column_header、span 恒 1、num_cols 取最大
//     行宽、列数不齐不报错；
//  3. 不生成 prov；
//  4. 空文件返回空文档不报错。
//
// 与旧版（直出 content_list []Item）的差异：签名统一为 (*DoclingDocument, error)，
// 由 contentlist.go 的 ToContentList 统一简化；解析层不再按行数拆分大表
// （分段语义下沉至知识库切片层）；空内容返回空文档不报错，与 markdown.go
// 样板一致，由调用方决定回退。
package docparse

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"image"
	"math"
	"path"
	"sort"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"

	"github.com/unitedrhino/docling/internal/ooxml"
)

const (
	// xlsxFormulaMetaKey 在富单元格引用中保存原始 Excel 公式。
	xlsxFormulaMetaKey = "docparse__xlsx_formula"
	// xlsxPivotTableMetaKey 在表格节点中保存数据透视表定义。
	xlsxPivotTableMetaKey = "docparse__xlsx_pivot_table"
	// xlsxSharedFormulaPlaceholder 明确标记无法从损坏工作簿展开的共享公式。
	xlsxSharedFormulaPlaceholder = "SHARED_FORMULA"
)

// cellKey 0-based 单元格坐标（行,列），作为 occupied/visited 等集合的键。
type cellKey struct {
	row int
	col int
}

// sheetMerge 合并单元格的 0-based 锚点与跨度（行数/列数，对应 end 开区间偏移）。
type sheetMerge struct {
	anchorRow int
	anchorCol int
	rowSpan   int
	colSpan   int
}

// sheetRegionCell 连通区域内一个待产出的单元格（表内相对坐标 + 跨度）。
type sheetRegionCell struct {
	row     int64  // row 是区域内相对行号。
	col     int64  // col 是区域内相对列号。
	rowSpan int64  // rowSpan 是合并行跨度。
	colSpan int64  // colSpan 是合并列跨度。
	text    string // text 是单元格显示缓存值。
	formula string // formula 是不带等号的原始公式或共享公式占位。
}

// parseXLSXCellFormulas 从工作表 XML 发现全部公式单元格，再由 excelize
// 尽力展开共享公式；即使缓存值为空，公式单元格也会进入表格区域。
func parseXLSXCellFormulas(reader *zip.Reader, sheetPath string, file *excelize.File, sheetName string) map[cellKey]string {
	formulas := map[cellKey]string{}
	if reader == nil || sheetPath == "" {
		return formulas
	}
	data, err := readZipFileBytes(reader, sheetPath)
	if err != nil || len(data) == 0 {
		return formulas
	}
	decoder := xml.NewDecoder(bytes.NewReader(data))
	for {
		token, err := decoder.Token()
		if err != nil {
			return formulas
		}
		start, ok := token.(xml.StartElement)
		if !ok || start.Name.Local != "c" {
			continue
		}
		axis := xmlAttrVal(start.Attr, "r")
		raw, collectErr := collectSubTree(decoder, start)
		if collectErr != nil {
			return formulas
		}
		formula, formulaType, sharedIndex, found := parseXLSXFormulaCell(raw)
		if !found {
			continue
		}
		column, row, coordinateErr := excelize.CellNameToCoordinates(axis)
		if coordinateErr != nil || row <= 0 || column <= 0 {
			continue
		}
		if file != nil {
			if expanded, formulaErr := file.GetCellFormula(sheetName, axis); formulaErr == nil && strings.TrimSpace(expanded) != "" {
				formula = strings.TrimSpace(expanded)
			}
		}
		if formula == "" {
			formula = xlsxSharedFormulaPlaceholder
			if formulaType == "shared" && sharedIndex != "" {
				formula += "[" + sharedIndex + "]"
			}
		}
		formulas[cellKey{row: row - 1, col: column - 1}] = formula
	}
}

// parseXLSXFormulaCell 读取单个 c 元素中的 f 内容和共享公式属性。
func parseXLSXFormulaCell(data []byte) (formula, formulaType, sharedIndex string, found bool) {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	inFormula := false
	var text strings.Builder
	for {
		token, err := decoder.Token()
		if err != nil {
			return strings.TrimSpace(text.String()), formulaType, sharedIndex, found
		}
		switch value := token.(type) {
		case xml.StartElement:
			if value.Name.Local == "f" {
				found = true
				inFormula = true
				formulaType = xmlAttrVal(value.Attr, "t")
				sharedIndex = xmlAttrVal(value.Attr, "si")
			}
		case xml.CharData:
			if inFormula {
				text.Write(value)
			}
		case xml.EndElement:
			if value.Name.Local == "f" {
				inFormula = false
			}
		}
	}
}

// xlsxSheetPart 记录 workbook.xml 中工作表对应的 OOXML 部件路径与类型。
type xlsxSheetPart struct {
	path       string
	chartSheet bool
}

// xlsxDrawingChart 表示工作表 DrawingML 内的图表关系及单元格锚点。
type xlsxDrawingChart struct {
	relID          string
	row, col       int
	endRow, endCol int
}

// xlsxPivotFieldMeta 保存数据透视表一个行、列、值或筛选字段。
type xlsxPivotFieldMeta struct {
	Data          string   `json:"data"`                     // Data 是源数据字段名。
	Name          string   `json:"name,omitempty"`           // Name 是透视表中的自定义显示名。
	Subtotal      string   `json:"subtotal,omitempty"`       // Subtotal 是值字段的聚合方式。
	SelectedItems []string `json:"selected_items,omitempty"` // SelectedItems 是当前保留的筛选成员。
	ShowValuesAs  int      `json:"show_values_as,omitempty"` // ShowValuesAs 是 excelize 定义的值展示计算类型。
	BaseField     string   `json:"base_field,omitempty"`     // BaseField 是比较或累计计算的基准字段。
	BaseItem      string   `json:"base_item,omitempty"`      // BaseItem 是比较计算的基准项。
}

// xlsxPivotTableMeta 保存数据透视表的数据源、布局和字段语义。
type xlsxPivotTableMeta struct {
	Name         string               `json:"name"`                // Name 是数据透视表名称。
	DataRange    string               `json:"data_range"`          // DataRange 是源数据区域或命名区域。
	PivotRange   string               `json:"pivot_range"`         // PivotRange 是结果在工作表中的范围。
	Rows         []xlsxPivotFieldMeta `json:"rows,omitempty"`      // Rows 是行字段。
	Columns      []xlsxPivotFieldMeta `json:"columns,omitempty"`   // Columns 是列字段。
	Data         []xlsxPivotFieldMeta `json:"data,omitempty"`      // Data 是聚合值字段。
	Filters      []xlsxPivotFieldMeta `json:"filters,omitempty"`   // Filters 是页筛选字段。
	Style        string               `json:"style,omitempty"`     // Style 是内置数据透视表样式名。
	RowTotals    bool                 `json:"row_grand_totals"`    // RowTotals 表示是否显示行总计。
	ColumnTotals bool                 `json:"column_grand_totals"` // ColumnTotals 表示是否显示列总计。
}

// ParseXLSX 解析 xlsx 工作簿为 DoclingDocument：每个 sheet 依次产出
// 页面与 sheet 分组，sheet 内按 BFS 连通区域切分表格，并保留图片、
// 图表与批注。隐藏 sheet 使用 invisible 层；空内容不视为错误。
func ParseXLSX(data []byte) (*DoclingDocument, error) {
	var err error
	data, err = ooxml.NormalizeStrictOOXMLPackage(data)
	if err != nil {
		return nil, fmt.Errorf("docparse: 归一化 Strict XLSX 失败: %w", err)
	}
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	doc := NewDoclingDocument("xlsx")
	var reader *zip.Reader
	// 文档元数据（docProps/core.xml，增强能力：缺失不影响主解析）
	if zipReader, zipErr := zip.NewReader(bytes.NewReader(data), int64(len(data))); zipErr == nil {
		reader = zipReader
		doc.Meta = ooxmlCorePropsMeta(reader)
	}
	sheetParts := parseXLSXSheetParts(reader)
	pageNo := int64(0)
	for _, sheetName := range f.GetSheetList() {
		layer := LayerBody
		visible, visErr := f.GetSheetVisible(sheetName)
		if visErr == nil && !visible {
			layer = LayerInvisible
		}
		pageNo++
		part := sheetParts[sheetName]
		if part.chartSheet {
			convertXLSXChartSheet(doc, f, reader, sheetName, part, pageNo, layer)
			continue
		}
		convertXLSXSheet(doc, f, reader, sheetName, part, pageNo, layer)
	}
	// 页数取工作簿全部 sheet 数（含隐藏表与图表工作表）。
	if doc.Meta == nil && pageNo > 0 {
		doc.Meta = &DocMeta{}
	}
	if doc.Meta != nil {
		doc.Meta.PageCount = int(pageNo)
	}
	return doc, nil
}

// convertXLSXSheet 解析单个工作表：建 sheet 分组，按 BFS 连通区域产出
// 表格（必要时拆出首行标题），最后以内容包络登记页面尺寸（对齐 _convert_workbook
// 中 add_page 后用 _find_page_size 回填 size 的行为）。
func convertXLSXSheet(doc *DoclingDocument, f *excelize.File, reader *zip.Reader, sheetName string, part xlsxSheetPart, pageNo int64, layer ContentLayer) {
	group := doc.AddSectionGroup(sheetName, nil)
	doc.Groups[group.Idx].Label = GroupLabelSheet
	doc.Groups[group.Idx].ContentLayer = layer

	// GetRows 取全 sheet 值网格（公式取缓存结果，对齐 data_only 语义）
	rows, err := f.GetRows(sheetName)
	if err != nil {
		rows = nil
	}
	merges := parseSheetMerges(f, sheetName)
	formulas := parseXLSXCellFormulas(reader, part.path, f, sheetName)

	// 连通载体 = 值非空 cell + 合并区域全部位置（锚点与影子都参与连通判断）
	occupied := make(map[cellKey]bool)
	for r, row := range rows {
		for c, text := range row {
			if text != "" {
				occupied[cellKey{r, c}] = true
			}
		}
	}
	for key := range formulas {
		occupied[key] = true
	}
	for _, m := range merges {
		for r := m.anchorRow; r < m.anchorRow+m.rowSpan; r++ {
			for c := m.anchorCol; c < m.anchorCol+m.colSpan; c++ {
				occupied[cellKey{r, c}] = true
			}
		}
	}
	// 按行列序枚举 BFS 起点，保证多连通区域的产出顺序稳定
	starts := make([]cellKey, 0, len(occupied))
	for k := range occupied {
		starts = append(starts, k)
	}
	sort.Slice(starts, func(i, j int) bool {
		if starts[i].row != starts[j].row {
			return starts[i].row < starts[j].row
		}
		return starts[i].col < starts[j].col
	})

	visited := make(map[cellKey]bool)
	var width, height float64
	for _, start := range starts {
		if visited[start] {
			continue
		}
		region := floodFillRegion(start, occupied, visited)
		width, height = appendXLSXRegion(doc, region, rows, formulas, merges, pageNo, &group, layer, width, height)
	}
	width, height = appendXLSXPivotTables(doc, f, sheetName, pageNo, &group, layer, width, height)
	width, height = appendXLSXPictures(doc, f, sheetName, pageNo, &group, layer, width, height)
	width, height = appendXLSXCharts(doc, f, reader, part, pageNo, &group, layer, width, height)
	width, height = appendXLSXOfficeObjects(doc, reader, part, pageNo, &group, layer, width, height)
	var threadedCells map[string]bool
	width, height, threadedCells = appendXLSXThreadedComments(doc, reader, part, pageNo, group, layer, width, height)
	width, height = appendXLSXComments(doc, f, sheetName, pageNo, &group, layer, threadedCells, width, height)
	sortXLSXGroupChildren(doc, group)
	doc.AddPage(pageNo, width, height)
}

// appendXLSXPivotTables 把工作表的数据透视表定义挂到已解析的结果表；
// 文件未保存结果缓存时则生成一个两列语义表，避免字段、筛选和
// 聚合方式静默丢失。返回包含透视表范围的页面尺寸包络。
func appendXLSXPivotTables(doc *DoclingDocument, workbook *excelize.File, sheetName string, pageNo int64, parent *RefItem, layer ContentLayer, width, height float64) (float64, float64) {
	if doc == nil || workbook == nil || parent == nil {
		return width, height
	}
	pivots, err := workbook.GetPivotTables(sheetName)
	if err != nil || len(pivots) == 0 {
		return width, height
	}
	sort.SliceStable(pivots, func(i, j int) bool {
		iRow, iCol, _, _, iOK := parseXLSXCellRange(pivots[i].PivotTableRange)
		jRow, jCol, _, _, jOK := parseXLSXCellRange(pivots[j].PivotTableRange)
		if iOK != jOK {
			return iOK
		}
		if iRow != jRow {
			return iRow < jRow
		}
		if iCol != jCol {
			return iCol < jCol
		}
		return pivots[i].Name < pivots[j].Name
	})
	for _, pivot := range pivots {
		metadata := newXLSXPivotTableMeta(pivot)
		startRow, startCol, endRow, endCol, ok := parseXLSXCellRange(pivot.PivotTableRange)
		if !ok {
			startRow, startCol, endRow, endCol = int(height), 0, int(height)+1, 2
		}
		ref, found := findXLSXPivotResultTable(doc, *parent, pageNo, startRow, startCol)
		if !found {
			cells, rows, cols := xlsxPivotSemanticCells(metadata)
			ref = doc.AddTable(cells, rows, cols, []ProvenanceItem{
				xlsxCellProvenance(pageNo, startRow, startCol, max(startRow+int(rows), endRow), max(startCol+int(cols), endCol)),
			}, parent)
			doc.Tables[ref.Idx].ContentLayer = layer
		}
		if raw, marshalErr := json.Marshal(metadata); marshalErr == nil {
			if doc.Tables[ref.Idx].Meta == nil {
				doc.Tables[ref.Idx].Meta = BaseMeta{}
			}
			doc.Tables[ref.Idx].Meta[xlsxPivotTableMetaKey] = raw
		}
		caption := xlsxPivotCaption(metadata)
		if caption != "" {
			captionRef := RefItem{Kind: refTexts, Idx: int64(len(doc.Texts))}
			tableRef := ref
			doc.Texts = append(doc.Texts, TextItem{
				SelfRef: captionRef.String(), Parent: &tableRef, Children: []RefItem{},
				ContentLayer: layer, Label: LabelCaption,
				Prov: []ProvenanceItem{xlsxCellProvenance(pageNo, startRow, startCol, startRow+1, max(startCol+1, endCol))},
				Orig: caption, Text: caption,
			})
			doc.Tables[ref.Idx].Captions = append(doc.Tables[ref.Idx].Captions, captionRef)
		}
		width = math.Max(width, float64(endCol))
		height = math.Max(height, float64(endRow))
	}
	return width, height
}

// newXLSXPivotTableMeta 把 excelize 的数据透视表选项转为稳定 JSON 结构。
func newXLSXPivotTableMeta(pivot excelize.PivotTableOptions) xlsxPivotTableMeta {
	return xlsxPivotTableMeta{
		Name: pivot.Name, DataRange: pivot.DataRange, PivotRange: pivot.PivotTableRange,
		Rows: xlsxPivotFieldMetas(pivot.Rows), Columns: xlsxPivotFieldMetas(pivot.Columns),
		Data: xlsxPivotFieldMetas(pivot.Data), Filters: xlsxPivotFieldMetas(pivot.Filter),
		Style: pivot.PivotTableStyleName, RowTotals: pivot.RowGrandTotals, ColumnTotals: pivot.ColGrandTotals,
	}
}

// xlsxPivotFieldMetas 复制字段并隔离 SelectedItems 切片。
func xlsxPivotFieldMetas(fields []excelize.PivotTableField) []xlsxPivotFieldMeta {
	if len(fields) == 0 {
		return nil
	}
	result := make([]xlsxPivotFieldMeta, 0, len(fields))
	for _, field := range fields {
		result = append(result, xlsxPivotFieldMeta{
			Data: field.Data, Name: field.Name, Subtotal: field.Subtotal,
			SelectedItems: append([]string(nil), field.SelectedItems...),
			ShowValuesAs:  int(field.ShowValuesAs.Type), BaseField: field.ShowValuesAs.BaseField,
			BaseItem: field.ShowValuesAs.BaseItem,
		})
	}
	return result
}

// parseXLSXCellRange 把可带工作表名和绝对引用的 A1 范围转为
// 0 起始、结束开区间坐标。
func parseXLSXCellRange(reference string) (startRow, startCol, endRow, endCol int, ok bool) {
	if separator := strings.LastIndex(reference, "!"); separator >= 0 {
		reference = reference[separator+1:]
	}
	reference = strings.ReplaceAll(strings.TrimSpace(reference), "$", "")
	parts := strings.SplitN(reference, ":", 2)
	if len(parts) == 0 || strings.TrimSpace(parts[0]) == "" {
		return 0, 0, 0, 0, false
	}
	startColumn, startLine, err := excelize.CellNameToCoordinates(strings.TrimSpace(parts[0]))
	if err != nil || startColumn <= 0 || startLine <= 0 {
		return 0, 0, 0, 0, false
	}
	endColumn, endLine := startColumn, startLine
	if len(parts) == 2 {
		endColumn, endLine, err = excelize.CellNameToCoordinates(strings.TrimSpace(parts[1]))
		if err != nil || endColumn <= 0 || endLine <= 0 {
			return 0, 0, 0, 0, false
		}
	}
	return startLine - 1, startColumn - 1, endLine, endColumn, true
}

// findXLSXPivotResultTable 查找包含透视表左上角的同页结果表。
func findXLSXPivotResultTable(doc *DoclingDocument, parent RefItem, pageNo int64, row, col int) (RefItem, bool) {
	for index := range doc.Tables {
		table := &doc.Tables[index]
		if table.Parent == nil || *table.Parent != parent || !xlsxTableContainsCell(table, pageNo, row, col) {
			continue
		}
		return RefItem{Kind: refTables, Idx: int64(index)}, true
	}
	return RefItem{}, false
}

// xlsxPivotSemanticCells 生成未缓存透视表的可检索属性表。
func xlsxPivotSemanticCells(metadata xlsxPivotTableMeta) ([]DoclingTableCell, int64, int64) {
	rows := [][]string{{"属性", "内容"}}
	for _, entry := range [][2]string{
		{"名称", metadata.Name}, {"源数据", metadata.DataRange}, {"结果范围", metadata.PivotRange},
		{"行字段", xlsxPivotFieldsText(metadata.Rows)}, {"列字段", xlsxPivotFieldsText(metadata.Columns)},
		{"值字段", xlsxPivotFieldsText(metadata.Data)}, {"筛选字段", xlsxPivotFieldsText(metadata.Filters)},
	} {
		if strings.TrimSpace(entry[1]) != "" {
			rows = append(rows, []string{entry[0], entry[1]})
		}
	}
	cells := make([]DoclingTableCell, 0, len(rows)*2)
	for rowIndex, row := range rows {
		for colIndex, text := range row {
			cells = append(cells, DoclingTableCell{
				Text: text, RowSpan: 1, ColSpan: 1,
				StartRowOffsetIdx: int64(rowIndex), EndRowOffsetIdx: int64(rowIndex + 1),
				StartColOffsetIdx: int64(colIndex), EndColOffsetIdx: int64(colIndex + 1),
				ColumnHeader: rowIndex == 0,
			})
		}
	}
	return cells, int64(len(rows)), 2
}

// xlsxPivotCaption 构造能直接进入 Markdown、content_list 和向量检索的语义说明。
func xlsxPivotCaption(metadata xlsxPivotTableMeta) string {
	parts := []string{"\u6570\u636e\u900f\u89c6\u8868\uff1a" + strings.TrimSpace(metadata.Name)}
	for _, entry := range [][2]string{
		{"行字段", xlsxPivotFieldsText(metadata.Rows)}, {"列字段", xlsxPivotFieldsText(metadata.Columns)},
		{"值字段", xlsxPivotFieldsText(metadata.Data)}, {"筛选字段", xlsxPivotFieldsText(metadata.Filters)},
	} {
		if entry[1] != "" {
			parts = append(parts, entry[0]+"："+entry[1])
		}
	}
	if len(parts) == 1 && strings.TrimSpace(metadata.Name) == "" {
		return ""
	}
	return strings.Join(parts, "；")
}

// xlsxPivotFieldsText 把字段显示名、聚合方式和已选成员序列化为简短文本。
func xlsxPivotFieldsText(fields []xlsxPivotFieldMeta) string {
	values := make([]string, 0, len(fields))
	for _, field := range fields {
		name := strings.TrimSpace(field.Name)
		if name == "" {
			name = strings.TrimSpace(field.Data)
		}
		var qualifiers []string
		if subtotal := strings.TrimSpace(field.Subtotal); subtotal != "" {
			qualifiers = append(qualifiers, subtotal)
		}
		if len(field.SelectedItems) > 0 {
			qualifiers = append(qualifiers, strings.Join(field.SelectedItems, "、"))
		}
		if len(qualifiers) > 0 {
			name += "（" + strings.Join(qualifiers, "；") + "）"
		}
		if name != "" {
			values = append(values, name)
		}
	}
	return strings.Join(values, "、")
}

// xlsxAnchoredOfficeObject 保存工作表复杂对象及其左上角单元格锚点。
type xlsxAnchoredOfficeObject struct {
	record   officeObjectRecord
	row, col int
}

// appendXLSXOfficeObjects 提取 drawing 中的形状/SmartArt 与 worksheet 中的
// OLE 对象。嵌入对象只保留语义和关系目标，绝不加载或执行二进制载荷。
func appendXLSXOfficeObjects(doc *DoclingDocument, reader *zip.Reader, part xlsxSheetPart, pageNo int64, parent *RefItem, layer ContentLayer, width, height float64) (float64, float64) {
	if reader == nil || part.path == "" || part.chartSheet {
		return width, height
	}
	dir, name := path.Split(part.path)
	sheetXML, _ := readZipFileBytes(reader, part.path)
	relsXML, _ := readZipFileBytes(reader, path.Join(dir, "_rels", name+".rels"))
	drawingTargets := parseOOXMLRelationships(relsXML, "/drawing", path.Clean(dir))
	oleTargets := parseOOXMLRelationships(relsXML, "/oleObject", path.Clean(dir))

	for _, target := range drawingTargets {
		drawingXML, _ := readZipFileBytes(reader, target)
		drawingDir, drawingName := path.Split(target)
		drawingRels, _ := readZipFileBytes(reader, path.Join(drawingDir, "_rels", drawingName+".rels"))
		diagramTargets := parseOOXMLRelationships(drawingRels, "/diagramData", path.Clean(drawingDir))
		for _, object := range parseXLSXDrawingObjects(drawingXML) {
			if officeObjectClassName(object.record.kind) == "smartart" {
				object.record.target = diagramTargets[object.record.relationship]
				if data, err := readZipFileBytes(reader, object.record.target); err == nil && len(data) > 0 {
					object.record.text = collectDrawingMLText(data)
				}
			}
			prov := []ProvenanceItem{xlsxCellProvenance(pageNo, object.row, object.col, object.row+1, object.col+1)}
			addOfficeObjectPicture(doc, object.record, prov, parent, layer)
			if value := float64(object.col + 1); value > width {
				width = value
			}
			if value := float64(object.row + 1); value > height {
				height = value
			}
		}
	}
	for _, record := range parseXLSXOLEObjects(sheetXML) {
		record.target = oleTargets[record.relationship]
		addOfficeObjectPicture(doc, record, []ProvenanceItem{xlsxCellProvenance(pageNo, 0, 0, 1, 1)}, parent, layer)
		if width < 1 {
			width = 1
		}
		if height < 1 {
			height = 1
		}
	}
	return width, height
}

// parseXLSXDrawingObjects 解析 drawing 锚点中的形状和 SmartArt。
func parseXLSXDrawingObjects(data []byte) []xlsxAnchoredOfficeObject {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	var objects []xlsxAnchoredOfficeObject
	for {
		token, err := decoder.Token()
		if err != nil {
			return objects
		}
		start, ok := token.(xml.StartElement)
		if !ok || (start.Name.Local != "twoCellAnchor" && start.Name.Local != "oneCellAnchor" && start.Name.Local != "absoluteAnchor") {
			continue
		}
		raw, err := collectSubTree(decoder, start)
		if err != nil {
			return objects
		}
		objects = append(objects, parseXLSXAnchorObjects(raw)...)
	}
}

// parseXLSXAnchorObjects 解析一个 drawing 锚点中的对象及起始坐标。
func parseXLSXAnchorObjects(data []byte) []xlsxAnchoredOfficeObject {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	row, col := 0, 0
	inFrom, inRow, inCol := false, false, false
	var objects []xlsxAnchoredOfficeObject
	var shape *officeObjectRecord
	inText := false
	for {
		token, err := decoder.Token()
		if err != nil {
			break
		}
		switch value := token.(type) {
		case xml.StartElement:
			switch value.Name.Local {
			case "from":
				inFrom = true
			case "row":
				inRow = inFrom
			case "col":
				inCol = inFrom
			case "sp":
				shape = &officeObjectRecord{kind: "shape"}
			case "cNvPr":
				if shape != nil {
					shape.name = xmlAttrVal(value.Attr, "name")
				}
			case "prstGeom":
				if shape != nil {
					shape.geometry = xmlAttrVal(value.Attr, "prst")
				}
			case "relIds":
				objects = append(objects, xlsxAnchoredOfficeObject{record: officeObjectRecord{
					kind: "smartart", relationship: xmlAttrNS(value.Attr, officeRelNS, "dm"),
				}, row: row, col: col})
			case "t":
				inText = shape != nil
			}
		case xml.EndElement:
			switch value.Name.Local {
			case "from":
				inFrom = false
			case "row":
				inRow = false
			case "col":
				inCol = false
			case "sp":
				if shape != nil {
					shape.text = strings.TrimSpace(shape.text)
					objects = append(objects, xlsxAnchoredOfficeObject{record: *shape, row: row, col: col})
				}
				shape = nil
			case "t":
				inText = false
			}
		case xml.CharData:
			text := strings.TrimSpace(string(value))
			if inRow && text != "" {
				row, _ = strconv.Atoi(text)
			}
			if inCol && text != "" {
				col, _ = strconv.Atoi(text)
			}
			if inText && shape != nil {
				shape.text += string(value)
			}
		}
	}
	return objects
}

// parseXLSXOLEObjects 提取 worksheet 根中的嵌入对象引用。
func parseXLSXOLEObjects(data []byte) []officeObjectRecord {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	var records []officeObjectRecord
	for {
		token, err := decoder.Token()
		if err != nil {
			return records
		}
		start, ok := token.(xml.StartElement)
		if !ok || start.Name.Local != "oleObject" {
			continue
		}
		records = append(records, officeObjectRecord{
			kind: "ole", relationship: xmlAttrNS(start.Attr, officeRelNS, "id"),
			program: xmlAttrVal(start.Attr, "progId"), name: xmlAttrVal(start.Attr, "shapeId"),
		})
	}
}

// parseXLSXSheetParts 把 workbook.xml 的 sheet r:id 映射到 worksheet/
// chartsheet 部件；容器不完整时返回空映射，主解析仍由 excelize 降级完成。
func parseXLSXSheetParts(reader *zip.Reader) map[string]xlsxSheetPart {
	parts := map[string]xlsxSheetPart{}
	if reader == nil {
		return parts
	}
	workbookXML, err := readZipFileBytes(reader, "xl/workbook.xml")
	if err != nil || len(workbookXML) == 0 {
		return parts
	}
	relsXML, _ := readZipFileBytes(reader, "xl/_rels/workbook.xml.rels")
	worksheets := parseOOXMLRelationships(relsXML, "/worksheet", "xl")
	chartSheets := parseOOXMLRelationships(relsXML, "/chartsheet", "xl")
	var workbook struct {
		Sheets []struct {
			Name string `xml:"name,attr"`
			RID  string `xml:"id,attr"`
		} `xml:"sheets>sheet"`
	}
	if err := xml.Unmarshal(workbookXML, &workbook); err != nil {
		return parts
	}
	for _, sheet := range workbook.Sheets {
		if partPath := worksheets[sheet.RID]; partPath != "" {
			parts[sheet.Name] = xlsxSheetPart{path: partPath}
			continue
		}
		if partPath := chartSheets[sheet.RID]; partPath != "" {
			parts[sheet.Name] = xlsxSheetPart{path: partPath, chartSheet: true}
		}
	}
	return parts
}

// convertXLSXChartSheet 把图表工作表表示为 sheet 分组下的
// PictureItem。正常图表通过共享 OOXML helper 生成类型与缓存数据；
// 部件损坏或不支持时保留通用 chart 占位，避免整个 sheet 消失。
func convertXLSXChartSheet(doc *DoclingDocument, workbook *excelize.File, reader *zip.Reader, sheetName string, part xlsxSheetPart, pageNo int64, layer ContentLayer) {
	group := doc.AddSectionGroup(sheetName, nil)
	doc.Groups[group.Idx].Label = GroupLabelSheet
	doc.Groups[group.Idx].ContentLayer = layer
	width, height := appendXLSXCharts(doc, workbook, reader, part, pageNo, &group, layer, 0, 0)
	if len(doc.Groups[group.Idx].Children) == 0 {
		ref := doc.AddPicture(nil, []ProvenanceItem{xlsxCellProvenance(pageNo, 0, 0, 1, 1)}, &group)
		doc.Pictures[ref.Idx].ContentLayer = layer
		doc.Pictures[ref.Idx].Meta = fallbackXLSXChartMeta(sheetName)
		doc.Pictures[ref.Idx].Image = renderOOXMLChartSVG(doc.Pictures[ref.Idx].Meta)
		width, height = 1, 1
	}
	sortXLSXGroupChildren(doc, group)
	doc.AddPage(pageNo, width, height)
}

// fallbackXLSXChartMeta 为无法读取 chart XML 的图表工作表构造
// 最小官方 PictureMeta，ChartData 留为空网格供后续解析增量填充。
func fallbackXLSXChartMeta(title string) *PictureMeta {
	return &PictureMeta{
		Classification: &PictureClassificationMetaField{Predictions: []PictureClassificationPrediction{{
			PredictionMeta: PredictionMeta{CreatedBy: "docparse-xlsx"},
			ClassName:      "chart",
		}}},
		TabularChart: &TabularChartMetaField{
			PredictionMeta: PredictionMeta{CreatedBy: "docparse-xlsx"},
			Title:          title,
			ChartData:      &TableData{TableCells: []DoclingTableCell{}, Orientation: TableOrientation0},
		},
	}
}

// appendXLSXPictures 按锚点单元格顺序产出工作表内图片，
// ImageRef 保留 MIME、像素尺寸、DPI 与内嵌 data URI。
func appendXLSXPictures(doc *DoclingDocument, f *excelize.File, sheetName string, pageNo int64, parent *RefItem, layer ContentLayer, width, height float64) (float64, float64) {
	cells, err := f.GetPictureCells(sheetName)
	if err != nil {
		return width, height
	}
	unique := make(map[string]cellKey, len(cells))
	for _, cell := range cells {
		col, row, coordErr := excelize.CellNameToCoordinates(cell)
		if coordErr == nil {
			unique[cell] = cellKey{row: row - 1, col: col - 1}
		}
	}
	cells = cells[:0]
	for cell := range unique {
		cells = append(cells, cell)
	}
	sort.Slice(cells, func(i, j int) bool {
		a, b := unique[cells[i]], unique[cells[j]]
		if a.row != b.row {
			return a.row < b.row
		}
		return a.col < b.col
	})
	for _, cell := range cells {
		anchor := unique[cell]
		pictures, pictureErr := f.GetPictures(sheetName, cell)
		if pictureErr != nil {
			continue
		}
		for _, picture := range pictures {
			size := &ImageSize{}
			if config, _, decodeErr := image.DecodeConfig(bytes.NewReader(picture.File)); decodeErr == nil {
				size.Width, size.Height = float64(config.Width), float64(config.Height)
			}
			prov := []ProvenanceItem{xlsxCellProvenance(pageNo, anchor.row, anchor.col, anchor.row+1, anchor.col+1)}
			ref := doc.AddPicture(&ImageRef{
				Mimetype: ooxmlMediaMime(picture.Extension),
				Dpi:      defaultImageDPI,
				Size:     size,
				URI:      mediaToDataURI(picture.Extension, picture.File),
			}, prov, parent)
			doc.Pictures[ref.Idx].ContentLayer = layer
			caption := ""
			if picture.Format != nil {
				caption = strings.TrimSpace(picture.Format.AltText)
			}
			if caption != "" {
				captionRef := RefItem{Kind: refTexts, Idx: int64(len(doc.Texts))}
				doc.Texts = append(doc.Texts, TextItem{
					SelfRef: captionRef.String(), Parent: &ref, Children: []RefItem{},
					ContentLayer: layer, Label: LabelCaption, Prov: []ProvenanceItem{},
					Orig: caption, Text: caption,
				})
				doc.Pictures[ref.Idx].Captions = append(doc.Pictures[ref.Idx].Captions, captionRef)
			}
		}
		if value := float64(anchor.col + 1); value > width {
			width = value
		}
		if value := float64(anchor.row + 1); value > height {
			height = value
		}
	}
	return width, height
}

// appendXLSXComments 把单元格批注产出为 notes 层文本，并在
// 坐标命中的 TableItem.comments 中建立细粒度引用。
func appendXLSXComments(doc *DoclingDocument, f *excelize.File, sheetName string, pageNo int64, parent *RefItem, sheetLayer ContentLayer, skipCells map[string]bool, width, height float64) (float64, float64) {
	comments, err := f.GetComments(sheetName)
	if err != nil {
		return width, height
	}
	sort.SliceStable(comments, func(i, j int) bool {
		ic, ir, _ := excelize.CellNameToCoordinates(comments[i].Cell)
		jc, jr, _ := excelize.CellNameToCoordinates(comments[j].Cell)
		if ir != jr {
			return ir < jr
		}
		return ic < jc
	})
	for _, comment := range comments {
		if skipCells[comment.Cell] {
			// Excel 会为 threaded comment 留下兼容旧客户端的占位批注；现代
			// 部件已经提供完整回复链时不重复输出该占位内容。
			continue
		}
		col, row, coordErr := excelize.CellNameToCoordinates(comment.Cell)
		if coordErr != nil {
			continue
		}
		col--
		row--
		text := strings.TrimSpace(comment.Text)
		if text == "" {
			var parts []string
			for _, run := range comment.Paragraph {
				parts = append(parts, run.Text)
			}
			text = strings.TrimSpace(strings.Join(parts, ""))
		}
		if text == "" {
			continue
		}
		prov := []ProvenanceItem{xlsxCellProvenance(pageNo, row, col, row+1, col+1)}
		ref := doc.AddText(LabelText, text, prov, parent)
		noteLayer := LayerNotes
		if sheetLayer == LayerInvisible {
			noteLayer = LayerInvisible
		}
		doc.Texts[ref.Idx].ContentLayer = noteLayer
		doc.Texts[ref.Idx].Meta = BaseMeta{}
		for key, value := range map[string]string{
			"docparse__xlsx_cell":   comment.Cell,
			"docparse__xlsx_author": comment.Author,
		} {
			if raw, marshalErr := json.Marshal(value); marshalErr == nil {
				doc.Texts[ref.Idx].Meta[key] = raw
			}
		}
		for tableIndex := range doc.Tables {
			table := &doc.Tables[tableIndex]
			if xlsxTableContainsCell(table, pageNo, row, col) {
				table.Comments = append(table.Comments, FineRef{RefItem: ref})
				break
			}
		}
		if value := float64(col + 1); value > width {
			width = value
		}
		if value := float64(row + 1); value > height {
			height = value
		}
	}
	return width, height
}

// xlsxTableContainsCell 判断页内单元格坐标是否落在表格 prov 包络中。
func xlsxTableContainsCell(table *TableItem, pageNo int64, row, col int) bool {
	for _, prov := range table.Prov {
		if prov.PageNo == pageNo && prov.BBox != nil &&
			float64(col) >= prov.BBox.L && float64(col) < prov.BBox.R &&
			float64(row) >= prov.BBox.T && float64(row) < prov.BBox.B {
			return true
		}
	}
	return false
}

// xlsxCellProvenance 以 0-based 单元格索引构造 TOPLEFT 坐标 prov。
func xlsxCellProvenance(pageNo int64, row, col, endRow, endCol int) ProvenanceItem {
	return ProvenanceItem{
		PageNo: pageNo,
		BBox: &DoclingBBox{
			L: float64(col), T: float64(row), R: float64(endCol), B: float64(endRow),
			CoordOrigin: CoordOriginTopLeft,
		},
		CharSpan: [2]int64{0, 0},
	}
}

// appendXLSXCharts 解析 sheet 部件指向的 DrawingML，把其中图表
// 按锚点顺序转为 PictureItem，数据与分类由共享 OOXML helper 构造。
func appendXLSXCharts(doc *DoclingDocument, workbook *excelize.File, reader *zip.Reader, part xlsxSheetPart, pageNo int64, parent *RefItem, layer ContentLayer, width, height float64) (float64, float64) {
	if reader == nil || part.path == "" {
		return width, height
	}
	sheetXML, err := readZipFileBytes(reader, part.path)
	if err != nil {
		return width, height
	}
	drawingRels := parseOOXMLRelationshipsFromPart(reader, part.path, sheetXML, "/drawing")
	drawingIDs := make([]string, 0, len(drawingRels))
	for id := range drawingRels {
		drawingIDs = append(drawingIDs, id)
	}
	sort.Strings(drawingIDs)
	for _, drawingID := range drawingIDs {
		drawingPath := drawingRels[drawingID]
		drawingXML, readErr := readZipFileBytes(reader, drawingPath)
		if readErr != nil {
			continue
		}
		charts := parseXLSXDrawingCharts(drawingXML)
		chartTargets := parseOOXMLRelationshipsFromPart(reader, drawingPath, drawingXML, "/chart")
		sort.SliceStable(charts, func(i, j int) bool {
			if charts[i].row != charts[j].row {
				return charts[i].row < charts[j].row
			}
			if charts[i].col != charts[j].col {
				return charts[i].col < charts[j].col
			}
			return charts[i].relID < charts[j].relID
		})
		resolver := newExcelizeChartFormulaResolver(workbook)
		for _, chart := range charts {
			target := chartTargets[chart.relID]
			if target == "" {
				continue
			}
			endRow, endCol := chart.endRow, chart.endCol
			if endRow <= chart.row {
				endRow = chart.row + 1
			}
			if endCol <= chart.col {
				endCol = chart.col + 1
			}
			prov := []ProvenanceItem{xlsxCellProvenance(pageNo, chart.row, chart.col, endRow, endCol)}
			ref, ok := addOOXMLChartPictureFromPart(doc, reader, target, resolver, prov, parent)
			if !ok {
				continue
			}
			doc.Pictures[ref.Idx].ContentLayer = layer
			if doc.Pictures[ref.Idx].Meta != nil && doc.Pictures[ref.Idx].Meta.TabularChart != nil &&
				doc.Pictures[ref.Idx].Meta.TabularChart.Title == "" && part.chartSheet {
				doc.Pictures[ref.Idx].Meta.TabularChart.Title = doc.Groups[parent.Idx].Name
			}
			if value := float64(endCol); value > width {
				width = value
			}
			if value := float64(endRow); value > height {
				height = value
			}
		}
	}
	return width, height
}

// parseOOXMLRelationshipsFromPart 从指定部件的 XML 中筛出关系 id，
// 再从对应 .rels 解析指定类型的目标路径。
func parseOOXMLRelationshipsFromPart(reader *zip.Reader, partPath string, partXML []byte, relTypeSuffix string) map[string]string {
	result := map[string]string{}
	if reader == nil || partPath == "" || len(partXML) == 0 {
		return result
	}
	referenced := map[string]bool{}
	decoder := xml.NewDecoder(bytes.NewReader(partXML))
	for {
		token, err := decoder.Token()
		if err != nil {
			break
		}
		start, ok := token.(xml.StartElement)
		if !ok {
			continue
		}
		if id := xmlAttrNS(start.Attr, officeRelNS, "id"); id != "" {
			referenced[id] = true
		}
	}
	relsPath := path.Join(path.Dir(partPath), "_rels", path.Base(partPath)+".rels")
	relsXML, _ := readZipFileBytes(reader, relsPath)
	for id, target := range parseOOXMLRelationships(relsXML, relTypeSuffix, path.Dir(partPath)) {
		if referenced[id] {
			result[id] = target
		}
	}
	return result
}

// parseXLSXDrawingCharts 解析 DrawingML 中的单元格锚点与 chart r:id；
// chartsheet 使用 absoluteAnchor，其无单元格坐标时回退到 0,0,1,1。
func parseXLSXDrawingCharts(data []byte) []xlsxDrawingChart {
	if len(data) == 0 {
		return nil
	}
	type marker struct {
		Col int `xml:"col"`
		Row int `xml:"row"`
	}
	type chartRef struct {
		RID string `xml:"id,attr"`
	}
	type graphicData struct {
		Chart chartRef `xml:"chart"`
	}
	type graphic struct {
		Data graphicData `xml:"graphicData"`
	}
	type graphicFrame struct {
		Graphic graphic `xml:"graphic"`
	}
	type anchor struct {
		From         *marker      `xml:"from"`
		To           *marker      `xml:"to"`
		GraphicFrame graphicFrame `xml:"graphicFrame"`
	}
	decoder := xml.NewDecoder(bytes.NewReader(data))
	var charts []xlsxDrawingChart
	for {
		token, err := decoder.Token()
		if err != nil {
			break
		}
		start, ok := token.(xml.StartElement)
		if !ok || (start.Name.Local != "oneCellAnchor" && start.Name.Local != "twoCellAnchor" && start.Name.Local != "absoluteAnchor") {
			continue
		}
		raw, collectErr := collectSubTree(decoder, start)
		if collectErr != nil {
			continue
		}
		var item anchor
		if xml.Unmarshal(raw, &item) != nil || item.GraphicFrame.Graphic.Data.Chart.RID == "" {
			continue
		}
		chart := xlsxDrawingChart{relID: item.GraphicFrame.Graphic.Data.Chart.RID, endRow: 1, endCol: 1}
		if item.From != nil {
			chart.row, chart.col = item.From.Row, item.From.Col
			chart.endRow, chart.endCol = chart.row+1, chart.col+1
		}
		if item.To != nil {
			chart.endRow, chart.endCol = item.To.Row, item.To.Col
		}
		charts = append(charts, chart)
	}
	return charts
}

// sortXLSXGroupChildren 按 prov 的顶边、左边与元素类型对 sheet
// 直接子节点稳定排序，使表格/图片/图表/批注的读取顺序可重现。
func sortXLSXGroupChildren(doc *DoclingDocument, group RefItem) {
	children := doc.Groups[group.Idx].Children
	sort.SliceStable(children, func(i, j int) bool {
		iTop, iLeft, iRank, iOK := xlsxRefPosition(doc, children[i])
		jTop, jLeft, jRank, jOK := xlsxRefPosition(doc, children[j])
		if iOK != jOK {
			return iOK
		}
		if iTop != jTop {
			return iTop < jTop
		}
		if iLeft != jLeft {
			return iLeft < jLeft
		}
		return iRank < jRank
	})
	doc.Groups[group.Idx].Children = children
}

// xlsxRefPosition 返回参引元素首个 bbox 的顶边、左边与平局排序权重。
func xlsxRefPosition(doc *DoclingDocument, ref RefItem) (float64, float64, int, bool) {
	var prov []ProvenanceItem
	rank := 3
	switch ref.Kind {
	case refTables:
		if ref.Idx >= 0 && ref.Idx < int64(len(doc.Tables)) {
			prov, rank = doc.Tables[ref.Idx].Prov, 0
		}
	case refPictures:
		if ref.Idx >= 0 && ref.Idx < int64(len(doc.Pictures)) {
			prov, rank = doc.Pictures[ref.Idx].Prov, 1
		}
	case refTexts:
		if ref.Idx >= 0 && ref.Idx < int64(len(doc.Texts)) {
			prov, rank = doc.Texts[ref.Idx].Prov, 2
		}
	}
	if len(prov) == 0 || prov[0].BBox == nil {
		return 0, 0, rank, false
	}
	return prov[0].BBox.T, prov[0].BBox.L, rank, true
}

// parseSheetMerges 解析 sheet 全部合并区域为 0-based 锚点 + 跨度
// （excelize 的起止坐标为 1-based 闭区间，此处转换为行数/列数）。
func parseSheetMerges(f *excelize.File, sheetName string) []sheetMerge {
	raw, err := f.GetMergeCells(sheetName)
	if err != nil {
		return nil
	}
	merges := make([]sheetMerge, 0, len(raw))
	for _, m := range raw {
		startCol, startRow, errStart := excelize.CellNameToCoordinates(m.GetStartAxis())
		endCol, endRow, errEnd := excelize.CellNameToCoordinates(m.GetEndAxis())
		if errStart != nil || errEnd != nil {
			continue
		}
		merges = append(merges, sheetMerge{
			anchorRow: startRow - 1,
			anchorCol: startCol - 1,
			rowSpan:   endRow - startRow + 1,
			colSpan:   endCol - startCol + 1,
		})
	}
	return merges
}

// floodFillRegion 从 start 出发做 4 邻域 BFS（gap_tolerance=0：相邻即连通），
// 返回连通区域全部 cell，并把访问标记写入 visited 防止重复扫描。
func floodFillRegion(start cellKey, occupied, visited map[cellKey]bool) []cellKey {
	region := []cellKey{start}
	visited[start] = true
	for queue := []cellKey{start}; len(queue) > 0; {
		cur := queue[0]
		queue = queue[1:]
		for _, d := range [...][2]int{{0, 1}, {0, -1}, {1, 0}, {-1, 0}} {
			next := cellKey{row: cur.row + d[0], col: cur.col + d[1]}
			if visited[next] || !occupied[next] {
				continue
			}
			visited[next] = true
			region = append(region, next)
			queue = append(queue, next)
		}
	}
	return region
}

// appendXLSXRegion 把一个 BFS 连通区域产出为 DoclingDocument 元素：
// 包围盒内逐格提取（空洞补空 cell 保持矩形、合并影子不产独立 cell），必要时
// 先拆出首行跨列合并标题为 TEXT；prov 以 0-based 单元格索引坐标挂载；
// 返回累计后的页面宽高包络（对齐 _find_page_size：取各 prov bbox 的最大 r/b）。
func appendXLSXRegion(doc *DoclingDocument, region []cellKey, rows [][]string, formulas map[cellKey]string, merges []sheetMerge, pageNo int64, parent *RefItem, layer ContentLayer, width, height float64) (float64, float64) {
	minR, maxR := region[0].row, region[0].row
	minC, maxC := region[0].col, region[0].col
	for _, k := range region {
		if k.row < minR {
			minR = k.row
		}
		if k.row > maxR {
			maxR = k.row
		}
		if k.col < minC {
			minC = k.col
		}
		if k.col > maxC {
			maxC = k.col
		}
	}
	numRows, numCols := int64(maxR-minR+1), int64(maxC-minC+1)

	// 锚点跨度索引与合并成员索引（同锚点多区域时保留首个，对齐 setdefault 语义）
	anchorSpan := make(map[cellKey][2]int64, len(merges))
	mergeMember := make(map[cellKey]bool)
	for _, m := range merges {
		key := cellKey{m.anchorRow, m.anchorCol}
		if _, ok := anchorSpan[key]; !ok {
			anchorSpan[key] = [2]int64{int64(m.rowSpan), int64(m.colSpan)}
		}
		for r := m.anchorRow; r < m.anchorRow+m.rowSpan; r++ {
			for c := m.anchorCol; c < m.anchorCol+m.colSpan; c++ {
				mergeMember[cellKey{r, c}] = true
			}
		}
	}

	// 包围盒内逐格提取：空洞补空 cell 保持矩形，合并影子位置不产独立 cell
	cells := make([]sheetRegionCell, 0, len(region))
	for r := minR; r <= maxR; r++ {
		for c := minC; c <= maxC; c++ {
			key := cellKey{r, c}
			span, isAnchor := anchorSpan[key]
			if !isAnchor {
				if mergeMember[key] {
					continue
				}
				span = [2]int64{1, 1}
			}
			cells = append(cells, sheetRegionCell{
				row:     int64(r - minR),
				col:     int64(c - minC),
				rowSpan: span[0],
				colSpan: span[1],
				text:    sheetCellText(rows, r, c),
				formula: formulas[key],
			})
		}
	}

	// 首行"跨全列合并的单一标题"拆出为 TEXT（挂 sheet 分组下），表格锚点行下移
	title, tableCells, tableRows := splitLeadingTitleCells(cells, numRows, numCols)
	titleOffset := int64(0)
	if title != nil {
		titleOffset = 1
		titleProv := []ProvenanceItem{{
			PageNo: pageNo,
			BBox: &DoclingBBox{
				L:           float64(minC),
				T:           float64(minR),
				R:           float64(minC + int(title.colSpan)),
				B:           float64(minR + 1),
				CoordOrigin: CoordOriginTopLeft,
			},
			CharSpan: [2]int64{0, 0},
		}}
		ref := doc.AddText(LabelText, title.text, titleProv, parent)
		doc.Texts[ref.Idx].ContentLayer = layer
		setXLSXFormulaMeta(&doc.Texts[ref.Idx], title.formula)
		if h := float64(minR + 1); h > height {
			height = h
		}
		if w := float64(minC + int(title.colSpan)); w > width {
			width = w
		}
	}

	dc := make([]DoclingTableCell, 0, len(tableCells))
	for _, c := range tableCells {
		dc = append(dc, DoclingTableCell{
			Text:              c.text,
			RowSpan:           c.rowSpan,
			ColSpan:           c.colSpan,
			StartRowOffsetIdx: c.row,
			EndRowOffsetIdx:   c.row + c.rowSpan,
			StartColOffsetIdx: c.col,
			EndColOffsetIdx:   c.col + c.colSpan,
			ColumnHeader:      c.row == 0,
		})
	}
	prov := []ProvenanceItem{{
		PageNo: pageNo,
		BBox: &DoclingBBox{
			L:           float64(minC),
			T:           float64(minR + int(titleOffset)),
			R:           float64(minC + int(numCols)),
			B:           float64(minR + int(titleOffset) + int(tableRows)),
			CoordOrigin: CoordOriginTopLeft,
		},
		CharSpan: [2]int64{0, 0},
	}}
	ref := doc.AddTable(dc, tableRows, numCols, prov, parent)
	doc.Tables[ref.Idx].ContentLayer = layer
	for index, cell := range tableCells {
		if cell.formula == "" || index >= len(doc.Tables[ref.Idx].Data.TableCells) {
			continue
		}
		textRef := RefItem{Kind: refTexts, Idx: int64(len(doc.Texts))}
		item := TextItem{
			SelfRef: textRef.String(), Parent: &ref, Children: []RefItem{},
			ContentLayer: layer, Label: LabelText, Prov: []ProvenanceItem{},
			Orig: cell.text, Text: cell.text,
		}
		setXLSXFormulaMeta(&item, cell.formula)
		doc.Texts = append(doc.Texts, item)
		doc.Tables[ref.Idx].Data.TableCells[index].Ref = &textRef
	}

	if h := float64(minR + int(titleOffset) + int(tableRows)); h > height {
		height = h
	}
	if w := float64(minC + int(numCols)); w > width {
		width = w
	}
	return width, height
}

// setXLSXFormulaMeta 把公式写入文本节点扩展元数据；空公式不创建 meta。
func setXLSXFormulaMeta(item *TextItem, formula string) {
	if item == nil || strings.TrimSpace(formula) == "" {
		return
	}
	raw, err := json.Marshal(strings.TrimSpace(formula))
	if err != nil {
		return
	}
	if item.Meta == nil {
		item.Meta = BaseMeta{}
	}
	item.Meta[xlsxFormulaMetaKey] = raw
}

// splitLeadingTitleCells 复刻 msexcel_backend._split_leading_section_label：
// 若表首行是"跨全列合并的单一标题"则将其从表格拆出。命中时返回标题 cell、
// 行号整体上移后的剩余 cells 与缩减后的行数；未命中返回 nil 原样。
// 判定条件（全部满足才拆）：
//   - 表至少 2 行 2 列；
//   - 首行非空文本 cell 恰好 1 个；
//   - 该 cell 位于第 0 列、行跨度 1、列跨度 >1 且不超过表列数；
//   - 第二行存在至少 2 个非空单列文本 cell（表头行特征）。
func splitLeadingTitleCells(cells []sheetRegionCell, numRows, numCols int64) (*sheetRegionCell, []sheetRegionCell, int64) {
	if numRows < 2 || numCols < 2 {
		return nil, cells, numRows
	}
	var title *sheetRegionCell
	for i := range cells {
		c := &cells[i]
		if c.row != 0 || strings.TrimSpace(c.text) == "" {
			continue
		}
		if title != nil {
			return nil, cells, numRows // 首行非空文本 cell 多于 1 个，不拆
		}
		title = c
	}
	if title == nil || title.col != 0 || title.rowSpan != 1 || title.colSpan <= 1 || title.colSpan > numCols {
		return nil, cells, numRows
	}
	headerCount := 0
	for i := range cells {
		c := &cells[i]
		if c.row == 1 && strings.TrimSpace(c.text) != "" && c.colSpan == 1 {
			headerCount++
		}
	}
	if headerCount < 2 {
		return nil, cells, numRows
	}
	rest := make([]sheetRegionCell, 0, len(cells)-1)
	for _, c := range cells {
		if c.row == 0 {
			// 首行全部丢弃（含空文本 cell），对齐 Docling 的 row>0 过滤行为
			continue
		}
		c.row--
		rest = append(rest, c)
	}
	return title, rest, numRows - 1
}

// sheetCellText 取 sheet 值网格 (r,c) 的缓存值，越界视为空
// （GetRows 会裁剪行尾空 cell，包围盒补空洞时依赖此兜底）。
func sheetCellText(rows [][]string, r, c int) string {
	if r >= len(rows) || c >= len(rows[r]) {
		return ""
	}
	return rows[r][c]
}

// csvDelimiters 分隔符嗅探候选集，顺序即平局优先级（对齐 csv_backend 的
// _DELIMITERS = ",;\t|:"）。
var csvDelimiters = []rune{',', ';', '\t', '|', ':'}

// ParseCSV 解析 csv 为单 table 文档：整文件一个 table，首行 column_header、
// span 恒 1、num_cols 取最大行宽、列数不齐不报错、不生成 prov；空文件返回
// 空文档不报错。
func ParseCSV(data []byte) (*DoclingDocument, error) {
	doc := NewDoclingDocument("csv")
	// utf-8-sig：剥 Excel/Google Sheets 导出"CSV UTF-8"时写入的 BOM，
	// 避免混入首行首格（对齐 csv_backend 的解码行为）
	content := bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))

	rows, err := parseCSVRows(content, sniffCSVDelimiter(content))
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return doc, nil
	}

	numCols := 0
	for _, row := range rows {
		if len(row) > numCols {
			numCols = len(row)
		}
	}
	cells := make([]DoclingTableCell, 0, len(rows)*numCols)
	for r, row := range rows {
		for c := 0; c < numCols; c++ {
			text := ""
			if c < len(row) {
				text = row[c]
			}
			cells = append(cells, DoclingTableCell{
				Text:              text,
				RowSpan:           1,
				ColSpan:           1,
				StartRowOffsetIdx: int64(r),
				EndRowOffsetIdx:   int64(r + 1),
				StartColOffsetIdx: int64(c),
				EndColOffsetIdx:   int64(c + 1),
				ColumnHeader:      r == 0,
			})
		}
	}
	doc.AddTable(cells, int64(len(rows)), int64(numCols), nil, nil)
	return doc, nil
}

// sniffCSVDelimiter 嗅探分隔符：逐个候选试解析，取"各行字段数一致且列数最多"
// 的候选（一致多列即稳定证据；列数多者优先等价于 csv.Sniffer 的分隔符计数
// 启发式），平局按候选顺序取先者；全部候选失败（单列、数据不足、解析报错）
// 回退逗号。
func sniffCSVDelimiter(data []byte) rune {
	best := ','
	bestCols := 1
	for _, d := range csvDelimiters {
		rows, err := parseCSVRows(data, d)
		if err != nil || len(rows) == 0 {
			continue
		}
		width := len(rows[0])
		if width < 2 {
			continue // 单列无法证明该分隔符有效
		}
		uniform := true
		for _, row := range rows[1:] {
			if len(row) != width {
				uniform = false
				break
			}
		}
		if uniform && width > bestCols {
			best, bestCols = d, width
		}
	}
	return best
}

// parseCSVRows 以指定分隔符解析全部行；FieldsPerRecord=-1 允许列数不齐。
func parseCSVRows(data []byte, delim rune) ([][]string, error) {
	reader := csv.NewReader(bytes.NewReader(data))
	reader.Comma = delim
	reader.FieldsPerRecord = -1
	return reader.ReadAll()
}

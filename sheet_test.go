package docparse

import (
	"bytes"
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	"github.com/ledongthuc/pdf"
	"github.com/xuri/excelize/v2"
)

// ──────────────────────────────────────────────────────────────────────────────
// PDF 行聚合与标题分层（纯函数测试，无需真实 PDF 文件）
// ──────────────────────────────────────────────────────────────────────────────

// TestAssemblePDFLinesByY 验证字符按 Y 坐标聚合成行（页面上方在前）、
// 行内按 X 坐标排序拼接。
func TestAssemblePDFLinesByY(t *testing.T) {
	chars := []pdf.Text{
		{S: "第二行B", Y: 100, X: 50, FontSize: 10.5},
		{S: "标题一", Y: 200, X: 30, FontSize: 16},
		{S: "第二行A", Y: 100, X: 10, FontSize: 10.5},
		{S: "标题二", Y: 180, X: 30, FontSize: 14},
	}
	lines := assemblePDFLines(chars, 1)
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d: %+v", len(lines), lines)
	}
	// 页面上方（Y 大）在前
	if lines[0].Text != "标题一" || lines[0].MaxFontSize != 16 {
		t.Fatalf("line[0] = %+v, want 标题一/16", lines[0])
	}
	if lines[1].Text != "标题二" || lines[1].MaxFontSize != 14 {
		t.Fatalf("line[1] = %+v, want 标题二/14", lines[1])
	}
	// 同行内按 X 升序拼接
	if lines[2].Text != "第二行A第二行B" {
		t.Fatalf("line[2] = %q, want 拼接后的第二行", lines[2].Text)
	}
	if lines[2].PageIdx != 1 {
		t.Fatalf("line page idx = %d, want 1", lines[2].PageIdx)
	}
}

// TestClassifyPDFHeadingLevels 验证字号启发式：
// 1. 正文字号取行字号中位数；2. 超过正文 1.5pt 且长度受限的行为标题；
// 3. 标题字号降序映射层级（最大=1 级，次之=2 级）；4. 长行不误判。
func TestClassifyPDFHeadingLevels(t *testing.T) {
	lines := []pdfLine{
		{Text: "项目技术方案", MaxFontSize: 16, PageIdx: 1},
		{Text: "系统架构说明：本节描述系统的整体架构设计与模块划分，包含部署拓扑。", MaxFontSize: 14, PageIdx: 1},
		{Text: "本文档介绍系统架构，包括前端、后端与存储层的职责与交互方式，用于指导后续的开发与运维工作。", MaxFontSize: 10.5, PageIdx: 1},
		{Text: "前端采用 Vue3 单页应用，通过网关访问后端服务，支持按租户隔离的路由配置与权限控制。", MaxFontSize: 10.5, PageIdx: 1},
		{Text: "后端基于 go-zero 微服务框架，按域拆分服务边界，核心链路同步调用，旁路逻辑异步解耦。", MaxFontSize: 10.5, PageIdx: 1},
		{Text: "存储层使用 PostgreSQL 承载业务数据，Redis 承载缓存与会话，对象存储承载文件与媒体资源。", MaxFontSize: 10.5, PageIdx: 1},
		{Text: "运维侧通过 Prometheus 采集指标，Grafana 展示大盘，告警规则按分级策略推送值班群。", MaxFontSize: 10.5, PageIdx: 1},
		{Text: strings.Repeat("超", 90), MaxFontSize: 20, PageIdx: 1}, // 超长行即使字号大也不是标题
	}
	headings, bodyLines := classifyPDFLines(lines)
	if len(headings) != 2 {
		t.Fatalf("expected 2 headings, got %d: %+v", len(headings), headings)
	}
	// 字号 16 → 1 级，14 → 2 级
	if headings[0].Level != 1 || headings[0].Line.Text != "项目技术方案" {
		t.Fatalf("heading[0] = %+v, want level 1 项目技术方案", headings[0])
	}
	if headings[1].Level != 2 || headings[1].Line.Text != "系统架构说明：本节描述系统的整体架构设计与模块划分，包含部署拓扑。" {
		t.Fatalf("heading[1] = %+v, want level 2 架构标题", headings[1])
	}
	if len(bodyLines) != 6 {
		t.Fatalf("expected 6 body lines (5 body + 1 overlong), got %d", len(bodyLines))
	}
}

// TestBuildPDFContentItemsSectionPath 验证 PDF 元素携带章节路径栈：
// 标题更新路径，正文归入所在章节，正文按页分段保留 PageIdx。
func TestBuildPDFContentItemsSectionPath(t *testing.T) {
	elements := []pdfElement{
		{headingLevel: 1, line: pdfLine{Text: "部署指南", PageIdx: 1}},
		{headingLevel: 2, line: pdfLine{Text: "环境要求", PageIdx: 1}},
		{line: pdfLine{Text: "需要 Docker 环境。", PageIdx: 1}},
		{line: pdfLine{Text: "跨页的补充说明。", PageIdx: 2}},
	}
	items := buildPDFContentItems(elements)
	if len(items) != 4 {
		t.Fatalf("expected 4 items (2 headings + 2 body pages), got %d: %+v", len(items), items)
	}
	if items[0].Text != "部署指南" || items[0].TextLevel != 1 {
		t.Fatalf("item[0] = %+v", items[0])
	}
	if strings.Join(items[2].SectionPath, ">") != "部署指南>环境要求" || items[2].PageIdx != 1 {
		t.Fatalf("item[2] = %+v", items[2])
	}
	// 跨页正文同章节，PageIdx 保留
	if strings.Join(items[3].SectionPath, ">") != "部署指南>环境要求" || items[3].PageIdx != 2 {
		t.Fatalf("item[3] = %+v", items[3])
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// xlsx 多 sheet 与大表拆分
// ──────────────────────────────────────────────────────────────────────────────

// mustBuildTestXLSX 构造多 sheet 的 xlsx 字节流。
func mustBuildTestXLSX(t *testing.T, sheets map[string][][]string) []byte {
	t.Helper()
	f := excelize.NewFile()
	defer func() { _ = f.Close() }()
	first := true
	for name, rows := range sheets {
		if first {
			if err := f.SetSheetName("Sheet1", name); err != nil {
				t.Fatalf("rename sheet: %v", err)
			}
			first = false
		} else if _, err := f.NewSheet(name); err != nil {
			t.Fatalf("new sheet: %v", err)
		}
		for r, row := range rows {
			for c, cell := range row {
				cellName, _ := excelize.CoordinatesToCellName(c+1, r+1)
				if err := f.SetCellValue(name, cellName, cell); err != nil {
					t.Fatalf("set cell: %v", err)
				}
			}
		}
	}
	var buf bytes.Buffer
	if _, err := f.WriteTo(&buf); err != nil {
		t.Fatalf("write xlsx: %v", err)
	}
	return buf.Bytes()
}

// TestParseXLSXPreservesFormulaMetadata 验证没有缓存值的公式单元格仍参与
// 表格区域识别，并通过富单元格引用保存原始公式表达式。
func TestParseXLSXPreservesFormulaMetadata(t *testing.T) {
	file := excelize.NewFile()
	defer func() { _ = file.Close() }()
	if err := file.SetCellValue("Sheet1", "A1", "合计"); err != nil {
		t.Fatal(err)
	}
	if err := file.SetCellFormula("Sheet1", "B1", "SUM(A2:A3)"); err != nil {
		t.Fatal(err)
	}
	for cell, value := range map[string]int{"A2": 1, "A3": 2} {
		if err := file.SetCellValue("Sheet1", cell, value); err != nil {
			t.Fatal(err)
		}
	}
	var buffer bytes.Buffer
	if err := file.Write(&buffer); err != nil {
		t.Fatal(err)
	}

	doc, err := ParseXLSX(buffer.Bytes())
	if err != nil {
		t.Fatalf("parse formula xlsx: %v", err)
	}
	if len(doc.Tables) != 1 || doc.Tables[0].Data == nil || doc.Tables[0].Data.NumCols != 2 {
		t.Fatalf("formula table=%+v", doc.Tables)
	}
	var formulaRef *RefItem
	for index := range doc.Tables[0].Data.TableCells {
		cell := &doc.Tables[0].Data.TableCells[index]
		if cell.StartRowOffsetIdx == 0 && cell.StartColOffsetIdx == 1 {
			formulaRef = cell.Ref
			break
		}
	}
	if formulaRef == nil || formulaRef.Idx < 0 || formulaRef.Idx >= int64(len(doc.Texts)) {
		t.Fatalf("formula cell ref missing: %+v", doc.Tables[0].Data.TableCells)
	}
	formulaItem := doc.Texts[formulaRef.Idx]
	if string(formulaItem.Meta[xlsxFormulaMetaKey]) != `"SUM(A2:A3)"` {
		t.Fatalf("formula metadata=%+v", formulaItem.Meta)
	}
	validateDocumentWithDoclingCore110ForTest(t, "xlsx-formula", doc)
}

// ──────────────────────────────────────────────────────────────────────────────
// xlsx/csv Docling 化解析（BFS 连通区域、合并单元格、标题拆分、分隔符嗅探）
// ──────────────────────────────────────────────────────────────────────────────

// mustBuildMergedXLSX 构造带值 cell 与合并区域的单 sheet xlsx 字节流：
// values 键为 "A1" 形式坐标；merges 为 {"A1","C1"} 形式的合并区域对。
func mustBuildMergedXLSX(t *testing.T, sheet string, values map[string]string, merges [][2]string) []byte {
	t.Helper()
	f := excelize.NewFile()
	defer func() { _ = f.Close() }()
	if err := f.SetSheetName("Sheet1", sheet); err != nil {
		t.Fatalf("rename sheet: %v", err)
	}
	for cell, text := range values {
		if err := f.SetCellValue(sheet, cell, text); err != nil {
			t.Fatalf("set cell %s: %v", cell, err)
		}
	}
	for _, m := range merges {
		if err := f.MergeCell(sheet, m[0], m[1]); err != nil {
			t.Fatalf("merge %v: %v", m, err)
		}
	}
	var buf bytes.Buffer
	if _, err := f.WriteTo(&buf); err != nil {
		t.Fatalf("write xlsx: %v", err)
	}
	return buf.Bytes()
}

// findCellText 按 start 偏移查表格 cell 文本，未命中返回 false。
func findCellText(tb *TableItem, row, col int64) (string, bool) {
	if tb.Data == nil {
		return "", false
	}
	for _, c := range tb.Data.TableCells {
		if c.StartRowOffsetIdx == row && c.StartColOffsetIdx == col {
			return c.Text, true
		}
	}
	return "", false
}

// TestParseXLSXMultiSheetOneTablePerRegion 验证 xlsx 每个 sheet 产出独立
// sheet 分组，sheet 内连续区域各产出一个 table 元素，表格渲染为 GFM
// （含表头与分隔行）。旧断言"SectionPath 携带 sheet 名"已随 Docling 化调整：
// 分组节点在 ToContentList 中只透传子树、不产章节路径，sheet 归属改由
// DoclingDocument 分组树（AddSectionGroup，name=sheet 名）表达。
func TestParseXLSXMultiSheetOneTablePerRegion(t *testing.T) {
	data := mustBuildTestXLSX(t, map[string][][]string{
		"参数说明": {{"参数", "说明"}, {"timeout", "超时时间"}},
		"错误码":  {{"code", "含义"}, {"400", "参数错误"}},
	})
	doc, err := ParseXLSX(data)
	if err != nil {
		t.Fatalf("parse xlsx: %v", err)
	}
	items := ToContentList(doc, SourceGolight)
	var tables []Item
	for _, item := range items {
		if item.Type == ItemTypeTable {
			tables = append(tables, item)
		}
	}
	if len(tables) != 2 {
		t.Fatalf("expected 2 table items (one connected region per sheet), got %d: %+v", len(tables), items)
	}
	for _, tb := range tables {
		if !strings.Contains(tb.TableBody, "| --- |") {
			t.Fatalf("table body missing GFM separator: %q", tb.TableBody)
		}
	}
	// sheet 归属由分组树表达：table 的 parent 指向对应 sheet 分组
	groupName := func(ref *RefItem) string {
		if ref == nil || ref.Kind != refGroups || ref.Idx < 0 || ref.Idx >= int64(len(doc.Groups)) {
			return ""
		}
		return doc.Groups[ref.Idx].Name
	}
	paths := map[string]bool{}
	for i := range doc.Tables {
		paths[groupName(doc.Tables[i].Parent)] = true
	}
	if !paths["参数说明"] || !paths["错误码"] {
		t.Fatalf("sheet groups missing from table parents: %+v", doc.Tables)
	}
	text := JoinItemTexts(items)
	if !strings.Contains(text, "超时时间") || !strings.Contains(text, "参数错误") {
		t.Fatalf("aggregated text missing sheet content: %q", text)
	}
}

// TestParseXLSXLargeSheetSingleTable 验证大 sheet 不再按行数拆分：复刻
// Docling 连通区域语义后整个连续区域产出单个 table（251 行网格，prov 为
// 单元格索引坐标包络）。旧"按 100 行拆段并重复表头"的断言已随复刻规则移除，
// 分段语义下沉至知识库切片层。
func TestParseXLSXLargeSheetSingleTable(t *testing.T) {
	rows := [][]string{{"参数", "说明"}}
	for i := 0; i < 250; i++ {
		rows = append(rows, []string{"key_" + itoa(i), "value"})
	}
	data := mustBuildTestXLSX(t, map[string][][]string{"大表": rows})

	doc, err := ParseXLSX(data)
	if err != nil {
		t.Fatalf("parse xlsx: %v", err)
	}
	if len(doc.Tables) != 1 {
		t.Fatalf("expected single table for one connected region, got %d", len(doc.Tables))
	}
	tb := &doc.Tables[0]
	if tb.Data.NumRows != 251 || tb.Data.NumCols != 2 {
		t.Fatalf("unexpected table dims: %dx%d", tb.Data.NumRows, tb.Data.NumCols)
	}
	if len(tb.Data.TableCells) != 251*2 {
		t.Fatalf("unexpected cell count: %d", len(tb.Data.TableCells))
	}
	// prov 为 0-based 单元格索引坐标包络：{l:0,t:0,r:2,b:251}
	if p := tb.Prov[0].BBox; p.L != 0 || p.T != 0 || p.R != 2 || p.B != 251 {
		t.Fatalf("unexpected prov bbox: %+v", p)
	}
	if txt, ok := findCellText(tb, 0, 0); !ok || txt != "参数" {
		t.Fatalf("header cell wrong: %q", txt)
	}
}

// TestParseXLSXDisconnectedRegionsTwoTables 验证 BFS flood fill 连通区域检测
// （msexcel_backend._find_data_tables 核心规则）：同一 sheet 内相距空行/空列的
// 两个表块被切分为两个 table（而非整 sheet 一个表），各自包围盒、prov 单元格
// 索引坐标与表内偏移正确。
func TestParseXLSXDisconnectedRegionsTwoTables(t *testing.T) {
	// 块 1 占 A1:B2；空行 3 与空列 C 隔断；块 2 占 D4:E5
	data := mustBuildMergedXLSX(t, "分块", map[string]string{
		"A1": "a", "B1": "b",
		"A2": "c", "B2": "d",
		"D4": "e", "E4": "f",
		"D5": "g", "E5": "h",
	}, nil)
	doc, err := ParseXLSX(data)
	if err != nil {
		t.Fatalf("parse xlsx: %v", err)
	}
	if len(doc.Tables) != 2 {
		t.Fatalf("expected 2 tables for disconnected regions, got %d", len(doc.Tables))
	}
	// 产出顺序按锚点行列序：块 1 在前
	t1, t2 := &doc.Tables[0], &doc.Tables[1]
	if t1.Data.NumRows != 2 || t1.Data.NumCols != 2 {
		t.Fatalf("table1 dims: %dx%d", t1.Data.NumRows, t1.Data.NumCols)
	}
	if txt, ok := findCellText(t1, 0, 0); !ok || txt != "a" {
		t.Fatalf("table1 anchor cell: %q", txt)
	}
	if p := t1.Prov[0].BBox; p.L != 0 || p.T != 0 || p.R != 2 || p.B != 2 {
		t.Fatalf("table1 prov bbox: %+v", p)
	}
	if txt, ok := findCellText(t2, 0, 0); !ok || txt != "e" {
		t.Fatalf("table2 anchor cell: %q", txt)
	}
	// D4 为 0-based (3,3)：prov bbox = {l:3,t:3,r:5,b:5}
	if p := t2.Prov[0].BBox; p.L != 3 || p.T != 3 || p.R != 5 || p.B != 5 {
		t.Fatalf("table2 prov bbox: %+v", p)
	}
}

// TestParseXLSXMergedCellSpans 验证合并单元格：锚点承载行列跨度（end 开区间），
// 影子位置不产独立 cell，包围盒空洞补空 cell 保持矩形。
func TestParseXLSXMergedCellSpans(t *testing.T) {
	// B2:C3 合并（值在锚点 B2）；首行 h1/h2 两个非空 cell 避免触发标题拆分
	data := mustBuildMergedXLSX(t, "合并", map[string]string{
		"A1": "h1", "B1": "h2",
		"A2": "x", "B2": "大格",
		"A3": "y",
	}, [][2]string{{"B2", "C3"}})
	doc, err := ParseXLSX(data)
	if err != nil {
		t.Fatalf("parse xlsx: %v", err)
	}
	if len(doc.Texts) != 0 {
		t.Fatalf("no title expected, got %d texts", len(doc.Texts))
	}
	if len(doc.Tables) != 1 {
		t.Fatalf("expected 1 table, got %d", len(doc.Tables))
	}
	tb := &doc.Tables[0]
	// 合并区域把包围盒扩展到 A1:C3（3x3）
	if tb.Data.NumRows != 3 || tb.Data.NumCols != 3 {
		t.Fatalf("table dims: %dx%d", tb.Data.NumRows, tb.Data.NumCols)
	}
	// 合并影子位置不产独立 cell
	for _, pos := range [][2]int64{{1, 2}, {2, 1}, {2, 2}} {
		if _, ok := findCellText(tb, pos[0], pos[1]); ok {
			t.Fatalf("shadow cell at (%d,%d) should not exist", pos[0], pos[1])
		}
	}
	// 包围盒空洞补空 cell 保持矩形：C1 表内 (0,2) 存在且为空
	if txt, ok := findCellText(tb, 0, 2); !ok || txt != "" {
		t.Fatalf("hole cell (0,2) = %q, %v", txt, ok)
	}
	// 锚点跨度 2x2，end 开区间（end = start + span）
	for _, c := range tb.Data.TableCells {
		if c.StartRowOffsetIdx == 1 && c.StartColOffsetIdx == 1 {
			if c.RowSpan != 2 || c.ColSpan != 2 || c.EndRowOffsetIdx != 3 || c.EndColOffsetIdx != 3 {
				t.Fatalf("merged anchor spans: %+v", c)
			}
			if c.Text != "大格" {
				t.Fatalf("merged anchor text: %q", c.Text)
			}
		}
	}
}

// TestParseXLSXLeadingTitleSplit 验证首行"跨全列合并的单一标题"拆出为 TEXT
// 元素（挂 sheet 分组下，prov 为单元格索引坐标），表格锚点行下移、原第二行
// 成为表内首行表头。
func TestParseXLSXLeadingTitleSplit(t *testing.T) {
	data := mustBuildMergedXLSX(t, "带标题", map[string]string{
		"A1": "设备清单",
		"A2": "名称", "B2": "数量", "C2": "状态",
		"A3": "网关", "B3": "3", "C3": "ok",
	}, [][2]string{{"A1", "C1"}})
	doc, err := ParseXLSX(data)
	if err != nil {
		t.Fatalf("parse xlsx: %v", err)
	}
	// 标题拆出为 TEXT 挂 sheet 分组下
	if len(doc.Texts) != 1 || doc.Texts[0].Text != "设备清单" || doc.Texts[0].Label != LabelText {
		t.Fatalf("title text wrong: %+v", doc.Texts)
	}
	// TEXT prov = {l:0,t:0,r:3,b:1}
	if tp := doc.Texts[0].Prov[0].BBox; tp.L != 0 || tp.T != 0 || tp.R != 3 || tp.B != 1 {
		t.Fatalf("title prov bbox: %+v", tp)
	}
	// TEXT 与表格同挂一个 sheet 分组
	if doc.Texts[0].Parent == nil || doc.Tables[0].Parent == nil ||
		doc.Texts[0].Parent.Kind != refGroups || doc.Tables[0].Parent.Kind != refGroups ||
		doc.Texts[0].Parent.Idx != doc.Tables[0].Parent.Idx {
		t.Fatalf("title/table parent wrong: %+v %+v", doc.Texts[0].Parent, doc.Tables[0].Parent)
	}
	// 表格锚点下移：2 行 3 列，表内首行为原第二行表头
	if len(doc.Tables) != 1 {
		t.Fatalf("expected 1 table, got %d", len(doc.Tables))
	}
	tb := &doc.Tables[0]
	if tb.Data.NumRows != 2 || tb.Data.NumCols != 3 {
		t.Fatalf("table dims: %dx%d", tb.Data.NumRows, tb.Data.NumCols)
	}
	if txt, _ := findCellText(tb, 0, 0); txt != "名称" {
		t.Fatalf("table header cell: %q", txt)
	}
	if txt, _ := findCellText(tb, 1, 0); txt != "网关" {
		t.Fatalf("table data cell: %q", txt)
	}
	for _, c := range tb.Data.TableCells {
		if (c.StartRowOffsetIdx == 0) != c.ColumnHeader {
			t.Fatalf("column_header wrong on row %d: %+v", c.StartRowOffsetIdx, c)
		}
	}
	// 表 prov = {l:0,t:1,r:3,b:3}
	if p := tb.Prov[0].BBox; p.L != 0 || p.T != 1 || p.R != 3 || p.B != 3 {
		t.Fatalf("table prov bbox: %+v", p)
	}
}

// TestParseXLSXHiddenSheetUsesInvisibleLayer 验证隐藏 sheet 仍保留页面、
// 分组与内容，但统一标记为 invisible 层，避免丢失不可见数据。
func TestParseXLSXHiddenSheetUsesInvisibleLayer(t *testing.T) {
	f := excelize.NewFile()
	defer func() { _ = f.Close() }()
	if err := f.SetSheetName("Sheet1", "可见"); err != nil {
		t.Fatalf("rename sheet: %v", err)
	}
	if _, err := f.NewSheet("隐藏"); err != nil {
		t.Fatalf("new sheet: %v", err)
	}
	if err := f.SetCellValue("可见", "A1", "v"); err != nil {
		t.Fatalf("set cell: %v", err)
	}
	if err := f.SetCellValue("隐藏", "A1", "hidden"); err != nil {
		t.Fatalf("set cell: %v", err)
	}
	if err := f.SetSheetVisible("隐藏", false); err != nil {
		t.Fatalf("hide sheet: %v", err)
	}
	var buf bytes.Buffer
	if _, err := f.WriteTo(&buf); err != nil {
		t.Fatalf("write xlsx: %v", err)
	}

	doc, err := ParseXLSX(buf.Bytes())
	if err != nil {
		t.Fatalf("parse xlsx: %v", err)
	}
	if len(doc.Groups) != 2 || doc.Groups[0].Name != "可见" || doc.Groups[1].Name != "隐藏" {
		t.Fatalf("groups wrong: %+v", doc.Groups)
	}
	if doc.Groups[0].ContentLayer != LayerBody || doc.Groups[1].ContentLayer != LayerInvisible {
		t.Fatalf("sheet layers wrong: visible=%s hidden=%s", doc.Groups[0].ContentLayer, doc.Groups[1].ContentLayer)
	}
	if len(doc.Tables) != 2 {
		t.Fatalf("expected 2 tables, got %d", len(doc.Tables))
	}
	if txt, ok := findCellText(&doc.Tables[0], 0, 0); !ok || txt != "v" {
		t.Fatalf("visible sheet cell: %q", txt)
	}
	if txt, ok := findCellText(&doc.Tables[1], 0, 0); !ok || txt != "hidden" || doc.Tables[1].ContentLayer != LayerInvisible {
		t.Fatalf("hidden sheet table wrong: text=%q layer=%s", txt, doc.Tables[1].ContentLayer)
	}
	if _, ok := doc.Pages["2"]; !ok || doc.Meta == nil || doc.Meta.PageCount != 2 {
		t.Fatalf("hidden sheet page missing: pages=%+v meta=%+v", doc.Pages, doc.Meta)
	}
}

// TestParseXLSXPicturesCommentsAndCoordinateOrder 验证工作表内图片和批注均保留：
// 图片带资产信息和单元格坐标，批注进入 notes 层并关联表格；
// sheet 子节点最终按对象左上角坐标稳定排序，不受元素提取先后影响。
func TestParseXLSXPicturesCommentsAndCoordinateOrder(t *testing.T) {
	f := excelize.NewFile()
	defer func() { _ = f.Close() }()
	if err := f.SetCellValue("Sheet1", "D4", "数据"); err != nil {
		t.Fatalf("set cell: %v", err)
	}
	if err := f.AddPictureFromBytes("Sheet1", "A1", &excelize.Picture{
		Extension: ".png",
		File:      minimalPNG,
		Format:    &excelize.GraphicOptions{AltText: "设备图"},
	}); err != nil {
		t.Fatalf("add picture: %v", err)
	}
	if err := f.AddComment("Sheet1", excelize.Comment{Cell: "D4", Author: "审核人", Text: "需要确认"}); err != nil {
		t.Fatalf("add comment: %v", err)
	}
	var buf bytes.Buffer
	if _, err := f.WriteTo(&buf); err != nil {
		t.Fatalf("write xlsx: %v", err)
	}

	doc, err := ParseXLSX(buf.Bytes())
	if err != nil {
		t.Fatalf("parse xlsx: %v", err)
	}
	if len(doc.Pictures) != 1 || doc.Pictures[0].Image == nil {
		t.Fatalf("picture missing: %+v", doc.Pictures)
	}
	picture := &doc.Pictures[0]
	if picture.Image.Mimetype != "image/png" || picture.Image.Size == nil ||
		picture.Image.Size.Width != 1 || picture.Image.Size.Height != 1 ||
		!strings.HasPrefix(picture.Image.URI, "data:image/png;base64,") {
		t.Fatalf("picture asset wrong: %+v", picture.Image)
	}
	if len(picture.Prov) != 1 || picture.Prov[0].BBox.L != 0 || picture.Prov[0].BBox.T != 0 {
		t.Fatalf("picture coordinate wrong: %+v", picture.Prov)
	}
	if len(picture.Captions) != 1 || doc.Texts[picture.Captions[0].Idx].Text != "设备图" {
		t.Fatalf("picture caption wrong: %+v", picture.Captions)
	}

	var note *TextItem
	for i := range doc.Texts {
		if doc.Texts[i].ContentLayer == LayerNotes {
			note = &doc.Texts[i]
			break
		}
	}
	if note == nil || note.Text != "需要确认" || len(note.Prov) != 1 ||
		note.Prov[0].BBox.L != 3 || note.Prov[0].BBox.T != 3 {
		t.Fatalf("comment note wrong: %+v", note)
	}
	if len(doc.Tables) != 1 || len(doc.Tables[0].Comments) != 1 || doc.Tables[0].Comments[0].String() != note.SelfRef {
		t.Fatalf("comment reference missing: %+v", doc.Tables)
	}

	if len(doc.Groups) != 1 || len(doc.Groups[0].Children) < 3 {
		t.Fatalf("sheet children missing: %+v", doc.Groups)
	}
	children := doc.Groups[0].Children
	if children[0].Kind != refPictures || children[1].Kind != refTables || children[2].Kind != refTexts {
		t.Fatalf("coordinate order wrong: %+v", children)
	}
}

// TestParseXLSXEmbeddedChartUsesPictureMeta 验证普通工作表的嵌入图表
// 经 DrawingML 锚点解析后产出 PictureItem，且使用实际图表类型分类。
func TestParseXLSXEmbeddedChartUsesPictureMeta(t *testing.T) {
	f := excelize.NewFile()
	defer func() { _ = f.Close() }()
	for cell, value := range map[string]any{"A1": "月份", "B1": "销量", "A2": "1月", "B2": 10} {
		if err := f.SetCellValue("Sheet1", cell, value); err != nil {
			t.Fatalf("set cell %s: %v", cell, err)
		}
	}
	if err := f.AddChart("Sheet1", "D4", &excelize.Chart{
		Type: excelize.Line,
		Series: []excelize.ChartSeries{{
			Name:       "Sheet1!$B$1",
			Categories: "Sheet1!$A$2:$A$2",
			Values:     "Sheet1!$B$2:$B$2",
		}},
	}); err != nil {
		t.Fatalf("add chart: %v", err)
	}
	var buf bytes.Buffer
	if _, err := f.WriteTo(&buf); err != nil {
		t.Fatalf("write xlsx: %v", err)
	}

	doc, err := ParseXLSX(buf.Bytes())
	if err != nil {
		t.Fatalf("parse xlsx: %v", err)
	}
	if len(doc.Pictures) != 1 || doc.Pictures[0].Meta == nil || doc.Pictures[0].Meta.Classification == nil {
		t.Fatalf("embedded chart missing: %+v", doc.Pictures)
	}
	if got := doc.Pictures[0].Meta.Classification.Predictions[0].ClassName; got != "line_chart" {
		t.Fatalf("chart class = %q", got)
	}
	if len(doc.Pictures[0].Prov) != 1 || doc.Pictures[0].Prov[0].BBox.L != 3 || doc.Pictures[0].Prov[0].BBox.T != 3 {
		t.Fatalf("chart anchor wrong: %+v", doc.Pictures[0].Prov)
	}
	if doc.Pictures[0].Image == nil || doc.Pictures[0].Meta.TabularChart == nil {
		t.Fatalf("chart preview/data missing: %+v", doc.Pictures[0])
	}
	assertChartTable(t, doc.Pictures[0].Meta.TabularChart.ChartData, [][]string{
		{"类别", "销量"}, {"1月", "10"},
	})
	if len(doc.Groups) != 1 || len(doc.Groups[0].Children) != 2 ||
		doc.Groups[0].Children[0].Kind != refTables || doc.Groups[0].Children[1].Kind != refPictures {
		t.Fatalf("chart coordinate order wrong: %+v", doc.Groups)
	}
	validateDocumentWithDoclingCore110ForTest(t, "xlsx-chart-formula", doc)
}

// TestParseXLSXChartSheetBasicPicture 验证图表工作表不被 GetRows 失败路径
// 静默丢弃，至少产出一个可定位、可后续注入 chart data 的 PictureItem。
func TestParseXLSXChartSheetBasicPicture(t *testing.T) {
	f := excelize.NewFile()
	defer func() { _ = f.Close() }()
	if err := f.AddChartSheet("销量图", &excelize.Chart{
		Type:  excelize.Bar,
		Title: excelize.ChartTitle{Paragraph: []excelize.RichTextRun{{Text: "销量图"}}},
	}); err != nil {
		t.Fatalf("add chart sheet: %v", err)
	}
	var buf bytes.Buffer
	if _, err := f.WriteTo(&buf); err != nil {
		t.Fatalf("write xlsx: %v", err)
	}

	doc, err := ParseXLSX(buf.Bytes())
	if err != nil {
		t.Fatalf("parse xlsx: %v", err)
	}
	if len(doc.Groups) != 2 || doc.Groups[1].Name != "销量图" || doc.Groups[1].Label != GroupLabelSheet {
		t.Fatalf("chart sheet group missing: %+v", doc.Groups)
	}
	if len(doc.Pictures) != 1 || doc.Pictures[0].Parent == nil || doc.Pictures[0].Parent.Idx != 1 {
		t.Fatalf("chart sheet picture missing: %+v", doc.Pictures)
	}
	meta := doc.Pictures[0].Meta
	if meta == nil || meta.Classification == nil || len(meta.Classification.Predictions) != 1 ||
		meta.Classification.Predictions[0].ClassName != "bar_chart" {
		t.Fatalf("chart classification missing: %+v", meta)
	}
	if meta.TabularChart == nil || meta.TabularChart.Title != "销量图" || meta.TabularChart.ChartData == nil {
		t.Fatalf("chart data integration point missing: %+v", meta)
	}
	if _, ok := doc.Pages["2"]; !ok {
		t.Fatalf("chart sheet page missing: %+v", doc.Pages)
	}
}

// TestParseXLSXPreservesPivotTableSemantics 验证数据透视表的名称、源区域、
// 行列字段、值聚合与筛选项通过官方 TableItem 与 meta 保留。
func TestParseXLSXPreservesPivotTableSemantics(t *testing.T) {
	f := excelize.NewFile()
	defer func() { _ = f.Close() }()
	rows := [][]any{
		{"区域", "产品", "年份", "销售额"},
		{"华东", "网关", "2025", 10},
		{"华东", "传感器", "2025", 20},
		{"华南", "网关", "2024", 30},
		{"华南", "传感器", "2025", 40},
	}
	for rowIndex, row := range rows {
		for colIndex, value := range row {
			cell, _ := excelize.CoordinatesToCellName(colIndex+1, rowIndex+1)
			if err := f.SetCellValue("Sheet1", cell, value); err != nil {
				t.Fatalf("set %s: %v", cell, err)
			}
		}
	}
	if err := f.AddPivotTable(&excelize.PivotTableOptions{
		DataRange: "Sheet1!A1:D5", PivotTableRange: "Sheet1!F3:K10", Name: "销售汇总",
		Rows:    []excelize.PivotTableField{{Data: "区域"}},
		Columns: []excelize.PivotTableField{{Data: "产品"}},
		Data:    []excelize.PivotTableField{{Data: "销售额", Name: "销售总额", Subtotal: "Sum"}},
		Filter:  []excelize.PivotTableField{{Data: "年份", SelectedItems: []string{"2025"}}},
	}); err != nil {
		t.Fatalf("add pivot table: %v", err)
	}
	var buffer bytes.Buffer
	if _, err := f.WriteTo(&buffer); err != nil {
		t.Fatalf("write pivot xlsx: %v", err)
	}

	doc, err := ParseXLSX(buffer.Bytes())
	if err != nil {
		t.Fatalf("parse pivot xlsx: %v", err)
	}
	var pivotTable *TableItem
	for index := range doc.Tables {
		if len(doc.Tables[index].Meta[xlsxPivotTableMetaKey]) > 0 {
			pivotTable = &doc.Tables[index]
			break
		}
	}
	if pivotTable == nil || len(pivotTable.Captions) != 1 {
		t.Fatalf("pivot table semantics missing: %+v", doc.Tables)
	}
	var metadata xlsxPivotTableMeta
	if err := json.Unmarshal(pivotTable.Meta[xlsxPivotTableMetaKey], &metadata); err != nil ||
		metadata.Name != "销售汇总" || metadata.DataRange != "Sheet1!A1:D5" ||
		len(metadata.Rows) != 1 || metadata.Rows[0].Data != "区域" ||
		len(metadata.Columns) != 1 || metadata.Columns[0].Data != "产品" ||
		len(metadata.Data) != 1 || metadata.Data[0].Subtotal != "Sum" ||
		len(metadata.Filters) != 1 || len(metadata.Filters[0].SelectedItems) != 1 || metadata.Filters[0].SelectedItems[0] != "2025" {
		t.Fatalf("pivot metadata=%+v err=%v", metadata, err)
	}
	caption := doc.Texts[pivotTable.Captions[0].Idx].Text
	if !strings.Contains(caption, "数据透视表：销售汇总") || !strings.Contains(caption, "销售总额（Sum）") || !strings.Contains(caption, "2025") {
		t.Fatalf("pivot caption=%q", caption)
	}
	validateDocumentWithDoclingCore110ForTest(t, "xlsx-pivot-table", doc)
}

// TestParseCSVDelimiterSniffing 验证分隔符嗅探（csv_backend 核心规则）：
// 逗号/分号/制表符三种方言都能按"各行字段数一致且列数最多"的候选正确识别，
// span 恒 1、首行 column_header、不生成 prov。
func TestParseCSVDelimiterSniffing(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"逗号", "名称,数量\n网关,3\n\"摄像头,红外\",2\n"},
		{"分号", "名称;数量\n网关;3\n摄像头;2\n"},
		{"制表符", "名称\t数量\n网关\t3\n摄像头\t2\n"},
	}
	for _, tc := range cases {
		doc, err := ParseCSV([]byte(tc.body))
		if err != nil {
			t.Fatalf("%s: parse csv: %v", tc.name, err)
		}
		if len(doc.Tables) != 1 {
			t.Fatalf("%s: expected 1 table, got %d", tc.name, len(doc.Tables))
		}
		tb := &doc.Tables[0]
		if tb.Data.NumRows != 3 || tb.Data.NumCols != 2 {
			t.Fatalf("%s: table dims: %dx%d", tc.name, tb.Data.NumRows, tb.Data.NumCols)
		}
		if txt, _ := findCellText(tb, 1, 1); txt != "3" {
			t.Fatalf("%s: cell (1,1) = %q", tc.name, txt)
		}
		for _, c := range tb.Data.TableCells {
			if (c.StartRowOffsetIdx == 0) != c.ColumnHeader {
				t.Fatalf("%s: column_header wrong on row %d", tc.name, c.StartRowOffsetIdx)
			}
			if c.RowSpan != 1 || c.ColSpan != 1 {
				t.Fatalf("%s: span should be 1: %+v", tc.name, c)
			}
		}
		// csv 不生成 prov
		if len(tb.Prov) != 0 {
			t.Fatalf("%s: csv should not have prov", tc.name)
		}
	}
	// 引号内的逗号不分列：逗号方言第 3 行首格为完整字段
	doc, err := ParseCSV([]byte(cases[0].body))
	if err != nil {
		t.Fatalf("parse csv: %v", err)
	}
	if txt, _ := findCellText(&doc.Tables[0], 2, 0); txt != "摄像头,红外" {
		t.Fatalf("quoted cell = %q", txt)
	}
}

// TestParseCSVQuotedNewlineAndStableWidth 验证引号内换行作为单个字段保存，
// 短行按最大列宽补空单元格，保证官方表格网格坐标稳定。
func TestParseCSVQuotedNewlineAndStableWidth(t *testing.T) {
	doc, err := ParseCSV([]byte("\xef\xbb\xbf名称,说明,备注\n网关,\"第一行\n第二行\",正常\n传感器,在线\n"))
	if err != nil {
		t.Fatalf("ParseCSV: %v", err)
	}
	if len(doc.Tables) != 1 {
		t.Fatalf("tables = %d", len(doc.Tables))
	}
	table := &doc.Tables[0]
	if table.Data.NumRows != 3 || table.Data.NumCols != 3 || len(table.Data.TableCells) != 9 {
		t.Fatalf("table grid unstable: rows=%d cols=%d cells=%d", table.Data.NumRows, table.Data.NumCols, len(table.Data.TableCells))
	}
	if text, ok := findCellText(table, 1, 1); !ok || text != "第一行\n第二行" {
		t.Fatalf("quoted newline cell = %q, ok=%v", text, ok)
	}
	if text, ok := findCellText(table, 2, 2); !ok || text != "" {
		t.Fatalf("padded cell = %q, ok=%v", text, ok)
	}
	if text, _ := findCellText(table, 0, 0); text != "名称" {
		t.Fatalf("BOM leaked into header: %q", text)
	}
}

// TestParseCSVEmptyAndFallback 验证空文件返回空文档不报错（csv_backend 空文件
// 警告语义）；无法判别分隔符时（单列）回退逗号正常产表。
func TestParseCSVEmptyAndFallback(t *testing.T) {
	doc, err := ParseCSV(nil)
	if err != nil {
		t.Fatalf("empty csv: %v", err)
	}
	if len(doc.Tables) != 0 || len(doc.Texts) != 0 {
		t.Fatalf("empty csv should produce empty document: %+v", doc)
	}

	// 单列：所有候选分隔符都解析为单列，回退逗号
	doc, err = ParseCSV([]byte("单列\n内容\n"))
	if err != nil {
		t.Fatalf("single column csv: %v", err)
	}
	if len(doc.Tables) != 1 {
		t.Fatalf("expected 1 table, got %d", len(doc.Tables))
	}
	if tb := &doc.Tables[0]; tb.Data.NumRows != 2 || tb.Data.NumCols != 1 {
		t.Fatalf("single column dims: %dx%d", tb.Data.NumRows, tb.Data.NumCols)
	}
}

// TestParseCSVLargeRowsSingleTable 验证 csv 大表不再按行数拆分（旧"100 行
// 拆段并重复表头"断言随复刻规则移除，分段语义下沉至知识库切片层）：
// 250 行数据产出单个 251 行网格 table。
func TestParseCSVLargeRowsSingleTable(t *testing.T) {
	var sb strings.Builder
	sb.WriteString("参数,说明\n")
	for i := 0; i < 250; i++ {
		sb.WriteString("key_" + itoa(i) + ",value\n")
	}
	doc, err := ParseCSV([]byte(sb.String()))
	if err != nil {
		t.Fatalf("parse csv: %v", err)
	}
	if len(doc.Tables) != 1 {
		t.Fatalf("expected single table, got %d", len(doc.Tables))
	}
	if tb := &doc.Tables[0]; tb.Data.NumRows != 251 || tb.Data.NumCols != 2 {
		t.Fatalf("table dims: %dx%d", tb.Data.NumRows, tb.Data.NumCols)
	}
}

// itoa 轻量 int 转字符串。
func itoa(i int) string {
	return strconv.Itoa(i)
}

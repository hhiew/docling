package docparse

import (
	"encoding/json"
	"strings"
	"testing"
)

// 构造带章节层级与各模态元素的示例文档。
func buildSampleDoclingDoc() *DoclingDocument {
	doc := NewDoclingDocument("sample")
	h1 := doc.AddHeading(1, "1 引言", nil, nil)
	p1 := doc.AddText(LabelText, "引言正文", []ProvenanceItem{{
		PageNo: 1, BBox: &DoclingBBox{L: 1, T: 2, R: 3, B: 4}, CharSpan: [2]int64{0, 4},
	}}, &h1)
	doc.AddFormula("E=mc^2", nil, &h1)
	doc.AddCode("print(1)", "python", nil, &h1)
	h2 := doc.AddHeading(2, "1.1 背景", nil, &h1)
	doc.AddText(LabelText, "背景正文", nil, &h2)
	cells := []DoclingTableCell{
		{StartRowOffsetIdx: 0, EndRowOffsetIdx: 1, StartColOffsetIdx: 0, EndColOffsetIdx: 1, Text: "列A", ColumnHeader: true},
		{StartRowOffsetIdx: 0, EndRowOffsetIdx: 1, StartColOffsetIdx: 1, EndColOffsetIdx: 2, Text: "列B", ColumnHeader: true},
		{StartRowOffsetIdx: 1, EndRowOffsetIdx: 2, StartColOffsetIdx: 0, EndColOffsetIdx: 1, Text: "a"},
		{StartRowOffsetIdx: 1, EndRowOffsetIdx: 2, StartColOffsetIdx: 1, EndColOffsetIdx: 2, Text: "b|c"},
	}
	doc.AddTable(cells, 2, 2, []ProvenanceItem{{PageNo: 2}}, &h2)
	_ = p1
	return doc
}

// 简化：先序扁平化 + 章节路径 + label 映射 + prov 基准转换。
func TestToContentListBasics(t *testing.T) {
	doc := buildSampleDoclingDoc()
	items := ToContentList(doc, SourceGolight)
	if len(items) != 7 {
		t.Fatalf("元素数应 7（2标题+2正文+公式+代码+表格），实际 %d: %+v", len(items), items)
	}
	// 顺序与类型（树先序：h1标题/引言正文/公式/代码/h2标题/背景正文/表格）
	expectTypes := []ItemType{ItemTypeText, ItemTypeText, ItemTypeEquation, ItemTypeText, ItemTypeText, ItemTypeText, ItemTypeTable}
	for i, et := range expectTypes {
		if items[i].Type != et {
			t.Errorf("items[%d].Type = %s, 期望 %s", i, items[i].Type, et)
		}
	}
	// 章节路径：标题自身含自己（对齐 go_light 旧行为），正文含祖先链
	if strings.Join(items[0].SectionPath, ">") != "1 引言" {
		t.Errorf("一级标题 SectionPath 应含自身: %v", items[0].SectionPath)
	}
	if items[1].Text != "引言正文" || strings.Join(items[1].SectionPath, ">") != "1 引言" {
		t.Errorf("引言正文路径错误: %q / %v", items[1].Text, items[1].SectionPath)
	}
	if items[5].Text != "背景正文" || strings.Join(items[5].SectionPath, ">") != "1 引言>1.1 背景" {
		t.Errorf("背景正文路径错误: %q / %v", items[5].Text, items[5].SectionPath)
	}
	if strings.Join(items[4].SectionPath, ">") != "1 引言>1.1 背景" {
		t.Errorf("二级标题 SectionPath 应含自身与祖先: %v", items[4].SectionPath)
	}
	// 标题层级
	if items[0].TextLevel != 1 || items[4].TextLevel != 2 {
		t.Errorf("标题层级错误: %d %d", items[0].TextLevel, items[4].TextLevel)
	}
	// 公式
	if items[2].LaTeX != "E=mc^2" {
		t.Errorf("公式 LaTeX 错误: %q", items[2].LaTeX)
	}
	// prov → PageIdx 0-based + BBox
	if items[1].PageIdx != 0 || items[1].BBox == nil || items[1].BBox.Left != 1 {
		t.Errorf("prov 映射错误: %+v", items[1])
	}
	if items[6].PageIdx != 1 {
		t.Errorf("表格 PageIdx 应为 page_no-1: %d", items[6].PageIdx)
	}
	// OrderIndex 连续、Source 填充
	for i, it := range items {
		if it.OrderIndex != int64(i) {
			t.Errorf("OrderIndex 断链: items[%d]=%d", i, it.OrderIndex)
		}
		if it.Source != SourceGolight {
			t.Errorf("Source 未填充: items[%d]=%q", i, it.Source)
		}
	}
	// 表格 Markdown：表头分隔行 + 管道转义
	if !strings.Contains(items[6].TableBody, "| 列A | 列B |") ||
		!strings.Contains(items[6].TableBody, "b\\|c") {
		t.Errorf("表格 Markdown 渲染错误:\n%s", items[6].TableBody)
	}
}

// TestToContentListChartDataAdjacent 验证图表仍以 PictureItem 派生图片项，同时
// 紧邻追加可检索的 chart-data 表格项；DoclingDocument 本身不新增 TableItem。
func TestToContentListChartDataAdjacent(t *testing.T) {
	doc := NewDoclingDocument("chart-content-list")
	ref := doc.AddPicture(nil, []ProvenanceItem{{PageNo: 2}}, nil)
	doc.Pictures[ref.Idx].Meta = &PictureMeta{TabularChart: &TabularChartMetaField{
		Title: "季度销量",
		ChartData: &TableData{NumRows: 2, NumCols: 2, TableCells: []DoclingTableCell{
			{Text: "季度", RowSpan: 1, ColSpan: 1, StartRowOffsetIdx: 0, EndRowOffsetIdx: 1, StartColOffsetIdx: 0, EndColOffsetIdx: 1, ColumnHeader: true},
			{Text: "销量", RowSpan: 1, ColSpan: 1, StartRowOffsetIdx: 0, EndRowOffsetIdx: 1, StartColOffsetIdx: 1, EndColOffsetIdx: 2, ColumnHeader: true},
			{Text: "一季度", RowSpan: 1, ColSpan: 1, StartRowOffsetIdx: 1, EndRowOffsetIdx: 2, StartColOffsetIdx: 0, EndColOffsetIdx: 1},
			{Text: "10", RowSpan: 1, ColSpan: 1, StartRowOffsetIdx: 1, EndRowOffsetIdx: 2, StartColOffsetIdx: 1, EndColOffsetIdx: 2},
		}},
	}}

	items := ToContentList(doc, SourceGolight)
	if len(items) != 2 || items[0].Type != ItemTypeImage || items[1].Type != ItemTypeTable {
		t.Fatalf("items = %+v, want adjacent image and table", items)
	}
	if items[0].ImageCaption != "季度销量" || items[0].PageIdx != 1 ||
		items[1].TableCaption != "季度销量" || !strings.Contains(items[1].TableBody, "一季度") {
		t.Fatalf("chart items = %+v, want title/provenance/chart data", items)
	}
	if items[0].OrderIndex != 0 || items[1].OrderIndex != 1 ||
		items[0].Source != SourceGolight || items[1].Source != SourceGolight {
		t.Fatalf("chart item ordering/source = %+v", items)
	}
	if len(doc.Tables) != 0 || len(doc.Pictures) != 1 {
		t.Fatalf("docling collections changed: pictures=%d tables=%d", len(doc.Pictures), len(doc.Tables))
	}
	chunks := ChunkContentList(items, ContentChunkOptions{})
	if len(chunks) != 2 || chunks[0].Content != "季度销量" ||
		!strings.Contains(chunks[1].Content, "一季度") {
		t.Fatalf("chart searchable chunks = %+v", chunks)
	}
}

// TestToContentListNormalizesBBoxOrigin 验证 content_list 不扩展历史 BBox 协议，
// 无论 Docling provenance 使用 TOPLEFT 或 BOTTOMLEFT，均保存数值较小的 top
// 与数值较大的 bottom，保证既有知识库 bbox 并集逻辑继续有效。
func TestToContentListNormalizesBBoxOrigin(t *testing.T) {
	doc := NewDoclingDocument("bbox")
	doc.AddText(LabelText, "正文", []ProvenanceItem{{
		PageNo: 1,
		BBox: &DoclingBBox{
			L: 10, T: 100, R: 80, B: 40, CoordOrigin: CoordOriginBottomLeft,
		},
	}}, nil)
	items := ToContentList(doc, SourceGolight)
	if len(items) != 1 || items[0].BBox == nil ||
		*items[0].BBox != (BBox{Left: 10, Top: 40, Right: 80, Bottom: 100}) {
		t.Fatalf("normalized bbox = %+v", items)
	}
}

// ParseDoclingDocument 兼容线上旧版形态：heading-N label 与 latex 字段。
func TestParseDoclingDocumentLegacy(t *testing.T) {
	raw := `{
		"schema_name": "DoclingDocument", "version": "1.0.0", "name": "legacy",
		"body": {"self_ref": "#/body", "children": [{"$ref": "#/texts/0"}, {"$ref": "#/texts/1"}]},
		"texts": [
			{"self_ref": "#/texts/0", "label": "heading-1", "children": [],
			 "content_layer": "body", "text": "章节标题", "orig": "章节标题"},
			{"self_ref": "#/texts/1", "label": "formula", "children": [],
			 "content_layer": "body", "text": "", "orig": "x=y", "latex": "x=y"}
		],
		"pictures": [], "tables": [], "groups": [],
		"key_value_items": [], "form_items": [], "pages": {}
	}`
	doc, err := ParseDoclingDocument(json.RawMessage(raw))
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if doc.Texts[0].Label != LabelSectionHeader || doc.Texts[0].TextLevel != 1 {
		t.Errorf("heading-1 归一化失败: %+v", doc.Texts[0])
	}
	if doc.Texts[1].Label != LabelFormula {
		t.Errorf("formula label 错误: %q", doc.Texts[1].Label)
	}
	// 归一化后 Text 回填，简化器产出 equation
	items := ToContentList(doc, SourceDocling)
	if len(items) != 2 || items[1].Type != ItemTypeEquation || items[1].LaTeX != "x=y" {
		t.Errorf("旧版公式简化错误: %+v", items)
	}
	if items[0].Source != SourceDocling {
		t.Errorf("Source 应为 docling: %q", items[0].Source)
	}
}

// 空输入与字符串包裹的 json_content 兼容。
func TestParseDoclingDocumentEdge(t *testing.T) {
	if _, err := ParseDoclingDocument(nil); err == nil {
		t.Errorf("空输入应报错")
	}
	inner := `{"schema_name":"DoclingDocument","version":"1.10.0","name":"x",
		"body":{"self_ref":"#/body","children":[]},"groups":[],"texts":[],
		"pictures":[],"tables":[],"key_value_items":[],"form_items":[],"pages":{}}`
	wrapped, _ := json.Marshal(inner)
	doc, err := ParseDoclingDocument(wrapped)
	if err != nil {
		t.Fatalf("字符串包裹形态解析失败: %v", err)
	}
	if len(doc.Texts) != 0 {
		t.Errorf("空文档不应有 texts")
	}
	if items := ToContentList(doc, SourceGolight); items != nil {
		t.Errorf("空文档简化应返回 nil")
	}
}

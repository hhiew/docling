package docling

import (
	"encoding/json"
	"strings"
	"testing"
)

// 空文档序列化：顶层键齐全且顺序无关，body/furniture 初始化对齐 docling-core。
func TestDoclingEmptyDocumentJSON(t *testing.T) {
	doc := NewDoclingDocument("demo")
	data, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("解析产物失败: %v", err)
	}
	// 顶层键必须齐全（空集合也输出，对齐 export_to_dict 的 exclude_none 行为）
	for _, key := range []string{
		"schema_name", "version", "name", "body", "furniture", "groups",
		"texts", "pictures", "tables", "key_value_items", "form_items", "pages",
	} {
		if _, ok := m[key]; !ok {
			t.Errorf("顶层缺少键 %q，产物: %s", key, data)
		}
	}
	if m["schema_name"] != DoclingSchemaName {
		t.Errorf("schema_name = %v, 期望 %s", m["schema_name"], DoclingSchemaName)
	}
	if m["version"] != DoclingSchemaVersion {
		t.Errorf("version = %v, 期望 %s", m["version"], DoclingSchemaVersion)
	}
	// body 初始形态：self_ref=#/body、name=_root_
	body, _ := m["body"].(map[string]any)
	if body == nil {
		t.Fatalf("body 缺失")
	}
	if body["self_ref"] != "#/body" || body["name"] != "_root_" {
		t.Errorf("body 初始形态不符: %v", body)
	}
	if arr, ok := body["children"].([]any); !ok || len(arr) != 0 {
		t.Errorf("body.children 应为空数组: %v", body["children"])
	}
}

// 树构建 API：self_ref 编号、parent 缺省挂 body、children 引用挂接。
func TestDoclingTreeBuilding(t *testing.T) {
	doc := NewDoclingDocument("demo")
	h := doc.AddHeading(1, "引言", nil, nil)
	doc.AddText(LabelText, "正文段落", nil, &h)
	g := doc.AddListGroup("", &h)
	doc.AddListItem(g, "第一项", false, "·", nil)
	doc.AddListItem(g, "第二项", true, "1.", nil)
	doc.AddTable([]DoclingTableCell{{
		RowSpan: 1, ColSpan: 1,
		StartRowOffsetIdx: 0, EndRowOffsetIdx: 1,
		StartColOffsetIdx: 0, EndColOffsetIdx: 1,
		Text: "A", ColumnHeader: true,
	}}, 1, 1, nil, nil)

	if doc.Texts[0].SelfRef != "#/texts/0" || doc.Texts[1].SelfRef != "#/texts/1" {
		t.Errorf("texts self_ref 编号错误: %q %q", doc.Texts[0].SelfRef, doc.Texts[1].SelfRef)
	}
	// 标题挂 body，段落与列表挂标题
	if doc.Texts[0].Parent == nil || doc.Texts[0].Parent.Kind != refBody {
		t.Errorf("标题 parent 应为 body")
	}
	if doc.Texts[1].Parent == nil || doc.Texts[1].Parent.String() != "#/texts/0" {
		t.Errorf("段落 parent 应为 #/texts/0")
	}
	if len(doc.Texts[0].Children) != 2 {
		t.Errorf("标题 children 应含段落与列表组，实际 %d", len(doc.Texts[0].Children))
	}
	// 列表项专属字段
	if doc.Texts[2].Label != LabelListItem || doc.Texts[2].Enumerated == nil || *doc.Texts[2].Enumerated {
		t.Errorf("list_item enumerated 判定错误")
	}
	if doc.Texts[3].Enumerated == nil || !*doc.Texts[3].Enumerated || doc.Texts[3].Marker != "1." {
		t.Errorf("有序列表项 marker/enumerated 错误: %+v", doc.Texts[3])
	}
	// 序列化后引用形态
	data, _ := json.Marshal(doc)
	var m map[string]any
	_ = json.Unmarshal(data, &m)
	body := m["body"].(map[string]any)
	children := body["children"].([]any)
	if children[0].(map[string]any)["$ref"] != "#/texts/0" {
		t.Errorf("body.children[0] $ref 形态错误: %v", children[0])
	}
	// heading 专属字段
	texts := m["texts"].([]any)
	h0 := texts[0].(map[string]any)
	if h0["label"] != "section_header" || h0["level"] != float64(1) {
		t.Errorf("section_header label/level 错误: %v", h0)
	}
}

// TableData.grid 现算：普通单元格 end=start+1；合并单元格按 span 跨格填充。
func TestDoclingTableGridComputed(t *testing.T) {
	cells := []DoclingTableCell{
		{StartRowOffsetIdx: 0, EndRowOffsetIdx: 1, StartColOffsetIdx: 0, EndColOffsetIdx: 2, Text: "表头", ColumnHeader: true, ColSpan: 2},
		{StartRowOffsetIdx: 1, EndRowOffsetIdx: 2, StartColOffsetIdx: 0, EndColOffsetIdx: 1, Text: "a"},
		{StartRowOffsetIdx: 1, EndRowOffsetIdx: 3, StartColOffsetIdx: 1, EndColOffsetIdx: 2, Text: "跨行", RowSpan: 2},
		{StartRowOffsetIdx: 2, EndRowOffsetIdx: 3, StartColOffsetIdx: 0, EndColOffsetIdx: 1, Text: "b"},
	}
	doc := NewDoclingDocument("demo")
	doc.AddTable(cells, 3, 2, nil, nil)

	data, err := json.Marshal(doc.Tables[0])
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	td := m["data"].(map[string]any)
	grid := td["grid"].([]any)
	if len(grid) != 3 {
		t.Fatalf("grid 行数错误: %d", len(grid))
	}
	row0 := grid[0].([]any)
	if row0[0].(map[string]any)["text"] != "表头" || row0[1].(map[string]any)["text"] != "表头" {
		t.Errorf("跨列单元格未填满首行: %v", row0)
	}
	row2 := grid[2].([]any)
	if row2[1].(map[string]any)["text"] != "跨行" {
		t.Errorf("跨行单元格未覆盖第 3 行: %v", row2)
	}
}

// 往返：UnmarshalJSON 能解析 $ref（含 body 特例），pages 为字符串键。
func TestDoclingRoundTrip(t *testing.T) {
	doc := NewDoclingDocument("demo")
	h := doc.AddHeading(1, "章节", nil, nil)
	doc.AddText(LabelText, "段落", []ProvenanceItem{{
		PageNo:   1,
		BBox:     &DoclingBBox{L: 10, T: 20, R: 100, B: 30},
		CharSpan: [2]int64{0, 3},
	}}, &h)
	doc.AddPage(1, 612, 792)

	data, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}
	var back DoclingDocument
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatalf("反序列化失败: %v", err)
	}
	if back.Body.SelfRef != "#/body" {
		t.Errorf("body self_ref 往返错误: %q", back.Body.SelfRef)
	}
	if len(back.Texts) != 2 || back.Texts[1].Parent == nil || back.Texts[1].Parent.String() != "#/texts/0" {
		t.Fatalf("parent $ref 往返错误: %+v", back.Texts)
	}
	prov := back.Texts[1].Prov
	if len(prov) != 1 || prov[0].PageNo != 1 || prov[0].BBox == nil || prov[0].BBox.L != 10 {
		t.Errorf("prov 往返错误: %+v", prov)
	}
	if _, ok := back.Pages["1"]; !ok {
		t.Errorf("pages 应以字符串页号为键: %v", back.Pages)
	}
}

// AddCheckbox 清除前缀符号并按选中态分派 label。
func TestDoclingCheckbox(t *testing.T) {
	doc := NewDoclingDocument("demo")
	doc.AddCheckbox(true, "☑ 已选项", nil, nil)
	doc.AddCheckbox(false, "☐ 未选项", nil, nil)
	if doc.Texts[0].Label != LabelCheckboxSelected || doc.Texts[0].Text != "已选项" {
		t.Errorf("选中复选框错误: %+v", doc.Texts[0])
	}
	if doc.Texts[1].Label != LabelCheckboxUnselected || strings.Contains(doc.Texts[1].Text, "☐") {
		t.Errorf("未选复选框错误: %+v", doc.Texts[1])
	}
}

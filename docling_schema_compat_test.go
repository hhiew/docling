// docling_schema_compat_test.go 验证 Docling Core 1.10 协议的规范输出、
// 旧字段读取兼容、来源信息和默认正文层导出行为。
package docling

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"strings"
	"testing"
)

// TestDocling10CanonicalJSON 验证新产物只包含 Docling 1.10 官方字段。
func TestDocling10CanonicalJSON(t *testing.T) {
	doc := NewDoclingDocument("demo")
	doc.Meta = &DocMeta{Title: "旧版标题"}
	doc.AddHeading(2, "章节", nil, nil)

	data, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}
	var root map[string]any
	if err := json.Unmarshal(data, &root); err != nil {
		t.Fatalf("解析产物失败: %v", err)
	}
	if _, exists := root["meta"]; exists {
		t.Fatalf("顶层不应输出非官方 meta 字段: %s", data)
	}
	for _, key := range []string{"groups", "pictures", "tables", "key_value_items", "form_items"} {
		value, exists := root[key]
		if !exists || value == nil {
			t.Fatalf("缺少官方顶层集合 %s: %s", key, data)
		}
	}
	if _, exists := root["pages"]; !exists {
		t.Fatalf("缺少官方 pages 集合: %s", data)
	}
	furniture := root["furniture"].(map[string]any)
	if furniture["content_layer"] != string(LayerFurniture) {
		t.Fatalf("furniture.content_layer=%v，期望 furniture", furniture["content_layer"])
	}
	heading := root["texts"].([]any)[0].(map[string]any)
	if heading["level"] != float64(2) {
		t.Fatalf("section_header.level=%v，期望 2；产物=%s", heading["level"], data)
	}
	if _, exists := heading["text_level"]; exists {
		t.Fatalf("不应继续输出 text_level: %s", data)
	}
}

// TestDocling10LegacyJSONCanonicalization 验证旧字段可读但不会再次输出。
func TestDocling10LegacyJSONCanonicalization(t *testing.T) {
	raw := json.RawMessage(`{
		"schema_name":"DoclingDocument","version":"1.10.0","name":"legacy",
		"meta":{"title":"旧标题"},
		"body":{"self_ref":"#/body","children":[{"$ref":"#/texts/0"}],"content_layer":"body","label":"unspecified","name":"_root_"},
		"furniture":{"self_ref":"#/furniture","children":[],"content_layer":"body","label":"unspecified","name":"_root_"},
		"groups":[],
		"texts":[{"self_ref":"#/texts/0","parent":{"$ref":"#/body"},"children":[],"content_layer":"body","label":"section_header","orig":"标题","text":"标题","text_level":3}],
		"pictures":[],"tables":[],"key_value_items":[],"form_items":[],"pages":{}
	}`)
	doc, err := ParseDoclingDocument(raw)
	if err != nil {
		t.Fatalf("兼容读取失败: %v", err)
	}
	if len(doc.Texts) != 1 || doc.Texts[0].TextLevel != 3 {
		t.Fatalf("旧 text_level 未归一化: %+v", doc.Texts)
	}
	data, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("规范化序列化失败: %v", err)
	}
	if strings.Contains(string(data), `"text_level"`) || strings.Contains(string(data), `"meta":{"title"`) {
		t.Fatalf("旧字段被重新输出: %s", data)
	}
	if !strings.Contains(string(data), `"level":3`) {
		t.Fatalf("缺少规范 level 字段: %s", data)
	}
	if doc.Furniture.ContentLayer != LayerFurniture {
		t.Fatalf("旧 furniture 层未归一化: %s", doc.Furniture.ContentLayer)
	}
}

// TestDocling10TableGridUsesEmptyCells 验证 grid 空洞使用官方空单元格对象，而非 null。
func TestDocling10TableGridUsesEmptyCells(t *testing.T) {
	data, err := json.Marshal(TableData{
		NumRows: 2,
		NumCols: 2,
		TableCells: []DoclingTableCell{{
			RowSpan: 1, ColSpan: 1,
			StartRowOffsetIdx: 0, EndRowOffsetIdx: 1,
			StartColOffsetIdx: 0, EndColOffsetIdx: 1,
			Text: "A",
		}},
	})
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("解析产物失败: %v", err)
	}
	grid := payload["grid"].([]any)
	for rowIndex, rowValue := range grid {
		row := rowValue.([]any)
		for colIndex, cell := range row {
			if cell == nil {
				t.Fatalf("grid[%d][%d] 不应为 null: %s", rowIndex, colIndex, data)
			}
		}
	}
}

// TestDocling10PictureMetaRoundTrip 验证官方图表元数据不会在往返时丢失。
func TestDocling10PictureMetaRoundTrip(t *testing.T) {
	raw := json.RawMessage(`{
		"schema_name":"DoclingDocument","version":"1.10.0","name":"chart",
		"body":{"self_ref":"#/body","children":[{"$ref":"#/pictures/0"}],"content_layer":"body","label":"unspecified","name":"_root_"},
		"furniture":{"self_ref":"#/furniture","children":[],"content_layer":"furniture","label":"unspecified","name":"_root_"},
		"groups":[],"texts":[],
		"pictures":[{"self_ref":"#/pictures/0","parent":{"$ref":"#/body"},"children":[],"content_layer":"body","label":"picture","prov":[],"captions":[],"references":[],"footnotes":[],"annotations":[],"meta":{"classification":{"predictions":[{"class_name":"bar_chart","confidence":0.99,"created_by":"fixture"}]},"tabular_chart":{"title":"销量","chart_data":{"table_cells":[],"num_rows":0,"num_cols":0,"orientation":"rot_0","grid":[]}}}}],
		"tables":[],"key_value_items":[],"form_items":[],"pages":{}
	}`)
	doc, err := ParseDoclingDocument(raw)
	if err != nil {
		t.Fatalf("解析图表元数据失败: %v", err)
	}
	data, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("序列化图表元数据失败: %v", err)
	}
	for _, fragment := range []string{`"label":"picture"`, `"classification"`, `"bar_chart"`, `"tabular_chart"`, `"chart_data"`} {
		if !strings.Contains(string(data), fragment) {
			t.Fatalf("图表元数据缺少 %s: %s", fragment, data)
		}
	}
}

// TestParseByExtAddsOfficialOrigin 验证统一入口补齐文件来源信息。
func TestParseByExtAddsOfficialOrigin(t *testing.T) {
	input := []byte("hello")
	doc, err := ParseByExtWithOptions("folder/示例.txt", input, ParseOptions{OriginURI: "s3://bucket/示例.txt"})
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if doc.Name != "示例" || doc.Origin == nil {
		t.Fatalf("名称或 origin 未补齐: name=%q origin=%+v", doc.Name, doc.Origin)
	}
	hash := sha256.Sum256(input)
	wantHash := binary.BigEndian.Uint64(hash[len(hash)-8:])
	if doc.Origin.Filename != "示例.txt" || doc.Origin.Mimetype != "text/plain" || doc.Origin.BinaryHash != wantHash {
		t.Fatalf("origin 不符合 Docling 口径: %+v", doc.Origin)
	}
	if doc.Origin.URI != "s3://bucket/示例.txt" {
		t.Fatalf("origin URI 未保留: %+v", doc.Origin)
	}
}

// TestExportMarkdownDefaultsToBodyLayer 验证默认导出不会混入家具层内容。
func TestExportMarkdownDefaultsToBodyLayer(t *testing.T) {
	doc := NewDoclingDocument("layers")
	doc.AddText(LabelText, "正文", nil, nil)
	ref := doc.AddText(LabelPageHeader, "页眉", nil, nil)
	doc.Texts[ref.Idx].ContentLayer = LayerFurniture
	got := doc.ToMarkdown()
	if strings.Contains(got, "页眉") || !strings.Contains(got, "正文") {
		t.Fatalf("默认导出层过滤错误: %q", got)
	}
	html := doc.ToHTML()
	if strings.Contains(html, "页眉") || !strings.Contains(html, "正文") {
		t.Fatalf("HTML 默认导出层过滤错误: %q", html)
	}
}

// TestExportLayerOptions 验证显式选项可以导出家具层，并保持正文层可组合。
func TestExportLayerOptions(t *testing.T) {
	doc := NewDoclingDocument("layers")
	doc.AddText(LabelText, "正文", nil, nil)
	ref := doc.AddText(LabelPageHeader, "页眉", nil, nil)
	doc.Texts[ref.Idx].ContentLayer = LayerFurniture
	got := doc.ToMarkdownWithOptions(ExportOptions{Layers: []ContentLayer{LayerBody, LayerFurniture}})
	if !strings.Contains(got, "正文") || !strings.Contains(got, "页眉") {
		t.Fatalf("显式导出层未生效: %q", got)
	}
	html := doc.ToHTMLWithOptions(ExportOptions{Layers: []ContentLayer{LayerBody, LayerFurniture}})
	if !strings.Contains(html, "正文") || !strings.Contains(html, "页眉") {
		t.Fatalf("HTML 显式导出层未生效: %q", html)
	}
}

// TestChartDataExport 验证 PictureMeta 中的图表数据可导出为可读表格。
func TestChartDataExport(t *testing.T) {
	doc := NewDoclingDocument("chart")
	ref := doc.AddPicture(nil, nil, nil)
	doc.Pictures[ref.Idx].Meta = &PictureMeta{TabularChart: &TabularChartMetaField{
		Title: "销量",
		ChartData: &TableData{NumRows: 2, NumCols: 2, TableCells: []DoclingTableCell{
			{Text: "月份", RowSpan: 1, ColSpan: 1, StartRowOffsetIdx: 0, EndRowOffsetIdx: 1, StartColOffsetIdx: 0, EndColOffsetIdx: 1, ColumnHeader: true},
			{Text: "销量", RowSpan: 1, ColSpan: 1, StartRowOffsetIdx: 0, EndRowOffsetIdx: 1, StartColOffsetIdx: 1, EndColOffsetIdx: 2, ColumnHeader: true},
			{Text: "一月", RowSpan: 1, ColSpan: 1, StartRowOffsetIdx: 1, EndRowOffsetIdx: 2, StartColOffsetIdx: 0, EndColOffsetIdx: 1},
			{Text: "10", RowSpan: 1, ColSpan: 1, StartRowOffsetIdx: 1, EndRowOffsetIdx: 2, StartColOffsetIdx: 1, EndColOffsetIdx: 2},
		}},
	}}
	markdown := doc.ToMarkdown()
	html := doc.ToHTML()
	if !strings.Contains(markdown, "| 月份 | 销量 |") || !strings.Contains(markdown, "| 一月 | 10 |") {
		t.Fatalf("Markdown 未渲染图表数据: %q", markdown)
	}
	if !strings.Contains(html, "<th>月份</th>") || !strings.Contains(html, "<td>10</td>") {
		t.Fatalf("HTML 未渲染图表数据: %q", html)
	}
}

// TestLegacyCaptionAndAnnotationsMigration 验证旧 caption/annotations 迁移到官方引用与 meta。
func TestLegacyCaptionAndAnnotationsMigration(t *testing.T) {
	raw := json.RawMessage(`{
		"schema_name":"DoclingDocument","version":"1.10.0","name":"legacy-picture",
		"body":{"self_ref":"#/body","children":[{"$ref":"#/pictures/0"}],"content_layer":"body","label":"unspecified","name":"_root_"},
		"furniture":{"self_ref":"#/furniture","children":[],"content_layer":"furniture","label":"unspecified","name":"_root_"},
		"groups":[],"texts":[],
		"pictures":[{"self_ref":"#/pictures/0","parent":{"$ref":"#/body"},"children":[],"content_layer":"body","label":"chart","prov":[],"caption":"销量图","captions":[],"references":[],"footnotes":[],"annotations":[{"kind":"classification","provenance":"legacy-model","predicted_classes":[{"class_name":"bar_chart","confidence":0.9}]},{"kind":"description","text":"柱状图","provenance":"legacy-model"}]}],
		"tables":[],"key_value_items":[],"form_items":[],"pages":{}
	}`)
	doc, err := ParseDoclingDocument(raw)
	if err != nil {
		t.Fatalf("兼容读取失败: %v", err)
	}
	data, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("规范化序列化失败: %v", err)
	}
	var root map[string]any
	if err := json.Unmarshal(data, &root); err != nil {
		t.Fatalf("解析规范产物失败: %v", err)
	}
	picture := root["pictures"].([]any)[0].(map[string]any)
	if picture["label"] != "picture" || len(picture["captions"].([]any)) != 1 {
		t.Fatalf("图表标签或标题引用未规范化: %s", data)
	}
	if _, exists := picture["caption"]; exists {
		t.Fatalf("旧 caption 不应重新输出: %s", data)
	}
	if len(picture["annotations"].([]any)) != 0 {
		t.Fatalf("旧 annotations 应迁移后清空: %s", data)
	}
	meta := picture["meta"].(map[string]any)
	if _, ok := meta["classification"]; !ok || meta["description"].(map[string]any)["text"] != "柱状图" {
		t.Fatalf("旧 annotations 未迁移到 meta: %s", data)
	}
	texts := root["texts"].([]any)
	if len(texts) != 1 || texts[0].(map[string]any)["text"] != "销量图" {
		t.Fatalf("旧 caption 文本未无损保留: %s", data)
	}
}

// TestOfficialRequiredDefaults 验证旧对象重新输出时补齐必需的嵌套默认值。
func TestOfficialRequiredDefaults(t *testing.T) {
	doc := NewDoclingDocument("defaults")
	doc.AddPage(1, 0, 0)
	doc.AddCode("echo ok", "bash", []ProvenanceItem{{PageNo: 1}}, nil)
	doc.AddTable([]DoclingTableCell{{
		Text: "A", StartRowOffsetIdx: 0, EndRowOffsetIdx: 1, StartColOffsetIdx: 0, EndColOffsetIdx: 1,
	}}, 1, 1, nil, nil)
	data, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}
	var root map[string]any
	if err := json.Unmarshal(data, &root); err != nil {
		t.Fatalf("解析产物失败: %v", err)
	}
	code := root["texts"].([]any)[0].(map[string]any)
	if code["self_ref"] != "#/texts/0" || code["code_language"] != "Bash" || code["prov"].([]any)[0].(map[string]any)["bbox"] == nil {
		t.Fatalf("代码语言或来源默认值错误: %s", data)
	}
	for _, key := range []string{"captions", "references", "footnotes"} {
		if refs, exists := code[key].([]any); !exists || len(refs) != 0 {
			t.Fatalf("代码项缺少官方空引用集合 %s: %s", key, data)
		}
	}
	cell := root["tables"].([]any)[0].(map[string]any)["data"].(map[string]any)["table_cells"].([]any)[0].(map[string]any)
	if cell["row_span"] != float64(1) || cell["col_span"] != float64(1) {
		t.Fatalf("表格跨度默认值错误: %s", data)
	}
}

// TestLegacyPageNumberCanonicalization 验证旧平铺页尺寸按 pages 键补齐 page_no。
func TestLegacyPageNumberCanonicalization(t *testing.T) {
	doc, err := ParseDoclingDocument(json.RawMessage(`{"body":{"children":[]},"pages":{"3":{"width":100,"height":200}}}`))
	if err != nil {
		t.Fatalf("读取旧页面失败: %v", err)
	}
	if doc.Pages["3"].PageNo != 3 || doc.Pages["3"].Size == nil {
		t.Fatalf("旧页面未规范化: %+v", doc.Pages["3"])
	}
}

// TestOriginHexHashCompatibility 验证官方允许的十六进制哈希输入会截取低 64 位。
func TestOriginHexHashCompatibility(t *testing.T) {
	var origin DocumentOrigin
	err := json.Unmarshal([]byte(`{"mimetype":"text/plain","binary_hash":"0123456789abcdef1122334455667788","filename":"a.txt"}`), &origin)
	if err != nil {
		t.Fatalf("读取十六进制哈希失败: %v", err)
	}
	if origin.BinaryHash != 0x1122334455667788 {
		t.Fatalf("binary_hash=%x，期望低 64 位", origin.BinaryHash)
	}
}

// TestFurnitureReferenceRoundTrip 验证 furniture 根引用不会被错误改写为 body。
func TestFurnitureReferenceRoundTrip(t *testing.T) {
	var ref RefItem
	if err := json.Unmarshal([]byte(`{"$ref":"#/furniture"}`), &ref); err != nil {
		t.Fatalf("解析 furniture 引用失败: %v", err)
	}
	data, err := json.Marshal(ref)
	if err != nil {
		t.Fatalf("序列化 furniture 引用失败: %v", err)
	}
	if string(data) != `{"$ref":"#/furniture"}` {
		t.Fatalf("furniture 引用被改写: %s", data)
	}
}

// TestLegacyLatexCanonicalization 验证旧 latex 在官方化往返后仍作为公式文本保留。
func TestLegacyLatexCanonicalization(t *testing.T) {
	raw := json.RawMessage(`{
		"body":{"children":[{"$ref":"#/texts/0"}]},
		"texts":[{"self_ref":"#/texts/0","parent":{"$ref":"#/body"},"children":[],"content_layer":"body","label":"formula","prov":[],"orig":"E=mc2","latex":"E=mc^2"}]
	}`)
	doc, err := ParseDoclingDocument(raw)
	if err != nil {
		t.Fatalf("读取旧公式失败: %v", err)
	}
	if doc.Texts[0].Latex != "E=mc^2" || doc.Texts[0].Text != "E=mc^2" {
		t.Fatalf("旧公式未归一化: %+v", doc.Texts[0])
	}
	data, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("官方化序列化失败: %v", err)
	}
	roundTrip, err := ParseDoclingDocument(data)
	if err != nil {
		t.Fatalf("读取官方化公式失败: %v", err)
	}
	if roundTrip.Texts[0].Text != "E=mc^2" {
		t.Fatalf("官方化往返丢失公式: %s", data)
	}
	items := ToContentList(roundTrip, SourceDocling)
	if len(items) != 1 || items[0].LaTeX != "E=mc^2" {
		t.Fatalf("content_list 丢失官方公式文本: %+v", items)
	}
}

// chunker_test.go 验证 DoclingDocument 的层级语义分块：标题路径按 level
// 更新、列表分组整体序列化、表格保持 Markdown 结构，以及来源引用与跨页坐标保留。
package docling

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// TestHierarchicalChunksHeadingLevelJump 验证标题跳级与嵌套树遍历均按官方
// level 更新上下文；标题本身不产出内容 chunk，也不触发长度切分。
func TestHierarchicalChunksHeadingLevelJump(t *testing.T) {
	doc := NewDoclingDocument("层级文档")
	doc.Origin = &DocumentOrigin{Mimetype: "text/markdown", BinaryHash: 42, Filename: "guide.md"}
	title := doc.AddTitle("使用手册", nil, nil)
	h1 := doc.AddHeading(1, "安装", nil, &title)
	section := doc.AddSectionGroup("跳级章节", &h1)
	h3 := doc.AddHeading(3, "Linux", nil, &section)
	linux := doc.AddText(LabelParagraph, "执行安装命令", []ProvenanceItem{
		testChunkProv(1, 10, 20, 110, 40),
		testChunkProv(2, 10, 30, 150, 55),
	}, &h3)
	h2 := doc.AddHeading(2, "升级", nil, nil)
	upgrade := doc.AddText(LabelText, "下载新版本", nil, &h2)

	chunks := HierarchicalChunks(doc)
	if len(chunks) != 2 {
		t.Fatalf("chunk 数量 = %d, want 2: %+v", len(chunks), chunks)
	}
	if chunks[0].Text != "执行安装命令" ||
		!reflect.DeepEqual(chunks[0].Meta.Headings, []string{"使用手册", "安装", "Linux"}) {
		t.Fatalf("跳级标题上下文错误: %+v", chunks[0])
	}
	if chunks[0].Meta.SchemaName != DocChunkMetaSchemaName || chunks[0].Meta.Version != DocChunkMetaVersion {
		t.Fatalf("DocMeta 协议标识错误: %+v", chunks[0].Meta)
	}
	if !reflect.DeepEqual(chunks[0].Meta.DocRefs, []string{linux.String()}) {
		t.Fatalf("首 chunk 引用 = %v, want %s", chunks[0].Meta.DocRefs, linux.String())
	}
	if chunks[0].Meta.Origin == nil || *chunks[0].Meta.Origin != *doc.Origin {
		t.Fatalf("origin 未保留: %+v", chunks[0].Meta.Origin)
	}
	if len(chunks[0].Meta.Provenance) != 2 ||
		chunks[0].Meta.Provenance[0].PageNo != 1 || chunks[0].Meta.Provenance[1].PageNo != 2 ||
		chunks[0].Meta.Provenance[1].BBox == nil || chunks[0].Meta.Provenance[1].BBox.R != 150 {
		t.Fatalf("跨页 provenance 未完整保留: %+v", chunks[0].Meta.Provenance)
	}
	if chunks[1].Text != "下载新版本" ||
		!reflect.DeepEqual(chunks[1].Meta.Headings, []string{"使用手册", "安装", "升级"}) ||
		!reflect.DeepEqual(chunks[1].Meta.DocRefs, []string{upgrade.String()}) {
		t.Fatalf("标题回退上下文错误: %+v", chunks[1])
	}
}

// TestHierarchicalChunksMergesListGroup 验证一个列表分组只生成一个 chunk，
// 嵌套列表保持 Markdown 层级，元数据引用所有被合并的列表项。
func TestHierarchicalChunksMergesListGroup(t *testing.T) {
	doc := NewDoclingDocument("列表文档")
	doc.AddHeading(1, "步骤", nil, nil)
	list := doc.AddListGroup("步骤列表", nil)
	first := doc.AddListItem(list, "准备环境", false, "", []ProvenanceItem{testChunkProv(1, 1, 2, 20, 8)})
	second := doc.AddListItem(list, "启动服务", true, "2.", []ProvenanceItem{testChunkProv(2, 1, 2, 20, 8)})
	nested := doc.AddListGroup("子步骤", &first)
	child := doc.AddListItem(nested, "检查配置", false, "", []ProvenanceItem{testChunkProv(1, 3, 4, 25, 9)})

	chunks := HierarchicalChunks(doc)
	if len(chunks) != 1 {
		t.Fatalf("列表应整体生成一个 chunk，实际 %d: %+v", len(chunks), chunks)
	}
	wantText := "- 准备环境\n  - 检查配置\n2. 启动服务"
	if chunks[0].Text != wantText {
		t.Fatalf("列表 Markdown = %q, want %q", chunks[0].Text, wantText)
	}
	wantRefs := []string{first.String(), child.String(), second.String()}
	if !reflect.DeepEqual(chunks[0].Meta.DocRefs, wantRefs) {
		t.Fatalf("列表引用 = %v, want %v", chunks[0].Meta.DocRefs, wantRefs)
	}
	if len(chunks[0].Meta.DocItems) != len(wantRefs) {
		t.Fatalf("列表 doc_items 数量 = %d, want %d", len(chunks[0].Meta.DocItems), len(wantRefs))
	}
	if !reflect.DeepEqual(chunks[0].Meta.Headings, []string{"步骤"}) {
		t.Fatalf("列表标题上下文错误: %v", chunks[0].Meta.Headings)
	}
	if len(chunks[0].Meta.Provenance) != 3 {
		t.Fatalf("列表 provenance 数量 = %d, want 3", len(chunks[0].Meta.Provenance))
	}
}

// TestHierarchicalChunksSerializesTableMarkdown 验证表格单独成块并以 GFM
// Markdown 网格序列化，保留表格自身引用和来源坐标。
func TestHierarchicalChunksSerializesTableMarkdown(t *testing.T) {
	doc := NewDoclingDocument("表格文档")
	doc.AddHeading(1, "参数", nil, nil)
	cells := []DoclingTableCell{
		{StartRowOffsetIdx: 0, EndRowOffsetIdx: 1, StartColOffsetIdx: 0, EndColOffsetIdx: 1, Text: "名称", ColumnHeader: true},
		{StartRowOffsetIdx: 0, EndRowOffsetIdx: 1, StartColOffsetIdx: 1, EndColOffsetIdx: 2, Text: "值", ColumnHeader: true},
		{StartRowOffsetIdx: 1, EndRowOffsetIdx: 2, StartColOffsetIdx: 0, EndColOffsetIdx: 1, Text: "模式"},
		{StartRowOffsetIdx: 1, EndRowOffsetIdx: 2, StartColOffsetIdx: 1, EndColOffsetIdx: 2, Text: "安全"},
	}
	table := doc.AddTable(cells, 2, 2, []ProvenanceItem{testChunkProv(3, 12, 30, 190, 90)}, nil)
	caption := doc.AddText(LabelCaption, "表 1：运行参数", []ProvenanceItem{testChunkProv(3, 12, 20, 190, 28)}, nil)
	doc.Tables[table.Idx].Captions = []RefItem{caption}

	chunks := HierarchicalChunks(doc)
	if len(chunks) != 1 {
		t.Fatalf("表格 chunk 数量 = %d, want 1: %+v", len(chunks), chunks)
	}
	want := "表 1：运行参数\n\n| 名称 | 值 |\n| --- | --- |\n| 模式 | 安全 |"
	if chunks[0].Text != want {
		t.Fatalf("表格 Markdown = %q, want %q", chunks[0].Text, want)
	}
	if !reflect.DeepEqual(chunks[0].Meta.DocRefs, []string{caption.String(), table.String()}) ||
		len(chunks[0].Meta.DocItems) != 2 || len(chunks[0].Meta.Provenance) != 2 ||
		chunks[0].Meta.Provenance[1].PageNo != 3 {
		t.Fatalf("表格来源元数据错误: %+v", chunks[0].Meta)
	}
}

// TestHierarchicalChunksNilAndEmpty 验证空输入不会生成占位 chunk。
func TestHierarchicalChunksNilAndEmpty(t *testing.T) {
	if chunks := HierarchicalChunks(nil); len(chunks) != 0 {
		t.Fatalf("nil 文档返回 %d 个 chunk", len(chunks))
	}
	if chunks := HierarchicalChunks(NewDoclingDocument("empty")); len(chunks) != 0 {
		t.Fatalf("空文档返回 %d 个 chunk", len(chunks))
	}
}

// TestHierarchicalChunksCarriesOfficialDocItems 验证使用官方 DocMeta schema_name
// 时同步输出必填 doc_items，并保留现有 doc_refs/provenance 便捷溯源字段。
func TestHierarchicalChunksCarriesOfficialDocItems(t *testing.T) {
	doc := NewDoclingDocument("官方来源项")
	textRef := doc.AddText(LabelText, "正文", []ProvenanceItem{testChunkProv(1, 1, 2, 3, 4)}, nil)

	chunks := HierarchicalChunks(doc)
	if len(chunks) != 1 {
		t.Fatalf("chunks = %+v, want one", chunks)
	}
	meta := chunks[0].Meta
	if len(meta.DocItems) != 1 || !reflect.DeepEqual(meta.DocRefs, []string{textRef.String()}) {
		t.Fatalf("chunk source metadata = %+v", meta)
	}
	var item map[string]any
	if err := json.Unmarshal(meta.DocItems[0], &item); err != nil {
		t.Fatalf("decode doc_items[0]: %v", err)
	}
	if item["self_ref"] != textRef.String() || item["label"] != string(LabelText) || item["text"] != "正文" {
		t.Fatalf("doc_items[0] = %+v", item)
	}
}

// TestHybridChunksSplitsByContextualizedTokens 验证标题上下文也计入 token
// 预算，超长单元素被切分后仍保留来源引用。
func TestHybridChunksSplitsByContextualizedTokens(t *testing.T) {
	doc := NewDoclingDocument("hybrid")
	heading := doc.AddHeading(1, "章", nil, nil)
	textRef := doc.AddText(LabelText, "甲乙丙丁戊己庚辛壬癸", nil, &heading)
	chunks := HybridChunks(doc, HybridChunkOptions{
		MaxTokens: 6,
		CountTokens: func(text string) int {
			return len([]rune(text))
		},
		DisablePeerMerge: true,
	})
	if len(chunks) < 2 {
		t.Fatalf("hybrid chunks=%+v, want split", chunks)
	}
	var rebuilt strings.Builder
	for index, chunk := range chunks {
		if tokens := len([]rune(ContextualizeDocChunk(chunk))); tokens > 6 {
			t.Fatalf("chunks[%d] tokens=%d text=%q headings=%v", index, tokens, chunk.Text, chunk.Meta.Headings)
		}
		if !reflect.DeepEqual(chunk.Meta.DocRefs, []string{textRef.String()}) || len(chunk.Meta.DocItems) != 1 {
			t.Fatalf("chunks[%d] source=%+v", index, chunk.Meta)
		}
		rebuilt.WriteString(chunk.Text)
	}
	if rebuilt.String() != "甲乙丙丁戊己庚辛壬癸" {
		t.Fatalf("rebuilt=%q", rebuilt.String())
	}
}

// TestHybridChunksMergesPeersWithMatchingHeadings 验证同标题下的相邻小块
// 在 token 预算内合并，并同步合并 doc_items 和 provenance。
func TestHybridChunksMergesPeersWithMatchingHeadings(t *testing.T) {
	doc := NewDoclingDocument("hybrid peers")
	heading := doc.AddHeading(1, "H", nil, nil)
	first := doc.AddText(LabelText, "one", []ProvenanceItem{testChunkProv(1, 1, 2, 3, 4)}, &heading)
	second := doc.AddText(LabelText, "two", []ProvenanceItem{testChunkProv(2, 1, 2, 3, 4)}, &heading)
	chunks := HybridChunks(doc, HybridChunkOptions{
		MaxTokens: 12,
		CountTokens: func(text string) int {
			return len([]rune(text))
		},
	})
	if len(chunks) != 1 || chunks[0].Text != "one\n\ntwo" ||
		!reflect.DeepEqual(chunks[0].Meta.DocRefs, []string{first.String(), second.String()}) ||
		len(chunks[0].Meta.DocItems) != 2 || len(chunks[0].Meta.Provenance) != 2 {
		t.Fatalf("merged peers=%+v", chunks)
	}
}

// TestHybridChunksRepeatsTableHeader 验证超大表格按行切分时每段重复
// Markdown 表头，且不会超过用户提供的 token 上限。
func TestHybridChunksRepeatsTableHeader(t *testing.T) {
	doc := NewDoclingDocument("hybrid table")
	var cells []DoclingTableCell
	rows := [][]string{{"名称", "值"}, {"A", "1111"}, {"B", "2222"}, {"C", "3333"}, {"D", "4444"}}
	for rowIndex, row := range rows {
		for colIndex, value := range row {
			cells = append(cells, DoclingTableCell{
				Text: value, RowSpan: 1, ColSpan: 1,
				StartRowOffsetIdx: int64(rowIndex), EndRowOffsetIdx: int64(rowIndex + 1),
				StartColOffsetIdx: int64(colIndex), EndColOffsetIdx: int64(colIndex + 1),
				ColumnHeader: rowIndex == 0,
			})
		}
	}
	doc.AddTable(cells, int64(len(rows)), 2, nil, nil)
	chunks := HybridChunks(doc, HybridChunkOptions{
		MaxTokens: 44,
		CountTokens: func(text string) int {
			return len([]rune(text))
		},
		DisablePeerMerge: true,
	})
	if len(chunks) < 2 {
		t.Fatalf("table chunks=%+v, want split", chunks)
	}
	header := "| 名称 | 值 |\n| --- | --- |"
	for index, chunk := range chunks {
		if !strings.HasPrefix(chunk.Text, header) || len([]rune(ContextualizeDocChunk(chunk))) > 44 {
			t.Fatalf("table chunks[%d]=%q", index, chunk.Text)
		}
	}
}

// testChunkProv 构造 chunk 测试使用的页面来源坐标。
func testChunkProv(page int64, l, t, r, b float64) ProvenanceItem {
	return ProvenanceItem{
		PageNo: page,
		BBox:   &DoclingBBox{L: l, T: t, R: r, B: b, CoordOrigin: CoordOriginTopLeft},
	}
}

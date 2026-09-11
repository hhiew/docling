// 本文件验证 content_list 知识库分块策略，包括章节聚合、长度限制、
// 大表拆分、多模态分流、噪声过滤和来源定位信息保留。
package docling

import (
	"fmt"
	"strings"
	"testing"
)

// TestChunkContentListHeadingJump 验证标题层级跳跃时以持久化章节路径为准，
// 不会把相邻但路径不同的正文错误合并。
func TestChunkContentListHeadingJump(t *testing.T) {
	items := []Item{
		{Type: ItemTypeText, Text: "总览", TextLevel: 1, SectionPath: []string{"总览"}, OrderIndex: 1},
		{Type: ItemTypeText, Text: "三级标题", TextLevel: 3, SectionPath: []string{"总览", "三级标题"}, OrderIndex: 2},
		{Type: ItemTypeText, Text: "正文甲", SectionPath: []string{"总览", "三级标题"}, OrderIndex: 3},
		{Type: ItemTypeText, Text: "正文乙", SectionPath: []string{"总览", "三级标题"}, OrderIndex: 4},
		{Type: ItemTypeText, Text: "下一章", TextLevel: 1, SectionPath: []string{"下一章"}, OrderIndex: 5},
	}

	chunks := ChunkContentList(items, ContentChunkOptions{})
	if len(chunks) != 4 {
		t.Fatalf("chunks = %d, want 4: %+v", len(chunks), chunks)
	}
	if chunks[1].HeadingPath != "总览 > 三级标题" || chunks[1].Content != "三级标题" {
		t.Fatalf("jump heading chunk = %+v", chunks[1])
	}
	if chunks[2].Content != "正文甲\n正文乙" || chunks[2].HeadingPath != "总览 > 三级标题" {
		t.Fatalf("merged body chunk = %+v", chunks[2])
	}
}

// TestChunkContentListLongText 验证默认 900 rune 上限和自定义上限均生效。
func TestChunkContentListLongText(t *testing.T) {
	items := []Item{{Type: ItemTypeText, Text: strings.Repeat("甲", 1901), PageIdx: 2, OrderIndex: 7}}
	chunks := ChunkContentList(items, ContentChunkOptions{})
	if len(chunks) != 3 {
		t.Fatalf("chunks = %d, want 3", len(chunks))
	}
	for i, chunk := range chunks {
		if got := len([]rune(chunk.Content)); got > 900 {
			t.Fatalf("chunks[%d] runes = %d, want <= 900", i, got)
		}
		if chunk.PageIdx != 2 || chunk.OrderIndex != 7 || chunk.ChunkIndex != 0 {
			t.Fatalf("chunks[%d] metadata = %+v", i, chunk)
		}
	}

	custom := ChunkContentList(items, ContentChunkOptions{MaxRunes: 1000})
	if len(custom) != 2 || len([]rune(custom[0].Content)) != 1000 {
		t.Fatalf("custom chunks = %+v", custom)
	}
}

// TestChunkContentListLargeTable 验证超大 GFM 表格按数据行拆段并重复表头。
func TestChunkContentListLargeTable(t *testing.T) {
	rows := make([][]string, 0, 206)
	rows = append(rows, []string{"序号", "值"})
	for i := 1; i <= 205; i++ {
		rows = append(rows, []string{fmt.Sprint(i), fmt.Sprintf("值%d", i)})
	}
	items := []Item{{
		Type: ItemTypeTable, TableCaption: "统计表", TableBody: RenderMarkdownTable(rows),
		TableFootnote: "数据来源", SectionPath: []string{"报表"}, PageIdx: 3, OrderIndex: 9,
	}}

	chunks := ChunkContentList(items, ContentChunkOptions{})
	if len(chunks) != 3 {
		t.Fatalf("chunks = %d, want 3", len(chunks))
	}
	wantRows := []int{101, 101, 6}
	for i, chunk := range chunks {
		if got := len(ParseMarkdownTable(chunk.Content)); got != wantRows[i] {
			t.Fatalf("chunks[%d] rows = %d, want %d", i, got, wantRows[i])
		}
		if !strings.HasPrefix(chunk.Content, "统计表\n") || !strings.HasSuffix(chunk.Content, "\n数据来源") {
			t.Fatalf("chunks[%d] caption/footnote missing: %q", i, chunk.Content)
		}
		if chunk.HeadingPath != "报表" || chunk.PageIdx != 3 || chunk.OrderIndex != 9 {
			t.Fatalf("chunks[%d] metadata = %+v", i, chunk)
		}
	}
}

// TestChunkContentListMultimodalRouting 验证 Docling 多模态元素默认留给多模态
// 模块处理，而纯 Go 元素仍进入普通检索分块。
func TestChunkContentListMultimodalRouting(t *testing.T) {
	items := []Item{
		{Type: ItemTypeImage, ImageCaption: "Docling 图片", Source: SourceDocling, OrderIndex: 1},
		{Type: ItemTypeTable, TableBody: "| A |\n|---|\n| 1 |", Source: SourceDocling, OrderIndex: 2},
		{Type: ItemTypeEquation, LaTeX: "x=1", Source: SourceDocling, OrderIndex: 3},
		{Type: ItemTypeImage, ImageCaption: "纯 Go 图片", Source: SourceGolight, OrderIndex: 4},
		{Type: ItemTypeEquation, LaTeX: "y=2", Source: SourceGolight, OrderIndex: 5},
	}

	chunks := ChunkContentList(items, ContentChunkOptions{})
	if len(chunks) != 2 || chunks[0].Content != "纯 Go 图片" || chunks[1].Content != "y=2" {
		t.Fatalf("default chunks = %+v", chunks)
	}
	all := ChunkContentList(items, ContentChunkOptions{IncludeDoclingMultimodal: true})
	if len(all) != 5 {
		t.Fatalf("included chunks = %d, want 5: %+v", len(all), all)
	}
}

// TestChunkContentListNoiseAndBBox 验证噪声标签在文本聚合前不会丢失，且同页
// 相邻正文合并后保留首页、阅读顺序与边界框并集。
func TestChunkContentListNoiseAndBBox(t *testing.T) {
	items := []Item{
		{Type: ItemTypeText, Text: "重复页眉", Label: "page_header", PageIdx: 1, OrderIndex: 1},
		{Type: ItemTypeText, Text: "正文一", PageIdx: 2, OrderIndex: 2, BBox: &BBox{Left: 10, Top: 20, Right: 30, Bottom: 40}},
		{Type: ItemTypeText, Text: "正文二", PageIdx: 2, OrderIndex: 3, BBox: &BBox{Left: 5, Top: 15, Right: 50, Bottom: 60}},
		{Type: ItemTypeText, Text: "目录项", Label: "document_index", PageIdx: 2, OrderIndex: 4},
		{Type: ItemTypeText, Text: "重复页脚", Label: "page_footer", PageIdx: 2, OrderIndex: 5},
	}

	chunks := ChunkContentList(items, ContentChunkOptions{})
	if len(chunks) != 1 {
		t.Fatalf("chunks = %+v", chunks)
	}
	chunk := chunks[0]
	if chunk.Content != "正文一\n正文二" || chunk.PageIdx != 2 || chunk.OrderIndex != 2 {
		t.Fatalf("chunk = %+v", chunk)
	}
	if chunk.BBox == nil || *chunk.BBox != (BBox{Left: 5, Top: 15, Right: 50, Bottom: 60}) {
		t.Fatalf("bbox = %+v", chunk.BBox)
	}
}

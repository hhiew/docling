// content_chunk.go 实现面向知识库检索的 content_list 分块适配器。
// 它保持 Item 持久化协议不变，在消费侧统一完成章节聚合、长度控制、
// 大表拆分、版式噪声过滤和 Docling 多模态元素分流。
package docparse

import (
	"math"
	"strings"
)

const (
	// DefaultContentChunkMaxRunes 是普通检索分块的默认字符上限。
	DefaultContentChunkMaxRunes = 900
	// DefaultContentTableMaxRows 是单个表格分块允许包含的默认数据行数。
	DefaultContentTableMaxRows = 100
)

// ContentChunkOptions 控制 content_list 到知识库分块的兼容策略。
// 零值默认口径：900 rune、表格每段 100 个数据行、过滤
// 页眉/页脚/目录，并把 Docling 图片、表格和公式交给多模态模块。
type ContentChunkOptions struct {
	// MaxRunes 指定普通分块字符上限；小于等于零时使用 900。
	MaxRunes int
	// TableMaxRows 指定表格每段的数据行上限；小于等于零时使用 100。
	TableMaxRows int
	// IncludeDoclingMultimodal 为 true 时把 Docling 图片、表格和公式也
	// 输出为普通检索分块；默认 false，以避免与多模态 artifact 重复。
	IncludeDoclingMultimodal bool
	// NoiseLabels 指定不进入检索的 label；nil 使用页眉、页脚和目录默认值，
	// 非 nil 空切片可显式关闭噪声过滤。
	NoiseLabels []string
}

// ContentChunk 是 content_list 的知识库检索分块，字段与历史
// knowledgeChunkSpan 对齐，便于迁移时无损映射来源位置和阅读顺序。
type ContentChunk struct {
	// Content 是进入向量检索的纯文本内容。
	Content string `json:"content"`
	// ChunkIndex 保留历史序号字段；content_list 路径保持零值。
	ChunkIndex int64 `json:"chunk_index,omitempty"`
	// CharStart 保留历史字符起点字段；content_list 路径保持零值。
	CharStart int64 `json:"char_start,omitempty"`
	// CharEnd 保留历史字符终点字段；content_list 路径保持零值。
	CharEnd int64 `json:"char_end,omitempty"`
	// TokenEstimate 保留历史 token 估算字段；由上层需要时填充。
	TokenEstimate int64 `json:"token_estimate,omitempty"`
	// HeadingPath 是用“ > ”连接的章节路径。
	HeadingPath string `json:"heading_path,omitempty"`
	// PageIdx 是分块首个元素的页码。
	PageIdx int64 `json:"page_idx,omitempty"`
	// OrderIndex 是分块首个元素的阅读顺序。
	OrderIndex int64 `json:"order_index,omitempty"`
	// BBox 是同组元素边界框的并集。
	BBox *BBox `json:"bbox,omitempty"`
}

// ChunkContentList 把历史 content_list 转为知识库检索分块。
// 相邻且章节路径、标题层级、来源及 label 相同的文本先合并；普通内容再按
// rune 数切分，表格按数据行切分并在每段重复标题、表头和脚注。
func ChunkContentList(items []Item, options ContentChunkOptions) []ContentChunk {
	if len(items) == 0 {
		return nil
	}
	options = normalizeContentChunkOptions(options)
	noiseLabels := contentChunkNoiseLabels(options.NoiseLabels)
	grouped := groupContentChunkItems(items, options.MaxRunes)
	chunks := make([]ContentChunk, 0, len(grouped))
	for _, item := range grouped {
		if shouldRouteContentMultimodal(item, options) || noiseLabels[item.Label] {
			continue
		}
		if item.Type == ItemTypeTable {
			for _, part := range splitContentChunkTable(item, options.TableMaxRows) {
				if strings.TrimSpace(part) == "" {
					continue
				}
				chunks = append(chunks, newContentChunk(item, part))
			}
			continue
		}
		text := ItemToText(item)
		for _, part := range splitContentChunkText(text, options.MaxRunes) {
			if strings.TrimSpace(part) == "" {
				continue
			}
			chunks = append(chunks, newContentChunk(item, part))
		}
	}
	return chunks
}

// normalizeContentChunkOptions 填充零值选项对应的生产默认值。
func normalizeContentChunkOptions(options ContentChunkOptions) ContentChunkOptions {
	if options.MaxRunes <= 0 {
		options.MaxRunes = DefaultContentChunkMaxRunes
	}
	if options.TableMaxRows <= 0 {
		options.TableMaxRows = DefaultContentTableMaxRows
	}
	return options
}

// contentChunkNoiseLabels 构造本次分块使用的噪声标签集合。
func contentChunkNoiseLabels(labels []string) map[string]bool {
	if labels == nil {
		labels = []string{"page_header", "page_footer", "document_index"}
	}
	result := make(map[string]bool, len(labels))
	for _, label := range labels {
		if label = strings.TrimSpace(label); label != "" {
			result[label] = true
		}
	}
	return result
}

// shouldRouteContentMultimodal 判断元素是否应交给 Docling 多模态处理链，
// 避免普通文本 chunk 与图片、表格或公式描述重复入库。
func shouldRouteContentMultimodal(item Item, options ContentChunkOptions) bool {
	if options.IncludeDoclingMultimodal || item.Source != SourceDocling {
		return false
	}
	return item.Type == ItemTypeImage || item.Type == ItemTypeTable || item.Type == ItemTypeEquation
}

// groupContentChunkItems 按章节语义聚合相邻文本，并为无结构文本流提前
// 控制组大小，避免整篇纯文本在中间阶段形成超大字符串。
func groupContentChunkItems(items []Item, maxRunes int) []Item {
	result := make([]Item, 0, len(items))
	var current *Item
	flush := func() {
		if current == nil {
			return
		}
		result = append(result, *current)
		current = nil
	}
	for i := range items {
		item := cloneContentChunkItem(items[i])
		if item.Type != ItemTypeText {
			flush()
			result = append(result, item)
			continue
		}
		unstructured := len(item.SectionPath) == 0 && item.TextLevel == 0
		if current != nil && unstructured && sameContentChunkTextGroup(*current, item) &&
			len([]rune(current.Text))+1+len([]rune(item.Text)) > maxRunes {
			flush()
		}
		if current == nil {
			current = &item
			continue
		}
		if !sameContentChunkTextGroup(*current, item) {
			flush()
			current = &item
			continue
		}
		if current.Text != "" {
			current.Text += "\n"
		}
		current.Text += item.Text
		current.BBox = unionContentChunkBBox(current.BBox, item.BBox)
	}
	flush()
	return result
}

// sameContentChunkTextGroup 判断两个相邻文本能否安全合并；来源与 label 也
// 必须一致，避免聚合时丢失多模态来源或把页眉噪声混入正文。
func sameContentChunkTextGroup(a, b Item) bool {
	return a.TextLevel == b.TextLevel && a.Label == b.Label && a.Source == b.Source &&
		strings.Join(a.SectionPath, "\x00") == strings.Join(b.SectionPath, "\x00")
}

// cloneContentChunkItem 复制分块阶段会引用或修改的切片与边界框字段。
func cloneContentChunkItem(item Item) Item {
	item.SectionPath = append([]string(nil), item.SectionPath...)
	item.BBox = cloneContentChunkBBox(item.BBox)
	return item
}

// splitContentChunkText 按 rune 数切分普通内容，避免按字节截断中文或其他
// 多字节字符；空白片段由调用方过滤。
func splitContentChunkText(text string, maxRunes int) []string {
	runes := []rune(text)
	if len(runes) <= maxRunes {
		return []string{text}
	}
	parts := make([]string, 0, (len(runes)+maxRunes-1)/maxRunes)
	for start := 0; start < len(runes); start += maxRunes {
		end := start + maxRunes
		if end > len(runes) {
			end = len(runes)
		}
		parts = append(parts, string(runes[start:end]))
	}
	return parts
}

// splitContentChunkTable 按数据行拆分 GFM 表格；解析失败或未超限时保持
// ItemToText 原文，超限时每段重复 caption、表头和 footnote。
func splitContentChunkTable(item Item, maxRows int) []string {
	rows := ParseMarkdownTable(item.TableBody)
	if len(rows) <= maxRows+1 {
		return []string{ItemToText(item)}
	}
	header := append([]string(nil), rows[0]...)
	parts := make([]string, 0, (len(rows)-1+maxRows-1)/maxRows)
	for start := 1; start < len(rows); start += maxRows {
		end := start + maxRows
		if end > len(rows) {
			end = len(rows)
		}
		segment := make([][]string, 0, end-start+1)
		segment = append(segment, header)
		segment = append(segment, rows[start:end]...)
		textParts := make([]string, 0, 3)
		if item.TableCaption != "" {
			textParts = append(textParts, item.TableCaption)
		}
		textParts = append(textParts, RenderMarkdownTable(segment))
		if item.TableFootnote != "" {
			textParts = append(textParts, item.TableFootnote)
		}
		parts = append(parts, strings.Join(textParts, "\n"))
	}
	return parts
}

// newContentChunk 从来源 Item 复制检索内容与溯源元数据。
func newContentChunk(item Item, content string) ContentChunk {
	return ContentChunk{
		Content:     content,
		HeadingPath: ItemHeadingPath(item),
		PageIdx:     item.PageIdx,
		OrderIndex:  item.OrderIndex,
		BBox:        cloneContentChunkBBox(item.BBox),
	}
}

// unionContentChunkBBox 返回两个边界框的并集，并始终返回独立副本。
func unionContentChunkBBox(a, b *BBox) *BBox {
	if a == nil {
		return cloneContentChunkBBox(b)
	}
	if b == nil {
		return cloneContentChunkBBox(a)
	}
	return &BBox{
		Left:   math.Min(a.Left, b.Left),
		Top:    math.Min(a.Top, b.Top),
		Right:  math.Max(a.Right, b.Right),
		Bottom: math.Max(a.Bottom, b.Bottom),
	}
}

// cloneContentChunkBBox 复制边界框，防止调用方修改输出时污染历史 Item。
func cloneContentChunkBBox(bbox *BBox) *BBox {
	if bbox == nil {
		return nil
	}
	copy := *bbox
	return &copy
}

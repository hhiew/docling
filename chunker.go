// chunker.go 实现 DoclingDocument 的官方层级语义分块：标题只作为上下文，
// 普通文档元素各自成块，连续列表按 ListGroup 整体序列化，表格保持 GFM
// Markdown 结构。该分块器不做长度裁剪，检索尺寸策略由 ContentChunk 适配器负责。
package docparse

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

const (
	// DocChunkMetaSchemaName 是官方 DocMeta 的固定 schema_name。
	DocChunkMetaSchemaName = "docling_core.transforms.chunker.DocMeta"
	// DocChunkMetaVersion 是当前对齐的官方 DocMeta schema 版本。
	DocChunkMetaVersion = "1.0.0"
)

// DocChunk 表示一个不经过长度切分的层级语义块。
type DocChunk struct {
	Text string       `json:"text"`
	Meta DocChunkMeta `json:"meta"`
}

// DocChunkMeta 保存 chunk 的标题上下文和可回溯来源。DocItems 对齐官方
// DocMeta 必填字段；DocRefs 与 Provenance 是 Go 适配层保留的便捷溯源视图。
type DocChunkMeta struct {
	SchemaName string            `json:"schema_name"`
	Version    string            `json:"version"`
	DocItems   []json.RawMessage `json:"doc_items"`
	DocRefs    []string          `json:"doc_refs"`
	Headings   []string          `json:"headings,omitempty"`
	Provenance []ProvenanceItem  `json:"provenance"`
	Origin     *DocumentOrigin   `json:"origin,omitempty"`
}

// HierarchicalChunks 按 Docling 文档树生成官方语义 chunk。标题依据 level
// 替换当前路径，列表分组整体成块，其他可序列化正文元素各自成块。
func HierarchicalChunks(doc *DoclingDocument) []DocChunk {
	if doc == nil || doc.Body == nil {
		return nil
	}
	walker := hierarchicalChunkWalker{
		doc:             doc,
		headingsByLevel: make(map[int64]string),
		visited:         make(map[string]struct{}),
		excludedCaption: tableCaptionRefs(doc),
	}
	walker.walkChildren(doc.Body.Children)
	return walker.chunks
}

// hierarchicalChunkWalker 保存一次层级遍历的标题路径、去重集合和输出。
type hierarchicalChunkWalker struct {
	doc             *DoclingDocument
	headingsByLevel map[int64]string
	visited         map[string]struct{}
	excludedCaption map[string]struct{}
	chunks          []DocChunk
}

// walkChildren 按文档树顺序遍历一组引用。
func (w *hierarchicalChunkWalker) walkChildren(children []RefItem) {
	for _, ref := range children {
		w.walkRef(ref)
	}
}

// walkRef 根据引用类型更新标题上下文或生成 chunk。
func (w *hierarchicalChunkWalker) walkRef(ref RefItem) {
	refKey := ref.String()
	if _, ok := w.visited[refKey]; ok {
		return
	}
	switch ref.Kind {
	case refTexts:
		w.walkText(ref)
	case refTables:
		w.walkTable(ref)
	case refGroups:
		w.walkGroup(ref)
	case refPictures:
		// 官方分块默认图片占位为空，但仍需越过图片节点遍历其结构化子项。
		w.visited[refKey] = struct{}{}
		if ref.Idx >= 0 && ref.Idx < int64(len(w.doc.Pictures)) {
			w.walkChildren(w.doc.Pictures[ref.Idx].Children)
		}
	default:
		w.visited[refKey] = struct{}{}
	}
}

// walkText 处理标题上下文及普通文本元素。
func (w *hierarchicalChunkWalker) walkText(ref RefItem) {
	refKey := ref.String()
	w.visited[refKey] = struct{}{}
	if ref.Idx < 0 || ref.Idx >= int64(len(w.doc.Texts)) {
		return
	}
	item := w.doc.Texts[ref.Idx]
	if !chunkBodyLayer(item.ContentLayer) {
		w.walkChildren(item.Children)
		return
	}
	if item.Label == LabelTitle || item.Label == LabelSectionHeader {
		level := item.TextLevel
		if item.Label == LabelTitle {
			level = 0
		} else if level <= 0 {
			level = 1
		}
		w.updateHeading(level, chunkText(item))
		w.walkChildren(item.Children)
		return
	}
	if _, excluded := w.excludedCaption[refKey]; !excluded {
		if text := serializeChunkTextItem(item); strings.TrimSpace(text) != "" {
			w.appendChunk(text, []string{itemSelfRef(item.SelfRef, ref)}, item.Prov)
		}
	}
	w.walkChildren(item.Children)
}

// walkGroup 将列表分组整体序列化，其他组织分组仅透传其子节点。
func (w *hierarchicalChunkWalker) walkGroup(ref RefItem) {
	refKey := ref.String()
	w.visited[refKey] = struct{}{}
	if ref.Idx < 0 || ref.Idx >= int64(len(w.doc.Groups)) {
		return
	}
	group := w.doc.Groups[ref.Idx]
	if group.Label != GroupLabelList || !chunkBodyLayer(group.ContentLayer) {
		w.walkChildren(group.Children)
		return
	}
	var builder strings.Builder
	var refs []string
	var provenance []ProvenanceItem
	w.serializeListGroup(ref, group, 0, &builder, &refs, &provenance)
	if text := strings.TrimSpace(builder.String()); text != "" {
		w.appendChunk(text, refs, provenance)
	}
}

// serializeListGroup 按 Markdown 层级序列化一个列表及其嵌套列表。
func (w *hierarchicalChunkWalker) serializeListGroup(ref RefItem, group GroupItem, depth int, builder *strings.Builder, refs *[]string, provenance *[]ProvenanceItem) {
	w.visited[ref.String()] = struct{}{}
	orderedIndex := 0
	for _, child := range group.Children {
		if child.Kind == refGroups {
			if child.Idx >= 0 && child.Idx < int64(len(w.doc.Groups)) {
				nested := w.doc.Groups[child.Idx]
				if nested.Label == GroupLabelList && chunkBodyLayer(nested.ContentLayer) {
					w.serializeListGroup(child, nested, depth+1, builder, refs, provenance)
				}
			}
			continue
		}
		if child.Kind != refTexts || child.Idx < 0 || child.Idx >= int64(len(w.doc.Texts)) {
			continue
		}
		item := w.doc.Texts[child.Idx]
		if item.Label != LabelListItem || !chunkBodyLayer(item.ContentLayer) {
			continue
		}
		w.visited[child.String()] = struct{}{}
		marker := "-"
		if item.Enumerated != nil && *item.Enumerated {
			orderedIndex++
			marker = strings.TrimSpace(item.Marker)
			if marker == "" {
				marker = fmt.Sprintf("%d.", orderedIndex)
			}
		}
		text := chunkText(item)
		if strings.TrimSpace(text) != "" {
			builder.WriteString(strings.Repeat("  ", depth))
			builder.WriteString(marker)
			builder.WriteByte(' ')
			builder.WriteString(text)
			builder.WriteByte('\n')
			*refs = append(*refs, itemSelfRef(item.SelfRef, child))
			*provenance = append(*provenance, cloneChunkProvenance(item.Prov)...)
		}
		for _, nestedRef := range item.Children {
			if nestedRef.Kind != refGroups || nestedRef.Idx < 0 || nestedRef.Idx >= int64(len(w.doc.Groups)) {
				continue
			}
			nested := w.doc.Groups[nestedRef.Idx]
			if nested.Label == GroupLabelList && chunkBodyLayer(nested.ContentLayer) {
				w.serializeListGroup(nestedRef, nested, depth+1, builder, refs, provenance)
			}
		}
	}
}

// walkTable 将表格说明与网格一起序列化为单个 Markdown chunk。
func (w *hierarchicalChunkWalker) walkTable(ref RefItem) {
	w.visited[ref.String()] = struct{}{}
	if ref.Idx < 0 || ref.Idx >= int64(len(w.doc.Tables)) {
		return
	}
	item := w.doc.Tables[ref.Idx]
	if !chunkBodyLayer(item.ContentLayer) || item.Data == nil {
		w.walkChildren(item.Children)
		return
	}
	markdown := renderTableMarkdown(item.Data.TableCells, item.Data.NumRows, item.Data.NumCols)
	if strings.TrimSpace(markdown) == "" {
		w.walkChildren(item.Children)
		return
	}
	refs := make([]string, 0, len(item.Captions)+1)
	provenance := make([]ProvenanceItem, 0, len(item.Prov))
	captionParts := make([]string, 0, len(item.Captions)+1)
	if strings.TrimSpace(item.Caption) != "" {
		captionParts = append(captionParts, strings.TrimSpace(item.Caption))
	}
	for _, captionRef := range item.Captions {
		if captionRef.Kind != refTexts || captionRef.Idx < 0 || captionRef.Idx >= int64(len(w.doc.Texts)) {
			continue
		}
		caption := w.doc.Texts[captionRef.Idx]
		w.visited[captionRef.String()] = struct{}{}
		if text := strings.TrimSpace(chunkText(caption)); text != "" {
			captionParts = append(captionParts, text)
			refs = append(refs, itemSelfRef(caption.SelfRef, captionRef))
			provenance = append(provenance, cloneChunkProvenance(caption.Prov)...)
		}
	}
	if len(captionParts) > 0 {
		markdown = strings.Join(captionParts, "\n") + "\n\n" + markdown
	}
	refs = append(refs, itemSelfRef(item.SelfRef, ref))
	provenance = append(provenance, cloneChunkProvenance(item.Prov)...)
	w.appendChunk(markdown, refs, provenance)
	w.walkChildren(item.Children)
}

// updateHeading 以新标题层级替换同级及更深层级的上下文。
func (w *hierarchicalChunkWalker) updateHeading(level int64, text string) {
	for existingLevel := range w.headingsByLevel {
		if existingLevel >= level {
			delete(w.headingsByLevel, existingLevel)
		}
	}
	if strings.TrimSpace(text) != "" {
		w.headingsByLevel[level] = text
	}
}

// appendChunk 创建带固定 DocMeta schema、来源引用与标题上下文的 chunk。
func (w *hierarchicalChunkWalker) appendChunk(text string, refs []string, provenance []ProvenanceItem) {
	if strings.TrimSpace(text) == "" || len(refs) == 0 {
		return
	}
	w.chunks = append(w.chunks, DocChunk{
		Text: text,
		Meta: DocChunkMeta{
			SchemaName: DocChunkMetaSchemaName,
			Version:    DocChunkMetaVersion,
			DocItems:   w.chunkDocItems(refs),
			DocRefs:    append([]string(nil), refs...),
			Headings:   w.currentHeadings(),
			Provenance: cloneChunkProvenance(provenance),
			Origin:     cloneDocumentOrigin(w.doc.Origin),
		},
	})
}

// chunkDocItems 按稳定 self_ref 复制 chunk 来源元素，生成官方 DocMeta
// 要求的完整 doc_items；无法解析的损坏引用跳过，正常树引用始终可命中。
func (w *hierarchicalChunkWalker) chunkDocItems(refs []string) []json.RawMessage {
	items := make([]json.RawMessage, 0, len(refs))
	for _, ref := range refs {
		parts := strings.Split(strings.TrimPrefix(ref, "#/"), "/")
		if len(parts) != 2 {
			continue
		}
		idx, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil || idx < 0 {
			continue
		}
		var data []byte
		switch parts[0] {
		case string(refTexts):
			if idx >= int64(len(w.doc.Texts)) {
				continue
			}
			item := w.doc.Texts[idx]
			if strings.TrimSpace(item.SelfRef) == "" {
				item.SelfRef = fmt.Sprintf("#/texts/%d", idx)
			}
			data, err = json.Marshal(item)
		case string(refTables):
			if idx >= int64(len(w.doc.Tables)) {
				continue
			}
			item := w.doc.Tables[idx]
			if strings.TrimSpace(item.SelfRef) == "" {
				item.SelfRef = fmt.Sprintf("#/tables/%d", idx)
			}
			data, err = json.Marshal(item)
		case string(refPictures):
			if idx >= int64(len(w.doc.Pictures)) {
				continue
			}
			item := w.doc.Pictures[idx]
			if strings.TrimSpace(item.SelfRef) == "" {
				item.SelfRef = fmt.Sprintf("#/pictures/%d", idx)
			}
			data, err = json.Marshal(item)
		default:
			continue
		}
		if err == nil {
			items = append(items, json.RawMessage(data))
		}
	}
	return items
}

// currentHeadings 按标题 level 升序复制当前上下文。
func (w *hierarchicalChunkWalker) currentHeadings() []string {
	levels := make([]int64, 0, len(w.headingsByLevel))
	for level := range w.headingsByLevel {
		levels = append(levels, level)
	}
	sort.Slice(levels, func(i, j int) bool { return levels[i] < levels[j] })
	if len(levels) == 0 {
		return nil
	}
	headings := make([]string, 0, len(levels))
	for _, level := range levels {
		headings = append(headings, w.headingsByLevel[level])
	}
	return headings
}

// serializeChunkTextItem 按元素语义生成单块文本。
func serializeChunkTextItem(item TextItem) string {
	text := chunkText(item)
	switch item.Label {
	case LabelCode:
		return "```" + canonicalCodeLanguage(item.CodeLanguage) + "\n" + text + "\n```"
	case LabelFormula:
		return "$$" + text + "$$"
	case LabelCheckboxSelected:
		return "- [x] " + text
	case LabelCheckboxUnselected:
		return "- [ ] " + text
	case LabelListItem:
		marker := "-"
		if item.Enumerated != nil && *item.Enumerated && strings.TrimSpace(item.Marker) != "" {
			marker = strings.TrimSpace(item.Marker)
		}
		return marker + " " + text
	default:
		return text
	}
}

// chunkText 返回元素规范文本，兼容只填 orig 的旧文档。
func chunkText(item TextItem) string {
	if item.Text != "" {
		return item.Text
	}
	return item.Orig
}

// chunkBodyLayer 判断节点是否属于默认参与分块的正文层。
func chunkBodyLayer(layer ContentLayer) bool {
	return layer == "" || layer == LayerBody
}

// itemSelfRef 优先保留元素原 self_ref，旧对象缺失时回退到树引用。
func itemSelfRef(selfRef string, fallback RefItem) string {
	if strings.TrimSpace(selfRef) != "" {
		return selfRef
	}
	return fallback.String()
}

// tableCaptionRefs 收集表格说明引用，避免说明同时生成独立文本 chunk。
func tableCaptionRefs(doc *DoclingDocument) map[string]struct{} {
	refs := make(map[string]struct{})
	for _, table := range doc.Tables {
		for _, ref := range table.Captions {
			refs[ref.String()] = struct{}{}
		}
	}
	return refs
}

// cloneChunkProvenance 深拷贝来源坐标，避免调用方修改 chunk 影响原文档。
func cloneChunkProvenance(provenance []ProvenanceItem) []ProvenanceItem {
	if len(provenance) == 0 {
		return []ProvenanceItem{}
	}
	cloned := make([]ProvenanceItem, len(provenance))
	copy(cloned, provenance)
	for i := range cloned {
		if provenance[i].BBox != nil {
			bbox := *provenance[i].BBox
			cloned[i].BBox = &bbox
		}
	}
	return cloned
}

// cloneDocumentOrigin 复制文档来源，保证 chunk 元数据与输入文档互不影响。
func cloneDocumentOrigin(origin *DocumentOrigin) *DocumentOrigin {
	if origin == nil {
		return nil
	}
	cloned := *origin
	return &cloned
}

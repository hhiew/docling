// export.go 实现 DoclingDocument → Markdown 的内容级完整还原导出：
// 按文档树遍历产出结构完整的可读 Markdown（标题层级/段落/列表嵌套与
// 序号/GFM 表格/代码块/公式/图片），用于"还原原始文档"的预览与重建场景。
//
// 层级口径：标题以 TextLevel 优先（语义层级）；TextLevel 缺失时按树深度。
// 列表：list_item 带 Marker（"1."）用之，否则无序 "-"；嵌套按树深度缩进。
package docling

import (
	"fmt"
	"strings"
)

// ExportOptions 控制文档导出范围；Layers 为空时只导出正文层。
type ExportOptions struct {
	Layers []ContentLayer
}

// exportLayerSet 把导出选项转换为便于查询的内容层集合。
func exportLayerSet(options ExportOptions) map[ContentLayer]struct{} {
	layers := options.Layers
	if len(layers) == 0 {
		layers = []ContentLayer{LayerBody}
	}
	set := make(map[ContentLayer]struct{}, len(layers))
	for _, layer := range layers {
		set[layer] = struct{}{}
	}
	return set
}

// exportLayerAllowed 判断元素是否属于本次导出范围；旧文档的空层按正文处理。
func exportLayerAllowed(layers map[ContentLayer]struct{}, layer ContentLayer) bool {
	if layer == "" {
		layer = LayerBody
	}
	_, ok := layers[layer]
	return ok
}

// ExportMarkdown 把 DoclingDocument 还原为完整 Markdown 文档。
// doc 或 body 为空时返回空串。
func ExportMarkdown(doc *DoclingDocument) string {
	return ExportMarkdownWithOptions(doc, ExportOptions{})
}

// ExportMarkdownWithOptions 按指定内容层把文档还原为 Markdown。
func ExportMarkdownWithOptions(doc *DoclingDocument, options ExportOptions) string {
	if doc == nil || doc.Body == nil {
		return ""
	}
	e := &markdownExporter{doc: doc, layers: exportLayerSet(options), visited: map[string]struct{}{}}
	e.walkChildren(doc.Body.Children, 0)
	if doc.Furniture != nil {
		e.walkChildren(doc.Furniture.Children, 0)
	}
	return strings.TrimRight(e.b.String(), "\n") + "\n"
}

// ToMarkdown 把文档还原为 Markdown 的方法形式入口（等价 ExportMarkdown），
// 与 ToHTML/ToContentList 一同构成导出方法族。
func (d *DoclingDocument) ToMarkdown() string { return ExportMarkdown(d) }

// ToMarkdownWithOptions 按指定内容层导出 Markdown。
func (d *DoclingDocument) ToMarkdownWithOptions(options ExportOptions) string {
	return ExportMarkdownWithOptions(d, options)
}

// markdownExporter 还原导出过程状态。
type markdownExporter struct {
	doc     *DoclingDocument
	b       strings.Builder
	layers  map[ContentLayer]struct{}
	visited map[string]struct{}
}

// walkChildren 遍历一组树引用并按类型还原。
func (e *markdownExporter) walkChildren(children []RefItem, depth int) {
	for _, ref := range children {
		if _, exists := e.visited[ref.String()]; exists {
			continue
		}
		e.visited[ref.String()] = struct{}{}
		switch ref.Kind {
		case refTexts:
			if ref.Idx >= 0 && ref.Idx < int64(len(e.doc.Texts)) {
				item := e.doc.Texts[ref.Idx]
				if exportLayerAllowed(e.layers, item.ContentLayer) {
					e.writeText(item, depth)
				}
				// 标题/文本子树（该节点下属内容）继续还原
				e.walkChildren(item.Children, depth)
			}
		case refTables:
			if ref.Idx >= 0 && ref.Idx < int64(len(e.doc.Tables)) {
				item := e.doc.Tables[ref.Idx]
				if exportLayerAllowed(e.layers, item.ContentLayer) {
					e.writeTable(item)
				}
			}
		case refPictures:
			if ref.Idx >= 0 && ref.Idx < int64(len(e.doc.Pictures)) {
				item := e.doc.Pictures[ref.Idx]
				if exportLayerAllowed(e.layers, item.ContentLayer) {
					e.writePicture(item)
				}
			}
		case refGroups:
			if ref.Idx < 0 || ref.Idx >= int64(len(e.doc.Groups)) {
				continue
			}
			g := e.doc.Groups[ref.Idx]
			if !exportLayerAllowed(e.layers, g.ContentLayer) {
				e.walkChildren(g.Children, depth)
				continue
			}
			switch g.Label {
			case GroupLabelList:
				e.walkList(g, depth)
			case GroupLabelSheet:
				// xlsx 还原：sheet 名作为二级标题分隔各 sheet 内容
				fmt.Fprintf(&e.b, "\n## %s\n\n", g.Name)
				e.walkChildren(g.Children, depth+1)
			default:
				// section/slide/chapter 等组织分组仅透传子树
				e.walkChildren(g.Children, depth)
			}
		}
	}
}

// writeText 还原单个文本元素。
func (e *markdownExporter) writeText(item TextItem, depth int) {
	text := item.Orig
	if text == "" {
		text = item.Text
	}
	if strings.TrimSpace(text) == "" {
		return
	}
	switch item.Label {
	case LabelTitle:
		e.b.WriteString("# " + text + "\n\n")
	case LabelCheckboxSelected, LabelCheckboxUnselected:
		// GFM 任务列表：checkbox_selected → [x]，checkbox_unselected → [ ]
		mark := " "
		if item.Label == LabelCheckboxSelected {
			mark = "x"
		}
		e.b.WriteString("- [" + mark + "] " + text + "\n")
	case LabelSectionHeader:
		level := int(item.TextLevel)
		if level <= 0 {
			level = depth + 1
		}
		if level > 6 {
			level = 6
		}
		e.b.WriteString(strings.Repeat("#", level) + " " + text + "\n\n")
	case LabelListItem:
		// 列表项由 walkList 统一处理（正常不会走到这里），兜底按段落
		e.b.WriteString(text + "\n\n")
	case LabelCode:
		e.b.WriteString("```" + item.CodeLanguage + "\n" + text + "\n```\n\n")
	case LabelFormula:
		e.b.WriteString("$$" + text + "$$\n\n")
	default:
		// text/paragraph/page_header/page_footer/document_index/checkbox 等
		e.b.WriteString(text + "\n\n")
	}
}

// walkList 还原列表分组：无序用 "-"，有序优先用 list_item.Marker（缺省按序号生成）；
// 嵌套列表（挂在列表项 children 下的子分组）按深度缩进。
func (e *markdownExporter) walkList(g GroupItem, depth int) {
	num := 0
	for _, ref := range g.Children {
		if ref.Kind != refTexts || ref.Idx < 0 || ref.Idx >= int64(len(e.doc.Texts)) {
			continue
		}
		item := e.doc.Texts[ref.Idx]
		if !exportLayerAllowed(e.layers, item.ContentLayer) {
			continue
		}
		e.visited[ref.String()] = struct{}{}
		text := item.Orig
		if text == "" {
			text = item.Text
		}
		marker := item.Marker
		if item.Enumerated != nil && *item.Enumerated {
			num++
			if marker == "" {
				marker = fmt.Sprintf("%d.", num)
			}
		} else {
			marker = "-"
		}
		if text != "" {
			e.b.WriteString(strings.Repeat("  ", int(depth)) + marker + " " + text + "\n")
		}
		// 列表项下挂的嵌套列表/其他内容
		e.walkChildren(item.Children, depth+1)
	}
	e.b.WriteString("\n")
}

// writeTable 还原表格为 GFM Markdown（网格渲染）。
func (e *markdownExporter) writeTable(item TableItem) {
	if item.Data == nil {
		return
	}
	if body := renderTableMarkdown(item.Data.TableCells, item.Data.NumRows, item.Data.NumCols); body != "" {
		e.b.WriteString(body + "\n\n")
	}
}

// writePicture 还原图片：有 URI 用图片语法，否则占位（caption 优先兜底）。
func (e *markdownExporter) writePicture(item PictureItem) {
	caption := item.Caption
	if caption == "" && len(item.Captions) > 0 {
		if ref := item.Captions[0]; ref.Kind == refTexts && ref.Idx >= 0 && ref.Idx < int64(len(e.doc.Texts)) {
			caption = e.doc.Texts[ref.Idx].Text
		}
	}
	uri := ""
	if item.Image != nil {
		uri = item.Image.URI
	}
	if uri != "" {
		fmt.Fprintf(&e.b, "![%s](%s)\n\n", caption, uri)
	} else if caption != "" {
		e.b.WriteString("[" + caption + "]\n\n")
	}
	if item.Meta != nil && item.Meta.TabularChart != nil && item.Meta.TabularChart.ChartData != nil {
		e.writeTable(TableItem{Data: item.Meta.TabularChart.ChartData})
	}
}

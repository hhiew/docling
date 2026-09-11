// markdown.go 实现 Markdown 到官方 DoclingDocument 的结构化转换。
package docling

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	extast "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/text"
)

// ParseMarkdown 用 goldmark（GFM 扩展）把 Markdown 解析为 DoclingDocument，
// 复刻 Docling md_backend 的 label 规则：一级标题→title、其余标题→section_header、
// 围栏代码块→code（带语言）、列表→list_group+list_item（嵌套列表挂上一列表项，
// 对齐 md_backend 嵌套组织）、GFM 表格→table（首行 column_header，行列为表内偏移）。
// 标题按 Docling Markdown 后端平铺到 body：H1→title，H2-H6→level 1-5；
// content_list 的章节路径由 ToContentList 根据平铺标题顺序推导。
// 输入为空白或解析后无有效内容时返回元素为空的文档（不视为错误），由调用方回退。
func ParseMarkdown(data []byte) (*DoclingDocument, error) {
	source := data
	doc := NewDoclingDocument("markdown")
	astDoc := newKnowledgeMarkdownParser().Parser().Parse(text.NewReader(source))

	for node := astDoc.FirstChild(); node != nil; node = node.NextSibling() {
		switch n := node.(type) {
		case *ast.Heading:
			title := strings.TrimSpace(extractInlinePlainText(n, source))
			if title == "" {
				continue
			}
			if n.Level == 1 {
				ref := doc.AddTitle(title, nil, nil)
				markFlatHeading(doc, ref)
			} else {
				ref := doc.AddHeading(int64(n.Level-1), title, nil, nil)
				markFlatHeading(doc, ref)
			}
		case *extast.Table:
			rows := extractMarkdownTableRows(n, source)
			if len(rows) == 0 {
				continue
			}
			// 数据行直接入网格（分隔行已在提取阶段跳过）；
			// 清洗与分隔行生成统一由渲染链（renderTableMarkdown）处理，
			// 此处不得提前 Postprocess——否则分隔行会入网格并被二次插入
			if len(rows) == 0 {
				continue
			}
			doc.AddTable(tableCellsFromRows(rows), int64(len(rows)), int64(len(rows[0])), nil, nil)
		case *ast.FencedCodeBlock:
			code := fencedCodeText(n, source)
			if strings.TrimSpace(code) == "" {
				continue
			}
			doc.AddCode(code, string(n.Language(source)), nil, nil)
		case *ast.CodeBlock:
			code := strings.TrimSpace(collectCodeBlockLines(n, source))
			if code == "" {
				continue
			}
			doc.AddCode(code, "", nil, nil)
		case *ast.List:
			group := doc.AddListGroup("", nil)
			appendMarkdownList(doc, group, n, source)
		case *ast.HTMLBlock:
			htmlSource := strings.TrimSpace(renderMarkdownBlockText(n, source))
			if htmlSource != "" {
				if sub, err := ParseHTML([]byte(htmlSource)); err == nil {
					mergeOCRSubDocument(doc, sub, 0)
				}
			}
		case *ast.ThematicBreak:
			// 分隔线没有可检索语义。
		default:
			if images := markdownImages(n); len(images) > 0 {
				plainText := strings.TrimSpace(extractInlinePlainText(n, source))
				var imageText strings.Builder
				for _, image := range images {
					caption := strings.TrimSpace(string(image.Title))
					if caption == "" {
						caption = strings.TrimSpace(extractInlinePlainText(image, source))
					}
					imageText.WriteString(strings.TrimSpace(extractInlinePlainText(image, source)))
					addPicturePlaceholder(doc, string(image.Destination), caption, nil)
				}
				// 图片与文字混排时保留段落文字；纯图片段落的 alt 已通过 caption
				// 保存，不再重复生成文本项。
				if plainText != "" && plainText != strings.TrimSpace(imageText.String()) {
					ref := doc.AddText(LabelText, plainText, nil, nil)
					formatting, hyperlink := markdownInlineMeta(n)
					doc.Texts[ref.Idx].Formatting = formatting
					doc.Texts[ref.Idx].Hyperlink = hyperlink
				}
				continue
			}
			blockText := strings.TrimSpace(extractInlinePlainText(n, source))
			if blockText == "" {
				continue
			}
			ref := doc.AddText(LabelText, blockText, nil, nil)
			formatting, hyperlink := markdownInlineMeta(n)
			doc.Texts[ref.Idx].Formatting = formatting
			doc.Texts[ref.Idx].Hyperlink = hyperlink
		}
	}
	return doc, nil
}

// flatHeadingMetaKey 标记由顺序推导章节路径的官方平铺标题。
const flatHeadingMetaKey = "docling__flat_heading"

// markFlatHeading 给 Markdown/HTML 平铺标题写入命名空间元数据，避免把 PDF
// 封面标题等同样挂 body 的非章节标题误用于 content_list 路径。
func markFlatHeading(doc *DoclingDocument, ref RefItem) {
	if ref.Kind != refTexts || ref.Idx < 0 || ref.Idx >= int64(len(doc.Texts)) {
		return
	}
	if doc.Texts[ref.Idx].Meta == nil {
		doc.Texts[ref.Idx].Meta = BaseMeta{}
	}
	doc.Texts[ref.Idx].Meta[flatHeadingMetaKey] = json.RawMessage("true")
}

// markdownImages 收集块中的图片节点，保持源码出现顺序。
func markdownImages(node ast.Node) []*ast.Image {
	var images []*ast.Image
	_ = ast.Walk(node, func(current ast.Node, entering bool) (ast.WalkStatus, error) {
		if entering {
			if image, ok := current.(*ast.Image); ok {
				images = append(images, image)
			}
		}
		return ast.WalkContinue, nil
	})
	return images
}

// markdownInlineMeta 提取块内统一样式和首个链接。Docling TextItem 只允许
// 一组格式与链接字段，因此混合行内内容按“出现过即标记”的稳定策略保存。
func markdownInlineMeta(node ast.Node) (*Formatting, string) {
	formatting := &Formatting{}
	hasFormatting := false
	hyperlink := ""
	_ = ast.Walk(node, func(current ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch inline := current.(type) {
		case *ast.Emphasis:
			hasFormatting = true
			if inline.Level >= 2 {
				formatting.Bold = true
			} else {
				formatting.Italic = true
			}
		case *extast.Strikethrough:
			hasFormatting = true
			formatting.Strikethrough = true
		case *ast.Link:
			if hyperlink == "" {
				hyperlink = string(inline.Destination)
			}
		}
		return ast.WalkContinue, nil
	})
	if !hasFormatting {
		formatting = nil
	}
	return formatting, hyperlink
}

// addPicturePlaceholder 追加无法直接读取像素数据的外部图片占位，并把标题
// 说明保存为官方 captions 引用。
func addPicturePlaceholder(doc *DoclingDocument, uri, caption string, parent *RefItem) RefItem {
	uri = strings.TrimSpace(uri)
	ext := strings.ToLower(filepath.Ext(strings.SplitN(strings.SplitN(uri, "?", 2)[0], "#", 2)[0]))
	mimeType := documentMIMEForExt(ext)
	if strings.HasPrefix(uri, "data:") {
		if end := strings.IndexAny(uri, ";,"); end > len("data:") {
			mimeType = uri[len("data:"):end]
		}
	}
	ref := doc.AddPicture(&ImageRef{Mimetype: mimeType, Dpi: defaultImageDPI, Size: &ImageSize{}, URI: uri}, nil, parent)
	caption = strings.TrimSpace(caption)
	if caption != "" {
		captionRef := RefItem{Kind: refTexts, Idx: int64(len(doc.Texts))}
		doc.Texts = append(doc.Texts, TextItem{
			SelfRef: captionRef.String(), Parent: &ref, Children: []RefItem{},
			ContentLayer: LayerBody, Label: LabelCaption, Prov: []ProvenanceItem{},
			Orig: caption, Text: caption,
		})
		doc.Pictures[ref.Idx].Captions = append(doc.Pictures[ref.Idx].Captions, captionRef)
	}
	return ref
}

// appendMarkdownList 把 goldmark 列表转为 list_group + 逐项 list_item；
// 列表项内的嵌套列表建子分组挂到该列表项下（对齐 md_backend 嵌套组织）。
func appendMarkdownList(doc *DoclingDocument, group RefItem, list *ast.List, source []byte) {
	ordered := list.IsOrdered()
	start := 1
	if list.Start > 0 {
		start = list.Start
	}
	for li, idx := list.FirstChild(), 0; li != nil; li = li.NextSibling() {
		marker := ""
		enumerated := false
		if ordered {
			idx++
			enumerated = true
			marker = fmt.Sprintf("%d.", start+idx-1)
		}
		ref := doc.AddListItem(group, markdownListItemText(li, source), enumerated, marker, nil)
		formatting, hyperlink := markdownInlineMeta(li)
		doc.Texts[ref.Idx].Formatting = formatting
		doc.Texts[ref.Idx].Hyperlink = hyperlink
		for child := li.FirstChild(); child != nil; child = child.NextSibling() {
			if sub, ok := child.(*ast.List); ok {
				subGroup := doc.AddListGroup("", &ref)
				appendMarkdownList(doc, subGroup, sub, source)
			}
		}
	}
}

// markdownListItemText 取列表项内除嵌套列表外的内容文本（保留源码行标记）。
func markdownListItemText(li ast.Node, source []byte) string {
	var lines []string
	for child := li.FirstChild(); child != nil; child = child.NextSibling() {
		if _, ok := child.(*ast.List); ok {
			continue // 嵌套列表由 appendMarkdownList 单独处理
		}
		collectMarkdownBlockLines(child, source, &lines)
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

// tableCellsFromRows 把行列表转为 DoclingTableCell 网格：首行 column_header，
// 偏移 start 闭 end 开（普通单元格 end=start+1）。
func tableCellsFromRows(rows [][]string) []DoclingTableCell {
	cells := make([]DoclingTableCell, 0, len(rows)*len(rows[0]))
	for r, row := range rows {
		for c, text := range row {
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
	return cells
}

// fencedCodeText 取围栏代码块的代码内容（不含围栏行与语言标记），
// 围栏表达由简化器统一补齐。
func fencedCodeText(node *ast.FencedCodeBlock, source []byte) string {
	var b strings.Builder
	if segs := node.Lines(); segs != nil {
		for i := 0; i < segs.Len(); i++ {
			seg := segs.At(i)
			b.Write(seg.Value(source))
		}
	}
	return strings.TrimRight(b.String(), "\r\n")
}

// collectCodeBlockLines 收集缩进代码块的代码行。
func collectCodeBlockLines(node ast.Node, source []byte) string {
	var lines []string
	if segs := node.Lines(); segs != nil {
		for i := 0; i < segs.Len(); i++ {
			seg := segs.At(i)
			line := strings.TrimRight(string(seg.Value(source)), "\r\n")
			if strings.TrimSpace(line) != "" {
				lines = append(lines, line)
			}
		}
	}
	return strings.Join(lines, "\n")
}

// newKnowledgeMarkdownParser 创建启用 GFM 扩展（表格/删除线/任务列表/链接自动识别）的解析器。
// goldmark 对任意输入都能解析，不会产生解析错误。
func newKnowledgeMarkdownParser() goldmark.Markdown {
	return goldmark.New(goldmark.WithExtensions(extension.GFM))
}

// renderMarkdownBlockText 把块级节点还原为保留原始 Markdown 语法的文本：
// 叶子块按源码区间切取原文（保留列表标记、引用前缀等），容器块（列表/引用）
// 递归子块后按行拼接。
func renderMarkdownBlockText(node ast.Node, source []byte) string {
	lines := make([]string, 0, 8)
	collectMarkdownBlockLines(node, source, &lines)
	return strings.Join(lines, "\n")
}

// collectMarkdownBlockLines 递归收集块级节点的源码文本行。
// 引用与列表的源码区间保留原始标记（列表/引用/代码块统一文本化）。
func collectMarkdownBlockLines(node ast.Node, source []byte, lines *[]string) {
	if segs := node.Lines(); segs != nil && segs.Len() > 0 {
		for i := 0; i < segs.Len(); i++ {
			seg := segs.At(i)
			line := strings.TrimRight(string(seg.Value(source)), "\r\n")
			if strings.TrimSpace(line) != "" {
				*lines = append(*lines, line)
			}
		}
		return
	}
	// 容器块（列表/引用等）无自身文本行，递归处理子块
	for child := node.FirstChild(); child != nil; child = child.NextSibling() {
		collectMarkdownBlockLines(child, source, lines)
	}
}

// export_html.go 实现 DoclingDocument → HTML 的结构化导出，与 ExportMarkdown
// 保持同构的元素覆盖面：title/section 标题（h1-h6 按层级）、段落文本、列表
// （组与列表项嵌套，有序/无序自动判定）、表格（首行表头 th + 跨行跨列）、
// 图片（<img src=URI> + caption）、代码块（<pre><code data-lang>）与公式
// （LaTeX 原文以 <span class="formula"> 包裹，不要求前端渲染），
// 输出完整 <!DOCTYPE html> 文档，用于 Web 预览与富文本下游消费。
package docparse

import (
	"fmt"
	"html"
	"strings"
)

// ExportHTML 把 DoclingDocument 导出为完整 HTML 文档（<!DOCTYPE html>）。
// doc 或 body 为空时返回仅含骨架的空文档；<title> 优先取 Meta.Title，
// 缺省回退文档名。
func ExportHTML(doc *DoclingDocument) string {
	return ExportHTMLWithOptions(doc, ExportOptions{})
}

// ExportHTMLWithOptions 按指定内容层把文档导出为完整 HTML 文档。
func ExportHTMLWithOptions(doc *DoclingDocument, options ExportOptions) string {
	var b strings.Builder
	b.WriteString("<!DOCTYPE html>\n<html>\n<head>\n<meta charset=\"utf-8\">\n")
	title := ""
	if doc != nil {
		if doc.Meta != nil && doc.Meta.Title != "" {
			title = doc.Meta.Title
		} else {
			title = doc.Name
		}
	}
	b.WriteString("<title>" + html.EscapeString(title) + "</title>\n</head>\n<body>\n")
	if doc == nil || doc.Body == nil {
		b.WriteString("</body>\n</html>\n")
		return b.String()
	}
	e := &htmlExporter{doc: doc, layers: exportLayerSet(options), visited: map[string]struct{}{}}
	e.walkChildren(doc.Body.Children, 0)
	if doc.Furniture != nil {
		e.walkChildren(doc.Furniture.Children, 0)
	}
	b.WriteString(e.b.String())
	b.WriteString("</body>\n</html>\n")
	return b.String()
}

// ToHTML 导出 HTML 的方法形式入口（等价 ExportHTML）。
func (d *DoclingDocument) ToHTML() string { return ExportHTML(d) }

// ToHTMLWithOptions 按指定内容层导出 HTML。
func (d *DoclingDocument) ToHTMLWithOptions(options ExportOptions) string {
	return ExportHTMLWithOptions(d, options)
}

// htmlExporter HTML 导出过程状态。
type htmlExporter struct {
	doc     *DoclingDocument
	b       strings.Builder
	layers  map[ContentLayer]struct{}
	visited map[string]struct{}
}

// walkChildren 遍历一组树引用并按类型渲染（与 ExportMarkdown 同构的分派）。
func (e *htmlExporter) walkChildren(children []RefItem, depth int) {
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
				// 标题/文本子树（该节点下属内容）继续渲染
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
				// xlsx：sheet 名作为二级标题分隔各 sheet 内容（对齐 ExportMarkdown）
				fmt.Fprintf(&e.b, "<h2>%s</h2>\n", html.EscapeString(g.Name))
				e.walkChildren(g.Children, depth+1)
			default:
				// section/slide/chapter 等组织分组仅透传子树
				e.walkChildren(g.Children, depth)
			}
		}
	}
}

// writeText 渲染单个文本元素（覆盖面与 ExportMarkdown.writeText 一致）。
func (e *htmlExporter) writeText(item TextItem, depth int) {
	text := item.Orig
	if text == "" {
		text = item.Text
	}
	if strings.TrimSpace(text) == "" {
		return
	}
	switch item.Label {
	case LabelTitle:
		e.b.WriteString("<h1>" + html.EscapeString(text) + "</h1>\n")
	case LabelSectionHeader:
		level := int(item.TextLevel)
		if level <= 0 {
			level = depth + 1
		}
		if level > 6 {
			level = 6
		}
		fmt.Fprintf(&e.b, "<h%d>%s</h%d>\n", level, html.EscapeString(text), level)
	case LabelListItem:
		// 列表项由 walkList 统一处理（正常不会走到这里），兜底按段落
		e.b.WriteString("<p>" + html.EscapeString(text) + "</p>\n")
	case LabelCode:
		lang := ""
		if item.CodeLanguage != "" {
			lang = ` data-lang="` + html.EscapeString(item.CodeLanguage) + `"`
		}
		e.b.WriteString("<pre><code" + lang + ">" + html.EscapeString(text) + "</code></pre>\n")
	case LabelFormula:
		// 公式存 LaTeX 原文，formula 类名供前端按需接入渲染库
		e.b.WriteString(`<p><span class="formula">` + html.EscapeString(text) + "</span></p>\n")
	default:
		// text/paragraph/page_header/page_footer/document_index/checkbox 等
		e.b.WriteString("<p>" + html.EscapeString(text) + "</p>\n")
	}
}

// walkList 渲染列表分组：组内任一列表项有序（Enumerated）用 <ol>，否则 <ul>；
// 列表项下挂的嵌套列表/内容递归渲染进 <li>。
func (e *htmlExporter) walkList(g GroupItem, depth int) {
	ordered := false
	for _, ref := range g.Children {
		if ref.Kind != refTexts || ref.Idx < 0 || ref.Idx >= int64(len(e.doc.Texts)) {
			continue
		}
		if t := e.doc.Texts[ref.Idx]; t.Enumerated != nil && *t.Enumerated {
			ordered = true
			break
		}
	}
	tag := "ul"
	if ordered {
		tag = "ol"
	}
	e.b.WriteString("<" + tag + ">\n")
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
		if text == "" && len(item.Children) == 0 {
			continue
		}
		// 子内容经独立 exporter 渲染（追加进当前 <li> 内部）
		sub := &htmlExporter{doc: e.doc, layers: e.layers, visited: e.visited}
		sub.walkChildren(item.Children, depth+1)
		e.b.WriteString("<li>" + html.EscapeString(text))
		if sub.b.Len() > 0 {
			e.b.WriteString("\n" + strings.TrimRight(sub.b.String(), "\n"))
		}
		e.b.WriteString("</li>\n")
	}
	e.b.WriteString("</" + tag + ">\n")
}

// writeTable 渲染表格为 HTML <table>：按 cell 行列偏移铺格还原网格，
// 首行 column_header 行输出 <thead><th>，其余输出 <tbody><td>；单元格仅在其
// start 偏移锚点输出一次，跨行/跨列以 rowspan/colspan 表达（被覆盖格跳过）。
func (e *htmlExporter) writeTable(item TableItem) {
	if item.Data == nil || item.Data.NumRows <= 0 || item.Data.NumCols <= 0 || len(item.Data.TableCells) == 0 {
		return
	}
	td := item.Data
	rows := make([][]*DoclingTableCell, td.NumRows)
	for i := range rows {
		rows[i] = make([]*DoclingTableCell, td.NumCols)
	}
	for i := range td.TableCells {
		c := &td.TableCells[i]
		for r := c.StartRowOffsetIdx; r < c.EndRowOffsetIdx && r < td.NumRows; r++ {
			for col := c.StartColOffsetIdx; col < c.EndColOffsetIdx && col < td.NumCols; col++ {
				rows[r][col] = c
			}
		}
	}
	hasHeader := false
	for col := int64(0); col < td.NumCols; col++ {
		if rows[0][col] != nil && rows[0][col].ColumnHeader {
			hasHeader = true
			break
		}
	}
	e.b.WriteString("<table>\n")
	if hasHeader {
		e.b.WriteString("<thead>\n<tr>")
		for col := int64(0); col < td.NumCols; col++ {
			e.b.WriteString("<th>" + html.EscapeString(tableCellText(rows[0][col])) + "</th>")
		}
		e.b.WriteString("</tr>\n</thead>\n")
	}
	e.b.WriteString("<tbody>\n")
	for r := int64(0); r < td.NumRows; r++ {
		if hasHeader && r == 0 {
			continue
		}
		e.b.WriteString("<tr>")
		for col := int64(0); col < td.NumCols; col++ {
			c := rows[r][col]
			// 非 start 锚点的覆盖格不再输出（span 已由锚格表达）
			if c == nil || c.StartRowOffsetIdx != r || c.StartColOffsetIdx != col {
				continue
			}
			tag := "td"
			if c.ColumnHeader {
				tag = "th"
			}
			attrs := ""
			if c.ColSpan > 1 {
				attrs += fmt.Sprintf(` colspan="%d"`, c.ColSpan)
			}
			if c.RowSpan > 1 {
				attrs += fmt.Sprintf(` rowspan="%d"`, c.RowSpan)
			}
			fmt.Fprintf(&e.b, "<%s%s>%s</%s>", tag, attrs, html.EscapeString(c.Text), tag)
		}
		e.b.WriteString("</tr>\n")
	}
	e.b.WriteString("</tbody>\n</table>\n")
}

// tableCellText 取格子文本（nil 格为空串，对应网格空洞）。
func tableCellText(c *DoclingTableCell) string {
	if c == nil {
		return ""
	}
	return c.Text
}

// writePicture 渲染图片：<figure> 包裹 <img>（URI 为空时输出占位块）与
// <figcaption>（caption 文本，旧版字符串 Caption 优先，新版 Captions 引用兜底）。
func (e *htmlExporter) writePicture(item PictureItem) {
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
	e.b.WriteString("<figure>\n")
	if uri != "" {
		fmt.Fprintf(&e.b, "<img src=\"%s\" alt=\"%s\">\n", html.EscapeString(uri), html.EscapeString(caption))
	} else {
		e.b.WriteString("<div class=\"image-placeholder\">[图片]</div>\n")
	}
	if caption != "" {
		e.b.WriteString("<figcaption>" + html.EscapeString(caption) + "</figcaption>\n")
	}
	e.b.WriteString("</figure>\n")
	if item.Meta != nil && item.Meta.TabularChart != nil && item.Meta.TabularChart.ChartData != nil {
		e.writeTable(TableItem{Data: item.Meta.TabularChart.ChartData})
	}
}

// html.go 把 HTML 文档解析为 DoclingDocument，是 docling 通用组件的 HTML 后端。
// 块级标签分发、平铺标题、列表/表格/代码块/图片语义、<br> 哨兵换行与
// 隐藏元素过滤均复刻 docling 的 html_backend.py；
// 图片占位和首标题前 furniture 分层对齐 Docling HTML 后端；prov 全空不生成。
package docling

import (
	"bytes"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/net/html"
)

// htmlBrSentinel <br> 哨兵字符（Unicode 私有区）：复刻 docling 用哨兵区分
// 显式换行与源码换行的做法——单个哨兵表示段内换行，连续两个及以上表示分段。
const htmlBrSentinel = "\ue000"

// htmlBlockTags 触发独立 Docling 元素的块级标签（对齐 html_backend 的 _BLOCK_TAGS）。
var htmlBlockTags = map[string]bool{
	"address": true, "details": true, "dl": true, "figure": true,
	"footer": true, "h1": true, "h2": true, "h3": true, "h4": true,
	"h5": true, "h6": true, "ol": true, "p": true, "pre": true,
	"summary": true, "table": true, "ul": true,
}

// htmlInlineTags 行内容器标签：其子树文本并入当前段落缓冲
// （取 html_backend 的 _INLINE_HTML_TAGS 与 _FORMAT_TAG_MAP 的并集）。
var htmlInlineTags = map[string]bool{
	"a": true, "abbr": true, "b": true, "bdi": true, "bdo": true,
	"cite": true, "code": true, "data": true, "del": true, "dfn": true,
	"em": true, "i": true, "ins": true, "kbd": true, "mark": true,
	"q": true, "s": true, "samp": true, "small": true, "span": true,
	"strong": true, "sub": true, "sup": true, "u": true, "var": true,
}

// htmlDisplayNoneRe 内联样式中 display:none 的匹配（隐藏元素判定之一）。
var htmlDisplayNoneRe = regexp.MustCompile(`(?i)display\s*:\s*none`)

// htmlNewlineSpacesRe 压缩换行符两侧空格（对齐 docling 的 " *\n *" → "\n" 清洗）。
var htmlNewlineSpacesRe = regexp.MustCompile(` *\n *`)

// htmlLeadingIntRe 提取属性值开头连续数字（colspan/rowspan 解析容错）。
var htmlLeadingIntRe = regexp.MustCompile(`^\d+`)

// ParseHTML 解析 HTML 为 DoclingDocument（复刻 docling html_backend 语义）。
// html 解析器自容错，畸形输入不报错；空输入或无有效内容时返回元素为空的文档。
func ParseHTML(data []byte) (*DoclingDocument, error) {
	doc := NewDoclingDocument("html")
	if strings.TrimSpace(string(data)) == "" {
		return doc, nil
	}
	root, err := html.Parse(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("docling: 解析 html 失败: %w", err)
	}
	body := htmlFindElement(root, "body")
	if body == nil {
		body = root
	}
	hasBodyHeading := htmlFindAnyHeading(body) != nil
	if title := htmlFindElement(root, "title"); title != nil {
		if value := strings.TrimSpace(htmlElementText(title)); value != "" {
			furniture := doc.furnitureRef()
			doc.AddTitle(value, nil, &furniture)
		}
	}
	p := &htmlParser{doc: doc, hasDocumentHeading: hasBodyHeading}
	p.walk(body, nil)
	setTreeContentLayer(doc, doc.Furniture.Children, LayerFurniture)
	return doc, nil
}

// htmlParser HTML 遍历状态：标题按官方协议平铺，首个标题前的块挂 furniture。
type htmlParser struct {
	doc                *DoclingDocument
	hasDocumentHeading bool
	seenHeading        bool
}

// htmlInlineBuf 段落行内文本缓冲：收集行内标签与文本节点的纯文本，
// <br> 以哨兵记录；hyperlink 记录段落内首个链接目标。
type htmlInlineBuf struct {
	text          string
	hyperlink     string
	formatting    Formatting
	hasFormatting bool
}

// addText 追加折叠空白后的文本片段；两侧均非空时按 docling
// simplify_text_elements 的分隔语义补一个空格。
func (b *htmlInlineBuf) addText(s string) {
	if s == "" {
		return
	}
	if b.text != "" && !strings.HasSuffix(b.text, " ") && !strings.HasPrefix(s, " ") {
		b.text += " "
	}
	b.text += s
}

// addBR 追加一个 <br> 哨兵。
func (b *htmlInlineBuf) addBR() {
	b.text += htmlBrSentinel
}

// nearestHeadingParent 在首标题之前返回 furniture，之后标题与正文均平铺到 body。
func (p *htmlParser) nearestHeadingParent() *RefItem {
	if p.hasDocumentHeading && !p.seenHeading {
		ref := p.doc.furnitureRef()
		return &ref
	}
	return nil
}

// walk 遍历块级上下文的子节点：行内文本进缓冲，块级标签在缓冲冲刷后分发处理，
// 复刻 html_backend _walk 的"只在块边界产出文本节点"语义。
func (p *htmlParser) walk(n *html.Node, parent *RefItem) {
	buf := &htmlInlineBuf{}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		switch {
		case c.Type == html.TextNode:
			buf.addText(htmlCollapseSpaces(c.Data))
		case c.Type != html.ElementNode:
			// 注释等非元素节点不产出内容
		case htmlIsSuppressed(c):
			// script/style/隐藏子树整体忽略
		case c.Data == "br":
			buf.addBR()
		case c.Data == "img":
			p.flushInline(buf, parent)
			p.emitImageAlt(c, parent)
		case htmlBlockTags[c.Data]:
			p.flushInline(buf, parent)
			p.handleBlock(c, parent)
		case htmlInlineTags[c.Data]:
			// 行内标签极少见地包裹了块级内容时退化为递归遍历，避免块结构丢失
			if htmlContainsBlock(c) {
				p.flushInline(buf, parent)
				p.walk(c, parent)
			} else {
				p.collectInline(c, buf)
			}
		default:
			// div/section 等通用容器：含块级后代时递归遍历，否则按行内文本并入段落
			if htmlContainsBlock(c) {
				p.flushInline(buf, parent)
				p.walk(c, parent)
			} else {
				p.collectInline(c, buf)
			}
		}
	}
	p.flushInline(buf, parent)
}

// collectInline 把元素子树的文本并入行内缓冲（保留 <br> 哨兵与首个链接目标）。
func (p *htmlParser) collectInline(n *html.Node, buf *htmlInlineBuf) {
	switch {
	case n.Type == html.TextNode:
		buf.addText(htmlCollapseSpaces(n.Data))
	case n.Type != html.ElementNode:
		// 注释等非元素节点不产出内容
	case htmlIsSuppressed(n):
		// 隐藏/脚本子树整体忽略
	case n.Data == "br":
		buf.addBR()
	case n.Data == "a":
		if buf.hyperlink == "" {
			if href := strings.TrimSpace(htmlAttr(n, "href")); href != "" {
				buf.hyperlink = href
			}
		}
		p.collectChildrenInline(n, buf)
	case n.Data == "strong" || n.Data == "b":
		buf.hasFormatting = true
		buf.formatting.Bold = true
		p.collectChildrenInline(n, buf)
	case n.Data == "em" || n.Data == "i":
		buf.hasFormatting = true
		buf.formatting.Italic = true
		p.collectChildrenInline(n, buf)
	case n.Data == "u" || n.Data == "ins":
		buf.hasFormatting = true
		buf.formatting.Underline = true
		p.collectChildrenInline(n, buf)
	case n.Data == "s" || n.Data == "del":
		buf.hasFormatting = true
		buf.formatting.Strikethrough = true
		p.collectChildrenInline(n, buf)
	default:
		p.collectChildrenInline(n, buf)
	}
}

// collectChildrenInline 递归收集元素全部子节点进行内缓冲。
func (p *htmlParser) collectChildrenInline(n *html.Node, buf *htmlInlineBuf) {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		p.collectInline(c, buf)
	}
}

// flushInline 把行内缓冲按 <br> 规则切分产出 text 元素：单个哨兵为段内换行，
// 连续两个以上为分段；段落整体的首个链接目标写入 Hyperlink。
// parent 为 nil 时挂最近的标题父节点（对齐 markdown.go 的标题父挂接模式）。
func (p *htmlParser) flushInline(buf *htmlInlineBuf, parent *RefItem) {
	if parent == nil {
		parent = p.nearestHeadingParent()
	}
	text, hyperlink := buf.text, buf.hyperlink
	formatting, hasFormatting := buf.formatting, buf.hasFormatting
	buf.text, buf.hyperlink, buf.formatting, buf.hasFormatting = "", "", Formatting{}, false
	for _, seg := range splitByHtmlBR(text) {
		ref := p.doc.AddText(LabelText, seg, nil, parent)
		if hyperlink != "" {
			p.doc.Texts[ref.Idx].Hyperlink = hyperlink
		}
		if hasFormatting {
			p.doc.Texts[ref.Idx].Formatting = &formatting
		}
	}
}

// splitByHtmlBR 按哨兵切分段落：双哨兵为段界，单哨兵替换为换行并压缩换行
// 两侧空格（对齐 html_backend split_by_newline），空段丢弃。
func splitByHtmlBR(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, htmlBrSentinel+htmlBrSentinel)
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.ReplaceAll(part, htmlBrSentinel, "\n")
		part = htmlNewlineSpacesRe.ReplaceAllString(part, "\n")
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

// handleBlock 块级标签分发（对齐 html_backend _handle_block 的分支结构）。
// parent 为 nil 时取最近的标题父节点：顶层块的挂接跟随当前标题栈。
func (p *htmlParser) handleBlock(n *html.Node, parent *RefItem) {
	if parent == nil {
		parent = p.nearestHeadingParent()
	}
	switch n.Data {
	case "h1", "h2", "h3", "h4", "h5", "h6":
		p.handleHeading(n, parent)
	case "p", "address", "summary":
		p.handleParagraph(n, parent)
	case "ul", "ol":
		p.handleList(n, parent)
	case "dl":
		p.handleDescList(n, parent)
	case "pre":
		p.handlePre(n, parent)
	case "table":
		p.handleTable(n, parent)
	case "figure":
		p.handleFigure(n, parent)
	case "details", "footer":
		// 语义容器：建 SECTION 分组后递归处理内容（对齐 _use_details/_use_footer）
		group := p.doc.AddSectionGroup(n.Data, parent)
		p.walk(n, &group)
	}
}

// handleHeading 处理 h1-h6：h1 产出 title 并清空层级栈（开启新文档级章节），
// h2-h6 产出 section_header（TextLevel 保持原始级别）；标题作为后续内容父节点。
func (p *htmlParser) handleHeading(n *html.Node, _ *RefItem) {
	buf := &htmlInlineBuf{}
	p.collectInline(n, buf)
	// 标题为单行语义：哨兵换行折叠为空格
	title := strings.Join(strings.Fields(strings.ReplaceAll(buf.text, htmlBrSentinel, " ")), " ")
	if title == "" {
		return
	}
	level := int(n.Data[1] - '0')
	p.seenHeading = true
	if level == 1 {
		ref := p.doc.AddTitle(title, nil, nil)
		markFlatHeading(p.doc, ref)
	} else {
		ref := p.doc.AddHeading(int64(level-1), title, nil, nil)
		markFlatHeading(p.doc, ref)
	}
}

// handleParagraph 处理 p/address/summary：行内文本按 <br> 规则切分产出
// text 元素，段内嵌套 img 的 alt 兜底产出 caption 文本。
func (p *htmlParser) handleParagraph(n *html.Node, parent *RefItem) {
	buf := &htmlInlineBuf{}
	p.collectInline(n, buf)
	p.flushInline(buf, parent)
	p.emitNestedImages(n, parent)
}

// emitNestedImages 产出块元素子树内 img 的 alt 文本（跳过嵌套表格/图片容器，
// 避免与其各自分支重复产出）。
func (p *htmlParser) emitNestedImages(n *html.Node, parent *RefItem) {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type != html.ElementNode || htmlIsSuppressed(c) {
			continue
		}
		switch c.Data {
		case "img":
			p.emitImageAlt(c, parent)
		case "table", "figure":
			// 表格/图片容器内的图片由各自分支处理
		default:
			p.emitNestedImages(c, parent)
		}
	}
}

// emitImageAlt 为 img 产出 PictureItem 占位，alt 作为 captions 引用。
func (p *htmlParser) emitImageAlt(n *html.Node, parent *RefItem) {
	alt := strings.TrimSpace(htmlAttr(n, "alt"))
	if parent == nil {
		parent = p.nearestHeadingParent()
	}
	addPicturePlaceholder(p.doc, htmlAttr(n, "src"), alt, parent)
}

// handlePre 处理 pre 代码块：保留换行提取文本，语言从 class 的
// language-xxx / lang-xxx 前缀提取。
func (p *htmlParser) handlePre(n *html.Node, parent *RefItem) {
	code := strings.TrimSpace(htmlPreText(n))
	if code == "" {
		return
	}
	p.doc.AddCode(code, htmlCodeLanguage(n), nil, parent)
}

// htmlPreText 保留换行提取 pre 子树文本：文本节点原样保留，<br> 转换行，
// script/style 等隐藏子树跳过。
func htmlPreText(n *html.Node) string {
	var b strings.Builder
	var rec func(*html.Node)
	rec = func(x *html.Node) {
		if x.Type == html.TextNode {
			b.WriteString(x.Data)
			return
		}
		if x.Type != html.ElementNode || htmlIsSuppressed(x) {
			return
		}
		if x.Data == "br" {
			b.WriteString("\n")
			return
		}
		for c := x.FirstChild; c != nil; c = c.NextSibling {
			rec(c)
		}
	}
	rec(n)
	return b.String()
}

// htmlCodeLanguage 从 pre 及其内 code 元素的 class 中提取
// language-xxx / lang-xxx 形式的语言提示（对齐 _code_language_hint 的前缀优先策略）。
func htmlCodeLanguage(pre *html.Node) string {
	var classes []string
	var rec func(*html.Node)
	rec = func(x *html.Node) {
		if x.Type != html.ElementNode {
			return
		}
		classes = append(classes, htmlClassList(x)...)
		// 仅深入 pre 自身与 code 子元素（对齐 hint 只看这两个元素）
		if x != pre && x.Data != "code" {
			return
		}
		for c := x.FirstChild; c != nil; c = c.NextSibling {
			rec(c)
		}
	}
	rec(pre)
	for _, cl := range classes {
		for _, prefix := range []string{"language-", "lang-"} {
			if strings.HasPrefix(cl, prefix) {
				return strings.TrimPrefix(cl, prefix)
			}
		}
	}
	return ""
}

// handleList 处理 ul/ol：建 list 分组后逐项产出 list_item，列表项内的
// 嵌套列表挂到该列表项下（对齐 html_backend 的列表项上下文）；ol 产出
// "N." 有序编号标记（start 属性修正起点）。
func (p *htmlParser) handleList(n *html.Node, parent *RefItem) {
	ordered := n.Data == "ol"
	name := "list"
	start := 1
	if ordered {
		name = "ordered list"
		if s, err := strconv.Atoi(strings.TrimSpace(htmlAttr(n, "start"))); err == nil && s >= 0 {
			start = s
		}
	}
	group := p.doc.AddListGroup(name, parent)
	counter := 0
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type != html.ElementNode || htmlIsSuppressed(c) {
			continue
		}
		switch c.Data {
		case "li":
			text := htmlListItemText(c)
			marker, enumerated := "", false
			if ordered {
				counter++
				enumerated = true
				marker = fmt.Sprintf("%d.", start+counter-1)
			}
			if text == "" {
				// 空列表项不产出，其嵌套列表回退挂到列表分组
				for _, sub := range htmlNestedLists(c) {
					p.handleBlock(sub, &group)
				}
				continue
			}
			itemRef := p.doc.AddListItem(group, text, enumerated, marker, nil)
			for _, sub := range htmlNestedLists(c) {
				p.handleBlock(sub, &itemRef)
			}
		case "ul", "ol", "dl":
			// 病态 HTML 中直接嵌在列表下的子列表（对齐 html_backend :2532 的容错）
			p.handleBlock(c, &group)
		}
	}
}

// htmlNestedLists 找列表项子树中的顶层嵌套列表（穿透 div 等容器，
// 不深入已找到的列表内部，避免重复展开）。
func htmlNestedLists(li *html.Node) []*html.Node {
	var out []*html.Node
	var rec func(*html.Node)
	rec = func(x *html.Node) {
		for c := x.FirstChild; c != nil; c = c.NextSibling {
			if c.Type != html.ElementNode || htmlIsSuppressed(c) {
				continue
			}
			switch c.Data {
			case "ul", "ol", "dl":
				out = append(out, c)
			case "table", "figure":
				// 表格/图片容器内的列表由各自分支处理
			default:
				rec(c)
			}
		}
	}
	rec(li)
	return out
}

// htmlListItemText 提取列表项纯文本：排除嵌套列表/表格子树，
// 全部空白折叠为单空格（对齐 html_backend 列表项的单行文本语义）。
func htmlListItemText(li *html.Node) string {
	var parts []string
	var rec func(*html.Node)
	rec = func(x *html.Node) {
		switch {
		case x.Type == html.TextNode:
			if s := htmlCollapseSpaces(x.Data); s != "" {
				parts = append(parts, s)
			}
			return
		case x.Type != html.ElementNode:
			return
		case htmlIsSuppressed(x):
			return
		}
		switch x.Data {
		case "ul", "ol", "dl", "table", "figure":
			return // 嵌套列表/表格由独立分支处理
		case "br":
			return // 列表项为单行语义，显式换行并入空格
		}
		for c := x.FirstChild; c != nil; c = c.NextSibling {
			rec(c)
		}
	}
	rec(li)
	return strings.Join(parts, " ")
}

// handleDescList 处理 dl：dt/dd 均简化为普通 text 元素挂列表分组
// （docling 的 dt 加粗与 descriptions 子分组在 Go 协议无对应字段，
// 按任务约定简化），项内嵌套列表挂到对应项。
func (p *htmlParser) handleDescList(n *html.Node, parent *RefItem) {
	group := p.doc.AddListGroup("description list", parent)
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type != html.ElementNode || htmlIsSuppressed(c) {
			continue
		}
		switch c.Data {
		case "dt", "dd":
			text := htmlListItemText(c)
			if text == "" {
				continue
			}
			ref := p.doc.AddText(LabelText, text, nil, &group)
			for _, sub := range htmlNestedLists(c) {
				p.handleBlock(sub, &ref)
			}
		case "ul", "ol", "dl":
			p.handleBlock(c, &group)
		}
	}
}

// handleFigure 处理 figure：img 产出图片占位，figcaption 优先、alt 兜底。
func (p *htmlParser) handleFigure(n *html.Node, parent *RefItem) {
	caption := ""
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && c.Data == "figcaption" && !htmlIsSuppressed(c) {
			buf := &htmlInlineBuf{}
			p.collectInline(c, buf)
			caption = strings.Join(strings.Fields(strings.ReplaceAll(buf.text, htmlBrSentinel, " ")), " ")
			break
		}
	}
	img := htmlFindElement(n, "img")
	if img == nil {
		if strings.TrimSpace(caption) != "" {
			p.doc.AddText(LabelCaption, strings.TrimSpace(caption), nil, parent)
		}
		return
	}
	if caption == "" {
		caption = strings.TrimSpace(htmlAttr(img, "alt"))
	}
	addPicturePlaceholder(p.doc, htmlAttr(img, "src"), strings.TrimSpace(caption), parent)
}

// handleTable 处理 table：thead/tbody 解包（tfoot 内行按 docling recursive=False
// 的行为不参与）、不递归嵌套表；行内无 td 则该行标记 column_header，数据行中的
// th 单元格标记 row_header；colspan/rowspan 转单元格跨度（start 闭 end 开），
// 已占格跳过。单元格定位逻辑逐条对齐 html_backend parse_table_data。
func (p *htmlParser) handleTable(n *html.Node, parent *RefItem) {
	rows := htmlTableRows(n)
	// 先算行列规模：row_header 行（全 th 且跨多行）不计入行数，colspan 计入列数
	numRows, numCols := 0, 0
	for _, row := range rows {
		colCount, rowHeader := 0, true
		for _, cell := range htmlRowCells(row) {
			colSpan, rowSpan := htmlCellSpans(cell)
			colCount += colSpan
			if cell.Data == "td" || rowSpan == 1 {
				rowHeader = false
			}
		}
		if colCount > numCols {
			numCols = colCount
		}
		if !rowHeader {
			numRows++
		}
	}
	if numRows == 0 || numCols == 0 {
		return
	}
	occupied := make([][]bool, numRows)
	for i := range occupied {
		occupied[i] = make([]bool, numCols)
	}
	cells := make([]DoclingTableCell, 0, numRows*numCols)
	richCells := make([]htmlInlineBuf, 0, numRows*numCols)
	rowIdx, startRowSpan := -1, 0
	for _, row := range rows {
		colHeader, rowHeader := true, true
		for _, cell := range htmlRowCells(row) {
			_, rowSpan := htmlCellSpans(cell)
			if cell.Data == "td" {
				colHeader, rowHeader = false, false
			} else if rowSpan == 1 {
				rowHeader = false
			}
		}
		if !rowHeader {
			rowIdx++
			startRowSpan = 0
		} else {
			// row_header 行不占用数据行号，其后单元格行号顺延偏移
			startRowSpan++
		}
		colIdx := 0
		for _, cell := range htmlRowCells(row) {
			text := htmlCellText(cell)
			var rich htmlInlineBuf
			p.collectInline(cell, &rich)
			colSpan, rowSpan := htmlCellSpans(cell)
			if rowHeader {
				rowSpan--
			}
			// 跳过已被前方 rowspan 单元格占用的列
			for colIdx < numCols && rowIdx+startRowSpan < numRows && occupied[rowIdx+startRowSpan][colIdx] {
				colIdx++
			}
			for r := startRowSpan; r < startRowSpan+rowSpan; r++ {
				for c := 0; c < colSpan; c++ {
					if rowIdx+r < numRows && colIdx+c < numCols {
						occupied[rowIdx+r][colIdx+c] = true
					}
				}
			}
			cells = append(cells, DoclingTableCell{
				Text:              text,
				RowSpan:           int64(rowSpan),
				ColSpan:           int64(colSpan),
				StartRowOffsetIdx: int64(rowIdx + startRowSpan),
				EndRowOffsetIdx:   int64(rowIdx + startRowSpan + rowSpan),
				StartColOffsetIdx: int64(colIdx),
				EndColOffsetIdx:   int64(colIdx + colSpan),
				ColumnHeader:      colHeader,
				RowHeader:         !colHeader && cell.Data == "th",
			})
			richCells = append(richCells, rich)
		}
	}
	tableRef := p.doc.AddTable(cells, int64(numRows), int64(numCols), nil, parent)
	for index, rich := range richCells {
		if index >= len(p.doc.Tables[tableRef.Idx].Data.TableCells) ||
			(!rich.hasFormatting && rich.hyperlink == "") {
			continue
		}
		text := p.doc.Tables[tableRef.Idx].Data.TableCells[index].Text
		textRef := RefItem{Kind: refTexts, Idx: int64(len(p.doc.Texts))}
		item := TextItem{
			SelfRef: textRef.String(), Parent: &tableRef, Children: []RefItem{},
			ContentLayer: LayerBody, Label: LabelText, Prov: []ProvenanceItem{},
			Orig: text, Text: text, Hyperlink: rich.hyperlink,
		}
		if rich.hasFormatting {
			formatting := rich.formatting
			item.Formatting = &formatting
		}
		p.doc.Texts = append(p.doc.Texts, item)
		p.doc.Tables[tableRef.Idx].Data.TableCells[index].Ref = &textRef
	}
}

// htmlTableRows 收集表格数据行：table 直接子级的 tr，加上 thead/tbody
// 直接子级的 tr（模拟 docling 的 thead/tbody unwrap）。
func htmlTableRows(table *html.Node) []*html.Node {
	var rows []*html.Node
	for c := table.FirstChild; c != nil; c = c.NextSibling {
		if c.Type != html.ElementNode {
			continue
		}
		switch c.Data {
		case "tr":
			rows = append(rows, c)
		case "thead", "tbody":
			for cc := c.FirstChild; cc != nil; cc = cc.NextSibling {
				if cc.Type == html.ElementNode && cc.Data == "tr" {
					rows = append(rows, cc)
				}
			}
		}
	}
	return rows
}

// htmlRowCells 取表格行直接子级的 td/th 单元格（不递归，天然不支持嵌套表）。
func htmlRowCells(row *html.Node) []*html.Node {
	var cells []*html.Node
	for c := row.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && (c.Data == "td" || c.Data == "th") {
			cells = append(cells, c)
		}
	}
	return cells
}

// htmlCellSpans 取单元格 colspan/rowspan（缺省或非法时为 1）。
func htmlCellSpans(cell *html.Node) (int, int) {
	return htmlAttrInt(cell, "colspan"), htmlAttrInt(cell, "rowspan")
}

// htmlAttrInt 解析整型属性值：取开头连续数字，缺省/非法返回 1，超大值钳制
// 到 4096（对齐 _get_cell_spans 的容错语义，并防御病态 colspan 导致的长循环）。
func htmlAttrInt(n *html.Node, name string) int {
	if m := htmlLeadingIntRe.FindString(strings.TrimSpace(htmlAttr(n, name))); m != "" {
		if num, err := strconv.Atoi(m); err == nil {
			if num > 4096 {
				return 4096
			}
			return num
		}
	}
	return 1
}

// htmlCellText 提取单元格文本：<br> 转换行，p/li/th/td 子级拼接后补尾空格
// （对齐 html_backend get_text 的分隔行为），去除首尾空白。
func htmlCellText(cell *html.Node) string {
	var b strings.Builder
	var rec func(*html.Node)
	rec = func(x *html.Node) {
		switch {
		case x.Type == html.TextNode:
			b.WriteString(strings.ReplaceAll(x.Data, htmlBrSentinel, "\n"))
			return
		case x.Type != html.ElementNode:
			return
		case htmlIsSuppressed(x):
			return
		}
		if x.Data == "br" {
			b.WriteString("\n")
			return
		}
		for c := x.FirstChild; c != nil; c = c.NextSibling {
			rec(c)
		}
		if x.Data == "p" || x.Data == "li" || x.Data == "th" || x.Data == "td" {
			b.WriteString(" ")
		}
	}
	rec(cell)
	return strings.TrimSpace(b.String())
}

// htmlAttr 取元素属性值（不存在返回空串）。
func htmlAttr(n *html.Node, name string) string {
	for _, a := range n.Attr {
		if a.Key == name {
			return a.Val
		}
	}
	return ""
}

// htmlHasAttr 判断元素是否存在指定属性（无值属性如 hidden 也算存在）。
func htmlHasAttr(n *html.Node, name string) bool {
	for _, a := range n.Attr {
		if a.Key == name {
			return true
		}
	}
	return false
}

// htmlFindElement 深度优先查找第一个指定名称的元素（含起点自身）。
func htmlFindElement(n *html.Node, name string) *html.Node {
	if n.Type == html.ElementNode && strings.EqualFold(n.Data, name) {
		return n
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if found := htmlFindElement(c, name); found != nil {
			return found
		}
	}
	return nil
}

// htmlFindAnyHeading 深度优先查找正文中的首个 h1-h6。
func htmlFindAnyHeading(n *html.Node) *html.Node {
	if n.Type == html.ElementNode && len(n.Data) == 2 && n.Data[0] == 'h' && n.Data[1] >= '1' && n.Data[1] <= '6' {
		return n
	}
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		if found := htmlFindAnyHeading(child); found != nil {
			return found
		}
	}
	return nil
}

// htmlElementText 提取元素的可见纯文本并折叠空白。
func htmlElementText(n *html.Node) string {
	var parts []string
	var walk func(*html.Node)
	walk = func(current *html.Node) {
		if current.Type == html.TextNode {
			if text := htmlCollapseSpaces(current.Data); text != "" {
				parts = append(parts, text)
			}
			return
		}
		for child := current.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(n)
	return strings.Join(parts, " ")
}

// setTreeContentLayer 把指定根引用的完整子树设置为同一内容层。用于 HTML
// 首标题前内容：节点在构建时已挂 furniture，此处统一修正节点层与说明引用。
func setTreeContentLayer(doc *DoclingDocument, refs []RefItem, layer ContentLayer) {
	for _, ref := range refs {
		switch ref.Kind {
		case refTexts:
			if ref.Idx >= 0 && ref.Idx < int64(len(doc.Texts)) {
				doc.Texts[ref.Idx].ContentLayer = layer
				setTreeContentLayer(doc, doc.Texts[ref.Idx].Children, layer)
			}
		case refTables:
			if ref.Idx >= 0 && ref.Idx < int64(len(doc.Tables)) {
				doc.Tables[ref.Idx].ContentLayer = layer
				setTreeContentLayer(doc, doc.Tables[ref.Idx].Captions, layer)
				if doc.Tables[ref.Idx].Data != nil {
					for _, cell := range doc.Tables[ref.Idx].Data.TableCells {
						if cell.Ref != nil {
							setTreeContentLayer(doc, []RefItem{*cell.Ref}, layer)
						}
					}
				}
				setTreeContentLayer(doc, doc.Tables[ref.Idx].Children, layer)
			}
		case refPictures:
			if ref.Idx >= 0 && ref.Idx < int64(len(doc.Pictures)) {
				doc.Pictures[ref.Idx].ContentLayer = layer
				setTreeContentLayer(doc, doc.Pictures[ref.Idx].Captions, layer)
				setTreeContentLayer(doc, doc.Pictures[ref.Idx].Children, layer)
			}
		case refGroups:
			if ref.Idx >= 0 && ref.Idx < int64(len(doc.Groups)) {
				doc.Groups[ref.Idx].ContentLayer = layer
				setTreeContentLayer(doc, doc.Groups[ref.Idx].Children, layer)
			}
		}
	}
}

// htmlIsSuppressed 元素是否应整体忽略：脚本/样式类标签，或带 hidden 属性、
// aria-hidden、display:none 内联样式的隐藏元素（对齐 html_backend 的
// script/noscript/style 清理与 _is_invisible_tag 判定）。
func htmlIsSuppressed(n *html.Node) bool {
	switch n.Data {
	case "script", "noscript", "style", "template", "head", "title":
		return true
	}
	if htmlHasAttr(n, "hidden") {
		return true
	}
	switch strings.ToLower(strings.TrimSpace(htmlAttr(n, "aria-hidden"))) {
	case "true", "1", "yes":
		return true
	}
	if htmlDisplayNoneRe.MatchString(htmlAttr(n, "style")) {
		return true
	}
	return false
}

// htmlContainsBlock 判断子树中是否包含块级标签或 img（决定通用容器
// 走递归遍历还是行内并入）。
func htmlContainsBlock(n *html.Node) bool {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type != html.ElementNode || htmlIsSuppressed(c) {
			continue
		}
		if htmlBlockTags[c.Data] || c.Data == "img" || htmlContainsBlock(c) {
			return true
		}
	}
	return false
}

// htmlClassList 取元素 class 属性按空白切分的列表。
func htmlClassList(n *html.Node) []string {
	return strings.Fields(htmlAttr(n, "class"))
}

// htmlCollapseSpaces 折叠文本片段内空白为单个空格并去首尾（对齐 HTML 规范的
// 源码换行折叠行为；docling 用 " ".join(text.split()) 实现）。
func htmlCollapseSpaces(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

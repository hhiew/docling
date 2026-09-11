// docx.go 实现 docx（WordprocessingML）的纯规则解析后端，产出 DoclingDocument。
// 行为复刻 Docling 的 msword_backend.py（.reference/docling 源码核对），复刻要点：
//   - body 逐元素遍历（_walk_linear）：w:p 段落、w:tbl 表格、w:sdt 递归；
//     含图片（a:blip / v:imagedata）的段落产出 picture 元素（rId 经
//     document.xml.rels 定位 word/media 部件内嵌 data URI，段落文本作
//     caption；超出 Docling msword 后端"仅保留文本"行为的 P0 增强）；
//     c:chart 关系委托共享 OOXML 图表解析器生成带 tabular_chart 的 picture；
//     w:txbxContent 文本框按锚点顺序递归产出正文元素；
//   - 页眉页脚关系部件输出 page_header/page_footer 并固定为 furniture 层；
//     comments.xml 输出 notes 层 comment_section，并回填正文 Comments 引用；
//     footnotes.xml 输出 footnote 元素，并与正文或表格单元格建立双向引用；
//   - run 级粗体/斜体/下划线/删除线/上下标汇总到 formatting，关系超链接
//     写入 hyperlink；表格单元格通过 ref 关联同等富文本元数据；
//   - label 判定（_get_label_and_level）：样式 id/name/base_style 任一含
//     "heading"（不区分大小写）→ section_header，层级优先取样式定义的
//     w:outlineLvl+1（仅 1-9 有效），否则从 "Heading N" 形态解析；
//     Title 样式 → title（挂 body 顶层，后续一级标题挂其下）；
//   - Code 样式（id/name 精确匹配集合，沿 base_style 链回溯）→ code 元素，
//     连续 Code 段落合并为同一元素（\n 连接，空段缓冲一个换行）；
//   - w14:checkbox 段落 → checkbox_selected / checkbox_unselected（勾选状态
//     由 w14:checked val 判定，前缀 ☐☑☒ 由 AddCheckbox 清除）；
//   - m:oMath：独立公式段落 → formula，常见 OMML 结构纯 Go 转 LaTeX，
//     行内公式按位置并入段落文本；未知节点递归保留可见内容；
//   - 列表（_manage_list_structure 简化版）：w:numPr（numId+ilvl，ilvl 缺省 0，
//     numId=0 视为无列表）→ list_group + list_item；numFmt 属于可见编号集合
//     （decimal/lowerRoman/upperRoman/lowerLetter/upperLetter/decimalZero）时
//     为有序（marker "1."/"2." 组内自增），否则无序（marker ""）；
//     不同 numId 切换新建分组；ilvl 增加时子分组挂上一级列表项（嵌套）；
//   - 标题层级树（_add_heading）：维护 parents 槽位数组（键 0 为 Title 槽），
//     跳级时补隐式 section 分组（name="header-i"），标题挂 parents[level-1]；
//   - 表格（_handle_tables）：1x1 表视为版式容器（单元格内容当正文递归处理，
//     不产出 table）；gridSpan → ColSpan；vMerge continue 扩展锚单元格的
//     EndRowOffsetIdx 与 RowSpan；gridBefore 处理行首列偏移；首行
//     column_header=true；单元格多段落 "\n" 连接；
//   - 不生成 prov（docx 无页面概念，对齐 Docling）。
//
// 无样式标题 fallback 为超出 Docling msword 后端的增强（对齐版面模型的识别
// 效果，Docling 纯规则后端只认样式）：第一遍解析后全文档零 heading/title
// 元素时，对收集的段落中间表示（run 加粗/字号）重跑标题判定，命中的手工
// 加粗/大字号短段改标 section_header 并线性重挂层级；见 applyUnstyledHeadingFallback。
//
// 文本抽取保留流式状态机口径：w:t 文本、w:tab→\t、w:br→\n。
// 内容为空时返回元素为空的文档（不视为错误）；畸形 zip/xml 返回 error。
package docparse

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"image"
	"io"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// OOXML 命名空间 URL（按 Space 精确识别的元素才区分归属）。
const (
	// wordNS WordprocessingML 主命名空间。
	wordNS = "http://schemas.openxmlformats.org/wordprocessingml/2006/main"
	// wordMathNS OMML 公式命名空间（m:oMath / m:t）。
	wordMathNS = "http://schemas.openxmlformats.org/officeDocument/2006/math"
	// wordW14NS Word 2010 扩展命名空间（w14:checkbox）。
	wordW14NS = "http://schemas.microsoft.com/office/word/2010/wordml"
	// wordRevisionsMetaKey 保存 Word 修订记录，不改变当前接受视图文本。
	wordRevisionsMetaKey = "docparse__word_revisions"
	// wordFieldsMetaKey 保存 Word 域指令，正文仍使用文档内缓存的显示结果。
	wordFieldsMetaKey = "docparse__word_fields"
)

// ParseDocx 解析 docx（zip 打包的 WordprocessingML）为 DoclingDocument。
// 参数 data 为 docx 文件字节流；返回文档树与错误。
// word/document.xml 缺失、zip 损坏或 XML 畸形时返回 error；
// 文档内容为空时返回元素为空的文档（不视为错误）。
// word/styles.xml 与 word/numbering.xml 缺失或畸形时忽略（按无样式/无编号处理）。
// 解析完成后若全文档零 heading/title 元素，触发无样式标题 fallback
// （手工加粗/大字号短段重判为标题，超出 Docling msword 后端的增强）。
func ParseDocx(data []byte) (*DoclingDocument, error) {
	var err error
	data, err = normalizeStrictOOXMLPackage(data)
	if err != nil {
		return nil, fmt.Errorf("docparse: 归一化 Strict DOCX 失败: %w", err)
	}
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}
	documentXML, err := readZipFileBytes(reader, "word/document.xml")
	if err != nil {
		return nil, err
	}
	if documentXML == nil {
		return nil, fmt.Errorf("word/document.xml not found")
	}
	// 辅助部件缺失/畸形不影响主解析，按无样式与无编号定义兜底
	stylesXML, _ := readZipFileBytes(reader, "word/styles.xml")
	numberingXML, _ := readZipFileBytes(reader, "word/numbering.xml")
	// 图片关系映射（rId → media 部件路径）与文档元数据（docProps/core.xml）：
	// 均为增强能力，缺失/畸形不影响主解析
	relsXML, _ := readZipFileBytes(reader, "word/_rels/document.xml.rels")
	imageRels := parseOOXMLRelationships(relsXML, "/image", "word")
	hyperlinkRels := parseDocxRelationshipTargets(relsXML, "/hyperlink", "word")
	chartRels := parseOOXMLRelationships(relsXML, "/chart", "word")
	diagramRels := parseOOXMLRelationships(relsXML, "/diagramData", "word")
	oleRels := parseOOXMLRelationships(relsXML, "/oleObject", "word")

	w := &docxWalker{
		doc:             NewDoclingDocument("docx"),
		styles:          parseWordStyles(stylesXML),
		numbering:       parseWordNumbering(numberingXML),
		parents:         map[int64]RefItem{},
		paraMetas:       map[int64]docxParaMeta{},
		reader:          reader,
		imageRels:       imageRels,
		hyperlinks:      hyperlinkRels,
		chartRels:       chartRels,
		diagramRels:     diagramRels,
		oleRels:         oleRels,
		mediaCache:      map[string]*ImageRef{},
		commentTargets:  map[string][]RefItem{},
		footnoteTargets: map[string][]RefItem{},
		endnoteTargets:  map[string][]RefItem{},
	}
	w.doc.Meta = ooxmlCorePropsMeta(reader)
	if err := w.walkDocument(documentXML); err != nil {
		return nil, err
	}
	// 第一遍解析照旧，全文档无标题时对段落中间表示重跑标题判定并重建层级
	w.applyUnstyledHeadingFallback()
	// 脚注、批注与页眉页脚在正文树稳定后追加，避免附属内容参与标题 fallback。
	w.addWordFootnotes(reader)
	w.addWordEndnotes(reader)
	w.addWordComments(reader)
	w.addWordFurniture(documentXML, relsXML)
	return w.doc, nil
}

// readZipFileBytes 读取 zip 内指定条目的字节内容；条目不存在时返回 (nil, nil)。
func readZipFileBytes(reader *zip.Reader, name string) ([]byte, error) {
	for _, f := range reader.File {
		if f.Name != name {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		content, err := io.ReadAll(rc)
		rc.Close()
		return content, err
	}
	return nil, nil
}

// parseDocxRelationshipTargets 解析 DOCX 关系目标。外部关系（如超链接）
// 保留原始 URI，内部关系按所属部件目录归一化为 zip 内路径。
func parseDocxRelationshipTargets(xmlBytes []byte, relTypeSuffix, baseDir string) map[string]string {
	targets := map[string]string{}
	dec := xml.NewDecoder(bytes.NewReader(xmlBytes))
	for {
		token, err := dec.Token()
		if err != nil {
			return targets
		}
		start, ok := token.(xml.StartElement)
		if !ok || start.Name.Local != "Relationship" {
			continue
		}
		relType := xmlAttrVal(start.Attr, "Type")
		id := xmlAttrVal(start.Attr, "Id")
		target := xmlAttrVal(start.Attr, "Target")
		if id == "" || target == "" || !strings.HasSuffix(relType, relTypeSuffix) {
			continue
		}
		if !strings.EqualFold(xmlAttrVal(start.Attr, "TargetMode"), "External") {
			if strings.HasPrefix(target, "/") {
				target = strings.TrimPrefix(target, "/")
			} else {
				target = path.Clean(path.Join(baseDir, target))
			}
		}
		targets[id] = target
	}
}

// docxWalker docx 解析状态机：持有文档树、样式/编号表、标题层级树、
// 列表序列状态与 code 合并链状态。
type docxWalker struct {
	doc       *DoclingDocument
	styles    map[string]wordStyleDef
	numbering *wordNumbering
	// parents 标题层级树槽位：键 0 为 Title 槽位，1.. 为各级标题/隐式分组，
	// 与 msword_backend 的 self.parents 字典同构（缺失键等价 None）。
	parents map[int64]RefItem

	// 图片提取状态（P0 增强）：zip 读取器（media 部件惰性读取）、
	// rId → media 部件路径映射与已读取部件的 data URI 缓存。
	reader      *zip.Reader
	imageRels   map[string]string
	mediaCache  map[string]*ImageRef
	hyperlinks  map[string]string
	chartRels   map[string]string
	diagramRels map[string]string
	oleRels     map[string]string

	// commentTargets 记录正文段落命中的批注 id，待 body 解析完成后再建立
	// notes 层 comment_section 并回填细粒度引用，确保正文阅读顺序不被打断。
	commentTargets map[string][]RefItem
	// footnoteTargets 记录正文或表格单元格中的 w:footnoteReference，待正文
	// 树稳定后从 footnotes.xml 建立官方 footnotes 引用。
	footnoteTargets map[string][]RefItem
	// endnoteTargets 与 footnoteTargets 同构；Docling 统一使用 footnote 标签，
	// 但仍保留 Word 尾注正文与引用关系。
	endnoteTargets map[string][]RefItem

	// 列表序列状态（_manage_list_structure 简化版）：同一 numId 的连续列表项
	// 共享分组栈；任何非列表元素产出后关闭。
	listActive   bool
	listNumID    int64
	listGroups   []RefItem // 各 ilvl 的 list_group（下标 = ilvl）
	listItems    []RefItem // 各 ilvl 最近的列表项（子分组挂靠目标）
	listCounters []int64   // 各 ilvl 的有序 marker 计数

	// code 合并链状态（:2196）：仅当上一个产出元素是 code 且未被其他元素
	// 打断时才合并；forceNewCode 在 1x1 表边界阻断跨边界合并。
	lastCode         RefItem
	lastIsCode       bool
	pendingCodeBlank int
	forceNewCode     bool

	// paraMetas 无样式标题 fallback 的段落中间表示（键为 Texts 下标）：
	// 仅记录普通 text 元素（列表/代码/复选框/公式元素天然不参与重判）。
	paraMetas map[int64]docxParaMeta
}

// wordComment 表示 comments.xml 中一个批注或回复实体。
type wordComment struct {
	id       string
	paraID   string
	parentID string
	author   string
	initials string
	created  string
	resolved *bool
	text     string
}

// wordCommentExtension 保存 commentsExtended.xml 通过末段 paraId 提供的
// 回复父级和解决状态。
type wordCommentExtension struct {
	parentParaID string
	resolved     *bool
}

// parseWordComments 按 XML 顺序读取批注标识、作者、时间和段落文本。
func parseWordComments(xmlBytes, extensionsXML []byte) []wordComment {
	extensions := parseWordCommentExtensions(extensionsXML)
	dec := xml.NewDecoder(bytes.NewReader(xmlBytes))
	var comments []wordComment
	for {
		token, err := dec.Token()
		if err != nil {
			return comments
		}
		start, ok := token.(xml.StartElement)
		if !ok || start.Name.Local != "comment" {
			continue
		}
		raw, err := collectSubTree(dec, start)
		if err != nil {
			return comments
		}
		parts := wordParagraphTexts(raw)
		text := strings.TrimSpace(strings.Join(parts, "\n"))
		if id := xmlAttrVal(start.Attr, "id"); id != "" && text != "" {
			paraID := lastWordCommentParaID(raw)
			extension := extensions[paraID]
			comments = append(comments, wordComment{
				id: id, paraID: paraID, author: xmlAttrVal(start.Attr, "author"),
				initials: xmlAttrVal(start.Attr, "initials"), created: xmlAttrVal(start.Attr, "date"),
				resolved: extension.resolved, text: text,
			})
		}
	}
}

// lastWordCommentParaID 返回批注正文最后一个段落的 w14:paraId，用于关联
// commentsExtended.xml 中的回复和解决状态。
func lastWordCommentParaID(raw []byte) string {
	dec := xml.NewDecoder(bytes.NewReader(raw))
	paraID := ""
	for {
		token, err := dec.Token()
		if err != nil {
			return paraID
		}
		start, ok := token.(xml.StartElement)
		if ok && start.Name.Local == "p" {
			if value := xmlAttrVal(start.Attr, "paraId"); value != "" {
				paraID = value
			}
		}
	}
}

// parseWordCommentExtensions 读取 Office 2013+ commentsExtended.xml；键为
// comments.xml 批注末段 paraId。
func parseWordCommentExtensions(xmlBytes []byte) map[string]wordCommentExtension {
	extensions := map[string]wordCommentExtension{}
	dec := xml.NewDecoder(bytes.NewReader(xmlBytes))
	for {
		token, err := dec.Token()
		if err != nil {
			return extensions
		}
		start, ok := token.(xml.StartElement)
		if !ok || start.Name.Local != "commentEx" {
			continue
		}
		paraID := xmlAttrVal(start.Attr, "paraId")
		if paraID == "" {
			continue
		}
		extensions[paraID] = wordCommentExtension{
			parentParaID: xmlAttrVal(start.Attr, "paraIdParent"),
			resolved:     officeResolvedStatus(xmlAttrVal(start.Attr, "done")),
		}
	}
}

// wordParagraphTexts 收集任意 WordprocessingML 部件中的段落文本，保持文档顺序。
func wordParagraphTexts(xmlBytes []byte) []string {
	dec := xml.NewDecoder(bytes.NewReader(xmlBytes))
	var paragraphs []string
	for {
		token, err := dec.Token()
		if err != nil {
			return paragraphs
		}
		start, ok := token.(xml.StartElement)
		if !ok || start.Name.Local != "p" {
			continue
		}
		raw, err := collectSubTree(dec, start)
		if err != nil {
			return paragraphs
		}
		info, err := parseWordParagraph(raw)
		if err == nil {
			paragraphs = append(paragraphs, strings.TrimSpace(sanitizeText(info.text)))
		}
		for _, textBox := range extractWordTextBoxes(raw) {
			paragraphs = append(paragraphs, wordParagraphTexts(textBox)...)
		}
	}
}

// wordFootnote 保存 footnotes.xml 中一个用户脚注的编号与正文。
type wordFootnote struct {
	id   string // id 对应 document.xml 的 w:footnoteReference。
	text string // text 是按段落顺序拼接并保留公式 LaTeX 的脚注正文。
}

// parseWordFootnotes 按部件顺序解析用户脚注；OOXML 预留的分隔线和续页
// 分隔线使用非正数 id，不作为文档语义输出。
func parseWordFootnotes(xmlBytes []byte) []wordFootnote {
	return parseWordNoteElements(xmlBytes, "footnote")
}

// parseWordEndnotes 按部件顺序解析用户尾注。
func parseWordEndnotes(xmlBytes []byte) []wordFootnote {
	return parseWordNoteElements(xmlBytes, "endnote")
}

// parseWordNoteElements 解析 footnote/endnote 共用的 WordprocessingML 结构。
func parseWordNoteElements(xmlBytes []byte, elementName string) []wordFootnote {
	decoder := xml.NewDecoder(bytes.NewReader(xmlBytes))
	var footnotes []wordFootnote
	for {
		token, err := decoder.Token()
		if err != nil {
			return footnotes
		}
		start, ok := token.(xml.StartElement)
		if !ok || start.Name.Local != elementName || (start.Name.Space != "" && start.Name.Space != wordNS) {
			continue
		}
		id := xmlAttrVal(start.Attr, "id")
		numericID, parseErr := strconv.ParseInt(id, 10, 64)
		raw, collectErr := collectSubTree(decoder, start)
		if collectErr != nil {
			return footnotes
		}
		if parseErr != nil || numericID <= 0 {
			continue
		}
		paragraphs := wordParagraphTexts(raw)
		filtered := paragraphs[:0]
		for _, paragraph := range paragraphs {
			if text := strings.TrimSpace(paragraph); text != "" {
				filtered = append(filtered, text)
			}
		}
		if text := strings.Join(filtered, "\n"); text != "" {
			footnotes = append(footnotes, wordFootnote{id: id, text: text})
		}
	}
}

// addWordFootnotes 将被正文引用的脚注追加为 target 子节点，并同步填充
// 官方 footnotes 引用；未引用脚注及损坏部件不会影响正文解析。
func (w *docxWalker) addWordFootnotes(reader *zip.Reader) {
	footnotesXML, err := readZipFileBytes(reader, "word/footnotes.xml")
	if err != nil || len(footnotesXML) == 0 {
		return
	}
	for _, footnote := range parseWordFootnotes(footnotesXML) {
		for _, target := range w.footnoteTargets[footnote.id] {
			ref := w.doc.AddText(LabelFootnote, footnote.text, nil, &target)
			w.attachWordFootnote(target, ref)
		}
	}
}

// addWordEndnotes 将 Word 尾注按 Docling 的 footnote 标签追加到引用目标。
func (w *docxWalker) addWordEndnotes(reader *zip.Reader) {
	endnotesXML, err := readZipFileBytes(reader, "word/endnotes.xml")
	if err != nil || len(endnotesXML) == 0 {
		return
	}
	for _, endnote := range parseWordEndnotes(endnotesXML) {
		for _, target := range w.endnoteTargets[endnote.id] {
			ref := w.doc.AddText(LabelFootnote, endnote.text, nil, &target)
			w.attachWordFootnote(target, ref)
		}
	}
}

// attachWordFootnote 把脚注引用附加到正文文本、图片或表格。
func (w *docxWalker) attachWordFootnote(target, footnote RefItem) {
	switch target.Kind {
	case refTexts:
		if target.Idx >= 0 && target.Idx < int64(len(w.doc.Texts)) {
			w.doc.Texts[target.Idx].Footnotes = append(w.doc.Texts[target.Idx].Footnotes, footnote)
		}
	case refPictures:
		if target.Idx >= 0 && target.Idx < int64(len(w.doc.Pictures)) {
			w.doc.Pictures[target.Idx].Footnotes = append(w.doc.Pictures[target.Idx].Footnotes, footnote)
		}
	case refTables:
		if target.Idx >= 0 && target.Idx < int64(len(w.doc.Tables)) {
			w.doc.Tables[target.Idx].Footnotes = append(w.doc.Tables[target.Idx].Footnotes, footnote)
		}
	}
}

// addWordComments 把 DOCX 批注转换为 notes 层 comment_section，并把被批注
// 元素的 Comments 指向该分组。当前字符范围按元素全文记录。
func (w *docxWalker) addWordComments(reader *zip.Reader) {
	commentsXML, err := readZipFileBytes(reader, "word/comments.xml")
	if err != nil || len(commentsXML) == 0 {
		return
	}
	extensionsXML, _ := readZipFileBytes(reader, "word/commentsExtended.xml")
	comments := parseWordComments(commentsXML, extensionsXML)
	extensions := parseWordCommentExtensions(extensionsXML)
	paraToID := make(map[string]string, len(comments))
	for _, comment := range comments {
		if comment.paraID != "" {
			paraToID[comment.paraID] = comment.id
		}
	}
	for i := range comments {
		if extension := extensions[comments[i].paraID]; extension.parentParaID != "" {
			comments[i].parentID = paraToID[extension.parentParaID]
		}
	}
	commentByID := make(map[string]wordComment, len(comments))
	for _, comment := range comments {
		commentByID[comment.id] = comment
	}
	rootID := func(id string) string {
		origin := id
		seen := map[string]bool{}
		for id != "" && !seen[id] {
			seen[id] = true
			parent := commentByID[id].parentID
			if parent == "" {
				return id
			}
			id = parent
		}
		// 畸形回复环仍各自落入稳定分组，不能产生空引用。
		return origin
	}
	groups := map[string]RefItem{}
	for _, comment := range comments {
		root := rootID(comment.id)
		if _, exists := groups[root]; exists {
			continue
		}
		rootComment := commentByID[root]
		meta := officeCommentMeta(officeCommentData{
			ID: rootComment.id, Author: officeCommentAuthor{Name: rootComment.author, Initials: rootComment.initials},
			Created: rootComment.created, Resolved: rootComment.resolved,
		})
		group := GroupItem{
			Parent: w.doc.parentOrBody(nil), Children: []RefItem{}, ContentLayer: LayerNotes,
			Label: GroupLabelCommentSection, Name: "comment-" + root, Meta: meta,
		}
		w.doc.Groups = append(w.doc.Groups, group)
		groupRef := RefItem{Kind: refGroups, Idx: int64(len(w.doc.Groups) - 1)}
		w.doc.Groups[groupRef.Idx].SelfRef = groupRef.String()
		w.doc.appendChild(nil, groupRef)
		groups[root] = groupRef
	}
	for _, comment := range comments {
		groupRef := groups[rootID(comment.id)]
		noteRef := w.doc.AddText(LabelText, comment.text, nil, &groupRef)
		w.doc.Texts[noteRef.Idx].ContentLayer = LayerNotes
		w.doc.Texts[noteRef.Idx].Meta = officeCommentMeta(officeCommentData{
			ID: comment.id, ParentID: comment.parentID,
			Author:  officeCommentAuthor{Name: comment.author, Initials: comment.initials},
			Created: comment.created, Resolved: comment.resolved,
		})
		for _, target := range w.commentTargets[comment.id] {
			w.attachWordComment(target, groupRef)
		}
	}
}

// attachWordComment 把 comment_section 引用附加到正文文本、图片或表格。
func (w *docxWalker) attachWordComment(target, comment RefItem) {
	fine := FineRef{RefItem: comment}
	switch target.Kind {
	case refTexts:
		if target.Idx < 0 || target.Idx >= int64(len(w.doc.Texts)) {
			return
		}
		span := [2]int64{0, int64(utf8.RuneCountInString(w.doc.Texts[target.Idx].Text))}
		fine.Range = &span
		w.doc.Texts[target.Idx].Comments = append(w.doc.Texts[target.Idx].Comments, fine)
	case refPictures:
		if target.Idx >= 0 && target.Idx < int64(len(w.doc.Pictures)) {
			w.doc.Pictures[target.Idx].Comments = append(w.doc.Pictures[target.Idx].Comments, fine)
		}
	case refTables:
		if target.Idx >= 0 && target.Idx < int64(len(w.doc.Tables)) {
			w.doc.Tables[target.Idx].Comments = append(w.doc.Tables[target.Idx].Comments, fine)
		}
	}
}

// wordFurnitureRef 表示 document.xml 中按出现顺序排列的页眉或页脚引用。
type wordFurnitureRef struct {
	relID string
	label DocItemLabel
}

// parseWordFurnitureRefs 读取各节的页眉页脚引用，供关系表定位实际部件。
func parseWordFurnitureRefs(documentXML []byte) []wordFurnitureRef {
	dec := xml.NewDecoder(bytes.NewReader(documentXML))
	var refs []wordFurnitureRef
	for {
		token, err := dec.Token()
		if err != nil {
			return refs
		}
		start, ok := token.(xml.StartElement)
		if !ok {
			continue
		}
		label := DocItemLabel("")
		switch start.Name.Local {
		case "headerReference":
			label = LabelPageHeader
		case "footerReference":
			label = LabelPageFooter
		}
		if label == "" {
			continue
		}
		if id := xmlAttrNS(start.Attr, officeRelNS, "id"); id != "" {
			refs = append(refs, wordFurnitureRef{relID: id, label: label})
		}
	}
}

// addWordFurniture 按 section 引用顺序解析页眉页脚部件，并挂到 furniture 根。
// 同一部件被多个节复用时只输出一次，避免重复的检索噪声。
func (w *docxWalker) addWordFurniture(documentXML, documentRelsXML []byte) {
	headerTargets := parseOOXMLRelationships(documentRelsXML, "/header", "word")
	footerTargets := parseOOXMLRelationships(documentRelsXML, "/footer", "word")
	seen := map[string]bool{}
	for _, ref := range parseWordFurnitureRefs(documentXML) {
		target := headerTargets[ref.relID]
		if ref.label == LabelPageFooter {
			target = footerTargets[ref.relID]
		}
		key := string(ref.label) + ":" + target
		if target == "" || seen[key] {
			continue
		}
		seen[key] = true
		partXML, err := readZipFileBytes(w.reader, target)
		if err != nil || len(partXML) == 0 {
			continue
		}
		dir, filename := path.Split(target)
		partRelsXML, _ := readZipFileBytes(w.reader, path.Join(dir, "_rels", filename+".rels"))
		imageTargets := parseOOXMLRelationships(partRelsXML, "/image", path.Clean(dir))
		hyperlinkTargets := parseDocxRelationshipTargets(partRelsXML, "/hyperlink", path.Clean(dir))
		chartTargets := parseOOXMLRelationships(partRelsXML, "/chart", path.Clean(dir))
		w.addWordFurniturePart(partXML, ref.label, imageTargets, hyperlinkTargets, chartTargets)
	}
}

// addWordFurniturePart 抽取一个页眉/页脚部件内的段落、图片、图表和文本框。
func (w *docxWalker) addWordFurniturePart(
	partXML []byte,
	label DocItemLabel,
	imageTargets, hyperlinkTargets, chartTargets map[string]string,
) {
	dec := xml.NewDecoder(bytes.NewReader(partXML))
	furniture := w.doc.furnitureRef()
	for {
		token, err := dec.Token()
		if err != nil {
			return
		}
		start, ok := token.(xml.StartElement)
		if !ok || start.Name.Local != "p" {
			continue
		}
		raw, err := collectSubTree(dec, start)
		if err != nil {
			return
		}
		info, err := parseWordParagraph(raw)
		if err != nil {
			continue
		}
		text := strings.TrimSpace(sanitizeText(info.text))
		if len(info.imageRIDs) > 0 || len(info.chartRIDs) > 0 {
			for index, id := range info.imageRIDs {
				target := imageTargets[id]
				picRef := w.doc.AddPicture(w.mediaImageTarget(target), nil, &furniture)
				picture := &w.doc.Pictures[picRef.Idx]
				picture.ContentLayer = LayerFurniture
				if index == 0 && text != "" {
					picture.Caption = text
				}
			}
			for _, id := range info.chartRIDs {
				if chartRef, ok := addOOXMLChartPictureFromPart(w.doc, w.reader, chartTargets[id], nil, nil, &furniture); ok {
					w.doc.Pictures[chartRef.Idx].ContentLayer = LayerFurniture
				}
			}
		} else if text != "" {
			textRef := w.doc.AddText(label, text, nil, &furniture)
			item := &w.doc.Texts[textRef.Idx]
			item.ContentLayer = LayerFurniture
			if info.formatting != nil {
				formatting := *info.formatting
				item.Formatting = &formatting
			}
			for _, id := range info.hyperlinkIDs {
				if target := hyperlinkTargets[id]; target != "" {
					item.Hyperlink = target
					break
				}
			}
		}
		for _, textBox := range extractWordTextBoxes(raw) {
			w.addWordFurniturePart(textBox, label, imageTargets, hyperlinkTargets, chartTargets)
		}
	}
}

// currentLevel 返回第一个空槽位下标（对齐 _get_level：第一个 None 键）。
func (w *docxWalker) currentLevel() int64 {
	for k := int64(0); ; k++ {
		if _, ok := w.parents[k]; !ok {
			return k
		}
	}
}

// parentAt 返回指定槽位的元素引用；槽位为空或下标为负时返回 nil（挂 body）。
func (w *docxWalker) parentAt(level int64) *RefItem {
	if level < 0 {
		return nil
	}
	if ref, ok := w.parents[level]; ok {
		return &ref
	}
	return nil
}

// breakListAndCode 关闭列表序列并打断 code 合并链（任何非列表元素产出后调用）。
func (w *docxWalker) breakListAndCode() {
	w.listActive = false
	w.lastIsCode = false
}

// breakList 仅关闭列表序列（code 段落产出后调用：列表关闭但 code 合并链保留，
// 对齐 msword_backend 中 code 分支不影响既有代码块的行为）。
func (w *docxWalker) breakList() {
	w.listActive = false
}

// walkDocument 遍历 document.xml，进入 w:body 后按 body 顶层元素序列处理。
func (w *docxWalker) walkDocument(xmlBytes []byte) error {
	dec := xml.NewDecoder(bytes.NewReader(xmlBytes))
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if start, ok := tok.(xml.StartElement); ok && start.Name.Local == "body" {
			return w.walkElements(dec, "body")
		}
	}
}

// walkElements 遍历当前位置起的一系列同级元素，直到遇到 parentTag 的结束
// 元素或流结束。body 顶层、w:sdtContent、w:tc（1x1 表降级）共用该遍历口径：
// w:p → 段落处理、w:tbl → 表格处理、w:sdt → 递归其 sdtContent，其余元素
// （sectPr/bookmark/drawing 等）整体跳过。
func (w *docxWalker) walkElements(dec *xml.Decoder, parentTag string) error {
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		switch t := tok.(type) {
		case xml.EndElement:
			if t.Name.Local == parentTag {
				return nil
			}
		case xml.StartElement:
			switch t.Name.Local {
			case "p":
				raw, err := collectSubTree(dec, t)
				if err != nil {
					return err
				}
				if err := w.handleParagraph(raw); err != nil {
					return err
				}
			case "tbl":
				raw, err := collectSubTree(dec, t)
				if err != nil {
					return err
				}
				if err := w.handleTable(raw); err != nil {
					return err
				}
			case "sdt":
				raw, err := collectSubTree(dec, t)
				if err != nil {
					return err
				}
				if err := w.walkSDTContent(raw); err != nil {
					return err
				}
			default:
				if err := skipSubTree(dec, t); err != nil {
					return err
				}
			}
		}
	}
}

// walkSDTContent 在 w:sdt 子树中定位 w:sdtContent 并递归遍历其子元素
// （目录等内容控件内的段落/表格照常解析）。
func (w *docxWalker) walkSDTContent(raw []byte) error {
	dec := xml.NewDecoder(bytes.NewReader(raw))
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if start, ok := tok.(xml.StartElement); ok && start.Name.Local == "sdtContent" {
			return w.walkElements(dec, "sdtContent")
		}
	}
}

// walkTableCellAsBody 把 1x1 表的单元格内容当正文处理（_handle_tables 的
// 1x1 分支）：跳过 tcPr，按 body 元素序列遍历单元格内部。
func (w *docxWalker) walkTableCellAsBody(raw []byte) error {
	dec := xml.NewDecoder(bytes.NewReader(raw))
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if start, ok := tok.(xml.StartElement); ok && start.Name.Local == "tc" {
			return w.walkElements(dec, "tc")
		}
	}
}

// collectSubTree 捕获当前元素的完整子树并重序列化为等价 XML 字节，
// 供段落/表格的二级解析器重放（调用时 decoder 已消费该元素的开始标签）。
// encoding/xml 的 Encoder 会为命名空间 URL 自动生成前缀声明，
// 重放解码后元素仍按 URL 归属命名空间，语义无损。
func collectSubTree(dec *xml.Decoder, root xml.StartElement) ([]byte, error) {
	var buf bytes.Buffer
	enc := xml.NewEncoder(&buf)
	depth := 1
	if err := enc.EncodeToken(root); err != nil {
		return nil, err
	}
	for depth > 0 {
		tok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			depth++
			err = enc.EncodeToken(t)
		case xml.EndElement:
			depth--
			err = enc.EncodeToken(t)
		case xml.CharData:
			err = enc.EncodeToken(t)
		case xml.Comment:
			err = enc.EncodeToken(t)
		case xml.ProcInst:
			// xml 声明只允许出现在文件头，子树内遇到时跳过不编码
			if t.Target != "xml" {
				err = enc.EncodeToken(t)
			}
		case xml.Directive:
			err = enc.EncodeToken(t)
		}
		if err != nil {
			return nil, err
		}
	}
	if err := enc.Flush(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// skipSubTree 消费当前元素的完整子树（调用时 decoder 已消费开始标签）。
func skipSubTree(dec *xml.Decoder, root xml.StartElement) error {
	depth := 1
	for depth > 0 {
		tok, err := dec.Token()
		if err != nil {
			return err
		}
		switch tok.(type) {
		case xml.StartElement:
			depth++
		case xml.EndElement:
			depth--
		}
	}
	return nil
}

// xmlAttrVal 按属性 local 名取属性值，不存在时返回空串。
func xmlAttrVal(attrs []xml.Attr, local string) string {
	for _, a := range attrs {
		if a.Name.Local == local {
			return a.Value
		}
	}
	return ""
}

// xmlAttrInt 按属性 local 名取整数值，缺失或非法时返回默认值 def。
func xmlAttrInt(attrs []xml.Attr, local string, def int64) int64 {
	v := xmlAttrVal(attrs, local)
	if v == "" {
		return def
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return def
	}
	return n
}

// wordRevision 保存 Word 修订的审计信息；Kind 使用 insert/delete，Text
// 是修订范围内可见字符，正文 Text 仍表达“接受全部修订”后的当前视图。
type wordRevision struct {
	Kind   string `json:"kind"`             // Kind 是插入或删除。
	ID     string `json:"id,omitempty"`     // ID 是 Word 修订编号。
	Author string `json:"author,omitempty"` // Author 是修订作者。
	Date   string `json:"date,omitempty"`   // Date 是 OOXML 原始时间。
	Text   string `json:"text"`             // Text 是修订范围文本。
}

// wordParagraphInfo 段落子树的一次性解析产物：样式、编号、复选框、文本、
// 媒体/链接/批注关系、修订/域以及 run 格式信息。
type wordParagraphInfo struct {
	styleID       string               // w:pStyle 样式 id（缺失为空）
	hasNumPr      bool                 // 是否携带 w:numPr 列表编号属性
	numID         int64                // w:numId 值（缺失/非法为 -1；0 在分派时视为无列表）
	ilvl          int64                // w:ilvl 缩进层级（缺失默认 0）
	hasCheckbox   bool                 // 段落内是否含 w14:checkbox
	checked       bool                 // 复选框是否勾选（w14:checked val="1"）
	text          string               // 段落全文（含行内公式文本）
	normText      string               // 仅普通文本（不含公式）
	mathText      string               // 仅公式 LaTeX（m:oMath 内；失败时为可见文本）
	imageRIDs     []string             // 段内图片引用（a:blip 的 r:embed / v:imagedata 的 r:id，按出现顺序）
	chartRIDs     []string             // 段内图表引用（c:chart 的 r:id，按锚点出现顺序）
	officeObjects []officeObjectRecord // 段内 SmartArt、艺术字、形状与嵌入对象
	hyperlinkIDs  []string             // 段内超链接关系 id，按首次出现顺序
	commentIDs    []string             // 段内批注范围/引用 id，去重后按出现顺序
	footnoteIDs   []string             // 段内脚注引用 id，去重后按出现顺序
	endnoteIDs    []string             // 段内尾注引用 id，去重后按出现顺序
	revisions     []wordRevision       // 段内插入、删除与移动修订
	fields        []string             // 段内 fldSimple 或复杂域指令
	formatting    *Formatting          // 段内任一可见 run 的统一格式汇总
	// —— 无样式标题 fallback 判定所需（仅含可见文本的 run 参与）——
	boldChars    int   // 加粗 run 的文本 rune 数（w:b 非 val="0"/"false"；w:bCs 不计）
	nonBoldChars int   // 非加粗 run 的文本 rune 数
	maxFontSize  int64 // 段内含文本 run 的最大 w:sz 半磅值（未指定为 0）
}

// parseWordParagraph 解析段落子树：pStyle/numPr、checkbox、文本流、图片/图表、
// 超链接、批注及 run 格式；txbxContent 留给独立文本框 walker，避免重复文本。
func parseWordParagraph(raw []byte) (wordParagraphInfo, error) {
	var info wordParagraphInfo
	info.numID = -1
	dec := xml.NewDecoder(bytes.NewReader(raw))
	var (
		inPPr            bool           // 处于 w:pPr 内（制表位定义等属性元素不产文本）
		inNumPr          bool           // 处于 w:numPr 内
		inCheckbox       bool           // 处于 w14:checkbox 内
		inText           bool           // 处于 w:t 内
		inInstruction    bool           // 处于 w:instrText 域指令内
		inRun            bool           // 处于 w:r 内（run 级加粗/字号属性仅在 run 内生效）
		runBold          bool           // 当前 run 是否加粗（w:b 非 val="0"/"false"）
		runItalic        bool           // 当前 run 是否斜体
		runUnderline     bool           // 当前 run 是否有下划线
		runStrike        bool           // 当前 run 是否有删除线
		runScript        ScriptPosition // 当前 run 上下标位置
		runSz            int64          // 当前 run 字号（w:sz 半磅值）
		runChars         int            // 当前 run 累计文本 rune 数
		textBoxDepth     int            // txbxContent 内容由独立 walker 处理，主段落需跳过
		text, norm, math strings.Builder
		fieldInstruction strings.Builder
	)
	formatting := Formatting{Script: ScriptBaseline}
	hasFormatting := false
	seenComments := map[string]bool{}
	seenFootnotes := map[string]bool{}
	seenEndnotes := map[string]bool{}
	seenFields := map[string]bool{}
	appendField := func(value string) {
		value = strings.Join(strings.Fields(value), " ")
		if value == "" || seenFields[value] {
			return
		}
		seenFields[value] = true
		info.fields = append(info.fields, value)
	}
	flushField := func() {
		appendField(fieldInstruction.String())
		fieldInstruction.Reset()
	}
	// writeSep 写入普通文本的 tab/br 分隔符；公式子树独立整体转换。
	writeSep := func(sep byte) {
		text.WriteByte(sep)
		norm.WriteByte(sep)
	}
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return info, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == "txbxContent" {
				textBoxDepth++
				continue
			}
			if textBoxDepth > 0 {
				continue
			}
			if t.Name.Local == "oMath" && t.Name.Space == wordMathNS {
				rawMath, collectErr := collectSubTree(dec, t)
				if collectErr != nil {
					return info, collectErr
				}
				latex := convertOMMLToLaTeX(rawMath)
				if latex == "" {
					latex = visibleOMMLText(rawMath)
				}
				text.WriteString(latex)
				math.WriteString(latex)
				continue
			}
			switch t.Name.Local {
			case "pPr":
				inPPr = true
			case "fldSimple":
				appendField(xmlAttrVal(t.Attr, "instr"))
			case "fldChar":
				switch strings.ToLower(xmlAttrVal(t.Attr, "fldCharType")) {
				case "begin":
					flushField()
				case "separate", "end":
					flushField()
				}
			case "instrText":
				inInstruction = true
			case "numPr":
				if inPPr {
					inNumPr = true
					info.hasNumPr = true
				}
			case "numId":
				if inNumPr {
					info.numID = xmlAttrInt(t.Attr, "val", -1)
				}
			case "ilvl":
				if inNumPr {
					info.ilvl = xmlAttrInt(t.Attr, "val", 0)
				}
			case "pStyle":
				if inPPr {
					info.styleID = xmlAttrVal(t.Attr, "val")
				}
			case "r":
				// w:r run 开始：重置 run 级加粗/字号跟踪（m:r 数学 run 不算）
				if t.Name.Space == "" || t.Name.Space == wordNS {
					inRun = true
					runBold = false
					runItalic = false
					runUnderline = false
					runStrike = false
					runScript = ScriptBaseline
					runSz = 0
					runChars = 0
				}
			case "b":
				// run 级加粗：w:b 缺省或 val 非 "0"/"false" 均视为加粗，
				// 显式 val="0"/"false" 为不加粗；w:bCs 是另一 local 名不算
				if inRun && (t.Name.Space == "" || t.Name.Space == wordNS) {
					switch xmlAttrVal(t.Attr, "val") {
					case "0", "false":
					default:
						runBold = true
					}
				}
			case "i":
				if inRun && (t.Name.Space == "" || t.Name.Space == wordNS) && wordOnOffEnabled(t.Attr) {
					runItalic = true
				}
			case "u":
				if inRun && (t.Name.Space == "" || t.Name.Space == wordNS) {
					value := strings.ToLower(xmlAttrVal(t.Attr, "val"))
					runUnderline = value != "none" && value != "0" && value != "false"
				}
			case "strike", "dstrike":
				if inRun && (t.Name.Space == "" || t.Name.Space == wordNS) && wordOnOffEnabled(t.Attr) {
					runStrike = true
				}
			case "vertAlign":
				if inRun && (t.Name.Space == "" || t.Name.Space == wordNS) {
					switch strings.ToLower(xmlAttrVal(t.Attr, "val")) {
					case "superscript":
						runScript = ScriptSuper
					case "subscript":
						runScript = ScriptSub
					}
				}
			case "sz":
				// run 字号（w:sz 半磅值；w:szCs 复杂文种字号是另一 local 名不算）
				if inRun && (t.Name.Space == "" || t.Name.Space == wordNS) {
					runSz = xmlAttrInt(t.Attr, "val", 0)
				}
			case "checkbox":
				if t.Name.Space == wordW14NS {
					inCheckbox = true
					info.hasCheckbox = true
				}
			case "checked":
				if inCheckbox {
					info.checked = xmlAttrVal(t.Attr, "val") == "1"
				}
			case "t":
				if t.Name.Space == "" || t.Name.Space == wordNS {
					inText = true
				}
			case "tab":
				if !inPPr {
					writeSep('\t')
				}
			case "br":
				if !inPPr {
					writeSep('\n')
				}
			case "blip":
				// a:blip 图片引用（r:embed 指向 word/media 关系），收集供图片提取
				if rid := xmlAttrNS(t.Attr, officeRelNS, "embed"); rid != "" {
					info.imageRIDs = append(info.imageRIDs, rid)
				}
			case "imagedata":
				// v:imagedata 旧式图片引用（r:id 指向 word/media 关系）
				if rid := xmlAttrNS(t.Attr, officeRelNS, "id"); rid != "" {
					info.imageRIDs = append(info.imageRIDs, rid)
				}
			case "chart":
				// c:chart 图表关系交由共享 OOXML 图表解析器读取 chart*.xml。
				if rid := xmlAttrNS(t.Attr, officeRelNS, "id"); rid != "" {
					info.chartRIDs = append(info.chartRIDs, rid)
				}
			case "hyperlink":
				if rid := xmlAttrNS(t.Attr, officeRelNS, "id"); rid != "" {
					info.hyperlinkIDs = append(info.hyperlinkIDs, rid)
				}
			case "commentRangeStart", "commentRangeEnd", "commentReference":
				if id := xmlAttrVal(t.Attr, "id"); id != "" && !seenComments[id] {
					seenComments[id] = true
					info.commentIDs = append(info.commentIDs, id)
				}
			case "footnoteReference":
				if id := xmlAttrVal(t.Attr, "id"); id != "" && !seenFootnotes[id] {
					seenFootnotes[id] = true
					info.footnoteIDs = append(info.footnoteIDs, id)
				}
			case "endnoteReference":
				if id := xmlAttrVal(t.Attr, "id"); id != "" && !seenEndnotes[id] {
					seenEndnotes[id] = true
					info.endnoteIDs = append(info.endnoteIDs, id)
				}
			}
		case xml.EndElement:
			if t.Name.Local == "txbxContent" && textBoxDepth > 0 {
				textBoxDepth--
				continue
			}
			if textBoxDepth > 0 {
				continue
			}
			switch t.Name.Local {
			case "pPr":
				inPPr = false
			case "numPr":
				inNumPr = false
			case "checkbox":
				inCheckbox = false
			case "r":
				// run 结束：仅含可见文本的 run 参与加粗投票与字号统计
				if inRun && (t.Name.Space == "" || t.Name.Space == wordNS) {
					inRun = false
					if runChars > 0 {
						if runBold {
							info.boldChars += runChars
						} else {
							info.nonBoldChars += runChars
						}
						if runSz > info.maxFontSize {
							info.maxFontSize = runSz
						}
						if runBold || runItalic || runUnderline || runStrike || runScript != ScriptBaseline {
							hasFormatting = true
							formatting.Bold = formatting.Bold || runBold
							formatting.Italic = formatting.Italic || runItalic
							formatting.Underline = formatting.Underline || runUnderline
							formatting.Strikethrough = formatting.Strikethrough || runStrike
							if runScript != ScriptBaseline {
								formatting.Script = runScript
							}
						}
					}
				}
			case "t":
				inText = false
			case "instrText":
				inInstruction = false
			}
		case xml.CharData:
			if textBoxDepth > 0 {
				continue
			}
			if inText {
				text.WriteString(string(t))
				norm.WriteString(string(t))
				if inRun {
					runChars += utf8.RuneCount(t)
				}
			}
			if inInstruction {
				fieldInstruction.Write(t)
			}
		}
	}
	flushField()
	info.text = text.String()
	info.normText = norm.String()
	info.mathText = math.String()
	info.officeObjects = extractWordOfficeObjects(raw)
	info.revisions = parseWordRevisions(raw)
	if hasFormatting {
		info.formatting = &formatting
	}
	return info, nil
}

// parseWordRevisions 提取插入、删除和移动修订。主文本解析天然保留 w:ins
// 中的 w:t、忽略 w:delText；本函数额外保存作者、时间和被删除内容。
func parseWordRevisions(raw []byte) []wordRevision {
	decoder := xml.NewDecoder(bytes.NewReader(raw))
	var revisions []wordRevision
	var current *wordRevision
	inText := false
	for {
		token, err := decoder.Token()
		if err != nil {
			return revisions
		}
		switch value := token.(type) {
		case xml.StartElement:
			switch value.Name.Local {
			case "ins", "moveTo":
				revision := wordRevision{Kind: "insert", ID: xmlAttrVal(value.Attr, "id"), Author: xmlAttrVal(value.Attr, "author"), Date: xmlAttrVal(value.Attr, "date")}
				current = &revision
			case "del", "moveFrom":
				revision := wordRevision{Kind: "delete", ID: xmlAttrVal(value.Attr, "id"), Author: xmlAttrVal(value.Attr, "author"), Date: xmlAttrVal(value.Attr, "date")}
				current = &revision
			case "t":
				inText = current != nil && current.Kind == "insert"
			case "delText":
				inText = current != nil && current.Kind == "delete"
			case "tab":
				if current != nil {
					current.Text += "\t"
				}
			case "br":
				if current != nil {
					current.Text += "\n"
				}
			}
		case xml.CharData:
			if current != nil && inText {
				current.Text += string(value)
			}
		case xml.EndElement:
			switch value.Name.Local {
			case "t", "delText":
				inText = false
			case "ins", "del", "moveTo", "moveFrom":
				if current != nil {
					current.Text = strings.TrimSpace(sanitizeText(current.Text))
					if current.Text != "" {
						revisions = append(revisions, *current)
					}
					current = nil
				}
			}
		}
	}
}

// wordDeletedRevisionText 拼接段落中当前接受视图不可见的删除内容。
func wordDeletedRevisionText(revisions []wordRevision) string {
	var parts []string
	for _, revision := range revisions {
		if revision.Kind == "delete" && strings.TrimSpace(revision.Text) != "" {
			parts = append(parts, strings.TrimSpace(revision.Text))
		}
	}
	return strings.Join(parts, "\n")
}

// wordOnOffEnabled 解析 OOXML on/off 属性；缺省值表示开启，0/false/off
// 表示关闭。
func wordOnOffEnabled(attrs []xml.Attr) bool {
	switch strings.ToLower(xmlAttrVal(attrs, "val")) {
	case "0", "false", "off", "none":
		return false
	default:
		return true
	}
}

// handleParagraph 处理 body/cell 内的一个段落，按复刻自 msword_backend
// _handle_text_elements 的分派顺序产出元素：列表 → title → heading →
// 独立公式 → code → checkbox/text；空段落不产出也不打断列表/代码链。
// 参数 raw 为段落子树的 XML 字节；返回解析错误。
func (w *docxWalker) handleParagraph(raw []byte) error {
	info, err := parseWordParagraph(raw)
	if err != nil {
		return err
	}
	if err := w.handleParagraphInfo(info); err != nil {
		return err
	}
	for _, textBox := range extractWordTextBoxes(raw) {
		if err := w.walkTextBoxContent(textBox); err != nil {
			return err
		}
	}
	return nil
}

// handleParagraphInfo 把已解析段落分派为 Docling 元素。文本框内容已从 info
// 排除，由 handleParagraph 在锚点段落之后按出现顺序递归处理。
func (w *docxWalker) handleParagraphInfo(info wordParagraphInfo) error {
	kind, level := w.classifyParagraphStyle(info.styleID)

	fullText := strings.TrimSpace(sanitizeText(info.text))
	normText := strings.TrimSpace(sanitizeText(info.normText))
	mathText := strings.TrimSpace(sanitizeText(info.mathText))

	// 图片段落：产出 picture 元素（URI 内嵌 data URI），段落文本作 caption
	// （P0 增强，见文件头注释；图片段落不参与列表/code 等其余分派）
	if len(info.imageRIDs) > 0 || len(info.chartRIDs) > 0 || len(info.officeObjects) > 0 {
		parent := w.parentAt(w.currentLevel() - 1)
		imageRIDs := info.imageRIDs
		refs := make([]RefItem, 0, len(imageRIDs)+len(info.chartRIDs)+len(info.officeObjects))
		// OLE 常把预览图与对象放在同一锚点；首张图片作为对象预览，避免
		// content_list 同时出现一张无语义图片和一个无预览对象。
		if len(info.officeObjects) > 0 && len(imageRIDs) > 0 {
			info.officeObjects[0].preview = w.mediaImage(imageRIDs[0])
			imageRIDs = imageRIDs[1:]
		}
		if len(info.officeObjects) > 0 && fullText != "" {
			objectText := strings.TrimSpace(info.officeObjects[0].text)
			if objectText == "" {
				info.officeObjects[0].text = fullText
			} else if !strings.Contains(objectText, fullText) {
				info.officeObjects[0].text = fullText + "\n" + objectText
			}
		}
		refs = append(refs, w.addWordPictures(imageRIDs, fullText, parent)...)
		refs = append(refs, w.addWordCharts(info.chartRIDs, w.parentAt(w.currentLevel()-1))...)
		refs = append(refs, w.addWordOfficeObjects(info.officeObjects, parent)...)
		for _, ref := range refs {
			w.attachParagraphMetadata(ref, info)
		}
		w.breakListAndCode()
		return nil
	}

	// 列表：携带有效 numId（0 视为无列表）且样式非 Title/Heading/Code
	if info.hasNumPr && info.numID > 0 &&
		kind != wordStyleTitle && kind != wordStyleHeading && kind != wordStyleCode {
		ordered := w.numbering.hasVisibleNumberingFormat(info.numID, info.ilvl)
		ref := w.addWordListItem(info.numID, info.ilvl, ordered, fullText)
		w.attachParagraphMetadata(ref, info)
		return nil
	}
	if kind == wordStyleTitle && fullText != "" {
		ref := w.addWordTitle(fullText)
		w.attachParagraphMetadata(ref, info)
		return nil
	}
	if kind == wordStyleHeading && fullText != "" {
		ref := w.addWordHeading(level, fullText)
		w.attachParagraphMetadata(ref, info)
		return nil
	}
	if normText == "" && mathText != "" {
		// 独立公式段落：整段仅由 m:oMath 构成，text 保存转换后的 LaTeX。
		ref := w.doc.AddFormula(mathText, nil, w.parentAt(w.currentLevel()-1))
		w.attachParagraphMetadata(ref, info)
		w.breakListAndCode()
		return nil
	}
	if kind == wordStyleCode && !info.hasCheckbox {
		// Code 样式段落（复选框段落让位给 checkbox 分支）：
		// 保留前导缩进，连续段合并为同一 code 元素
		codeText := strings.TrimRightFunc(sanitizeText(info.text), unicode.IsSpace)
		if w.lastIsCode && !w.forceNewCode {
			if strings.TrimSpace(codeText) == "" {
				// 空 Code 段缓冲：仅当后续还有代码时才落成换行
				w.pendingCodeBlank++
			} else {
				joiner := strings.Repeat("\n", w.pendingCodeBlank+1)
				item := &w.doc.Texts[w.lastCode.Idx]
				item.Text += joiner + codeText
				item.Orig += joiner + codeText
				w.attachParagraphMetadata(w.lastCode, info)
				w.pendingCodeBlank = 0
			}
		} else if strings.TrimSpace(codeText) != "" {
			ref := w.doc.AddCode(codeText, "", nil, w.parentAt(w.currentLevel()-1))
			w.attachParagraphMetadata(ref, info)
			w.lastCode = ref
			w.lastIsCode = true
			w.pendingCodeBlank = 0
			w.forceNewCode = false
		}
		// code 段落关闭列表但不打断自身合并链
		w.breakList()
		return nil
	}
	if fullText == "" {
		// “接受全部修订”视图中的纯删除段落没有正文；把删除内容放入 notes
		// 层并保留审计 meta，既不污染默认正文导出，也不让历史内容丢失。
		if deleted := wordDeletedRevisionText(info.revisions); deleted != "" {
			ref := w.doc.AddText(LabelText, deleted, nil, w.parentAt(w.currentLevel()-1))
			w.doc.Texts[ref.Idx].ContentLayer = LayerNotes
			w.attachParagraphMetadata(ref, info)
			w.breakListAndCode()
		}
		return nil
	}
	parent := w.parentAt(w.currentLevel() - 1)
	if info.hasCheckbox {
		ref := w.doc.AddCheckbox(info.checked, fullText, nil, parent)
		w.attachParagraphMetadata(ref, info)
	} else {
		ref := w.doc.AddText(LabelText, fullText, nil, parent)
		w.attachParagraphMetadata(ref, info)
		// 记录段落中间表示，供无样式标题 fallback 重判（仅普通 text 元素参与）
		w.paraMetas[ref.Idx] = docxParaMeta{
			text:  fullText,
			bold:  info.boldChars > 0 && info.boldChars > info.nonBoldChars,
			maxSz: info.maxFontSize,
		}
	}
	w.breakListAndCode()
	return nil
}

// attachParagraphMetadata 把段落级格式、首个可解析超链接与批注 id 关联到
// 已产出的文本/图片元素。批注实体在正文完成后统一创建。
func (w *docxWalker) attachParagraphMetadata(ref RefItem, info wordParagraphInfo) {
	if ref.Kind == "" {
		return
	}
	if ref.Kind == refTexts && ref.Idx >= 0 && ref.Idx < int64(len(w.doc.Texts)) {
		if info.formatting != nil {
			formatting := *info.formatting
			w.doc.Texts[ref.Idx].Formatting = &formatting
		}
		for _, id := range info.hyperlinkIDs {
			if target := w.hyperlinks[id]; target != "" {
				w.doc.Texts[ref.Idx].Hyperlink = target
				break
			}
		}
	}
	w.attachWordRevisionAndFieldMeta(ref, info.revisions, info.fields)
	for _, id := range info.commentIDs {
		w.commentTargets[id] = append(w.commentTargets[id], ref)
	}
	for _, id := range info.footnoteIDs {
		w.footnoteTargets[id] = append(w.footnoteTargets[id], ref)
	}
	for _, id := range info.endnoteIDs {
		w.endnoteTargets[id] = append(w.endnoteTargets[id], ref)
	}
}

// wordRevisionAndFieldMeta 把修订与域信息编码为可挂载到 DocItem
// 的扩展元数据；编码失败的单个字段不影响其他内容。
func wordRevisionAndFieldMeta(revisions []wordRevision, fields []string) BaseMeta {
	metadata := BaseMeta{}
	for key, value := range map[string]any{
		wordRevisionsMetaKey: revisions,
		wordFieldsMetaKey:    fields,
	} {
		switch typed := value.(type) {
		case []wordRevision:
			if len(typed) == 0 {
				continue
			}
		case []string:
			if len(typed) == 0 {
				continue
			}
		}
		if raw, err := json.Marshal(value); err == nil {
			metadata[key] = raw
		}
	}
	return metadata
}

// attachWordRevisionAndFieldMeta 把修订与域信息写入不同 DocItem 的扩展 meta。
func (w *docxWalker) attachWordRevisionAndFieldMeta(ref RefItem, revisions []wordRevision, fields []string) {
	metadata := wordRevisionAndFieldMeta(revisions, fields)
	if len(metadata) == 0 {
		return
	}
	switch ref.Kind {
	case refTexts:
		if ref.Idx >= 0 && ref.Idx < int64(len(w.doc.Texts)) {
			if w.doc.Texts[ref.Idx].Meta == nil {
				w.doc.Texts[ref.Idx].Meta = BaseMeta{}
			}
			for key, value := range metadata {
				w.doc.Texts[ref.Idx].Meta[key] = value
			}
		}
	case refTables:
		if ref.Idx >= 0 && ref.Idx < int64(len(w.doc.Tables)) {
			if w.doc.Tables[ref.Idx].Meta == nil {
				w.doc.Tables[ref.Idx].Meta = BaseMeta{}
			}
			for key, value := range metadata {
				w.doc.Tables[ref.Idx].Meta[key] = value
			}
		}
	case refPictures:
		if ref.Idx >= 0 && ref.Idx < int64(len(w.doc.Pictures)) {
			if w.doc.Pictures[ref.Idx].Meta.Extra == nil {
				w.doc.Pictures[ref.Idx].Meta.Extra = map[string]json.RawMessage{}
			}
			for key, value := range metadata {
				w.doc.Pictures[ref.Idx].Meta.Extra[key] = value
			}
		}
	}
}

// extractWordOfficeObjects 从段落锚点中提取 SmartArt、艺术字、DrawingML
// 形状与 OLE 嵌入对象。普通图片和图表由既有关系分支处理，不重复建项。
func extractWordOfficeObjects(raw []byte) []officeObjectRecord {
	decoder := xml.NewDecoder(bytes.NewReader(raw))
	var records []officeObjectRecord
	var shape *officeObjectRecord
	var inShape, inShapeText, shapeHasTextBox bool
	for {
		token, err := decoder.Token()
		if err != nil {
			break
		}
		switch value := token.(type) {
		case xml.StartElement:
			switch value.Name.Local {
			case "relIds":
				if id := xmlAttrNS(value.Attr, officeRelNS, "dm"); id != "" {
					records = append(records, officeObjectRecord{kind: "smartart", relationship: id})
				}
			case "OLEObject", "oleObject":
				id := xmlAttrNS(value.Attr, officeRelNS, "id")
				if id == "" {
					id = xmlAttrVal(value.Attr, "id")
				}
				records = append(records, officeObjectRecord{
					kind: "ole", relationship: id, program: xmlAttrVal(value.Attr, "ProgID"),
					name: xmlAttrVal(value.Attr, "ObjectID"),
				})
			case "textpath":
				text := xmlAttrVal(value.Attr, "string")
				if text != "" {
					records = append(records, officeObjectRecord{kind: "wordart", text: text})
				}
			case "wsp":
				inShape = true
				shapeHasTextBox = false
				shape = &officeObjectRecord{kind: "shape"}
			case "txbxContent":
				if inShape {
					shapeHasTextBox = true
				}
			case "prstGeom":
				if inShape && shape != nil {
					shape.geometry = xmlAttrVal(value.Attr, "prst")
				}
			case "prstTxWarp":
				if inShape && shape != nil {
					shape.kind = "wordart"
					shape.geometry = xmlAttrVal(value.Attr, "prst")
				}
			case "t":
				if inShape && value.Name.Space == "http://schemas.openxmlformats.org/drawingml/2006/main" {
					inShapeText = true
				}
			}
		case xml.EndElement:
			switch value.Name.Local {
			case "wsp":
				if shape != nil && !shapeHasTextBox {
					shape.text = strings.TrimSpace(shape.text)
					records = append(records, *shape)
				}
				shape = nil
				inShape = false
			case "t":
				inShapeText = false
			}
		case xml.CharData:
			if inShapeText && shape != nil {
				shape.text += string(value)
			}
		}
	}
	// 同一兼容性回退块可能同时出现 DrawingML 与 VML 描述，按稳定语义去重。
	seen := map[string]bool{}
	unique := make([]officeObjectRecord, 0, len(records))
	for _, record := range records {
		key := officeObjectClassName(record.kind) + "\x00" + record.relationship + "\x00" + record.text
		if seen[key] {
			continue
		}
		seen[key] = true
		unique = append(unique, record)
	}
	return unique
}

// addWordOfficeObjects 解析对象关系部件并按段落位置追加 PictureItem。
func (w *docxWalker) addWordOfficeObjects(records []officeObjectRecord, parent *RefItem) []RefItem {
	refs := make([]RefItem, 0, len(records))
	for _, record := range records {
		switch officeObjectClassName(record.kind) {
		case "smartart":
			record.target = w.diagramRels[record.relationship]
			if data, err := readZipFileBytes(w.reader, record.target); err == nil && len(data) > 0 {
				record.text = collectDrawingMLText(data)
			}
		case "embedded_object":
			record.target = w.oleRels[record.relationship]
		}
		refs = append(refs, addOfficeObjectPicture(w.doc, record, nil, parent, LayerBody))
	}
	return refs
}

// extractWordTextBoxes 返回段落内所有 w:txbxContent 子树，保持锚点出现顺序。
func extractWordTextBoxes(raw []byte) [][]byte {
	dec := xml.NewDecoder(bytes.NewReader(raw))
	var boxes [][]byte
	for {
		token, err := dec.Token()
		if err != nil {
			return boxes
		}
		start, ok := token.(xml.StartElement)
		if !ok || start.Name.Local != "txbxContent" {
			continue
		}
		box, err := collectSubTree(dec, start)
		if err != nil {
			return boxes
		}
		boxes = append(boxes, box)
	}
}

// walkTextBoxContent 按正文块规则解析文本框中的段落、表格与内容控件。
func (w *docxWalker) walkTextBoxContent(raw []byte) error {
	dec := xml.NewDecoder(bytes.NewReader(raw))
	if err := consumeRootElement(dec); err != nil {
		return err
	}
	return w.walkElements(dec, "txbxContent")
}

// mediaImage 返回关系 id 对应图片部件的完整 ImageRef（惰性读取并缓存）。
// 关系缺失或读取失败时仍返回空图片引用；超限媒体只留 MIME/DPI/尺寸信息。
func (w *docxWalker) mediaImage(relID string) *ImageRef {
	target, ok := w.imageRels[relID]
	if !ok {
		return &ImageRef{Size: &ImageSize{}}
	}
	return w.mediaImageTarget(target)
}

// mediaImageTarget 读取指定 zip 图片部件并补齐 MIME、DPI、尺寸与可选 data URI。
func (w *docxWalker) mediaImageTarget(target string) *ImageRef {
	if target == "" {
		return &ImageRef{Size: &ImageSize{}}
	}
	if cached, exists := w.mediaCache[target]; exists {
		copyImage := *cached
		if cached.Size != nil {
			size := *cached.Size
			copyImage.Size = &size
		}
		return &copyImage
	}
	imageRef := &ImageRef{Mimetype: ooxmlMediaMime(filepath.Ext(target)), Dpi: 72, Size: &ImageSize{}}
	if raw, err := readZipFileBytes(w.reader, target); err == nil {
		imageRef.URI = mediaToDataURI(filepath.Ext(target), raw)
		if config, _, decodeErr := image.DecodeConfig(bytes.NewReader(raw)); decodeErr == nil {
			imageRef.Size = &ImageSize{Width: float64(config.Width), Height: float64(config.Height)}
		}
	}
	w.mediaCache[target] = imageRef
	return w.mediaImageTarget(target)
}

// addWordPictures 把段落内的图片引用逐个产出为 picture 元素：URI 取
// media 关系的 data URI（超限/缺失时留空），首个元素附带段落文本作
// caption，挂 parent 指定的节点（nil 时挂 body）。
// 参数 relIDs 为段内图片引用 id 序列，caption 为段落文本。
func (w *docxWalker) addWordPictures(relIDs []string, caption string, parent *RefItem) []RefItem {
	refs := make([]RefItem, 0, len(relIDs))
	for i, relID := range relIDs {
		ref := w.doc.AddPicture(w.mediaImage(relID), nil, parent)
		if i == 0 && caption != "" {
			w.doc.Pictures[ref.Idx].Caption = caption
		}
		refs = append(refs, ref)
	}
	return refs
}

// addWordCharts 根据段落锚点中的关系 id 读取 chart*.xml，并委托共享 OOXML
// 图表解析器生成官方 PictureItem。缺失或不支持的图表部件被安全跳过。
func (w *docxWalker) addWordCharts(relIDs []string, parent *RefItem) []RefItem {
	refs := make([]RefItem, 0, len(relIDs))
	for _, relID := range relIDs {
		target := w.chartRels[relID]
		if target == "" {
			continue
		}
		if ref, ok := addOOXMLChartPictureFromPart(w.doc, w.reader, target, nil, nil, parent); ok {
			refs = append(refs, ref)
		}
	}
	return refs
}

// addWordTitle 处理 Title 样式段落（:2127）：清空层级树，title 挂 body 顶层，
// 后续内容的父槽位（parents[0]）指向该 title。
func (w *docxWalker) addWordTitle(text string) RefItem {
	w.parents = map[int64]RefItem{}
	ref := w.doc.AddTitle(text, nil, nil)
	w.parents[0] = ref
	w.breakListAndCode()
	return ref
}

// addWordHeading 处理标题段落（复刻 _add_heading）：
//   - 层级变深时逐级补隐式 section 分组（name="header-i"，挂 parents[i-1]）；
//   - 层级变浅时截断尾部槽位；
//   - 标题挂 parents[level-1]，并占住 parents[level] 槽位。
//
// 参数 level 为标题层级（1 起），text 为标题文本。
func (w *docxWalker) addWordHeading(level int64, text string) RefItem {
	cur := w.currentLevel()
	if level > cur {
		for i := cur; i < level; i++ {
			gr := w.doc.AddSectionGroup(fmt.Sprintf("header-%d", i), w.parentAt(i-1))
			w.parents[i] = gr
		}
	} else if level < cur {
		for k := range w.parents {
			if k >= level {
				delete(w.parents, k)
			}
		}
	}
	if level < 1 {
		level = 1
	}
	ref := w.doc.AddHeading(level, text, nil, w.parentAt(level-1))
	w.parents[level] = ref
	w.breakListAndCode()
	return ref
}

// addWordListItem 追加列表项（_manage_list_structure 简化版）：
//   - numId 切换或列表刚被关闭时重置分组栈，新建序列；
//   - ilvl 加深时逐级新建子分组，挂上一级最近的列表项（嵌套组织）；
//   - ilvl 变浅时截断分组栈；
//   - 有序 marker 组内自增（"1."/"2."），无序 marker 为空串。
//
// 参数 numID 为编号 id、ilvl 为缩进层级、ordered 为是否有序、text 为项文本。
func (w *docxWalker) addWordListItem(numID, ilvl int64, ordered bool, text string) RefItem {
	if text == "" {
		return RefItem{}
	}
	if !w.listActive || w.listNumID != numID {
		w.listActive = true
		w.listNumID = numID
		w.listGroups = nil
		w.listItems = nil
		w.listCounters = nil
	}
	// ilvl 加深：逐级补子分组，子分组挂上一级最近的列表项下
	for int64(len(w.listGroups)) <= ilvl {
		lv := int64(len(w.listGroups))
		var parent *RefItem
		if lv == 0 {
			parent = w.parentAt(w.currentLevel() - 1)
		} else {
			p := w.listItems[lv-1]
			parent = &p
		}
		w.listGroups = append(w.listGroups, w.doc.AddListGroup("list", parent))
		w.listItems = append(w.listItems, RefItem{})
		w.listCounters = append(w.listCounters, 0)
	}
	// ilvl 变浅：截断更深层级（计数随栈一并丢弃，符合重新编号语义）
	if int64(len(w.listGroups)) > ilvl+1 {
		w.listGroups = w.listGroups[:ilvl+1]
		w.listItems = w.listItems[:ilvl+1]
		w.listCounters = w.listCounters[:ilvl+1]
	}
	marker := ""
	if ordered {
		w.listCounters[ilvl]++
		marker = fmt.Sprintf("%d.", w.listCounters[ilvl])
	}
	w.listItems[ilvl] = w.doc.AddListItem(w.listGroups[ilvl], text, ordered, marker, nil)
	w.lastIsCode = false
	return w.listItems[ilvl]
}

// docxParaMeta 无样式标题 fallback 的段落中间表示：保留最终文本与 run 级
// 加粗/字号信息，仅在 fallback 标题重判中使用。
type docxParaMeta struct {
	text  string // 段落最终文本（sanitize+trim，与产出元素 Text 一致）
	bold  bool   // 段落主要 run 加粗（加粗 run 文本多于非加粗 run 文本）
	maxSz int64  // 段内含文本 run 的最大字号（w:sz 半磅值，未指定为 0）
}

// 无样式标题 fallback 的判定阈值。
const (
	// docxFallbackHeadingMaxRunes fallback 标题允许的最大文本长度（rune 数）。
	docxFallbackHeadingMaxRunes = 50
	// docxFallbackHeadingFontStep 字号显著大于正文众数的最小差值
	// （w:sz 半磅值口径，2 半磅 = 1pt）。
	docxFallbackHeadingFontStep = 2
)

// docxHasHeading 判断文档是否已含标题类元素（section_header/title）。
func docxHasHeading(doc *DoclingDocument) bool {
	for i := range doc.Texts {
		if doc.Texts[i].Label == LabelSectionHeader || doc.Texts[i].Label == LabelTitle {
			return true
		}
	}
	return false
}

// docxIsHeadingShapedText 预筛标题形文本：非空、长度不超上限、无句末标点
// （。？！；.?!）且不以冒号结尾（冒号常引出正文，判定为非标题）。
func docxIsHeadingShapedText(text string) bool {
	if text == "" || utf8.RuneCountInString(text) > docxFallbackHeadingMaxRunes {
		return false
	}
	if strings.ContainsAny(text, "。？！；.?!") {
		return false
	}
	return !strings.HasSuffix(text, "：") && !strings.HasSuffix(text, ":")
}

// docxModeValue 取出现次数最多的值（众数）；并列取较大值（判定更保守），
// 空映射返回 0。
func docxModeValue(counts map[int64]int64) int64 {
	mode, best := int64(0), int64(-1)
	for v, c := range counts {
		if c > best || (c == best && v > mode) {
			mode, best = v, c
		}
	}
	return mode
}

// reparentBodyChild 把 body 顶层元素改挂到新 parent：更新元素自身 Parent 并
// 追加到新 parent 的 children（旧 parent 必为 body，其 children 由调用方重建）。
func (d *DoclingDocument) reparentBodyChild(child RefItem, newParent *RefItem) {
	switch child.Kind {
	case refTexts:
		d.Texts[child.Idx].Parent = d.parentOrBody(newParent)
	case refGroups:
		d.Groups[child.Idx].Parent = d.parentOrBody(newParent)
	case refTables:
		d.Tables[child.Idx].Parent = d.parentOrBody(newParent)
	case refPictures:
		d.Pictures[child.Idx].Parent = d.parentOrBody(newParent)
	}
	d.appendChild(newParent, child)
}

// applyUnstyledHeadingFallback 无样式标题 fallback：第一遍解析后全文档零
// heading/title 元素时才启用（有标准样式的文档行为完全不变）。判定规则为
// 超出 Docling msword 后端的增强（Docling 纯规则后端只认样式，此处对齐版面
// 模型对"手工加粗/大字号标题"的识别效果）——段落级全部满足才判标题：
//  1. 段落主要 run 加粗（w:b 非 val="0"/"false"，w:bCs 不计），或字号显著
//     大于正文字号众数（w:sz 半磅值差 ≥ docxFallbackHeadingFontStep 即 +1pt；
//     众数排除标题候选自身，避免全大字文档互判；文档全等大时退化为仅加粗）；
//  2. 文本 ≤ docxFallbackHeadingMaxRunes 个 rune；
//  3. 无句末标点（。？！；.?!）且不以冒号结尾；
//  4. 非列表项/code/复选框/公式（paraMetas 只记录普通 text 元素来保证）；
//  5. 非空文本。
//
// 命中段落改标 section_header（保守 1 级）并线性重挂：每个 fallback 标题成为
// 后续内容（含平级标题）的最近祖先，首个标题之前的内容仍挂 body；
// 元素顺序与文本内容不变。
func (w *docxWalker) applyUnstyledHeadingFallback() {
	if len(w.paraMetas) == 0 || docxHasHeading(w.doc) {
		return
	}
	// 预筛标题候选：先按不依赖加粗/字号的文本条件过滤
	candidate := map[int64]bool{}
	for _, child := range w.doc.Body.Children {
		if child.Kind != refTexts {
			continue
		}
		if meta, ok := w.paraMetas[child.Idx]; ok && docxIsHeadingShapedText(meta.text) {
			candidate[child.Idx] = true
		}
	}
	// 正文字号众数：排除标题候选自身，避免全大字文档互判
	sizeCounts := map[int64]int64{}
	for _, child := range w.doc.Body.Children {
		if child.Kind != refTexts || candidate[child.Idx] {
			continue
		}
		if meta, ok := w.paraMetas[child.Idx]; ok && meta.maxSz > 0 {
			sizeCounts[meta.maxSz]++
		}
	}
	bodyMode := docxModeValue(sizeCounts)
	// 按序重挂：命中的候选改标标题并成为后续内容的最近祖先
	var prevHeading *RefItem
	newChildren := make([]RefItem, 0, len(w.doc.Body.Children))
	for _, child := range w.doc.Body.Children {
		if child.Kind == refTexts && candidate[child.Idx] {
			meta := w.paraMetas[child.Idx]
			if meta.bold || (bodyMode > 0 && meta.maxSz >= bodyMode+docxFallbackHeadingFontStep) {
				item := &w.doc.Texts[child.Idx]
				item.Label = LabelSectionHeader
				item.TextLevel = 1
				// 首个 fallback 标题留 body 顶层作挂接链根，后续标题挂前一标题下
				if prevHeading == nil {
					newChildren = append(newChildren, child)
				}
				w.doc.reparentBodyChild(child, prevHeading)
				heading := child
				prevHeading = &heading
				continue
			}
		}
		if prevHeading != nil {
			w.doc.reparentBodyChild(child, prevHeading)
		} else {
			newChildren = append(newChildren, child)
		}
	}
	w.doc.Body.Children = newChildren
}

// wordTableCell 表格单元格解析产物：跨列/跨行属性、纯文本与子树原始字节。
type wordTableCell struct {
	gridSpan       int64          // w:gridSpan 跨列数（缺省 1）
	vMergeContinue bool           // w:vMerge 存在且非 restart（继续合并）
	text           string         // 单元格文本（多段落 "\n" 连接）
	raw            []byte         // tc 子树原始字节（1x1 表降级时重放用）
	formatting     *Formatting    // 单元格内可见 run 的统一格式汇总
	hyperlinkIDs   []string       // 单元格内超链接关系 id
	commentIDs     []string       // 单元格内批注 id
	footnoteIDs    []string       // 单元格内脚注引用 id
	endnoteIDs     []string       // 单元格内尾注引用 id
	revisions      []wordRevision // 单元格内修订审计信息
	fields         []string       // 单元格内 Word 域指令
}

// wordTableCellMeta 保存与最终 table_cells 顺序一一对应的富文本元数据。
type wordTableCellMeta struct {
	formatting   *Formatting
	hyperlinkIDs []string
	commentIDs   []string
	footnoteIDs  []string
	endnoteIDs   []string
	revisions    []wordRevision
	fields       []string
}

// wordTableRow 表格行解析产物：行首列偏移与单元格序列。
type wordTableRow struct {
	gridBefore int64           // w:gridBefore 行首列偏移
	cells      []wordTableCell // 行内单元格
}

// consumeRootElement 消费子树根元素的开始标签：collectSubTree 的产物以根
// 开始标签开头，二级解析器（表格/行/单元格）遍历子元素前先跳过它。
func consumeRootElement(dec *xml.Decoder) error {
	for {
		tok, err := dec.Token()
		if err != nil {
			return err
		}
		if _, ok := tok.(xml.StartElement); ok {
			return nil
		}
	}
}

// handleTable 处理 w:tbl（复刻 _handle_tables）：
//  1. 统计 w:tblGrid 网格列数与各行（gridBefore + 单元格跨列/跨行属性）；
//  2. 1x1 表视为版式容器：单元格内容当正文递归处理，不产出 table；
//  3. 否则按网格列游标铺单元格：gridSpan → ColSpan，vMerge continue 扩展
//     锚单元格的 EndRowOffsetIdx 与 RowSpan，首行 column_header=true。
//
// 参数 raw 为 tbl 子树的 XML 字节；返回解析错误。
func (w *docxWalker) handleTable(raw []byte) error {
	dec := xml.NewDecoder(bytes.NewReader(raw))
	if err := consumeRootElement(dec); err != nil {
		return err
	}
	var rows []wordTableRow
	gridCols := int64(0)
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		start, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		switch start.Name.Local {
		case "tblGrid":
			// 统计网格列定义数（对齐 python-docx 的列数口径）
			depth := 1
			for depth > 0 {
				t, err := dec.Token()
				if err != nil {
					return err
				}
				switch tt := t.(type) {
				case xml.StartElement:
					if depth == 1 && tt.Name.Local == "gridCol" {
						gridCols++
					}
					depth++
				case xml.EndElement:
					depth--
				}
			}
		case "tr":
			trRaw, err := collectSubTree(dec, start)
			if err != nil {
				return err
			}
			row, err := parseWordTableRow(trRaw)
			if err != nil {
				return err
			}
			rows = append(rows, row)
		default:
			if err := skipSubTree(dec, start); err != nil {
				return err
			}
		}
	}
	numRows := int64(len(rows))
	if numRows == 0 {
		return nil
	}
	numCols := gridCols
	if numCols <= 0 {
		// 畸形表兜底：无 tblGrid 时取各行累计跨列数的最大值
		for _, row := range rows {
			total := int64(0)
			for _, c := range row.cells {
				total += c.gridSpan
			}
			if total > numCols {
				numCols = total
			}
		}
	}
	if numCols <= 0 {
		return nil
	}
	// 1x1 表：仅剩一个格子的表格视为版式容器，内容按正文继续走
	if numRows == 1 && numCols == 1 && len(rows[0].cells) == 1 {
		// 仍构成代码块边界：边界外的 code 不得与 cell 内 code 合并
		w.forceNewCode = true
		err := w.walkTableCellAsBody(rows[0].cells[0].raw)
		w.forceNewCode = true
		return err
	}
	cells, richCells := buildWordTableCells(rows, numCols)
	tableRef := w.doc.AddTable(cells, numRows, numCols, nil, w.parentAt(w.currentLevel()-1))
	for index, rich := range richCells {
		if index >= len(w.doc.Tables[tableRef.Idx].Data.TableCells) ||
			(rich.formatting == nil && len(rich.hyperlinkIDs) == 0 && len(rich.commentIDs) == 0 && len(rich.footnoteIDs) == 0 && len(rich.endnoteIDs) == 0 && len(rich.revisions) == 0 && len(rich.fields) == 0) {
			continue
		}
		cell := &w.doc.Tables[tableRef.Idx].Data.TableCells[index]
		textRef := RefItem{Kind: refTexts, Idx: int64(len(w.doc.Texts))}
		item := TextItem{
			SelfRef: textRef.String(), Parent: &tableRef, Children: []RefItem{},
			ContentLayer: LayerBody, Label: LabelText, Prov: []ProvenanceItem{},
			Orig: cell.Text, Text: cell.Text,
		}
		if rich.formatting != nil {
			formatting := *rich.formatting
			item.Formatting = &formatting
		}
		item.Meta = wordRevisionAndFieldMeta(rich.revisions, rich.fields)
		for _, id := range rich.hyperlinkIDs {
			if target := w.hyperlinks[id]; target != "" {
				item.Hyperlink = target
				break
			}
		}
		w.doc.Texts = append(w.doc.Texts, item)
		cell.Ref = &textRef
		for _, id := range rich.commentIDs {
			w.commentTargets[id] = append(w.commentTargets[id], textRef)
		}
		for _, id := range rich.footnoteIDs {
			w.footnoteTargets[id] = append(w.footnoteTargets[id], textRef)
		}
		for _, id := range rich.endnoteIDs {
			w.endnoteTargets[id] = append(w.endnoteTargets[id], textRef)
		}
	}
	w.lastIsCode = false
	return nil
}

// buildWordTableCells 按网格列游标把各行单元格铺为 DoclingTableCell：
// gridBefore 决定行首列偏移；vMerge continue 扩展同列锚单元格的行跨度；
// 首行单元格标记 column_header。参数 rows 为行序列、numCols 为网格列数。
func buildWordTableCells(rows []wordTableRow, numCols int64) ([]DoclingTableCell, []wordTableCellMeta) {
	cells := make([]DoclingTableCell, 0, len(rows)*int(numCols))
	metas := make([]wordTableCellMeta, 0, len(rows)*int(numCols))
	// openCells 记录各网格列上仍开放（可被 continue 扩展）的锚单元格
	openCells := map[int64]*DoclingTableCell{}
	for r, row := range rows {
		gridCol := row.gridBefore
		for _, tc := range row.cells {
			if gridCol >= numCols {
				break
			}
			span := tc.gridSpan
			if span < 1 {
				span = 1
			}
			if tc.vMergeContinue {
				if anchor := openCells[gridCol]; anchor != nil {
					anchor.EndRowOffsetIdx = int64(r) + 1
					anchor.RowSpan = anchor.EndRowOffsetIdx - anchor.StartRowOffsetIdx
					gridCol += span
					continue
				}
			}
			cells = append(cells, DoclingTableCell{
				Text:              tc.text,
				RowSpan:           1,
				ColSpan:           span,
				StartRowOffsetIdx: int64(r),
				EndRowOffsetIdx:   int64(r) + 1,
				StartColOffsetIdx: gridCol,
				EndColOffsetIdx:   gridCol + span,
				ColumnHeader:      r == 0,
			})
			metas = append(metas, wordTableCellMeta{
				formatting: tc.formatting, hyperlinkIDs: tc.hyperlinkIDs,
				commentIDs: tc.commentIDs, footnoteIDs: tc.footnoteIDs, endnoteIDs: tc.endnoteIDs,
				revisions: tc.revisions, fields: tc.fields,
			})
			openCells[gridCol] = &cells[len(cells)-1]
			gridCol += span
		}
	}
	return cells, metas
}

// parseWordTableRow 解析表格行子树：w:trPr 的 gridBefore 与直接子 w:tc 序列；
// 行内其他元素（trPr 定义等）跳过。
func parseWordTableRow(raw []byte) (wordTableRow, error) {
	var row wordTableRow
	dec := xml.NewDecoder(bytes.NewReader(raw))
	if err := consumeRootElement(dec); err != nil {
		return row, err
	}
	inTrPr := false
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			return row, nil
		}
		if err != nil {
			return row, err
		}
		if start, ok := tok.(xml.StartElement); ok {
			switch start.Name.Local {
			case "trPr":
				inTrPr = true
			case "gridBefore":
				if inTrPr {
					row.gridBefore = xmlAttrInt(start.Attr, "val", 0)
				}
			case "tc":
				tcRaw, err := collectSubTree(dec, start)
				if err != nil {
					return row, err
				}
				cell, err := parseWordTableCell(tcRaw)
				if err != nil {
					return row, err
				}
				cell.raw = tcRaw
				row.cells = append(row.cells, cell)
			default:
				if err := skipSubTree(dec, start); err != nil {
					return row, err
				}
			}
		} else if end, ok := tok.(xml.EndElement); ok && end.Name.Local == "trPr" {
			inTrPr = false
		}
	}
}

// parseWordTableCell 解析单元格子树：w:tcPr 的 gridSpan/vMerge 属性、直接
// 子 w:p 文本及格式/链接/批注；嵌套表格不计入。参数 raw 为 tc 子树 XML。
func parseWordTableCell(raw []byte) (wordTableCell, error) {
	cell := wordTableCell{gridSpan: 1}
	dec := xml.NewDecoder(bytes.NewReader(raw))
	if err := consumeRootElement(dec); err != nil {
		return cell, err
	}
	inTcPr := false
	var paragraphs []string
	formatting := Formatting{Script: ScriptBaseline}
	hasFormatting := false
	seenHyperlinks := map[string]bool{}
	seenComments := map[string]bool{}
	seenFootnotes := map[string]bool{}
	seenEndnotes := map[string]bool{}
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return cell, err
		}
		if start, ok := tok.(xml.StartElement); ok {
			switch start.Name.Local {
			case "tcPr":
				inTcPr = true
			case "gridSpan":
				if inTcPr {
					cell.gridSpan = xmlAttrInt(start.Attr, "val", 1)
				}
			case "vMerge":
				// vMerge 缺省 val 为 continue（OOXML 规范），restart 才是重新开格
				if inTcPr {
					cell.vMergeContinue = xmlAttrVal(start.Attr, "val") != "restart"
				}
			case "p":
				paragraphRaw, err := collectSubTree(dec, start)
				if err != nil {
					return cell, err
				}
				info, err := parseWordParagraph(paragraphRaw)
				if err != nil {
					return cell, err
				}
				paragraphs = append(paragraphs, sanitizeText(info.text))
				for _, textBox := range extractWordTextBoxes(paragraphRaw) {
					paragraphs = append(paragraphs, wordParagraphTexts(textBox)...)
				}
				if info.formatting != nil {
					hasFormatting = true
					formatting.Bold = formatting.Bold || info.formatting.Bold
					formatting.Italic = formatting.Italic || info.formatting.Italic
					formatting.Underline = formatting.Underline || info.formatting.Underline
					formatting.Strikethrough = formatting.Strikethrough || info.formatting.Strikethrough
					if info.formatting.Script != "" && info.formatting.Script != ScriptBaseline {
						formatting.Script = info.formatting.Script
					}
				}
				for _, id := range info.hyperlinkIDs {
					if !seenHyperlinks[id] {
						seenHyperlinks[id] = true
						cell.hyperlinkIDs = append(cell.hyperlinkIDs, id)
					}
				}
				for _, id := range info.commentIDs {
					if !seenComments[id] {
						seenComments[id] = true
						cell.commentIDs = append(cell.commentIDs, id)
					}
				}
				for _, id := range info.footnoteIDs {
					if !seenFootnotes[id] {
						seenFootnotes[id] = true
						cell.footnoteIDs = append(cell.footnoteIDs, id)
					}
				}
				for _, id := range info.endnoteIDs {
					if !seenEndnotes[id] {
						seenEndnotes[id] = true
						cell.endnoteIDs = append(cell.endnoteIDs, id)
					}
				}
				cell.revisions = append(cell.revisions, info.revisions...)
				cell.fields = append(cell.fields, info.fields...)
			case "tbl", "sdt":
				// 嵌套块整体消费，文本不计入本单元格
				if err := skipSubTree(dec, start); err != nil {
					return cell, err
				}
			default:
				if err := skipSubTree(dec, start); err != nil {
					return cell, err
				}
			}
		} else if end, ok := tok.(xml.EndElement); ok && end.Name.Local == "tcPr" {
			inTcPr = false
		}
	}
	cell.text = strings.Join(paragraphs, "\n")
	if hasFormatting {
		cell.formatting = &formatting
	}
	return cell, nil
}

// wordStyleDef 样式表中的段落样式定义。
type wordStyleDef struct {
	name       string // w:name 显示名
	basedOn    string // w:basedOn 父样式 id（空表示无）
	outlineLvl int64  // w:outlineLvl 大纲级别原始值（0 起算）
	hasOutline bool   // 是否定义了 outlineLvl（区分"未定义"与"级别 0"）
}

// parseWordStyles 解析 styles.xml 为 styleId → 样式定义映射。
// 只关心 name/basedOn/outlineLvl 三个判定字段；畸形 XML 返回已解析部分。
func parseWordStyles(xmlBytes []byte) map[string]wordStyleDef {
	styles := map[string]wordStyleDef{}
	if len(xmlBytes) == 0 {
		return styles
	}
	dec := xml.NewDecoder(bytes.NewReader(xmlBytes))
	curID := ""
	inStyle := false
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return styles
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == "style" && !inStyle {
				curID = xmlAttrVal(t.Attr, "styleId")
				styles[curID] = wordStyleDef{}
				inStyle = true
				continue
			}
			if !inStyle {
				continue
			}
			def := styles[curID]
			switch t.Name.Local {
			case "name":
				def.name = xmlAttrVal(t.Attr, "val")
			case "basedOn":
				def.basedOn = xmlAttrVal(t.Attr, "val")
			case "outlineLvl":
				def.outlineLvl = xmlAttrInt(t.Attr, "val", 0)
				def.hasOutline = true
			}
			styles[curID] = def
		case xml.EndElement:
			if t.Name.Local == "style" && inStyle {
				inStyle = false
				curID = ""
			}
		}
	}
	return styles
}

// wordNumbering numbering.xml 解析产物：numId → abstractNumId → ilvl → numFmt。
type wordNumbering struct {
	abstractNums map[string]map[int64]string // abstractNumId → ilvl → numFmt
	nums         map[string]string           // numId → abstractNumId
}

// wordVisibleNumberingFormats 会产生可见编号标记的 numFmt 集合
// （对齐 msword_backend._VISIBLE_NUMBERING_FORMATS）。
var wordVisibleNumberingFormats = map[string]bool{
	"decimal":     true,
	"lowerRoman":  true,
	"upperRoman":  true,
	"lowerLetter": true,
	"upperLetter": true,
	"decimalZero": true,
}

// parseWordNumbering 解析 numbering.xml：w:abstractNum 的各级 numFmt 定义与
// w:num 到 abstractNum 的映射；畸形 XML 返回已解析部分（缺失视为无编号）。
func parseWordNumbering(xmlBytes []byte) *wordNumbering {
	n := &wordNumbering{
		abstractNums: map[string]map[int64]string{},
		nums:         map[string]string{},
	}
	if len(xmlBytes) == 0 {
		return n
	}
	dec := xml.NewDecoder(bytes.NewReader(xmlBytes))
	var (
		abstractID string
		lvls       map[int64]string
		curLvl     int64
		curNumID   string // 当前 w:num 的 numId（abstractNumId 关联目标）
	)
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return n
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "abstractNum":
				abstractID = xmlAttrVal(t.Attr, "abstractNumId")
				lvls = map[int64]string{}
			case "lvl":
				if lvls != nil {
					curLvl = xmlAttrInt(t.Attr, "ilvl", 0)
				}
			case "numFmt":
				if lvls != nil {
					lvls[curLvl] = xmlAttrVal(t.Attr, "val")
				}
			case "num":
				curNumID = xmlAttrVal(t.Attr, "numId")
			case "abstractNumId":
				// w:num 直接子元素：把当前 num 关联到 abstractNum；
				// abstractNum 内的同名子元素已在 case "abstractNum" 分支处理
				if abstractID == "" && curNumID != "" && n.nums[curNumID] == "" {
					n.nums[curNumID] = xmlAttrVal(t.Attr, "val")
				}
			}
		case xml.EndElement:
			switch t.Name.Local {
			case "abstractNum":
				if abstractID != "" {
					n.abstractNums[abstractID] = lvls
				}
				abstractID = ""
				lvls = nil
			case "num":
				curNumID = ""
			}
		}
	}
	return n
}

// hasVisibleNumberingFormat 判断 numId/ilvl 对应的编号定义是否产生可见
// 编号标记（对齐 _has_visible_numbering_format）；无定义时返回 false。
func (n *wordNumbering) hasVisibleNumberingFormat(numID int64, ilvl int64) bool {
	abstractID, ok := n.nums[strconv.FormatInt(numID, 10)]
	if !ok {
		return false
	}
	lvls, ok := n.abstractNums[abstractID]
	if !ok {
		return false
	}
	return wordVisibleNumberingFormats[lvls[ilvl]]
}

// wordStyleKind 段落样式归类结果（对齐 _get_label_and_level 的归一产物）。
type wordStyleKind int

// 样式归类取值。
const (
	wordStyleText    wordStyleKind = iota // 正文
	wordStyleTitle                        // Title 样式 → title
	wordStyleHeading                      // heading 样式 → section_header
	wordStyleCode                         // Code 样式 → code
)

// wordCodeStyleNames Code 样式显示名精确匹配集合（case-fold，对齐 _CODE_STYLE_NAMES）。
var wordCodeStyleNames = map[string]bool{
	"source code":       true,
	"code":              true,
	"code block":        true,
	"code listing":      true,
	"html preformatted": true,
	"preformatted text": true,
	"preformatted":      true,
	"verbatim":          true,
}

// wordCodeStyleIDs Code 样式 id 精确匹配集合（case-fold，对齐 _CODE_STYLE_IDS）。
var wordCodeStyleIDs = map[string]bool{
	"sourcecode":       true,
	"source_code":      true,
	"code":             true,
	"codeblock":        true,
	"codelisting":      true,
	"htmlpreformatted": true,
	"preformattedtext": true,
	"preformatted":     true,
	"verbatim":         true,
}

// classifyParagraphStyle 归类段落样式（复刻 _get_label_and_level）：
//   - 样式 id/name/base_style（一层）任一含 "heading"（不区分大小写）→
//     heading：层级优先取样式定义 outlineLvl+1（仅 1-9 有效），否则依次从
//     id/name/base id/base name 解析 "Heading N" 形态，失败按 1 级；
//   - 样式链（含 base_style 回溯，深度上限 10）命中 Code 样式集合 → code；
//   - outlineLvl 有效且样式名不含 "title" → heading（语言无关的大纲标记）；
//   - 其余 → 正文。
//
// 参数 styleID 为段落样式 id；返回归类与标题层级（仅 heading 有效）。
func (w *docxWalker) classifyParagraphStyle(styleID string) (wordStyleKind, int64) {
	styleID = strings.TrimSpace(styleID)
	if styleID == "" {
		return wordStyleText, 0
	}
	def := w.styles[styleID]
	var base wordStyleDef
	if def.basedOn != "" {
		base = w.styles[def.basedOn]
	}
	idLower := strings.ToLower(styleID)
	nameLower := strings.ToLower(def.name)
	baseIDLower := strings.ToLower(def.basedOn)
	baseNameLower := strings.ToLower(base.name)
	hasBase := def.basedOn != ""

	// 样式自身定义的 outlineLvl（0 起算，+1 后仅 1-9 有效；未定义时忽略，
	// 样式表中缺失的样式条目同样视为未定义）
	outline := int64(0)
	if def.hasOutline {
		if lv := def.outlineLvl + 1; lv >= 1 && lv <= 9 {
			outline = lv
		}
	}

	isHeading := strings.Contains(idLower, "heading") ||
		strings.Contains(nameLower, "heading") ||
		(hasBase && (strings.Contains(baseIDLower, "heading") ||
			strings.Contains(baseNameLower, "heading")))

	if isHeading {
		// outlineLvl 是语言无关的大纲标记，权威于样式名解析
		if outline > 0 {
			return wordStyleHeading, outline
		}
		// 依次回退：id → name → base id → base name 中的 "Heading N" 形态
		for _, cand := range []struct {
			usable bool
			text   string
		}{
			{strings.Contains(idLower, "heading"), styleID},
			{strings.Contains(nameLower, "heading"), def.name},
			{hasBase && strings.Contains(baseIDLower, "heading"), def.basedOn},
			{hasBase && strings.Contains(baseNameLower, "heading"), base.name},
		} {
			if cand.usable {
				if lv := parseWordHeadingLevel(cand.text); lv > 0 {
					return wordStyleHeading, lv
				}
			}
		}
		return wordStyleHeading, 1
	}

	if w.isWordCodeStyle(styleID) {
		return wordStyleCode, 0
	}
	if outline > 0 && !isWordTitleStyle(styleID, def, base, def.basedOn) {
		return wordStyleHeading, outline
	}
	if strings.EqualFold(styleID, "Title") || strings.EqualFold(def.name, "Title") {
		return wordStyleTitle, 0
	}
	return wordStyleText, 0
}

// isWordTitleStyle 判断样式 id/name/base_style 任一含 "title"（不区分大小写），
// 用于 outlineLvl-only 分支的守卫（Title 样式不当 heading 处理）。
func isWordTitleStyle(styleID string, def, base wordStyleDef, baseID string) bool {
	for _, label := range []string{styleID, def.name, baseID} {
		if strings.Contains(strings.ToLower(label), "title") {
			return true
		}
	}
	return strings.Contains(strings.ToLower(base.name), "title")
}

// isWordCodeStyle 判断样式链（自身 + base_style 回溯，深度上限 10）是否命中
// Code 样式 id/name 精确集合（对齐 _is_code_style，不做字体启发式判定）。
func (w *docxWalker) isWordCodeStyle(styleID string) bool {
	id := strings.TrimSpace(styleID)
	for depth := 0; id != "" && depth < 10; depth++ {
		def := w.styles[id]
		if wordCodeStyleNames[strings.ToLower(strings.TrimSpace(def.name))] ||
			wordCodeStyleIDs[strings.ToLower(id)] {
			return true
		}
		if def.basedOn == "" || def.basedOn == id {
			break
		}
		id = def.basedOn
	}
	return false
}

// wordHeadingLevelRe 匹配 Word 标题样式 id/name 的 "Heading N" 形态
// （不区分大小写，允许空格分隔，不设层级上限，对齐 docling 的宽松解析）。
var wordHeadingLevelRe = regexp.MustCompile(`(?i)^heading\s*(\d+)$`)

// parseWordHeadingLevel 解析样式名/id 中的标题层级；非 "Heading N" 形态返回 0，
// 解析出小于 1 的层级时归一为 1（对齐 _get_heading_and_level 的兜底）。
func parseWordHeadingLevel(style string) int64 {
	style = strings.TrimSpace(style)
	m := wordHeadingLevelRe.FindStringSubmatch(style)
	if m == nil {
		return 0
	}
	n, _ := strconv.ParseInt(m[1], 10, 64)
	if n < 1 {
		n = 1
	}
	return n
}

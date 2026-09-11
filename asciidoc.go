// asciidoc.go 用逐行状态机把 AsciiDoc 文本解析为 DoclingDocument，
// 是 docparse 通用组件的 AsciiDoc 后端。
// 标题/列表/表格/字面块/图片/注释/段落语义复刻 docling 的 asciidoc_backend.py
// （docling 同样不依赖第三方 asciidoc 解析库，为纯逐行状态机）；
// 按任务约定补充/偏离的点（均已在对应位置注释说明）：
//   - `[source,lang]` + `----` 围栏块产出 code 元素（docling 未处理，任务要求补齐）；
//   - 表格 cell 装饰中的 `h` 标记映射 column_header=true，且装饰剥离扩展到
//     每个 "|" 之前（docling 仅剥行首/空白后装饰、剥后即弃，任务要求保留表头语义）；
//   - 标题作为后续内容的父节点（层级父栈模式），使 content_list 携带章节路径；
//     prov 全空不生成。
package docparse

import (
	"regexp"
	"strings"
)

// asciidoc 正则集合（逐条对齐 asciidoc_backend.py 的行级判定模式）。
var (
	// asciidocTitleRe 文档标题行：单个 "=" 后接空格。
	asciidocTitleRe = regexp.MustCompile(`^= `)
	// asciidocSectionRe 章节标题行：两个及以上 "=" 后接空白。
	asciidocSectionRe = regexp.MustCompile(`^==+\s+`)
	// asciidocHeadingPartsRe 剥离标题等号标记与正文。
	asciidocHeadingPartsRe = regexp.MustCompile(`^(=+)\s+(.*)`)
	// asciidocListItemRe 列表项：缩进 + marker（*/-/.+/数字./字母.）+ 空白 + 正文。
	asciidocListItemRe = regexp.MustCompile(`^(\s*)(\*|-|\.+|\d+\.|\w+\.)\s+(.*)`)
	// asciidocTableLineRe 表格数据行：可选 cell 装饰 + "|" 开头且行内有第二个 "|"。
	asciidocTableLineRe = regexp.MustCompile(`^(?:\d+(?:\.\d+)?[*+])*[<^>]?(?:\.[<^>])?[adehlms]?\|.*\|`)
	// asciidocPictureRe 图片宏：image::路径[属性列表]。
	asciidocPictureRe = regexp.MustCompile(`^image::(.+)\[(.*)\]$`)
	// asciidocCaptionRe 块标题行："." 开头且紧跟非空白（与 "." 列表 marker 划界）。
	asciidocCaptionRe = regexp.MustCompile(`^\.\S`)
	// asciidocSourceAttrRe [source,lang] 源码块属性行（lang 为可选第二参数）。
	asciidocSourceAttrRe = regexp.MustCompile(`^\[source(?:,([^,\]]+))?\]`)
)

// asciidocParser AsciiDoc 逐行解析状态。
// parents/indents 为层级父栈（0 层为 title，1 起 section 标题与列表分组交错共用，
// 对齐 docling parents 字典语义）；hasParent/hasIndent 表达对应槽位是否有效。
type asciidocParser struct {
	doc              *DoclingDocument
	parents          [10]RefItem // 层级父栈
	hasParent        [10]bool
	indents          [10]int // 各列表分组的缩进基准
	hasIndent        [10]bool
	inList           bool
	lastListItem     *RefItem // 列表续挂目标（字面块/图片续行时挂上一列表项）
	listContinuation bool     // 出现 "+" 续行标记，字面块/图片保持列表打开
	inTable          bool
	textData         []string           // 段落累积行（空行冲刷）
	tableData        []asciidocTableRow // 表格累积行
	captionData      []string           // 块标题累积行
}

// asciidocTableRow 表格数据行：各 cell 文本与 h 装饰（表头）标记。
type asciidocTableRow struct {
	cells   []string
	headers []bool
}

// asciiDocBlock 解析块：普通行或 `....` 字面块 / [source,lang] 源码块。
type asciiDocBlock struct {
	line     string
	literal  string // 非空表示 `....` 字面块内容
	code     string // 非空表示 [source,lang] + ---- 源码块内容
	codeLang string
}

// ParseAsciiDoc 解析 AsciiDoc 为 DoclingDocument（复刻 docling asciidoc_backend）。
// 逐行状态机天然容错，畸形输入不报错；空输入返回元素为空的文档。
func ParseAsciiDoc(data []byte) (*DoclingDocument, error) {
	doc := NewDoclingDocument("asciidoc")
	// 剥离 UTF-8 BOM：保留时 "= Title" 不再被识别为标题（对齐 utf-8-sig 解码）
	text := strings.TrimPrefix(string(data), "\uFEFF")
	p := &asciidocParser{doc: doc}
	p.parse(iterAsciiDocBlocks(strings.Split(text, "\n")))
	return doc, nil
}

// iterAsciiDocBlocks 把行序列切分为解析块：`....` 界定的字面块、
// [source,lang] + ---- 界定的源码块、其余逐行透传（字面块切分对齐 docling
// _iter_blocks，源码块为按任务约定补充）。
func iterAsciiDocBlocks(lines []string) []asciiDocBlock {
	blocks := make([]asciiDocBlock, 0, len(lines))
	inLiteral := false
	var literal []string
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		if strings.TrimSpace(line) == "...." {
			if !inLiteral {
				inLiteral = true
				literal = nil
			} else {
				blocks = append(blocks, asciiDocBlock{literal: strings.Join(literal, "\n")})
				inLiteral = false
			}
			continue
		}
		if inLiteral {
			literal = append(literal, strings.TrimRight(line, "\r\n"))
			continue
		}
		// [source,lang] 标记行且下一行为 ----：整体收敛为源码块
		if m := asciidocSourceAttrRe.FindStringSubmatch(line); m != nil && i+1 < len(lines) &&
			strings.TrimSpace(lines[i+1]) == "----" {
			var body []string
			j := i + 2
			for ; j < len(lines); j++ {
				if strings.TrimSpace(lines[j]) == "----" {
					break
				}
				body = append(body, strings.TrimRight(lines[j], "\r\n"))
			}
			blocks = append(blocks, asciiDocBlock{
				code:     strings.Join(body, "\n"),
				codeLang: strings.TrimSpace(m[1]),
			})
			i = j // 跳过闭合 ----（循环 i++ 后落在其后）
			continue
		}
		blocks = append(blocks, asciiDocBlock{line: line})
	}
	// 未闭合的字面块按 docling 行为整块产出
	if inLiteral {
		blocks = append(blocks, asciiDocBlock{literal: strings.Join(literal, "\n")})
	}
	return blocks
}

// parse 主状态机：按块顺序分派标题/列表/表格/图片/注释/段落处理，
// 收尾冲刷残留段落与未闭合表格（对齐 docling _parse）。
// 注意：标题分支不冲刷段落累积（对齐 docling 行为，标题前段落与标题后
// 段落在下一个空行处合并产出）。
func (p *asciidocParser) parse(blocks []asciiDocBlock) {
	for _, block := range blocks {
		if block.literal != "" || block.code != "" {
			// 字面块/源码块：先按续行规则决定列表去留，再冲刷文本与块标题
			p.closeListIfNeeded("<block>", true)
			p.flushText()
			p.flushCaption()
			parent := p.currentParent()
			if p.inList && p.lastListItem != nil {
				// "+" 续行后的块挂上一列表项（对齐 docling 列表续挂语义）
				parent = p.lastListItem
			}
			if block.literal != "" {
				p.doc.AddCode(block.literal, "", nil, parent)
			} else {
				p.doc.AddCode(block.code, block.codeLang, nil, parent)
			}
			p.listContinuation = false
			continue
		}

		line := block.line
		stripped := strings.TrimSpace(line)
		p.closeListIfNeeded(line, false)

		switch {
		case asciidocTitleRe.MatchString(line):
			// "= " 文档标题：占住父栈 0 层，作为后续内容的父节点
			ref := p.doc.AddTitle(strings.TrimSpace(line[2:]), nil, nil)
			p.parents[0] = ref
			p.hasParent[0] = true

		case asciidocSectionRe.MatchString(line):
			// "== "/"=== " 章节标题：级数为等号数-1，挂上一层父节点并清更深层
			m := asciidocHeadingPartsRe.FindStringSubmatch(line)
			level := strings.Count(m[1], "=") - 1
			var parent *RefItem
			if level >= 1 && level-1 < len(p.parents) {
				parent = p.parentAt(level - 1)
			}
			ref := p.doc.AddHeading(int64(level), strings.TrimSpace(m[2]), nil, parent)
			// 超出父栈容量的深层标题只产出元素不入栈（畸形输入防御）
			if level >= 1 && level < len(p.parents) {
				p.parents[level] = ref
				p.hasParent[level] = true
				for k := level + 1; k < len(p.parents); k++ {
					p.hasParent[k] = false
				}
			}

		case asciidocListItemRe.MatchString(line):
			p.handleListItem(line)

		case p.inList && stripped == "+":
			// 列表续行标记：保持列表打开，后续字面块/图片续挂上一列表项
			p.listContinuation = true

		case stripped == "|===" && !p.inTable:
			p.inTable = true

		case asciidocTableLineRe.MatchString(line):
			p.inTable = true
			p.tableData = append(p.tableData, parseAsciiDocTableRow(line))

		case p.inTable:
			// 表格内首个非表格行（含闭合 |===）：冲刷表格并带上前导块标题
			p.flushTable(true)

		case asciidocPictureRe.MatchString(line):
			// 图片宏：始终产出 PictureItem，前导块标题优先、alt 作为说明兜底。
			match := asciidocPictureRe.FindStringSubmatch(line)
			caption := strings.TrimSpace(strings.Join(p.captionData, " "))
			p.captionData = nil
			if caption == "" {
				caption = parseAsciiDocImageAlt(line)
			}
			parent := p.currentParent()
			if p.inList && p.lastListItem != nil {
				parent = p.lastListItem
			}
			addPicturePlaceholder(p.doc, strings.TrimSpace(match[1]), caption, parent)
			p.listContinuation = false

		case asciidocCaptionRe.MatchString(line) && len(p.captionData) == 0:
			p.captionData = append(p.captionData, line[1:])

		case stripped != "" && len(p.captionData) > 0:
			// 块标题续行（多行 caption，对齐 docling 的累积语义）
			p.captionData = append(p.captionData, stripped)

		case stripped == "" && len(p.textData) > 0:
			p.flushText()

		case stripped != "":
			p.textData = append(p.textData, stripped)
		}
	}
	// 收尾：冲刷残留段落与未闭合表格
	p.flushText()
	if p.inTable && len(p.tableData) > 0 {
		p.flushTable(false)
	}
}

// handleListItem 处理列表项：缩进驱动列表分组进入/嵌套/回退，产出 list_item；
// 编号型 marker（如 "1."）保留为列表项标记，符号/字母 marker 不保留
// （对齐 docling 的 isdigit 判定）。
func (p *asciidocParser) handleListItem(line string) {
	m := asciidocListItemRe.FindStringSubmatch(line)
	marker := m[2]
	text := strings.TrimSpace(m[3])
	indent := len(m[1])
	if strings.HasPrefix(marker, ".") {
		// "." 系 marker（"."/".."/"..."）按字面长度折算缩进深度
		indent += len(marker) - 1
	}
	numbered := marker != "*" && marker != "-"

	level := p.currentLevel()
	switch {
	case !p.inList:
		// 进入列表：冲刷前导块标题，建列表分组挂当前层父节点
		p.inList = true
		p.flushCaptionAt(p.parentAt(level))
		p.openListGroup(level, indent)
	case p.hasIndent[level] && indent > p.indents[level]:
		// 缩进加深：建嵌套分组挂上一层（对齐 docling 缩进驱动嵌套）
		p.openListGroup(level, indent)
	case p.hasIndent[level] && indent < p.indents[level]:
		// 缩进回落：逐层弹出更深分组直到缩进不小于该层基准
		for level > 0 && p.hasIndent[level] && indent < p.indents[level] {
			p.hasParent[level] = false
			p.hasIndent[level] = false
			level--
		}
	}

	itemMarker := ""
	if len(marker) > 1 && isASCIIDigits(marker[:len(marker)-1]) {
		itemMarker = marker
	}
	group := p.currentParent()
	if group == nil {
		return
	}
	ref := p.doc.AddListItem(*group, text, numbered, itemMarker, nil)
	p.lastListItem = &ref
	p.listContinuation = false
}

// openListGroup 在 level+1 层新建列表分组挂 parents[level]，并记录该层缩进基准
// （对齐 docling 列表分组的层级挂接与 indents 记录）。
func (p *asciidocParser) openListGroup(level, indent int) {
	if level+1 >= len(p.parents) {
		return
	}
	group := p.doc.AddListGroup("list", p.parentAt(level))
	p.parents[level+1] = group
	p.hasParent[level+1] = true
	p.indents[level+1] = indent
	p.hasIndent[level+1] = true
}

// closeListIfNeeded 在处理新块前决定列表去留：列表项、空行、续行标记、
// 或带续行标记的字面块/图片保持列表打开，其余内容关闭列表
// （对齐 docling _close_list_if_needed）。
func (p *asciidocParser) closeListIfNeeded(line string, isContinuationBlock bool) {
	stripped := strings.TrimSpace(line)
	if !p.inList || asciidocListItemRe.MatchString(line) ||
		stripped == "" || stripped == "+" ||
		(p.listContinuation && (isContinuationBlock || asciidocPictureRe.MatchString(line))) {
		return
	}
	p.hasParent[p.currentLevel()] = false
	p.inList = false
	p.lastListItem = nil
	p.listContinuation = false
}

// currentLevel 返回父栈第一个空位的层数-1（对齐 docling _get_current_level）。
func (p *asciidocParser) currentLevel() int {
	for k := 1; k < len(p.parents); k++ {
		if !p.hasParent[k] {
			return k - 1
		}
	}
	return 0
}

// currentParent 返回父栈第一个空位前一层的元素引用（无则挂 body），
// 对齐 docling _get_current_parent。
func (p *asciidocParser) currentParent() *RefItem {
	for k := 1; k < len(p.parents); k++ {
		if !p.hasParent[k] {
			return p.parentAt(k - 1)
		}
	}
	return nil
}

// parentAt 取指定层的父引用（层无效返回 nil，元素挂 body）。
func (p *asciidocParser) parentAt(level int) *RefItem {
	if level >= 0 && level < len(p.parents) && p.hasParent[level] {
		r := p.parents[level]
		return &r
	}
	return nil
}

// flushText 把累积的正文行合并为 paragraph 元素（空行触发或收尾时冲刷，
// 对齐 docling 的 " ".join(text_data) 合并语义）。
func (p *asciidocParser) flushText() {
	if len(p.textData) == 0 {
		return
	}
	p.doc.AddText(LabelParagraph, strings.Join(p.textData, " "), nil, p.currentParent())
	p.textData = nil
}

// flushCaption 把累积的块标题行按当前父节点冲刷为 caption 元素。
func (p *asciidocParser) flushCaption() {
	p.flushCaptionAt(p.currentParent())
}

// flushCaptionAt 按指定父节点冲刷累积的块标题行。
func (p *asciidocParser) flushCaptionAt(parent *RefItem) {
	if len(p.captionData) == 0 {
		return
	}
	p.doc.AddText(LabelCaption, strings.Join(p.captionData, " "), nil, parent)
	p.captionData = nil
}

// flushTable 冲刷累积的表格行并产出 table 元素；withCaption 时先冲刷前导
// 块标题为 caption。列数取各行最大值（短行即行数不齐时的补齐语义），首行
// 与带 h 装饰的 cell 标记 column_header。
func (p *asciidocParser) flushTable(withCaption bool) {
	if withCaption {
		p.flushCaption()
	}
	numRows := len(p.tableData)
	numCols := 0
	for _, row := range p.tableData {
		if len(row.cells) > numCols {
			numCols = len(row.cells)
		}
	}
	cells := make([]DoclingTableCell, 0, numRows*numCols)
	for r, row := range p.tableData {
		for c, text := range row.cells {
			header := r == 0
			if c < len(row.headers) && row.headers[c] {
				header = true
			}
			cells = append(cells, DoclingTableCell{
				Text:              text,
				RowSpan:           1,
				ColSpan:           1,
				StartRowOffsetIdx: int64(r),
				EndRowOffsetIdx:   int64(r + 1),
				StartColOffsetIdx: int64(c),
				EndColOffsetIdx:   int64(c + 1),
				ColumnHeader:      header,
			})
		}
	}
	p.doc.AddTable(cells, int64(numRows), int64(numCols), nil, p.currentParent())
	p.inTable = false
	p.tableData = nil
}

// parseAsciiDocTableRow 解析 `|===` 表格数据行：装饰段（如 "^.^h"）紧贴的
// "|" 与装饰共同构成带装饰的 cell 开始符（同时兼容 "^.^h|名称" 行首装饰与
// "|^.^h|名称" 分隔后装饰两种写法），裸 "|" 开启无装饰 cell；首个分隔符之前
// 的内容不属于任何 cell（对齐 split("|")[1:]），行尾多余的 "|" 产生空 cell。
// 装饰中的 h 标记记录为表头标记。
func parseAsciiDocTableRow(line string) asciidocTableRow {
	var row asciidocTableRow
	first := true // 尚未产出任何 cell
	var cur strings.Builder
	curH := false
	// flushCurrent 结束当前 cell（尚未产出过 cell 时忽略）
	flushCurrent := func() {
		if !first {
			row.cells = append(row.cells, strings.TrimSpace(cur.String()))
			row.headers = append(row.headers, curH)
		}
	}
	for pos := 0; pos < len(line); {
		if n, hasH := matchAsciiDocCellSpec(line[pos:]); n > 0 {
			// 装饰段 + "|"：带装饰的 cell 开始符
			flushCurrent()
			first = false
			cur.Reset()
			curH = hasH
			pos += n + 1
			continue
		}
		if line[pos] == '|' {
			// 裸 "|"：无装饰 cell 开始符（其后紧跟的装饰段并入本 cell）
			flushCurrent()
			first = false
			cur.Reset()
			pos++
			if n, hasH := matchAsciiDocCellSpec(line[pos:]); n > 0 {
				curH = hasH
				pos += n + 1
			} else {
				curH = false
			}
			continue
		}
		if first {
			// 首个分隔符之前的内容不属于任何 cell
			pos++
			continue
		}
		cur.WriteByte(line[pos])
		pos++
	}
	flushCurrent()
	return row
}

// matchAsciiDocCellSpec 匹配 s 开头的 cell 装饰段并要求其后紧跟 "|"，
// 返回装饰段长度与是否含 h 标记；无匹配返回 0。
// 装饰语法对齐 _CELL_SPEC：重复的宽度段（数字[.数字][*+]）+ 可选对齐段
// （[<>^] 与 .[<>^]）+ 可选样式字符（[adehlms]，h 表表头）。
// Go regexp 不支持前瞻断言，改为枚举可选段组合后校验后随字符。
func matchAsciiDocCellSpec(s string) (int, bool) {
	pos := 0
	// 重复宽度段：\d+(\.\d+)?[*+]
	for {
		j := pos
		for j < len(s) && s[j] >= '0' && s[j] <= '9' {
			j++
		}
		if j == pos {
			break
		}
		if j < len(s) && s[j] == '.' {
			// 小数宽度段
			k := j + 1
			for k < len(s) && s[k] >= '0' && s[k] <= '9' {
				k++
			}
			if k > j+1 {
				j = k
			}
		}
		if j < len(s) && (s[j] == '*' || s[j] == '+') {
			pos = j + 1
			continue
		}
		break
	}
	// 枚举 [<>^]? / .[<>^]? / [adehlms]? 组合，取能紧贴 "|" 的最长匹配
	best, bestH := 0, false
	for a := 0; a <= 1; a++ {
		p1 := pos
		if a == 1 {
			if p1 >= len(s) || (s[p1] != '<' && s[p1] != '>' && s[p1] != '^') {
				continue
			}
			p1++
		}
		for d := 0; d <= 1; d++ {
			p2 := p1
			if d == 1 {
				if p2+1 >= len(s) || s[p2] != '.' ||
					(s[p2+1] != '<' && s[p2+1] != '>' && s[p2+1] != '^') {
					continue
				}
				p2 += 2
			}
			for st := 0; st <= 1; st++ {
				p3 := p2
				hasH := false
				if st == 1 {
					if p3 >= len(s) || !isAsciiDocStyleByte(s[p3]) {
						continue
					}
					hasH = s[p3] == 'h'
					p3++
				}
				if p3 < len(s) && s[p3] == '|' && p3 > best {
					best, bestH = p3, hasH
				}
			}
		}
	}
	return best, bestH
}

// isAsciiDocStyleByte 是否为 cell 装饰的样式字符（a/d/e/h/l/m/s）。
func isAsciiDocStyleByte(b byte) bool {
	switch b {
	case 'a', 'd', 'e', 'h', 'l', 'm', 's':
		return true
	}
	return false
}

// isASCIIDigits 是否为非空且全为 ASCII 数字（对齐 marker 编号判定的 isdigit）。
func isASCIIDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// parseAsciiDocImageAlt 解析 image:: 宏的 alt 属性：首个属性为 alt 文本，
// 其后不含 "=" 的属性并入 alt（对齐 docling _parse_picture）。
func parseAsciiDocImageAlt(line string) string {
	m := asciidocPictureRe.FindStringSubmatch(line)
	if m == nil {
		return ""
	}
	attrs := strings.Split(m[2], ",")
	var parts []string
	if strings.TrimSpace(attrs[0]) != "" {
		parts = append(parts, strings.TrimSpace(attrs[0]))
	}
	for _, attr := range attrs[1:] {
		if strings.Contains(attr, "=") {
			continue
		}
		if s := strings.TrimSpace(attr); s != "" {
			parts = append(parts, s)
		}
	}
	return strings.Join(parts, ", ")
}

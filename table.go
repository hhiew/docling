package docparse

import (
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	extast "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/text"
)

// 表格与 Markdown 结构化解析：
// 1. GFM 表格的严格解析（基于 goldmark，正确处理 \| 转义与单元格边界）
//    与宽松按行切分兜底（缺失分隔行等非规范格式）；
// 2. 表格清洗链（去空行空列、合并相邻重复列、去重合并单元格文本、规范分隔行）；
// 3. 表格渲染（单元格管道符转义、换行压平）。

// newTableMarkdownParser 创建仅启用 GFM 表格扩展的解析器。
func newTableMarkdownParser() goldmark.Markdown {
	return goldmark.New(goldmark.WithExtensions(extension.Table))
}

// postprocessTableMarkdown 对 Markdown 表格进行后处理，
// 修复解析过程中常见的结构噪声：空行空列、相邻重复列、合并单元格展开后的重复文本。
// 该函数保持表格的 Markdown 语义，输出可直接作为 chunk 文本入向量库。
func postprocessTableMarkdown(md string) string {
	rows := ParseMarkdownTable(md)
	if len(rows) == 0 {
		return md
	}
	rows = postprocessTableRows(rows)
	if len(rows) == 0 {
		return ""
	}
	return RenderMarkdownTable(rows)
}

// postprocessTableRows 对二维表格执行统一清洗链：
// 去空行空列、合并相邻重复列、去重合并单元格文本、规范表头分隔行。
// 供 Markdown 字符串入口与 goldmark AST 入口共用。
func postprocessTableRows(rows [][]string) [][]string {
	rows = removeEmptyTableRows(rows)
	rows = removeEmptyTableColumns(rows)
	if len(rows) == 0 {
		return nil
	}
	rows = mergeDuplicateAdjacentColumns(rows)
	rows = deduplicateMergedCellText(rows)
	rows = normalizeTableSeparator(rows)
	return rows
}

// ParseMarkdownTable 把 Markdown 表格文本解析成二维字符串数组。
// 优先使用 goldmark 的 GFM 严格解析，正确处理 \| 转义与单元格边界；
// 输入不含合法 GFM 表格（如缺失分隔行的宽松格式）时回退按行切分的宽松解析，
// 保证行为不劣化。分隔行不进入数据数组。
func ParseMarkdownTable(md string) [][]string {
	if rows := parseMarkdownTableStrict(md); len(rows) > 0 {
		return rows
	}
	return parseMarkdownTableLoose(md)
}

// parseMarkdownTableStrict 用 goldmark 解析输入中的 GFM 表格并提取二维单元格数组。
// 输入不含可识别表格时返回 nil。
func parseMarkdownTableStrict(md string) [][]string {
	source := []byte(md)
	doc := newTableMarkdownParser().Parser().Parse(text.NewReader(source))
	var rows [][]string
	for node := doc.FirstChild(); node != nil; node = node.NextSibling() {
		if table, ok := node.(*extast.Table); ok {
			rows = append(rows, extractMarkdownTableRows(table, source)...)
		}
	}
	return rows
}

// parseMarkdownTableLoose 旧版按行切分的宽松解析：只要行内含 | 即按 | 拆分。
// 不处理转义管道符，仅作为 goldmark 无法识别格式时的兜底。
func parseMarkdownTableLoose(md string) [][]string {
	lines := strings.Split(md, "\n")
	var rows [][]string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || !strings.Contains(line, "|") {
			continue
		}
		// 分隔行不解析为数据，render 时会重新生成。
		if isTableSeparatorLine(line) {
			continue
		}
		cols := splitTableRow(line)
		if len(cols) == 0 {
			continue
		}
		rows = append(rows, cols)
	}
	return rows
}

// splitTableRow 按 | 拆分一行，去除每列首尾空白。
func splitTableRow(line string) []string {
	parts := strings.Split(line, "|")
	var cols []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" && len(parts) > 1 {
			// Markdown 表格行通常以 | 开头和结尾，首尾会有空字符串，跳过。
			continue
		}
		cols = append(cols, p)
	}
	return cols
}

// isTableSeparatorLine 判断一行是否只有 - 和 |。
func isTableSeparatorLine(line string) bool {
	for _, r := range line {
		if r != '|' && r != '-' && r != ':' && r != ' ' && r != '\t' {
			return false
		}
	}
	return true
}

// removeEmptyTableRows 去除所有单元格都为空的行。
func removeEmptyTableRows(rows [][]string) [][]string {
	out := make([][]string, 0, len(rows))
	for _, row := range rows {
		if isEmptyRow(row) {
			continue
		}
		out = append(out, row)
	}
	return out
}

// removeEmptyTableColumns 去除所有行对应列都为空的列。
func removeEmptyTableColumns(rows [][]string) [][]string {
	if len(rows) == 0 {
		return rows
	}
	maxCol := 0
	for _, row := range rows {
		if len(row) > maxCol {
			maxCol = len(row)
		}
	}
	if maxCol == 0 {
		return rows
	}

	empty := make([]bool, maxCol)
	for c := 0; c < maxCol; c++ {
		empty[c] = true
		for _, row := range rows {
			if c < len(row) && strings.TrimSpace(row[c]) != "" {
				empty[c] = false
				break
			}
		}
	}

	out := make([][]string, 0, len(rows))
	for _, row := range rows {
		newRow := make([]string, 0, len(row))
		for c := 0; c < len(row); c++ {
			if !empty[c] {
				newRow = append(newRow, row[c])
			}
		}
		out = append(out, newRow)
	}
	return out
}

// mergeDuplicateAdjacentColumns 合并相邻且内容完全相同的列。
// 常见于合并单元格被解析为多列，或 Excel 导出产生重复列。
func mergeDuplicateAdjacentColumns(rows [][]string) [][]string {
	if len(rows) == 0 {
		return rows
	}
	maxCol := 0
	for _, row := range rows {
		if len(row) > maxCol {
			maxCol = len(row)
		}
	}
	if maxCol <= 1 {
		return rows
	}

	// 对每一列，收集所有行在该列的文本（空值视为 ""）。
	getColText := func(c int) []string {
		col := make([]string, len(rows))
		for i, row := range rows {
			if c < len(row) {
				col[i] = row[c]
			}
		}
		return col
	}

	keep := make([]bool, maxCol)
	for c := 0; c < maxCol; c++ {
		keep[c] = true
	}

	for c := 1; c < maxCol; c++ {
		if slicesEqual(getColText(c-1), getColText(c)) {
			keep[c] = false
		}
	}

	out := make([][]string, 0, len(rows))
	for _, row := range rows {
		newRow := make([]string, 0, len(row))
		for c := 0; c < len(row) && c < maxCol; c++ {
			if keep[c] {
				newRow = append(newRow, row[c])
			}
		}
		out = append(out, newRow)
	}
	return out
}

// deduplicateMergedCellText 去除合并单元格被解析为多行后产生的连续重复文本。
// 例如某单元格合并了 3 行，解析器可能在每一行都填入相同文本，
// 这里把连续完全相同的文本只保留第一处，避免 chunk 中出现大量重复。
// 表头分隔行不参与去重比较。
func deduplicateMergedCellText(rows [][]string) [][]string {
	if len(rows) <= 1 {
		return rows
	}
	out := make([][]string, len(rows))
	for i, row := range rows {
		newRow := make([]string, len(row))
		for c, text := range row {
			if i > 0 && !isSeparatorRowContent(out[i-1]) && c < len(out[i-1]) &&
				strings.TrimSpace(text) != "" &&
				strings.TrimSpace(text) == strings.TrimSpace(out[i-1][c]) {
				newRow[c] = ""
			} else {
				newRow[c] = text
			}
		}
		out[i] = newRow
	}
	return out
}

// isSeparatorRowContent 判断一行是否全为 --- 或空（分隔行）。
func isSeparatorRowContent(row []string) bool {
	for _, c := range row {
		t := strings.TrimSpace(c)
		if t != "" && t != "---" {
			return false
		}
	}
	return true
}

// normalizeTableSeparator 确保 Markdown 表格在第一行表头后存在标准分隔行。
func normalizeTableSeparator(rows [][]string) [][]string {
	if len(rows) == 0 {
		return rows
	}
	maxCol := 0
	for _, row := range rows {
		if len(row) > maxCol {
			maxCol = len(row)
		}
	}
	sep := make([]string, maxCol)
	for i := range sep {
		sep[i] = "---"
	}
	return append([][]string{rows[0], sep}, rows[1:]...)
}

// escapeMarkdownTableCell 转义单元格内的管道符并压平换行，
// 保证渲染结果仍是合法的 GFM 表格（单元格内容不允许出现裸管道符与换行）。
func escapeMarkdownTableCell(text string) string {
	text = strings.ReplaceAll(text, "|", `\|`)
	text = strings.ReplaceAll(text, "\r", " ")
	text = strings.ReplaceAll(text, "\n", " ")
	return text
}

// RenderMarkdownTable 把二维数组渲染为 Markdown 表格字符串。
func RenderMarkdownTable(rows [][]string) string {
	if len(rows) == 0 {
		return ""
	}
	lines := make([]string, 0, len(rows))
	for _, row := range rows {
		cols := make([]string, len(row))
		for i, c := range row {
			cols[i] = " " + escapeMarkdownTableCell(strings.TrimSpace(c)) + " "
		}
		lines = append(lines, "|"+strings.Join(cols, "|")+"|")
	}
	return strings.Join(lines, "\n")
}

// isEmptyRow 判断一行是否所有单元格都为空。
func isEmptyRow(row []string) bool {
	for _, c := range row {
		if strings.TrimSpace(c) != "" {
			return false
		}
	}
	return true
}

// slicesEqual 判断两个字符串切片是否完全相同。
func slicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// extractMarkdownTableRows 从 GFM 表格 AST 提取二维单元格文本。
// goldmark 的表格结构为 Table 下平级挂 TableHeader 与 TableRow（每行的
// 子节点即 TableCell 序列）。单元格取行内纯文本；切列阶段已按转义管道
// 处理边界，但文本值保留 \| 原样，这里统一还原为 |。
func extractMarkdownTableRows(table *extast.Table, source []byte) [][]string {
	var rows [][]string
	for node := table.FirstChild(); node != nil; node = node.NextSibling() {
		switch node.(type) {
		case *extast.TableHeader, *extast.TableRow:
			cells := make([]string, 0, 4)
			for cell := node.FirstChild(); cell != nil; cell = cell.NextSibling() {
				cellText := strings.TrimSpace(extractInlinePlainText(cell, source))
				cellText = strings.ReplaceAll(cellText, `\|`, "|")
				cells = append(cells, cellText)
			}
			if len(cells) == 0 {
				continue
			}
			// 跳过 GFM 分隔行（如 --- | :---:）：渲染时统一生成规范分隔行，
			// 避免分隔行作为数据行混入导致重复
			isSeparator := true
			for _, c := range cells {
				trimmed := strings.Trim(strings.TrimSpace(c), ":")
				if trimmed == "" || strings.Trim(trimmed, "-") != "" {
					isSeparator = false
					break
				}
			}
			if isSeparator {
				continue
			}
			rows = append(rows, cells)
		}
	}
	return rows
}

// extractInlinePlainText 提取行内节点树的纯文本：保留字面内容，
// 去除加粗/斜体等 Markdown 修饰符号；软换行转空格，硬换行转换行符。
func extractInlinePlainText(node ast.Node, source []byte) string {
	var sb strings.Builder
	var walk func(n ast.Node)
	walk = func(n ast.Node) {
		switch t := n.(type) {
		case *ast.Text:
			sb.Write(t.Segment.Value(source))
			if t.HardLineBreak() {
				sb.WriteString("\n")
			} else if t.SoftLineBreak() {
				sb.WriteString(" ")
			}
		case *ast.String:
			sb.Write(t.Value)
		default:
			for c := n.FirstChild(); c != nil; c = c.NextSibling() {
				walk(c)
			}
		}
	}
	for c := node.FirstChild(); c != nil; c = c.NextSibling() {
		walk(c)
	}
	return sb.String()
}

// Package cli 导出 docling CLI 的通用输出辅助：Outline 生成渐进式披露的
// 结构地图（标题树/表格/图片/分组），RenderItemsMarkdown 把过滤后的条目
// 渲染为简化 Markdown，FilterItems 按章节/分组过滤，ParseLayers 解析内容层。
// 供 cmd/docling 与 ur CLI(ur doc parse) 复用，均只依赖公开 Item 字段。
package cli

import (
	"fmt"
	"strings"

	"github.com/unitedrhino/docling"
)

// buildOutline 生成文档结构地图：首行概览 + 标题树 + 表格清单 + 图片清单
// + 多分组（工作表/幻灯片）列表。输出控制在几十行内，供 AI 决定下一步。
func Outline(doc *docling.DoclingDocument) string {
	items := ToItems(doc)
	var b strings.Builder
	fmt.Fprintf(&b, "文档: %s | %d 页 | %d 文本块 | %d 表格 | %d 图片\n",
		doc.Name, len(doc.Pages), countType(items, docling.ItemTypeText), countType(items, docling.ItemTypeTable), countType(items, docling.ItemTypeImage))

	if sheets := collectGroups(items); len(sheets) > 1 {
		fmt.Fprintf(&b, "分组(%d): %s\n", len(sheets), strings.Join(sheets, " / "))
	}

	b.WriteString("\n标题树:\n")
	headings := 0
	for _, item := range items {
		if item.TextLevel <= 0 {
			continue
		}
		indent := strings.Repeat("  ", clampLevel(item.TextLevel))
		fmt.Fprintf(&b, "%s- %s (p%d)\n", indent, oneLine(item.Text), item.PageIdx+1)
		headings++
	}
	if headings == 0 {
		b.WriteString("  (无标题结构)\n")
	}

	b.WriteString("\n表格:\n")
	tables := 0
	for _, item := range items {
		if item.Type != docling.ItemTypeTable {
			continue
		}
		tables++
		rows, cols := estimateTableSize(item.TableBody)
		fmt.Fprintf(&b, "  #%d %d行x%d列 (p%d %s)\n", tables, rows, cols, item.PageIdx+1, sectionTail(item.SectionPath))
	}
	if tables == 0 {
		b.WriteString("  (无表格)\n")
	}

	images := 0
	for _, item := range items {
		if item.Type != docling.ItemTypeImage {
			continue
		}
		images++
		caption := oneLine(item.ImageCaption)
		if caption == "" || strings.HasPrefix(caption, "data:") {
			caption = "(无说明)"
		}
		fmt.Fprintf(&b, "图片 #%d: %s (p%d)\n", images, caption, item.PageIdx+1)
	}
	return b.String()
}

// renderItemsMarkdown 把过滤后的 content_list 渲染为简化 Markdown：标题按
// 层级映射 #、表格用现成 GFM TableBody、公式用 LaTeX、代码加围栏。
func RenderItemsMarkdown(items []docling.Item) string {
	var b strings.Builder
	for _, item := range items {
		switch {
		case item.Type == docling.ItemTypeTable:
			if item.TableCaption != "" {
				fmt.Fprintf(&b, "**%s**\n\n", strings.TrimSpace(item.TableCaption))
			}
			if item.TableBody != "" {
				b.WriteString(strings.TrimRight(item.TableBody, "\n"))
				b.WriteString("\n\n")
			}
			if item.TableFootnote != "" {
				fmt.Fprintf(&b, "> %s\n\n", strings.TrimSpace(item.TableFootnote))
			}
		case item.Type == docling.ItemTypeImage:
			caption := strings.TrimSpace(item.ImageCaption)
			if caption == "" {
				caption = "图片"
			}
			fmt.Fprintf(&b, "![%s](%s)\n\n", caption, item.ImgPath)
		case item.Label == "code":
			fmt.Fprintf(&b, "```\n%s\n```\n\n", strings.TrimRight(item.Text, "\n"))
		case item.Label == "formula" || item.LaTeX != "":
			latex := item.LaTeX
			if latex == "" {
				latex = item.Text
			}
			fmt.Fprintf(&b, "$$\n%s\n$$\n\n", strings.TrimSpace(latex))
		case item.TextLevel > 0:
			level := clampLevel(item.TextLevel) + 1
			fmt.Fprintf(&b, "%s %s\n\n", strings.Repeat("#", level), strings.TrimSpace(item.Text))
		case item.Label == "list_item":
			if item.Text == "" {
				continue
			}
			fmt.Fprintf(&b, "- %s\n", strings.TrimSpace(item.Text))
		default:
			text := strings.TrimRight(item.Text, "\n")
			if strings.TrimSpace(text) == "" {
				continue
			}
			if strings.HasSuffix(b.String(), "\n\n") || b.Len() == 0 {
				// 段落间已有空行
			} else {
				b.WriteString("\n")
			}
			b.WriteString(text)
			b.WriteString("\n\n")
		}
	}
	return b.String()
}

// countType 统计某类型条目数。
func countType(items []docling.Item, t docling.ItemType) int {
	n := 0
	for _, item := range items {
		if item.Type == t {
			n++
		}
	}
	return n
}

// collectGroups 按出现序收集首级分组名（工作表/幻灯片等）。
func collectGroups(items []docling.Item) []string {
	seen := map[string]bool{}
	var groups []string
	for _, item := range items {
		if len(item.SectionPath) == 0 {
			continue
		}
		first := item.SectionPath[0]
		if !seen[first] {
			seen[first] = true
			groups = append(groups, first)
		}
	}
	return groups
}

// sectionTail 返回章节路径最后两级,用于表格定位提示。
func sectionTail(path []string) string {
	if len(path) == 0 {
		return ""
	}
	if len(path) == 1 {
		return path[0]
	}
	return path[len(path)-2] + " > " + path[len(path)-1]
}

// estimateTableSize 从 GFM 表格文本估算行列数。
func estimateTableSize(body string) (rows, cols int) {
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "|") {
			continue
		}
		if strings.Contains(trimmed, "---") {
			continue
		}
		rows++
		if c := strings.Count(trimmed, "|") - 1; c > cols {
			cols = c
		}
	}
	if cols < 0 {
		cols = 0
	}
	return rows, cols
}

// clampLevel 把标题层级收敛到 0..5（渲染 # 数 = clamp+1,最高 6 级）。
func clampLevel(level int64) int {
	if level < 1 {
		return 0
	}
	if level > 6 {
		return 6
	}
	return int(level - 1)
}

// oneLine 压缩文本为首行摘要（去换行,截断 80 rune）。
func oneLine(s string) string {
	s = strings.TrimSpace(strings.SplitN(s, "\n", 2)[0])
	runes := []rune(s)
	if len(runes) > 80 {
		return string(runes[:80]) + "…"
	}
	return s
}

// ToItems 把文档转为 content_list(Golight 来源标记仅供溯源)。
func ToItems(doc *docling.DoclingDocument) []docling.Item {
	return docling.ToContentList(doc, docling.SourceGolight)
}

// matchSection 判断章节路径任一层级是否包含 section 前缀(标题自身也算)。
func matchSection(path []string, section string) bool {
	for _, part := range path {
		if strings.Contains(part, section) {
			return true
		}
	}
	return false
}

// FilterItems 按章节路径前缀与首级分组名过滤 content_list;两个条件均为
// 子串匹配(便于"第四章"命中"第四章 系统设计"),空条件跳过。
func FilterItems(items []docling.Item, section, sheet string) []docling.Item {
	section = strings.TrimSpace(section)
	sheet = strings.TrimSpace(sheet)
	out := make([]docling.Item, 0, len(items))
	for _, item := range items {
		if sheet != "" {
			if len(item.SectionPath) == 0 || !strings.Contains(item.SectionPath[0], sheet) {
				continue
			}
		}
		if section != "" && !matchSection(item.SectionPath, section) {
			continue
		}
		out = append(out, item)
	}
	return out
}

// ParseLayers 解析逗号分隔的内容层;未知值忽略,空/全未知回退 body。
func ParseLayers(s string) []docling.ContentLayer {
	valid := map[string]docling.ContentLayer{
		"body":       docling.LayerBody,
		"furniture":  docling.LayerFurniture,
		"background": docling.LayerBackground,
		"invisible":  docling.LayerInvisible,
		"notes":      docling.LayerNotes,
	}
	var layers []docling.ContentLayer
	for _, part := range strings.Split(s, ",") {
		if layer, ok := valid[strings.TrimSpace(strings.ToLower(part))]; ok {
			layers = append(layers, layer)
		}
	}
	if len(layers) == 0 {
		layers = []docling.ContentLayer{docling.LayerBody}
	}
	return layers
}

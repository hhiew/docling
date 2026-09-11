// office_object_render.go 为无法直接复用 Office 预览资源的复杂对象生成
// 稳定语义 SVG。该预览不追求像素级还原，但在纯 Go 环境中保证对象可见。
package docling

import (
	"encoding/base64"
	"fmt"
	"html"
	"strings"
	"unicode/utf8"
)

const (
	// officeObjectSVGWidth 是复杂对象语义预览宽度。
	officeObjectSVGWidth = 800
	// officeObjectSVGHeight 是复杂对象语义预览高度。
	officeObjectSVGHeight = 360
)

// renderOfficeObjectSVG 为复杂对象生成带类型和可见文本的 SVG 预览。
func renderOfficeObjectSVG(record officeObjectRecord) *ImageRef {
	className := officeObjectClassName(record.kind)
	title := map[string]string{
		"smartart": "SmartArt", "wordart": "艺术字", "shape": "形状", "embedded_object": "嵌入对象",
	}[className]
	if title == "" {
		title = "Office 对象"
	}
	caption := officeObjectCaption(record)
	lines := strings.Split(caption, "\n")
	if len(lines) > 4 {
		lines = lines[:4]
	}
	var body strings.Builder
	body.WriteString(`<svg xmlns="http://www.w3.org/2000/svg" width="800" height="360" viewBox="0 0 800 360" role="img">`)
	body.WriteString(`<rect width="800" height="360" rx="20" fill="#f8fafc" stroke="#94a3b8" stroke-width="3"/>`)
	body.WriteString(`<rect x="28" y="28" width="744" height="64" rx="12" fill="#e0e7ff"/>`)
	fmt.Fprintf(&body, `<text x="52" y="70" font-family="sans-serif" font-size="26" font-weight="600" fill="#1e293b">%s</text>`, html.EscapeString(title))
	for index, line := range lines {
		line = truncateOfficeObjectSVGText(strings.TrimSpace(line), 48)
		if line == "" {
			continue
		}
		fmt.Fprintf(&body, `<text x="52" y="%d" font-family="sans-serif" font-size="22" fill="#334155">%s</text>`, 142+index*46, html.EscapeString(line))
	}
	body.WriteString(`</svg>`)
	return &ImageRef{
		Mimetype: "image/svg+xml", Dpi: 96,
		Size: &ImageSize{Width: officeObjectSVGWidth, Height: officeObjectSVGHeight},
		URI:  "data:image/svg+xml;base64," + base64.StdEncoding.EncodeToString([]byte(body.String())),
	}
}

// truncateOfficeObjectSVGText 按 rune 截断预览文字，避免超出画布。
func truncateOfficeObjectSVGText(text string, limit int) string {
	if utf8.RuneCountInString(text) <= limit {
		return text
	}
	runes := []rune(text)
	return string(runes[:limit-1]) + "…"
}

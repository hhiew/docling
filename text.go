package docparse

import "strings"

// ParseText 解析纯文本为单 text 元素的 DoclingDocument（不做结构识别），
// 复刻 Docling 纯文本行为：整文件一个 TEXT 元素、无层级、无 prov。
// 输入为空白时返回元素为空的文档（不视为错误），由调用方决定回退或报错。
func ParseText(data []byte) (*DoclingDocument, error) {
	text := sanitizeText(strings.TrimSpace(strings.TrimPrefix(string(data), "\uFEFF")))
	doc := NewDoclingDocument("text")
	if text == "" {
		return doc, nil
	}
	doc.AddText(LabelText, text, nil, nil)
	return doc, nil
}

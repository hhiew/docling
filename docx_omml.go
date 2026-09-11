// docx_omml.go 实现 Office Math Markup Language 到 LaTeX 的纯 Go 转换。
// 转换器优先覆盖技术文档常见结构；未知标签递归保留可见子内容，确保公式
// 不因扩展属性或新版 Office 节点而丢失。
package docparse

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

const ommlMaxRenderDepth = 64

// ommlNode 是转换器使用的轻量 XML 节点，只保留本地名、属性、文本和有序子节点。
type ommlNode struct {
	name     string            // name 是不含命名空间的 OMML 标签名。
	attrs    map[string]string // attrs 以本地名保存属性，主要读取 m:val。
	text     string            // text 保存节点直接包含的字符数据。
	children []*ommlNode       // children 保持原始文档顺序。
}

// convertOMMLToLaTeX 把一个完整 m:oMath 子树转换为 LaTeX。
func convertOMMLToLaTeX(raw []byte) string {
	root, err := parseOMMLTree(raw)
	if err != nil || root == nil {
		return ""
	}
	return strings.TrimSpace(renderOMMLNode(root, 0))
}

// visibleOMMLText 在结构转换失败时仅提取 m:t 可见文本，保证安全降级不丢内容。
func visibleOMMLText(raw []byte) string {
	decoder := xml.NewDecoder(bytes.NewReader(raw))
	var result strings.Builder
	inText := false
	for {
		token, err := decoder.Token()
		if err != nil {
			return strings.TrimSpace(result.String())
		}
		switch value := token.(type) {
		case xml.StartElement:
			if value.Name.Local == "t" && (value.Name.Space == "" || value.Name.Space == wordMathNS) {
				inText = true
			}
		case xml.EndElement:
			if value.Name.Local == "t" {
				inText = false
			}
		case xml.CharData:
			if inText {
				result.Write(value)
			}
		}
	}
}

// parseOMMLTree 使用流式解码构造受控轻量树。
func parseOMMLTree(raw []byte) (*ommlNode, error) {
	decoder := xml.NewDecoder(bytes.NewReader(raw))
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			return nil, io.ErrUnexpectedEOF
		}
		if err != nil {
			return nil, err
		}
		if start, ok := token.(xml.StartElement); ok {
			return parseOMMLNode(decoder, start, 0)
		}
	}
}

// parseOMMLNode 递归解析一个 XML 子树，并限制深度防止畸形输入耗尽栈。
func parseOMMLNode(decoder *xml.Decoder, start xml.StartElement, depth int) (*ommlNode, error) {
	if depth > ommlMaxRenderDepth {
		return nil, fmt.Errorf("docparse: OMML nesting exceeds %d", ommlMaxRenderDepth)
	}
	node := &ommlNode{name: start.Name.Local, attrs: make(map[string]string, len(start.Attr))}
	for _, attr := range start.Attr {
		node.attrs[attr.Name.Local] = attr.Value
	}
	var text strings.Builder
	for {
		token, err := decoder.Token()
		if err != nil {
			return nil, err
		}
		switch value := token.(type) {
		case xml.StartElement:
			child, childErr := parseOMMLNode(decoder, value, depth+1)
			if childErr != nil {
				return nil, childErr
			}
			node.children = append(node.children, child)
		case xml.CharData:
			text.Write(value)
		case xml.EndElement:
			if value.Name == start.Name {
				node.text = text.String()
				return node, nil
			}
		}
	}
}

// renderOMMLNode 把节点递归渲染为 LaTeX；达到深度上限时只保留直接文本。
func renderOMMLNode(node *ommlNode, depth int) string {
	if node == nil {
		return ""
	}
	if depth > ommlMaxRenderDepth {
		return escapeOMMLText(node.text)
	}
	render := func(name string) string { return renderOMMLNode(ommlFirstChild(node, name), depth+1) }
	switch node.name {
	case "t":
		return escapeOMMLText(node.text)
	case "f":
		return `\frac{` + render("num") + `}{` + render("den") + `}`
	case "sSub":
		return `{` + render("e") + `}_{` + render("sub") + `}`
	case "sSup":
		return `{` + render("e") + `}^{` + render("sup") + `}`
	case "sSubSup":
		return `{` + render("e") + `}_{` + render("sub") + `}^{` + render("sup") + `}`
	case "sPre":
		return `{}_{` + render("sub") + `}^{` + render("sup") + `}{` + render("e") + `}`
	case "rad":
		base := render("e")
		degree := render("deg")
		if degree == "" || ommlPropertyBool(node, "radPr", "degHide") {
			return `\sqrt{` + base + `}`
		}
		return `\sqrt[` + degree + `]{` + base + `}`
	case "nary":
		operator := ommlNaryOperator(ommlPropertyValue(node, "naryPr", "chr", "∫"))
		return operator + ommlLimits(render("sub"), render("sup")) + `{` + render("e") + `}`
	case "d":
		begin := ommlDelimiter(ommlPropertyValue(node, "dPr", "begChr", "("))
		end := ommlDelimiter(ommlPropertyValue(node, "dPr", "endChr", ")"))
		separator := ommlPropertyValue(node, "dPr", "sepChr", "|")
		parts := ommlChildrenNamed(node, "e", depth+1)
		return `\left` + begin + strings.Join(parts, escapeOMMLText(separator)) + `\right` + end
	case "m":
		rows := make([]string, 0, len(node.children))
		for _, row := range node.children {
			if row.name == "mr" {
				rows = append(rows, strings.Join(ommlChildrenNamed(row, "e", depth+1), " & "))
			}
		}
		return `\begin{matrix}` + strings.Join(rows, ` \\ `) + `\end{matrix}`
	case "eqArr":
		return `\begin{aligned}` + strings.Join(ommlChildrenNamed(node, "e", depth+1), ` \\ `) + `\end{aligned}`
	case "limLow":
		return `{` + render("e") + `}_{` + render("lim") + `}`
	case "limUpp":
		return `{` + render("e") + `}^{` + render("lim") + `}`
	case "func":
		name := strings.TrimSpace(render("fName"))
		if name == "" {
			return render("e")
		}
		return `\operatorname{` + name + `}{` + render("e") + `}`
	case "bar":
		command := `\overline`
		if strings.EqualFold(ommlPropertyValue(node, "barPr", "pos", "top"), "bot") {
			command = `\underline`
		}
		return command + `{` + render("e") + `}`
	case "acc":
		command := ommlAccentCommand(ommlPropertyValue(node, "accPr", "chr", "ˆ"))
		return command + `{` + render("e") + `}`
	case "groupChr":
		position := ommlPropertyValue(node, "groupChrPr", "pos", "bot")
		character := ommlPropertyValue(node, "groupChrPr", "chr", "⏟")
		command := `\underbrace`
		if strings.EqualFold(position, "top") || character == "⏞" {
			command = `\overbrace`
		}
		return command + `{` + render("e") + `}`
	case "borderBox":
		return `\boxed{` + render("e") + `}`
	case "phant":
		return `\phantom{` + render("e") + `}`
	case "brk":
		return `\\`
	}
	if strings.HasSuffix(node.name, "Pr") || ommlPropertyOnlyNode(node.name) {
		return ""
	}
	return renderOMMLChildren(node, depth+1)
}

// renderOMMLChildren 按原顺序拼接全部子节点及直接文本。
func renderOMMLChildren(node *ommlNode, depth int) string {
	var result strings.Builder
	if strings.TrimSpace(node.text) != "" {
		result.WriteString(escapeOMMLText(node.text))
	}
	for _, child := range node.children {
		result.WriteString(renderOMMLNode(child, depth))
	}
	return result.String()
}

// ommlFirstChild 返回第一个指定本地名的直接子节点。
func ommlFirstChild(node *ommlNode, name string) *ommlNode {
	if node == nil {
		return nil
	}
	for _, child := range node.children {
		if child.name == name {
			return child
		}
	}
	return nil
}

// ommlChildrenNamed 渲染全部指定本地名的直接子节点。
func ommlChildrenNamed(node *ommlNode, name string, depth int) []string {
	result := make([]string, 0, len(node.children))
	for _, child := range node.children {
		if child.name == name {
			result = append(result, renderOMMLNode(child, depth))
		}
	}
	return result
}

// ommlPropertyValue 读取 property/child@m:val，不存在时返回默认值。
func ommlPropertyValue(node *ommlNode, property, childName, fallback string) string {
	propertyNode := ommlFirstChild(node, property)
	valueNode := ommlFirstChild(propertyNode, childName)
	if valueNode == nil {
		return fallback
	}
	if value := strings.TrimSpace(valueNode.attrs["val"]); value != "" {
		return value
	}
	return fallback
}

// ommlPropertyBool 读取 property/child 的 OOXML on/off 值。
func ommlPropertyBool(node *ommlNode, property, childName string) bool {
	propertyNode := ommlFirstChild(node, property)
	valueNode := ommlFirstChild(propertyNode, childName)
	if valueNode == nil {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(valueNode.attrs["val"])) {
	case "0", "false", "off":
		return false
	default:
		return true
	}
}

// ommlLimits 生成大型运算符的上下限。
func ommlLimits(subscript, superscript string) string {
	var result strings.Builder
	if subscript != "" {
		result.WriteString("_{" + subscript + "}")
	}
	if superscript != "" {
		result.WriteString("^{" + superscript + "}")
	}
	return result.String()
}

// ommlNaryOperator 把常见 OMML 大型运算符映射为 LaTeX 命令。
func ommlNaryOperator(value string) string {
	operators := map[string]string{
		"∫": `\int`, "∬": `\iint`, "∭": `\iiint`, "∮": `\oint`,
		"∑": `\sum`, "∏": `\prod`, "∐": `\coprod`,
		"⋂": `\bigcap`, "⋃": `\bigcup`, "⨀": `\bigodot`, "⨂": `\bigotimes`, "⨁": `\bigoplus`,
	}
	if operator := operators[value]; operator != "" {
		return operator
	}
	return escapeOMMLText(value)
}

// ommlDelimiter 把花括号等定界符转为可用于 left/right 的形式。
func ommlDelimiter(value string) string {
	switch value {
	case "{":
		return `\{`
	case "}":
		return `\}`
	case "":
		return "."
	default:
		return value
	}
}

// ommlAccentCommand 把常见重音字符映射为 LaTeX 命令。
func ommlAccentCommand(value string) string {
	switch value {
	case "¯", "̅":
		return `\bar`
	case "→", "⃗":
		return `\vec`
	case "˙", "̇":
		return `\dot`
	case "¨", "̈":
		return `\ddot`
	case "˜", "~":
		return `\tilde`
	default:
		return `\hat`
	}
}

// ommlPropertyOnlyNode 判断不应作为可见公式内容输出的属性叶节点。
func ommlPropertyOnlyNode(name string) bool {
	switch name {
	case "chr", "pos", "begChr", "endChr", "sepChr", "type", "grow", "degHide", "subHide", "supHide", "ctrlPr", "rPr":
		return true
	default:
		return false
	}
}

// escapeOMMLText 转义 LaTeX 特殊字符并映射常见 Unicode 数学符号。
func escapeOMMLText(value string) string {
	var result strings.Builder
	for _, current := range value {
		switch current {
		case '_':
			result.WriteString(`\_`)
		case '#':
			result.WriteString(`\#`)
		case '%':
			result.WriteString(`\%`)
		case '&':
			result.WriteString(`\&`)
		case '$':
			result.WriteString(`\$`)
		case '{':
			result.WriteString(`\{`)
		case '}':
			result.WriteString(`\}`)
		case '\\':
			result.WriteString(`\backslash{}`)
		case 'α':
			result.WriteString(`\alpha `)
		case 'β':
			result.WriteString(`\beta `)
		case 'γ':
			result.WriteString(`\gamma `)
		case 'δ':
			result.WriteString(`\delta `)
		case 'θ':
			result.WriteString(`\theta `)
		case 'λ':
			result.WriteString(`\lambda `)
		case 'μ':
			result.WriteString(`\mu `)
		case 'π':
			result.WriteString(`\pi `)
		case 'σ':
			result.WriteString(`\sigma `)
		case 'φ':
			result.WriteString(`\phi `)
		case 'ω':
			result.WriteString(`\omega `)
		case 'Γ':
			result.WriteString(`\Gamma `)
		case 'Δ':
			result.WriteString(`\Delta `)
		case 'Θ':
			result.WriteString(`\Theta `)
		case 'Λ':
			result.WriteString(`\Lambda `)
		case 'Π':
			result.WriteString(`\Pi `)
		case 'Σ':
			result.WriteString(`\Sigma `)
		case 'Φ':
			result.WriteString(`\Phi `)
		case 'Ω':
			result.WriteString(`\Omega `)
		case '≤':
			result.WriteString(`\leq `)
		case '≥':
			result.WriteString(`\geq `)
		case '≠':
			result.WriteString(`\neq `)
		case '≈':
			result.WriteString(`\approx `)
		case '±':
			result.WriteString(`\pm `)
		case '×':
			result.WriteString(`\times `)
		case '÷':
			result.WriteString(`\div `)
		case '∞':
			result.WriteString(`\infty `)
		case '∂':
			result.WriteString(`\partial `)
		case '∇':
			result.WriteString(`\nabla `)
		case '∈':
			result.WriteString(`\in `)
		case '∉':
			result.WriteString(`\notin `)
		default:
			result.WriteRune(current)
		}
	}
	return result.String()
}

// svg.go 实现 SVG 输入解析为 DoclingDocument：SVG 本质是 XML 文本，
// 解析时单遍完成三件事——安全消毒（剥离 script/foreignObject/事件属性/
// 外链引用，产出物可安全进入前端预览）、文本提取（text/tspan/title/desc
// 按文档顺序生成正文条目，供 LLM 与检索直接使用，无需多模态）、净化
// 重写（输出剥离危险内容后的 SVG 作为 PictureItem data URI 保留预览）。
// 纯图形 SVG 无可提取文本时，可回调 OCRHook 由调用方用文本模型描述。
package docling

import (
	"bytes"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// defaultSVGDPI 是 SVG 未携带物理分辨率时使用的 CSS 像素基准 DPI。
const defaultSVGDPI int64 = 96

// defaultSVGWidth / defaultSVGHeight 是 width/height 与 viewBox 均缺失时
// 使用的 SVG 规范默认尺寸。
const (
	defaultSVGWidth  = 300.0
	defaultSVGHeight = 150.0
)

// ParseSVG 解析 SVG 字节为 DoclingDocument（无 OCR 钩子的便捷版）。
func ParseSVG(data []byte) (*DoclingDocument, error) {
	return ParseSVGWithOptions(data, PDFOptions{})
}

// ParseSVGWithOptions 解析 SVG 字节为单页 DoclingDocument：根元素必须是
// <svg>（允许 XML 声明与注释前导）；文本条目来自 text/tspan/title/desc，
// 始终产出 1 个携带净化 SVG data URI 的 PictureItem。无可提取文本且配置
// OCR 钩子时以 pageNo=1 回调，失败或无效不影响文档返回。
func ParseSVGWithOptions(data []byte, opt PDFOptions) (*DoclingDocument, error) {
	if opt.MIMEType == "" {
		opt.MIMEType = "image/svg+xml"
	}
	var (
		cleanBuf    bytes.Buffer
		encoder     = xml.NewEncoder(&cleanBuf)
		rootSeen    bool
		width       float64
		height      float64
		hasSize     bool
		texts       []string
		title       string
		desc        string
		collectKind string // 当前正在收集字符的元素类型：text/title/desc
		varCollect  strings.Builder
	)
	decoder := xml.NewDecoder(bytes.NewReader(data))
	decoder.Strict = false
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("docling: SVG 不是合法 XML: %w", err)
		}
		switch t := token.(type) {
		case xml.StartElement:
			if !rootSeen {
				if t.Name.Local != "svg" {
					return nil, fmt.Errorf("docling: SVG 根元素必须是 <svg>，实际是 <%s>", t.Name.Local)
				}
				rootSeen = true
				width, height = svgRootSize(t.Attr)
				hasSize = width > 0 && height > 0
				if !hasSize {
					width, height = defaultSVGWidth, defaultSVGHeight
				}
			}
			// 危险子树整体剥离：script 含可执行脚本，foreignObject 可内嵌
			// 任意 HTML（含事件与外链），净化产物中均不允许出现。
			if t.Name.Local == "script" || t.Name.Local == "foreignObject" {
				if err := decoder.Skip(); err != nil {
					return nil, fmt.Errorf("docling: SVG 剥离危险元素失败: %w", err)
				}
				continue
			}
			switch t.Name.Local {
			case "text":
				if collectKind == "" {
					collectKind = "text"
					varCollect.Reset()
				}
			case "title":
				if collectKind == "" && title == "" {
					collectKind = "title"
					varCollect.Reset()
				}
			case "desc":
				if collectKind == "" && desc == "" {
					collectKind = "desc"
					varCollect.Reset()
				}
			}
			if err := encoder.EncodeToken(xml.StartElement{Name: t.Name, Attr: sanitizeSVGAttrs(t.Attr)}); err != nil {
				return nil, fmt.Errorf("docling: SVG 净化重写失败: %w", err)
			}
		case xml.CharData:
			if collectKind != "" {
				varCollect.Write(t)
			}
			if err := encoder.EncodeToken(t.Copy()); err != nil {
				return nil, fmt.Errorf("docling: SVG 净化重写失败: %w", err)
			}
		case xml.EndElement:
			switch {
			case t.Name.Local == collectKind:
				collected := strings.TrimSpace(varCollect.String())
				switch collectKind {
				case "text":
					if collected != "" {
						texts = append(texts, collected)
					}
				case "title":
					title = collected
				case "desc":
					desc = collected
				}
				collectKind = ""
			case collectKind == "text" && t.Name.Local == "tspan":
				// tspan 是 text 的行内子元素，结束不终止整段收集
			}
			if err := encoder.EncodeToken(xml.EndElement{Name: t.Name}); err != nil {
				return nil, fmt.Errorf("docling: SVG 净化重写失败: %w", err)
			}
		default:
			// XML 声明、DOCTYPE 与注释不进入净化产物：data URI 内无需声明，
			// DOCTYPE 携带实体定义存在解析风险，一并剥离
		}
	}
	if err := encoder.Flush(); err != nil {
		return nil, fmt.Errorf("docling: SVG 净化重写失败: %w", err)
	}
	if !rootSeen {
		return nil, fmt.Errorf("docling: SVG 未找到 <svg> 根元素")
	}

	doc := NewDoclingDocument("svg")
	doc.AddPage(1, width, height)
	prov := []ProvenanceItem{{PageNo: 1}}
	// 文档顺序：标题 → 描述 → 正文文本；prov 仅标注页号，SVG 文本坐标
	// 是基线锚点且可能被 transform 影响，构造 bbox 不可靠，不提供。
	if title != "" {
		doc.AddTitle(title, prov, nil)
	}
	if desc != "" {
		doc.AddText(LabelCaption, desc, prov, nil)
	}
	for _, text := range texts {
		doc.AddText(LabelText, text, prov, nil)
	}
	clean := cleanBuf.Bytes()
	if len(bytes.TrimSpace(clean)) > 0 && len(clean) <= maxMediaDataURIBytes {
		doc.AddPicture(&ImageRef{
			Mimetype: "image/svg+xml",
			Dpi:      defaultSVGDPI,
			Size:     &ImageSize{Width: width, Height: height},
			URI:      "data:image/svg+xml;base64," + base64.StdEncoding.EncodeToString(clean),
		}, []ProvenanceItem{{
			PageNo: 1,
			BBox:   &DoclingBBox{L: 0, T: 0, R: width, B: height, CoordOrigin: CoordOriginTopLeft},
		}}, nil)
	}
	doc.Meta = &DocMeta{PageCount: 1}

	if len(texts) > 0 || title != "" || desc != "" || !hasOCRHook(opt) {
		return doc, nil
	}
	// 纯图形 SVG：无可提取文本时交给调用方识别（SVG 源码是文本，调用方可
	// 直接交给文本模型而非视觉模型），结果经 Markdown 结构化并入文档。
	ocrText, ok := runOCR(opt, OCRRequest{PageNo: 1, Data: data})
	if !ok {
		return doc, nil
	}
	if sub, subErr := ParseMarkdown([]byte(ocrText)); subErr == nil &&
		len(sub.Texts)+len(sub.Tables) > 0 {
		mergeOCRSubDocument(doc, sub, 1)
	}
	return doc, nil
}

// sanitizeSVGAttrs 剥离事件属性与外部引用：on* 属性是可执行脚本入口，
// href/xlink:href 指向文档外（不以 # 开头）时构成外部请求面。
func sanitizeSVGAttrs(attrs []xml.Attr) []xml.Attr {
	out := make([]xml.Attr, 0, len(attrs))
	for _, attr := range attrs {
		local := strings.ToLower(attr.Name.Local)
		isEvent := strings.HasPrefix(local, "on")
		isExternalRef := local == "href" && !strings.HasPrefix(strings.TrimSpace(attr.Value), "#")
		if isEvent || isExternalRef {
			continue
		}
		out = append(out, attr)
	}
	return out
}

// svgRootSize 从 <svg> 根元素属性解析宽高：优先 width/height（支持
// px/pt/in/cm/mm 单位换算到 CSS 像素），缺失或百分比时回退 viewBox；
// 都无法解析时返回 0,0 由调用方使用规范默认尺寸。
func svgRootSize(attrs []xml.Attr) (float64, float64) {
	var width, height float64
	for _, attr := range attrs {
		switch strings.ToLower(attr.Name.Local) {
		case "width":
			width, _ = parseSVGLength(attr.Value)
		case "height":
			height, _ = parseSVGLength(attr.Value)
		case "viewbox":
			if width <= 0 || height <= 0 {
				fields := strings.Fields(attr.Value)
				if len(fields) == 4 {
					if w, err := strconv.ParseFloat(fields[2], 64); err == nil {
						width = w
					}
					if h, err := strconv.ParseFloat(fields[3], 64); err == nil {
						height = h
					}
				}
			}
		}
	}
	if width <= 0 || height <= 0 {
		return 0, 0
	}
	return width, height
}

// parseSVGLength 解析 SVG 长度属性为 CSS 像素；支持无单位与 px/pt/in/
// cm/mm/pc，百分比与其他相对单位返回 false。
func parseSVGLength(value string) (float64, bool) {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" || strings.HasSuffix(value, "%") {
		return 0, false
	}
	unit := ""
	for _, candidate := range []string{"px", "pt", "in", "cm", "mm", "pc"} {
		if strings.HasSuffix(value, candidate) {
			unit = candidate
			value = strings.TrimSpace(strings.TrimSuffix(value, candidate))
			break
		}
	}
	number, err := strconv.ParseFloat(value, 64)
	if err != nil || number < 0 {
		return 0, false
	}
	switch unit {
	case "", "px":
		return number, true
	case "pt":
		return number * 96 / 72, true
	case "in":
		return number * 96, true
	case "cm":
		return number * 96 / 2.54, true
	case "mm":
		return number * 96 / 25.4, true
	case "pc":
		return number * 16, true
	default:
		return 0, false
	}
}

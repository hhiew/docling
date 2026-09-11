// media.go 实现 OOXML 容器（docx/pptx/xlsx）共用的媒体资产与文档元数据提取：
//   - mediaToDataURI 把 zip 内图片字节封装为 data URI（内嵌 ImageRef.URI），
//     单图超过 maxMediaDataURIBytes 时返回空串（URI 留空，避免内存放大）；
//   - parseOOXMLRelationships 由 pptx.go 提供的通用关系解析，本文件的
//     ooxmlCorePropsMeta 借助它定位图片目标部件；
//   - parseOOXMLCoreProps 解析 docProps/core.xml 为文档级元数据 DocMeta。
//
// 三个 OOXML 后端（docx/pptx/xlsx）共用，避免各后端重复实现。
package docparse

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"encoding/xml"
	"strings"
)

// officeRelNS OOXML relationships 属性命名空间（r:embed/r:id 等引用属性）。
const officeRelNS = "http://schemas.openxmlformats.org/officeDocument/2006/relationships"

// maxMediaDataURIBytes 单图内嵌 data URI 的原始字节上限：超过则跳过 URI 填充
// （URI 留空），避免 base64 膨胀（约 1.33 倍）导致产物体积与内存放大。
const maxMediaDataURIBytes = 8 << 20 // 8MB

// mediaToDataURI 把媒体文件字节封装为 data URI；ext 为媒体文件扩展名
// （含点，如 ".png"），决定 MIME 类型。超过 maxMediaDataURIBytes 或空字节
// 返回空串（调用方保持 URI 为空），未知扩展名按 application/octet-stream。
func mediaToDataURI(ext string, data []byte) string {
	if len(data) == 0 || len(data) > maxMediaDataURIBytes {
		return ""
	}
	mimeType := ooxmlMediaMime(ext)
	var b strings.Builder
	b.Grow(len("data:;base64,") + len(mimeType) + base64.StdEncoding.EncodedLen(len(data)))
	b.WriteString("data:")
	b.WriteString(mimeType)
	b.WriteString(";base64,")
	enc := base64.NewEncoder(base64.StdEncoding, &b)
	_, _ = enc.Write(data)
	_ = enc.Close()
	return b.String()
}

// ooxmlMediaMime OOXML 媒体文件扩展名 → MIME 类型；未知扩展名回退
// application/octet-stream。ext 允许带或不带前导点，大小写不敏感。
func ooxmlMediaMime(ext string) string {
	switch strings.ToLower(strings.TrimPrefix(ext, ".")) {
	case "png":
		return "image/png"
	case "jpg", "jpeg":
		return "image/jpeg"
	case "gif":
		return "image/gif"
	case "bmp":
		return "image/bmp"
	case "webp":
		return "image/webp"
	case "tif", "tiff":
		return "image/tiff"
	case "svg":
		return "image/svg+xml"
	case "emf":
		return "image/emf"
	case "wmf":
		return "image/wmf"
	default:
		return "application/octet-stream"
	}
}

// ooxmlCorePropsMeta 从 OOXML zip 容器读取 docProps/core.xml 并解析为
// DocMeta；部件缺失或读取失败返回 nil（元数据尽力而为，不影响主解析）。
func ooxmlCorePropsMeta(reader *zip.Reader) *DocMeta {
	raw, err := readZipFileBytes(reader, "docProps/core.xml")
	if err != nil {
		return nil
	}
	return parseOOXMLCoreProps(raw)
}

// parseOOXMLCoreProps 解析 OOXML docProps/core.xml（DC/DCTERMS 命名空间）为
// DocMeta：dc:title/dc:creator/dc:subject/dc:language/dcterms:created；
// 畸形 XML 返回已解析部分，全部字段为空时返回 nil（避免空 meta 对象序列化）。
func parseOOXMLCoreProps(xmlBytes []byte) *DocMeta {
	if len(xmlBytes) == 0 {
		return nil
	}
	var meta DocMeta
	dec := xml.NewDecoder(bytes.NewReader(xmlBytes))
	var inTitle, inCreator, inSubject, inLanguage, inCreated bool
	for {
		tok, err := dec.Token()
		if err != nil {
			break // 含 io.EOF 与畸形 XML：已解析字段照常返回
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "title":
				inTitle = true
			case "creator":
				inCreator = true
			case "subject":
				inSubject = true
			case "language":
				inLanguage = true
			case "created":
				inCreated = true
			}
		case xml.EndElement:
			switch t.Name.Local {
			case "title":
				inTitle = false
			case "creator":
				inCreator = false
			case "subject":
				inSubject = false
			case "language":
				inLanguage = false
			case "created":
				inCreated = false
			}
		case xml.CharData:
			switch {
			case inTitle:
				meta.Title += string(t)
			case inCreator:
				meta.Author += string(t)
			case inSubject:
				meta.Subject += string(t)
			case inLanguage:
				meta.Language += string(t)
			case inCreated:
				meta.CreatedAt += string(t)
			}
		}
	}
	if meta == (DocMeta{}) {
		return nil
	}
	return &meta
}

// xmlAttrNS 按属性命名空间 URL 与 local 名取属性值，不存在时返回空串。
func xmlAttrNS(attrs []xml.Attr, space, local string) string {
	for _, a := range attrs {
		if a.Name.Local == local && a.Name.Space == space {
			return a.Value
		}
	}
	return ""
}

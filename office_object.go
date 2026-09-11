// office_object.go 统一保存 OOXML 复杂对象的语义、关系来源和可选预览。
// SmartArt、艺术字与嵌入对象均按 Docling 1.10 的 PictureItem 表示；
// 非官方信息只写入带 docling__ 前缀的 PictureMeta 扩展字段。
package docling

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"io"
	"path"
	"strings"
)

// officeObjectRecord 是 OOXML 复杂对象的中间表示。
type officeObjectRecord struct {
	kind         string
	name         string
	text         string
	relationship string
	target       string
	program      string
	geometry     string
	preview      *ImageRef
}

// officeObjectClassName 返回复杂对象在图片分类元数据中的稳定类别。
func officeObjectClassName(kind string) string {
	switch strings.ToLower(kind) {
	case "smartart":
		return "smartart"
	case "wordart":
		return "wordart"
	case "shape":
		return "shape"
	case "ole", "embedded_object":
		return "embedded_object"
	default:
		return "office_object"
	}
}

// officeObjectCaption 生成用于检索的对象说明，优先保留对象中的可见文本。
func officeObjectCaption(record officeObjectRecord) string {
	if text := strings.TrimSpace(record.text); text != "" {
		return text
	}
	if name := strings.TrimSpace(record.name); name != "" {
		return name
	}
	if record.program != "" {
		return record.program
	}
	if record.target != "" {
		return path.Base(record.target)
	}
	return officeObjectClassName(record.kind)
}

// officeObjectExtra 把 OOXML 来源字段编码为命名空间扩展，避免污染官方协议。
func officeObjectExtra(record officeObjectRecord) map[string]json.RawMessage {
	extra := map[string]json.RawMessage{}
	values := map[string]string{
		"docling__office_object_type":         officeObjectClassName(record.kind),
		"docling__office_object_name":         record.name,
		"docling__office_object_relationship": record.relationship,
		"docling__office_object_target":       record.target,
		"docling__office_object_program":      record.program,
		"docling__office_object_geometry":     record.geometry,
	}
	for key, value := range values {
		if value == "" {
			continue
		}
		encoded, err := json.Marshal(value)
		if err == nil {
			extra[key] = encoded
		}
	}
	return extra
}

// addOfficeObjectPicture 把复杂 Office 对象追加为官方 PictureItem，并把
// 可见语义放入 caption 引用，保证 content_list 与分块链路可以检索。
func addOfficeObjectPicture(doc *DoclingDocument, record officeObjectRecord, prov []ProvenanceItem, parent *RefItem, layer ContentLayer) RefItem {
	confidence := 1.0
	if record.preview == nil {
		record.preview = renderOfficeObjectSVG(record)
	}
	ref := doc.AddPicture(record.preview, prov, parent)
	picture := &doc.Pictures[ref.Idx]
	picture.ContentLayer = layer
	picture.Meta = &PictureMeta{
		Classification: &PictureClassificationMetaField{Predictions: []PictureClassificationPrediction{{
			PredictionMeta: PredictionMeta{Confidence: &confidence, CreatedBy: "docling-ooxml"},
			ClassName:      officeObjectClassName(record.kind),
		}}},
		Extra: officeObjectExtra(record),
	}
	caption := officeObjectCaption(record)
	if caption == "" {
		return ref
	}
	captionRef := RefItem{Kind: refTexts, Idx: int64(len(doc.Texts))}
	doc.Texts = append(doc.Texts, TextItem{
		SelfRef: captionRef.String(), Parent: &ref, Children: []RefItem{},
		ContentLayer: layer, Label: LabelCaption, Prov: []ProvenanceItem{},
		Orig: caption, Text: caption,
	})
	picture.Captions = append(picture.Captions, captionRef)
	return ref
}

// collectDrawingMLText 提取 DrawingML/DiagramML 子树内的可见文本并去重。
func collectDrawingMLText(data []byte) string {
	parts := collectXMLTextByLocalNames(data, map[string]bool{"t": true})
	seen := map[string]bool{}
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" || seen[part] {
			continue
		}
		seen[part] = true
		result = append(result, part)
	}
	return strings.Join(result, "\n")
}

// collectXMLTextByLocalNames 收集指定 local name 元素中的字符数据。
func collectXMLTextByLocalNames(data []byte, names map[string]bool) []string {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	var (
		active string
		parts  []string
	)
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			return parts
		}
		if err != nil {
			return parts
		}
		switch value := token.(type) {
		case xml.StartElement:
			if names[value.Name.Local] {
				active = value.Name.Local
			}
		case xml.EndElement:
			if value.Name.Local == active {
				active = ""
			}
		case xml.CharData:
			if active != "" {
				parts = append(parts, string(value))
			}
		}
	}
}

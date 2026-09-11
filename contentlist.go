// contentlist.go 实现 DoclingDocument（详细 JSON）到 content_list（简化 JSON）
// 的统一简化器，并附带 ParseDoclingDocument 兼容读取入口。
//
// 简化语义（上游转换层与简化层两路收敛）：
//   - body 树先序遍历产出扁平 Item；标题节点更新章节路径栈，
//     元素的 _section_path = 祖先标题链（Docling 产物因此获得章节路径增强）；
//   - label → 模态映射：title/section_header→text+TextLevel、formula→equation、
//     code/checkbox/paragraph/list_item/text→text、table→table、picture→image；
//   - 细粒度 label 透传：page_header/page_footer/document_index/list_item/code/
//     checkbox/caption 填充 Item.Label（formula 由 type=equation 表达不填），
//     供下游过滤/分流；
//   - prov 取首段：page_no-1 对齐 golight 的 0-based PageIdx 基准（行为变更点，
//     旧 Docling 路径曾直通 1-based page_no）；bbox 归一化为 top<=bottom，
//     保持历史 content_list 不携带 coord_origin 的协议可稳定求并集；
//   - 表格按 cell offset 铺格子渲染为 GFM Markdown，并做表格后处理清洗。
package docling

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
)

// 解析器来源标记，写入 Item.Source，供下游区分 go_light 与 Docling 产物
// （如多模态分流按 SourceDocling 判断）。
const (
	// SourceGolight Go 本地轻量解析（docling 各后端）。
	SourceGolight = "golight"
	// SourceDocling Docling 高级解析（docling-serve HTTP）。
	SourceDocling = "docling"
)

// ParseDoclingDocument 解析 DoclingDocument JSON（新版 docling-core 形态），
// 并兼容线上 docling-serve v1.21 的旧版差异：
//   - 旧版标题 label 为 "heading-N"，归一化为 section_header + TextLevel=N；
//   - 旧版公式 LaTeX 存独立 latex 字段，归一化到 Text/Formula 消费语义。
func ParseDoclingDocument(raw json.RawMessage) (*DoclingDocument, error) {
	if len(raw) == 0 {
		return nil, fmt.Errorf("docling: docling json 为空")
	}
	// docling-serve 可能把 json_content 序列化为字符串，兼容剥一层
	trimmed := raw
	var strDoc string
	if err := json.Unmarshal(raw, &strDoc); err == nil && strings.TrimSpace(strDoc) != "" {
		trimmed = json.RawMessage(strDoc)
	}
	doc := &DoclingDocument{}
	if err := json.Unmarshal(trimmed, doc); err != nil {
		return nil, fmt.Errorf("docling: 解析 docling json 失败: %w", err)
	}
	normalizeDoclingDocument(doc)
	return doc, nil
}

// normalizeDoclingDocument 归一化旧版 schema 差异：
//   - 旧版标题 label 形如 "heading-1"/"heading_1"/"heading1"，归一化
//     section_header + TextLevel；
//   - 旧版公式 LaTeX 存独立 latex 字段（TextItem.Latex），简化时优先取用；
//   - 旧版表格可能缺 num_rows/num_cols，从 cell 偏移包络推导。
func normalizeDoclingDocument(doc *DoclingDocument) {
	if doc.SchemaName == "" {
		doc.SchemaName = DoclingSchemaName
	}
	if doc.Version == "" {
		doc.Version = DoclingSchemaVersion
	}
	if doc.Body == nil {
		doc.Body = NewDoclingDocument(doc.Name).Body
	} else {
		doc.Body.SelfRef = "#/body"
		doc.Body.ContentLayer = LayerBody
		if doc.Body.Label == "" {
			doc.Body.Label = GroupLabelUnspecified
		}
		if doc.Body.Name == "" {
			doc.Body.Name = "_root_"
		}
		if doc.Body.Children == nil {
			doc.Body.Children = []RefItem{}
		}
	}
	if doc.Furniture == nil {
		doc.Furniture = NewDoclingDocument(doc.Name).Furniture
	} else {
		// 旧版 Go 产物曾错误写成 body；furniture 根节点按官方协议修正。
		doc.Furniture.SelfRef = "#/furniture"
		doc.Furniture.ContentLayer = LayerFurniture
		if doc.Furniture.Label == "" {
			doc.Furniture.Label = GroupLabelUnspecified
		}
		if doc.Furniture.Name == "" {
			doc.Furniture.Name = "_root_"
		}
		if doc.Furniture.Children == nil {
			doc.Furniture.Children = []RefItem{}
		}
	}
	if doc.Groups == nil {
		doc.Groups = []GroupItem{}
	}
	if doc.Texts == nil {
		doc.Texts = []TextItem{}
	}
	if doc.Pictures == nil {
		doc.Pictures = []PictureItem{}
	}
	if doc.Tables == nil {
		doc.Tables = []TableItem{}
	}
	if doc.KeyValueItems == nil {
		doc.KeyValueItems = []json.RawMessage{}
	}
	if doc.FormItems == nil {
		doc.FormItems = []json.RawMessage{}
	}
	if doc.Pages == nil {
		doc.Pages = map[string]PageItem{}
	}
	for i := range doc.Texts {
		item := &doc.Texts[i]
		item.SelfRef = fmt.Sprintf("#/texts/%d", i)
		if item.Children == nil {
			item.Children = []RefItem{}
		}
		if item.ContentLayer == "" {
			item.ContentLayer = LayerBody
		}
		item.Prov = nonNilProvenance(item.Prov)
		if strings.HasPrefix(string(item.Label), "heading") {
			// 剥前缀与分隔符（兼容连字符/下划线/紧贴写法）后解析层级
			levelStr := strings.TrimPrefix(string(item.Label), "heading")
			levelStr = strings.TrimLeft(strings.TrimSpace(levelStr), "-_ ")
			var level int64
			if _, err := fmt.Sscanf(levelStr, "%d", &level); err == nil && level > 0 {
				item.Label = LabelSectionHeader
				item.TextLevel = level
			}
		}
		if item.Text == "" && item.Orig != "" {
			item.Text = item.Orig
		}
		if item.Orig == "" && item.Text != "" {
			item.Orig = item.Text
		}
	}
	for i := range doc.Groups {
		doc.Groups[i].SelfRef = fmt.Sprintf("#/groups/%d", i)
		if doc.Groups[i].Children == nil {
			doc.Groups[i].Children = []RefItem{}
		}
		if doc.Groups[i].ContentLayer == "" {
			doc.Groups[i].ContentLayer = LayerBody
		}
		if doc.Groups[i].Label == "" {
			doc.Groups[i].Label = GroupLabelUnspecified
		}
		if doc.Groups[i].Name == "" {
			doc.Groups[i].Name = "group"
		}
	}
	for i := range doc.Tables {
		item := &doc.Tables[i]
		item.SelfRef = fmt.Sprintf("#/tables/%d", i)
		if item.Children == nil {
			item.Children = []RefItem{}
		}
		if item.ContentLayer == "" {
			item.ContentLayer = LayerBody
		}
		if item.Label == "" {
			item.Label = LabelTable
		}
		item.Prov = nonNilProvenance(item.Prov)
		if item.Captions == nil {
			item.Captions = []RefItem{}
		}
		if item.References == nil {
			item.References = []RefItem{}
		}
		if item.Footnotes == nil {
			item.Footnotes = []RefItem{}
		}
		migrateTableAnnotations(item)
		ensureLegacyCaption(doc, item.Caption, item.ContentLayer, item.Parent, &item.Captions)
		item.Caption = ""
		data := item.Data
		if data == nil {
			item.Data = &TableData{TableCells: []DoclingTableCell{}}
			data = item.Data
		}
		if data.NumRows <= 0 || data.NumCols <= 0 {
			for _, c := range data.TableCells {
				if c.EndRowOffsetIdx > data.NumRows {
					data.NumRows = c.EndRowOffsetIdx
				}
				if c.EndColOffsetIdx > data.NumCols {
					data.NumCols = c.EndColOffsetIdx
				}
			}
		}
	}
	for i := range doc.Pictures {
		item := &doc.Pictures[i]
		item.SelfRef = fmt.Sprintf("#/pictures/%d", i)
		if item.Children == nil {
			item.Children = []RefItem{}
		}
		if item.ContentLayer == "" {
			item.ContentLayer = LayerBody
		}
		if item.Label == "" {
			item.Label = LabelPicture
		}
		item.Prov = nonNilProvenance(item.Prov)
		if item.Label == LabelChart {
			item.Label = LabelPicture
		}
		if item.Captions == nil {
			item.Captions = []RefItem{}
		}
		if item.References == nil {
			item.References = []RefItem{}
		}
		if item.Footnotes == nil {
			item.Footnotes = []RefItem{}
		}
		migratePictureAnnotations(item)
		ensureLegacyCaption(doc, item.Caption, item.ContentLayer, item.Parent, &item.Captions)
		item.Caption = ""
	}
	for key, page := range doc.Pages {
		if page.PageNo == 0 {
			var pageNo int64
			if _, err := fmt.Sscanf(key, "%d", &pageNo); err == nil {
				page.PageNo = pageNo
			}
		}
		doc.Pages[key] = page
	}
}

// ensureLegacyCaption 把旧字符串 caption 转为官方 captions 文本引用。
func ensureLegacyCaption(doc *DoclingDocument, caption string, layer ContentLayer, parent *RefItem, captions *[]RefItem) {
	caption = strings.TrimSpace(caption)
	if caption == "" || len(*captions) > 0 {
		return
	}
	item := TextItem{
		SelfRef:      fmt.Sprintf("#/texts/%d", len(doc.Texts)),
		Parent:       parent,
		Children:     []RefItem{},
		ContentLayer: layer,
		Label:        LabelCaption,
		Prov:         []ProvenanceItem{},
		Orig:         caption,
		Text:         caption,
	}
	doc.Texts = append(doc.Texts, item)
	*captions = append(*captions, RefItem{Kind: refTexts, Idx: int64(len(doc.Texts) - 1)})
}

// migrateTableAnnotations 把旧表格 annotations 迁移到官方 meta。
func migrateTableAnnotations(item *TableItem) {
	if item.Meta == nil {
		item.Meta = BaseMeta{}
	}
	for _, annotation := range item.Annotations {
		migrateLegacyAnnotation(item.Meta, annotation)
	}
	if len(item.Meta) == 0 {
		item.Meta = nil
	}
	item.Annotations = []json.RawMessage{}
}

// migratePictureAnnotations 把旧图片 annotations 迁移到 PictureMeta。
func migratePictureAnnotations(item *PictureItem) {
	for _, annotation := range item.Annotations {
		var header struct {
			Kind       string `json:"kind"`
			Text       string `json:"text"`
			Provenance string `json:"provenance"`
		}
		if json.Unmarshal(annotation, &header) != nil {
			if item.Meta == nil {
				item.Meta = &PictureMeta{}
			}
			appendPictureLegacyMeta(item.Meta, "annotation", annotation)
			continue
		}
		if item.Meta == nil {
			item.Meta = &PictureMeta{}
		}
		switch header.Kind {
		case "classification":
			if item.Meta.Classification != nil {
				appendPictureLegacyMeta(item.Meta, header.Kind, annotation)
				continue
			}
			var legacy struct {
				Provenance       string `json:"provenance"`
				PredictedClasses []struct {
					ClassName  string   `json:"class_name"`
					Confidence *float64 `json:"confidence"`
				} `json:"predicted_classes"`
			}
			if json.Unmarshal(annotation, &legacy) == nil && len(legacy.PredictedClasses) > 0 {
				predictions := make([]PictureClassificationPrediction, 0, len(legacy.PredictedClasses))
				for _, prediction := range legacy.PredictedClasses {
					predictions = append(predictions, PictureClassificationPrediction{
						PredictionMeta: PredictionMeta{Confidence: prediction.Confidence, CreatedBy: legacy.Provenance},
						ClassName:      prediction.ClassName,
					})
				}
				item.Meta.Classification = &PictureClassificationMetaField{Predictions: predictions}
			} else {
				appendPictureLegacyMeta(item.Meta, header.Kind, annotation)
			}
		case "description":
			if len(item.Meta.Description) == 0 {
				item.Meta.Description, _ = json.Marshal(map[string]any{"text": header.Text, "created_by": header.Provenance})
			} else {
				appendPictureLegacyMeta(item.Meta, header.Kind, annotation)
			}
		case "tabular_chart_data":
			if item.Meta.TabularChart == nil {
				var legacy struct {
					Title     string     `json:"title"`
					ChartData *TableData `json:"chart_data"`
				}
				if json.Unmarshal(annotation, &legacy) == nil && legacy.ChartData != nil {
					item.Meta.TabularChart = &TabularChartMetaField{Title: legacy.Title, ChartData: legacy.ChartData}
				} else {
					appendPictureLegacyMeta(item.Meta, header.Kind, annotation)
				}
			} else {
				appendPictureLegacyMeta(item.Meta, header.Kind, annotation)
			}
		default:
			appendPictureLegacyMeta(item.Meta, header.Kind, annotation)
		}
	}
	item.Annotations = []json.RawMessage{}
}

// appendPictureLegacyMeta 把未能映射的旧图片注解保存在官方自定义 meta 命名空间。
func appendPictureLegacyMeta(meta *PictureMeta, kind string, annotation json.RawMessage) {
	if meta.Extra == nil {
		meta.Extra = map[string]json.RawMessage{}
	}
	appendLegacyMeta(meta.Extra, kind, annotation)
}

// migrateLegacyAnnotation 把通用旧注解写入官方命名空间 meta。
func migrateLegacyAnnotation(meta BaseMeta, annotation json.RawMessage) {
	var header struct {
		Kind       string          `json:"kind"`
		Text       string          `json:"text"`
		Provenance string          `json:"provenance"`
		Content    json.RawMessage `json:"content"`
	}
	if json.Unmarshal(annotation, &header) != nil {
		appendLegacyMeta(meta, "annotation", annotation)
		return
	}
	if header.Kind == "description" {
		if _, exists := meta["description"]; !exists {
			meta["description"], _ = json.Marshal(map[string]any{"text": header.Text, "created_by": header.Provenance})
		} else {
			appendLegacyMeta(meta, header.Kind, annotation)
		}
		return
	}
	value := annotation
	if header.Kind == "misc" && len(header.Content) > 0 {
		value = header.Content
	}
	appendLegacyMeta(meta, header.Kind, value)
}

// appendLegacyMeta 保存未知旧注解；同名多项使用数组避免数据丢失。
func appendLegacyMeta(meta map[string]json.RawMessage, kind string, value json.RawMessage) {
	if strings.TrimSpace(kind) == "" {
		kind = "annotation"
	}
	key := "docling_legacy__" + kind
	existing, exists := meta[key]
	if !exists {
		meta[key] = append(json.RawMessage(nil), value...)
		return
	}
	var values []json.RawMessage
	if json.Unmarshal(existing, &values) != nil {
		values = []json.RawMessage{existing}
	}
	values = append(values, value)
	meta[key], _ = json.Marshal(values)
}

// ToContentList 把 DoclingDocument 简化为扁平 content_list：先按 body 树先序
// 遍历，再从 furniture 树兼容追加页眉页脚；HTML title 等其他家具项保持不进入
// 知识正文。source 写入每个 Item.Source，doc/Body 为空时返回 nil。
func ToContentList(doc *DoclingDocument, source string) []Item {
	if doc == nil || doc.Body == nil {
		return nil
	}
	s := &contentListSimplifier{doc: doc, source: source}
	s.walk(doc.Body.Children)
	if doc.Furniture != nil {
		s.walkFurniture(doc.Furniture.Children)
	}
	return s.out
}

// contentListSimplifier 简化过程状态：树引用、章节路径栈与输出序号。
type contentListSimplifier struct {
	doc         *DoclingDocument
	source      string
	out         []Item
	sectionPath []string
	order       int64
}

// walkFurniture 仅把历史 content_list 一直可见的页眉页脚透传出来；命名分组
// 只用于递归定位，不把 furniture 中的 HTML title 等协议元数据混入正文。
func (s *contentListSimplifier) walkFurniture(children []RefItem) {
	for _, ref := range children {
		switch ref.Kind {
		case refTexts:
			if ref.Idx < 0 || ref.Idx >= int64(len(s.doc.Texts)) {
				continue
			}
			item := s.doc.Texts[ref.Idx]
			if item.Label == LabelPageHeader || item.Label == LabelPageFooter {
				s.appendItem(s.convertText(item, false))
			}
		case refGroups:
			if ref.Idx >= 0 && ref.Idx < int64(len(s.doc.Groups)) {
				s.walkFurniture(s.doc.Groups[ref.Idx].Children)
			}
		}
	}
}

// walk 先序遍历树 children，按节点类型分派。
func (s *contentListSimplifier) walk(children []RefItem) {
	for _, ref := range children {
		switch ref.Kind {
		case refTexts:
			if ref.Idx < 0 || ref.Idx >= int64(len(s.doc.Texts)) {
				continue
			}
			s.walkText(ref.Idx)
		case refTables:
			if ref.Idx < 0 || ref.Idx >= int64(len(s.doc.Tables)) {
				continue
			}
			s.appendItem(s.convertTable(s.doc.Tables[ref.Idx]))
			s.walk(s.doc.Tables[ref.Idx].Children)
		case refPictures:
			if ref.Idx < 0 || ref.Idx >= int64(len(s.doc.Pictures)) {
				continue
			}
			picture := s.doc.Pictures[ref.Idx]
			s.appendItem(s.convertPicture(picture))
			if chart, ok := s.convertChartData(picture); ok {
				s.appendItem(chart)
			}
			s.walk(picture.Children)
		case refGroups:
			if ref.Idx < 0 || ref.Idx >= int64(len(s.doc.Groups)) {
				continue
			}
			group := s.doc.Groups[ref.Idx]
			// 命名分组（sheet/slide/chapter）名字入章节路径（保留旧 xlsx 契约：
			// SectionPath 携带 sheet 名）；隐式 section/list 分组只透传子树
			named := group.Label == GroupLabelSheet || group.Label == GroupLabelSlide || group.Label == GroupLabelChapter
			if named && strings.TrimSpace(group.Name) != "" {
				s.sectionPath = append(s.sectionPath, group.Name)
			}
			s.walk(group.Children)
			if named && strings.TrimSpace(group.Name) != "" {
				s.sectionPath = s.sectionPath[:len(s.sectionPath)-1]
			}
		}
	}
}

// walkText 处理文本元素：标题产出后入章节栈再遍历子树，普通元素直接产出。
// 标题元素自身的 _section_path 含自身（对齐 go_light Markdown 旧行为：
// 标题与其章节内容归入同一切片组）。
func (s *contentListSimplifier) walkText(idx int64) {
	item := s.doc.Texts[idx]
	isHeading := item.Label == LabelSectionHeader || item.Label == LabelTitle
	if isHeading {
		// 官方 Markdown/HTML 标题直接平铺在 body。此类标题按 level 顺序
		// 更新持久章节栈，既保持 Docling JSON 官方结构，也兼容历史 content_list。
		if item.Parent != nil && item.Parent.Kind == refBody && len(item.Children) == 0 && isFlatHeading(item) {
			depth := 1
			if item.Label == LabelSectionHeader {
				depth = int(item.TextLevel) + 1
				if depth < 2 {
					depth = 2
				}
			}
			if keep := depth - 1; keep < len(s.sectionPath) {
				s.sectionPath = s.sectionPath[:keep]
			}
			title := item.Orig
			if title == "" {
				title = item.Text
			}
			s.sectionPath = append(s.sectionPath, title)
			it := s.convertText(item, true)
			it.SectionPath = s.pathCopy()
			if item.Label == LabelSectionHeader {
				it.TextLevel = int64(depth)
			}
			s.appendItem(it)
			return
		}
		it := s.convertText(item, true)
		title := item.Orig
		if title == "" {
			title = item.Text
		}
		it.SectionPath = append(it.SectionPath, title)
		s.appendItem(it)
		s.sectionPath = append(s.sectionPath, title)
		s.walk(item.Children)
		s.sectionPath = s.sectionPath[:len(s.sectionPath)-1]
		return
	}
	s.appendItem(s.convertText(item, false))
	s.walk(item.Children)
}

// isFlatHeading 判断标题是否由 Markdown/HTML 后端声明为顺序平铺标题。
func isFlatHeading(item TextItem) bool {
	if item.Meta == nil {
		return false
	}
	var flat bool
	return json.Unmarshal(item.Meta[flatHeadingMetaKey], &flat) == nil && flat
}

// appendItem 追加非空元素并推进阅读序号。
func (s *contentListSimplifier) appendItem(item Item) {
	if strings.TrimSpace(item.Text) == "" && strings.TrimSpace(item.TableBody) == "" &&
		strings.TrimSpace(item.ImageCaption) == "" && strings.TrimSpace(item.ImgPath) == "" {
		return
	}
	item.Source = s.source
	item.OrderIndex = s.order
	s.order++
	s.out = append(s.out, item)
}

// convertText 文本元素 → content_list Item（label→模态映射见文件头注释）。
func (s *contentListSimplifier) convertText(item TextItem, heading bool) Item {
	it := Item{SectionPath: s.pathCopy()}
	s.applyProv(&it, item.Prov)
	it.Text = item.Orig
	if it.Text == "" {
		it.Text = item.Text
	}
	switch item.Label {
	case LabelTitle:
		it.Type = ItemTypeText
		it.TextLevel = 1
	case LabelSectionHeader:
		it.Type = ItemTypeText
		if heading && item.TextLevel > 0 {
			it.TextLevel = item.TextLevel
		}
	case LabelFormula:
		it.Type = ItemTypeEquation
		// 旧版独立 latex 字段优先；官方结构用 sanitized text 保存 LaTeX，
		// 不能回退到可能已丢失公式控制符的 orig。
		it.LaTeX = item.Latex
		if it.LaTeX == "" {
			it.LaTeX = item.Text
		}
		if it.LaTeX == "" {
			it.LaTeX = it.Text
		}
	case LabelCode:
		// 代码块统一包裹围栏：go_light Markdown 路径产物本就含围栏，
		// Docling code 路径补齐后两路 chunk 表达一致（"这是代码"可识别）
		it.Type = ItemTypeText
		if it.Text != "" {
			it.Text = "```" + item.CodeLanguage + "\n" + it.Text + "\n```"
		}
	case LabelCheckboxSelected, LabelCheckboxUnselected,
		LabelListItem, LabelParagraph, LabelCaption, LabelText:
		it.Type = ItemTypeText
	case LabelPageHeader, LabelPageFooter, LabelDocumentIndex:
		// 版式家具/目录行归通用 text，细粒度分类经 Label 透传（见下）
		it.Type = ItemTypeText
	default:
		it.Type = ItemTypeGeneric
	}
	// 细粒度 label 透传：type 已归为通用 text/equation 但 label 仍有区分
	// 价值时填充 Item.Label，供下游过滤/分流（page_header/page_footer/
	// document_index/list_item/code/checkbox/caption 填充；formula 由
	// type=equation 表达、text/paragraph/section_header/title 已有专用
	// 表达或无区分价值，均不填充）。
	switch item.Label {
	case LabelPageHeader, LabelPageFooter, LabelDocumentIndex, LabelListItem,
		LabelCode, LabelCheckboxSelected, LabelCheckboxUnselected, LabelCaption:
		it.Label = string(item.Label)
	}
	return it
}

// convertTable 表格元素 → table Item；Grid 渲染为 Markdown 并做后处理清洗。
func (s *contentListSimplifier) convertTable(item TableItem) Item {
	it := Item{Type: ItemTypeTable, SectionPath: s.pathCopy()}
	s.applyProv(&it, item.Prov)
	it.TableCaption = s.captionText(item.Caption, item.Captions)
	if item.Data != nil {
		it.TableBody = renderTableMarkdown(item.Data.TableCells, item.Data.NumRows, item.Data.NumCols)
	}
	return it
}

// convertPicture 图片元素 → image Item；图片地址取 ImageRef.URI。
func (s *contentListSimplifier) convertPicture(item PictureItem) Item {
	it := Item{Type: ItemTypeImage, SectionPath: s.pathCopy()}
	s.applyProv(&it, item.Prov)
	it.ImageCaption = s.captionText(item.Caption, item.Captions)
	if it.ImageCaption == "" && item.Meta != nil && item.Meta.TabularChart != nil {
		it.ImageCaption = strings.TrimSpace(item.Meta.TabularChart.Title)
	}
	if item.Image != nil {
		it.ImgPath = item.Image.URI
	}
	return it
}

// convertChartData 把 PictureItem 的官方 tabular_chart.chart_data 派生为
// 紧邻图片项的可检索表格；该视图只存在于 content_list，不修改 Docling JSON。
func (s *contentListSimplifier) convertChartData(item PictureItem) (Item, bool) {
	if item.Meta == nil || item.Meta.TabularChart == nil || item.Meta.TabularChart.ChartData == nil {
		return Item{}, false
	}
	chart := item.Meta.TabularChart
	if chart.ChartData.NumRows <= 0 || chart.ChartData.NumCols <= 0 || len(chart.ChartData.TableCells) == 0 {
		return Item{}, false
	}
	it := Item{
		Type:         ItemTypeTable,
		SectionPath:  s.pathCopy(),
		TableCaption: strings.TrimSpace(chart.Title),
		TableBody: renderTableMarkdown(
			chart.ChartData.TableCells,
			chart.ChartData.NumRows,
			chart.ChartData.NumCols,
		),
	}
	s.applyProv(&it, item.Prov)
	return it, true
}

// captionText 优先读取旧字符串 caption，再解析官方 captions 引用。
func (s *contentListSimplifier) captionText(legacy string, refs []RefItem) string {
	if strings.TrimSpace(legacy) != "" {
		return legacy
	}
	var parts []string
	for _, ref := range refs {
		if ref.Kind != refTexts || ref.Idx < 0 || ref.Idx >= int64(len(s.doc.Texts)) {
			continue
		}
		text := s.doc.Texts[ref.Idx].Text
		if text == "" {
			text = s.doc.Texts[ref.Idx].Orig
		}
		if strings.TrimSpace(text) != "" {
			parts = append(parts, text)
		}
	}
	return strings.Join(parts, "")
}

// applyProv 把首段 prov 映射为 PageIdx（0-based）与 BBox。
func (s *contentListSimplifier) applyProv(it *Item, prov []ProvenanceItem) {
	if len(prov) == 0 {
		return
	}
	first := prov[0]
	if first.PageNo > 0 {
		it.PageIdx = first.PageNo - 1
	}
	if first.BBox != nil {
		it.BBox = &BBox{
			Left:   first.BBox.L,
			Top:    math.Min(first.BBox.T, first.BBox.B),
			Right:  first.BBox.R,
			Bottom: math.Max(first.BBox.T, first.BBox.B),
		}
	}
}

// pathCopy 拷贝当前章节路径（避免切片共享底层数组）。
func (s *contentListSimplifier) pathCopy() []string {
	if len(s.sectionPath) == 0 {
		return nil
	}
	out := make([]string, len(s.sectionPath))
	copy(out, s.sectionPath)
	return out
}

// renderTableMarkdown 按 cell 行列偏移铺格还原行列表，经统一清洗链后
// 渲染 GFM Markdown 表格（GFM 语法）。
func renderTableMarkdown(cells []DoclingTableCell, numRows, numCols int64) string {
	if numRows <= 0 || numCols <= 0 || len(cells) == 0 {
		return ""
	}
	rows := make([][]string, numRows)
	for i := range rows {
		rows[i] = make([]string, numCols)
	}
	for _, c := range cells {
		for r := c.StartRowOffsetIdx; r < c.EndRowOffsetIdx && r < numRows; r++ {
			for col := c.StartColOffsetIdx; col < c.EndColOffsetIdx && col < numCols; col++ {
				rows[r][col] = c.Text
			}
		}
	}
	rows = postprocessTableRows(rows)
	if len(rows) == 0 {
		return ""
	}
	return RenderMarkdownTable(rows)
}

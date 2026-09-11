// pptx.go 实现 PPTX（PowerPoint OOXML）到 DoclingDocument 的 Go 本地解析后端，
// 复刻 docling mspowerpoint_backend.py 的核心行为（纯 Go：archive/zip + encoding/xml
// 手写 token 流状态机，不依赖 python-pptx）：
//   - 每 slide 建 slide-N 分组（CHAPTER 组的 Go 侧用 AddSectionGroup 表达）并登记
//     页面尺寸（presentation.xml 的 sldSz EMU 原始值）；
//   - 形状按视觉位置排序（top 分行 + 行内 left 升序，容差 45720 EMU=0.05"）；
//   - title/ctrTitle 占位符 → title，其余有文本 shape → 逐段落 paragraph；
//   - 段落 bullet（a:buChar/buAutoNum/buNone）判定列表项，连续列表段落共享一个
//     list 分组，有序 marker 自增，lvl>0 的段落作为子列表挂上一列表项下；
//   - a:tbl 表格转 TableData：gridSpan/rowSpan → ColSpan/RowSpan（start 闭 end 开），
//     空文本 cell 丢弃但 num_rows/num_cols 按全尺寸声明，全空表不建；
//   - prov bbox 为 EMU 原始值 [l,t,l+w,t+h]，CoordOrigin=BOTTOMLEFT（pptx 页面
//     坐标系自底向上），无几何形状回退整页 (0,0,slideW,slideH)；
//   - 文档元数据读 docProps/core.xml，PageCount 取 slide 数（P0 增强）。
//
// 有意简化（复刻边界，注释逐条记录）：
//   - p:pic 图片提取为 picture 元素（r:embed 经 slide 关系表定位 ppt/media
//     部件内嵌 data URI，超限/缺失时 URI 留空；P0 增强，超出 docling 复刻
//     边界——Go 各后端原统一不做图片资产提取）；
//   - 图表 graphicFrame 保留 chart 关系引用，并委托共享 OOXML 图表解析器产出图片项；
//   - 备注取 notesSlide 的 body 占位符文本并写入 NOTES content layer；
//   - 幻灯片旧式及 Office 2021+ 现代 comments 部件转为 NOTES 文本；现代
//     批注保留 authors、replyLst 与解决状态，按批注坐标关联最近正文项；
//   - 空段落不产出元素（源码会产出空文本元素，知识库场景无需空块）。
package docling

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"image"
	"io"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/unitedrhino/docling/internal/ooxml"
)

// pptx OOXML 关系命名空间与关键部件路径。
const (
	// pptxOfficeRelNS OOXML relationships 属性命名空间（r:id 等引用属性）。
	pptxOfficeRelNS = officeRelNS
	// pptxPresentationPath 主展示文档部件路径。
	pptxPresentationPath = "ppt/presentation.xml"
	// pptxPresentationRelsPath 展示文档关系部件路径。
	pptxPresentationRelsPath = "ppt/_rels/presentation.xml.rels"
	// pptxShapeRowToleranceEMU 形状视觉分行容差（45720 EMU = 0.05 英寸），
	// 对齐源码 _SHAPE_ROW_TOLERANCE_EMU。
	pptxShapeRowToleranceEMU = 45720.0
)

// pptxShapeKind 形状类别：文本形状 / 表格 graphicFrame / 图片 p:pic / 其他（跳过）。
type pptxShapeKind int

// 形状类别取值。
const (
	pptxShapeText    pptxShapeKind = iota // p:sp 文本形状
	pptxShapeFrame                        // p:graphicFrame（表格或图表）
	pptxShapePicture                      // p:pic 图片形状
	pptxShapeSkip                         // 其余不产出内容的形状
)

// pptxBulletKind 段落项目符号类型，对齐源码 _parse_bullet_from_paragraph_properties
// 的简化子集（Go 侧不回退 lstStyle/母版样式）。
type pptxBulletKind int

// 项目符号类型取值。
const (
	pptxBulletNone    pptxBulletKind = iota // 无 bullet 声明（判定交回默认：非列表）
	pptxBulletChar                          // a:buChar 字符项目符号 → 无序列表项
	pptxBulletAutoNum                       // a:buAutoNum 自动编号 → 有序列表项
	pptxBulletOff                           // a:buNone 显式无 bullet → 普通段落
)

// pptxPara 文本形状内单个段落（a:p）的解析结果。
type pptxPara struct {
	text   string         // 段落纯文本（a:t 拼接，a:br→\n、a:tab→\t）
	level  int64          // a:pPr 的 lvl 属性（嵌套层级，0 起缺省）
	bullet pptxBulletKind // 项目符号类型
}

// pptxGroupTransform 保存组合形状坐标系到父坐标系的仿射变换参数。
type pptxGroupTransform struct {
	offX, offY     float64
	extX, extY     float64
	childX, childY float64
	childW, childH float64
	hasOff, hasExt bool
	hasChildOff    bool
	hasChildExt    bool
}

// pptxCell 表格单元格（a:tc）的解析结果。
type pptxCell struct {
	text    string // 单元格纯文本（多段落以 \n 连接，已去除首尾空白）
	rowSpan int64  // rowSpan 属性（缺省 1）
	colSpan int64  // gridSpan 属性（缺省 1）
}

// pptxShape 幻灯片内单个顶层形状（含 grpSp 拍平后的子形状）的解析结果。
type pptxShape struct {
	kind           pptxShapeKind            // 形状类别
	order          int                      // XML 出现序号（视觉排序的稳定回退键）
	hasOff, hasExt bool                     // 是否解析到 a:off / a:ext
	left, top      float64                  // a:off（EMU）
	width, height  float64                  // a:ext（EMU）
	isPlaceholder  bool                     // p:nvPr/p:ph 存在（占位符形状）
	phType         string                   // p:ph 的 type 属性（title/ctrTitle/subTitle/body…）
	phIdx          string                   // p:ph 的 idx 属性，用于 slide → layout → master 占位符匹配
	paras          []pptxPara               // 文本形状的段落序列
	defaultBullets map[int64]pptxBulletKind // txBody/lstStyle 或母版继承的分级项目符号
	table          *pptxTable               // graphicFrame 内的 a:tbl（无表格为 nil）
	pictureRID     string                   // p:pic 的 a:blip r:embed（图片关系引用）
	chartRID       string                   // graphicFrame 的 c:chart r:id，预留给共享图表解析器
	name           string                   // cNvPr 名称，作为复杂对象的无障碍说明回退
	officeObject   *officeObjectRecord      // SmartArt、艺术字或 OLE 嵌入对象语义
}

// hasGeom 是否解析出完整几何（a:off 与 a:ext 齐备），决定 prov 用实际
// 边界框还是回退整页。
func (s *pptxShape) hasGeom() bool { return s.hasOff && s.hasExt }

// pptxTable 表格（a:tbl）的解析结果：逐行的单元格序列。
type pptxTable struct {
	rows [][]pptxCell
}

// ParsePPTX 解析 PPTX（OOXML zip）为 DoclingDocument，复刻 docling
// mspowerpoint_backend.py 的 slide 遍历与形状处理行为（详见文件头注释）。
// 非 pptx / 损坏 zip / 缺少 presentation.xml 返回 error；
// slide 集合为空时返回元素为空的文档（不视为错误）。
func ParsePPTX(data []byte) (*DoclingDocument, error) {
	var err error
	data, err = ooxml.NormalizeStrictOOXMLPackage(data)
	if err != nil {
		return nil, fmt.Errorf("docling: 归一化 Strict PPTX 失败: %w", err)
	}
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("docling: 解析 pptx zip 失败: %w", err)
	}
	presentationXML, ok := readPptxZipFile(reader, pptxPresentationPath)
	if !ok {
		return nil, fmt.Errorf("docling: 缺少 %s，不是有效的 pptx", pptxPresentationPath)
	}

	// 解析 presentation.xml：sldSz 页面 EMU 尺寸 + sldIdLst 的 r:id 顺序
	slideW, slideH, slideRefs := parsePptxPresentation(presentationXML)
	// 关系表：rId → zip 内部件路径（Target 相对 ppt/ 目录解析）
	relsXML, _ := readPptxZipFile(reader, pptxPresentationRelsPath)
	relTargets := parseOOXMLRelationships(relsXML, "/slide", path.Dir(pptxPresentationPath))
	commentAuthors := parsePptxCommentAuthors(reader, relsXML)

	doc := NewDoclingDocument("pptx")
	// 文档元数据（docProps/core.xml）+ 页数（slide 数，含关系缺失的引用）；
	// core.xml 缺失时仍建 Meta 以携带 PageCount
	doc.Meta = ooxmlCorePropsMeta(reader)
	if doc.Meta == nil {
		doc.Meta = &DocMeta{}
	}
	doc.Meta.PageCount = len(slideRefs)
	for slideInd, refID := range slideRefs {
		slidePath, ok := relTargets[refID]
		if !ok {
			continue // 关系缺失：跳过该 slide（源码 python-pptx 会抛错，Go 侧宽松处理）
		}
		slideXML, ok := readPptxZipFile(reader, slidePath)
		if !ok {
			continue
		}
		shapes := parsePptxSlideShapes(slideXML)
		group := doc.AddSectionGroup(fmt.Sprintf("slide-%d", slideInd), nil)
		// 页号从 1 起；尺寸用 sldSz 的 EMU 原始值（对齐源码 add_page(size=Size(EMU))）
		doc.AddPage(int64(slideInd+1), slideW, slideH)

		// slide 关系表：图片目标（p:pic 的 r:embed）经此定位 ppt/media 部件
		dir, name := path.Split(slidePath)
		slideRelsXML, _ := readPptxZipFile(reader, path.Join(dir, "_rels", name+".rels"))
		imageTargets := parseOOXMLRelationships(slideRelsXML, "/image", path.Clean(dir))
		chartTargets := parseOOXMLRelationships(slideRelsXML, "/chart", path.Clean(dir))
		diagramTargets := parseOOXMLRelationships(slideRelsXML, "/diagramData", path.Clean(dir))
		oleTargets := parseOOXMLRelationships(slideRelsXML, "/oleObject", path.Clean(dir))
		applyPptxStyleFallback(reader, slidePath, slideRelsXML, shapes)

		for _, sh := range sortPptxShapes(shapes) {
			if sh.officeObject != nil {
				handlePptxOfficeObject(doc, sh, group, int64(slideInd), slideW, slideH, diagramTargets, oleTargets, reader)
				continue
			}
			switch sh.kind {
			case pptxShapeText:
				handlePptxTextShape(doc, sh, group, int64(slideInd), slideW, slideH)
			case pptxShapeFrame:
				handlePptxTableShape(doc, sh, group, int64(slideInd), slideW, slideH, chartTargets, reader)
			case pptxShapePicture:
				handlePptxPicture(doc, sh, group, int64(slideInd), slideW, slideH, imageTargets, reader)
			case pptxShapeSkip:
				// 图表 graphicFrame 不产出（复刻边界，见文件头注释）
			}
		}
		handlePptxNotes(doc, reader, slidePath, group, int64(slideInd))
		handlePptxComments(doc, reader, slidePath, group, int64(slideInd), slideRelsXML, commentAuthors)
	}
	return doc, nil
}

// setPptxDefaultBullet 设置文本形状指定层级的默认项目符号。
func setPptxDefaultBullet(shape *pptxShape, level int64, bullet pptxBulletKind) {
	if shape.defaultBullets == nil {
		shape.defaultBullets = map[int64]pptxBulletKind{}
	}
	shape.defaultBullets[level] = bullet
}

// pptxStyleLevel 解析 DrawingML 的 lvl1pPr..lvl9pPr 为 0 起层级。
func pptxStyleLevel(local string) (int64, bool) {
	if !strings.HasPrefix(local, "lvl") || !strings.HasSuffix(local, "pPr") {
		return 0, false
	}
	raw := strings.TrimSuffix(strings.TrimPrefix(local, "lvl"), "pPr")
	level, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || level < 1 {
		return 0, false
	}
	return level - 1, true
}

// applyPptxGroupTransforms 把形状局部坐标由内到外转换为幻灯片绝对坐标。
func applyPptxGroupTransforms(shape *pptxShape, transforms []pptxGroupTransform) {
	if !shape.hasGeom() {
		return
	}
	for i := len(transforms) - 1; i >= 0; i-- {
		g := transforms[i]
		if !g.hasOff || !g.hasExt || !g.hasChildOff || !g.hasChildExt || g.childW == 0 || g.childH == 0 {
			continue
		}
		scaleX := g.extX / g.childW
		scaleY := g.extY / g.childH
		shape.left = g.offX + (shape.left-g.childX)*scaleX
		shape.top = g.offY + (shape.top-g.childY)*scaleY
		shape.width *= scaleX
		shape.height *= scaleY
	}
}

// applyPptxStyleFallback 依次应用 slideLayout 与 slideMaster 的占位符属性。
// 仅在幻灯片形状未显式声明时回退，避免覆盖作者在当前页上的设置。
func applyPptxStyleFallback(reader *zip.Reader, slidePath string, slideRelsXML []byte, shapes []*pptxShape) {
	slideDir := path.Dir(slidePath)
	layoutTargets := parseOOXMLRelationships(slideRelsXML, "/slideLayout", slideDir)
	for _, layoutPath := range layoutTargets {
		layoutXML, ok := readPptxZipFile(reader, layoutPath)
		if !ok {
			continue
		}
		layoutShapes := parsePptxSlideShapes(layoutXML)
		layoutDir, layoutName := path.Split(layoutPath)
		layoutRelsXML, _ := readPptxZipFile(reader, path.Join(layoutDir, "_rels", layoutName+".rels"))
		masterTargets := parseOOXMLRelationships(layoutRelsXML, "/slideMaster", path.Clean(layoutDir))
		for _, masterPath := range masterTargets {
			masterXML, found := readPptxZipFile(reader, masterPath)
			if !found {
				continue
			}
			masterShapes := parsePptxSlideShapes(masterXML)
			masterStyles := parsePptxMasterTextStyles(masterXML)
			for _, layoutShape := range layoutShapes {
				if fallback := findPptxPlaceholder(masterShapes, layoutShape); fallback != nil {
					mergePptxShapeFallback(layoutShape, fallback)
				}
				mergePptxBulletFallback(layoutShape, masterStyles[pptxMasterStyleKind(layoutShape.phType)])
			}
			break
		}
		for _, shape := range shapes {
			if !shape.isPlaceholder {
				continue
			}
			if fallback := findPptxPlaceholder(layoutShapes, shape); fallback != nil {
				mergePptxShapeFallback(shape, fallback)
			}
		}
		break
	}
}

// findPptxPlaceholder 按 idx 优先、type 次优匹配占位符回退源。
func findPptxPlaceholder(candidates []*pptxShape, target *pptxShape) *pptxShape {
	for _, candidate := range candidates {
		if candidate.isPlaceholder && target.phIdx != "" && candidate.phIdx == target.phIdx {
			return candidate
		}
	}
	for _, candidate := range candidates {
		if candidate.isPlaceholder && target.phType != "" && candidate.phType == target.phType {
			return candidate
		}
	}
	return nil
}

// mergePptxShapeFallback 合并占位符类型、几何和默认列表样式。
func mergePptxShapeFallback(target, fallback *pptxShape) {
	if target.phType == "" {
		target.phType = fallback.phType
	}
	if !target.hasOff && fallback.hasOff {
		target.left, target.top, target.hasOff = fallback.left, fallback.top, true
	}
	if !target.hasExt && fallback.hasExt {
		target.width, target.height, target.hasExt = fallback.width, fallback.height, true
	}
	mergePptxBulletFallback(target, fallback.defaultBullets)
}

// mergePptxBulletFallback 只填充形状尚未声明的层级样式。
func mergePptxBulletFallback(target *pptxShape, fallback map[int64]pptxBulletKind) {
	for level, bullet := range fallback {
		if _, exists := target.defaultBullets[level]; !exists {
			setPptxDefaultBullet(target, level, bullet)
		}
	}
}

// pptxMasterStyleKind 把占位符类型映射到母版 title/body/other 样式区。
func pptxMasterStyleKind(phType string) string {
	switch phType {
	case "title", "ctrTitle":
		return "titleStyle"
	case "body", "subTitle", "obj":
		return "bodyStyle"
	default:
		return "otherStyle"
	}
}

// parsePptxMasterTextStyles 解析母版 txStyles 的分级项目符号默认值。
func parsePptxMasterTextStyles(xmlBytes []byte) map[string]map[int64]pptxBulletKind {
	styles := map[string]map[int64]pptxBulletKind{}
	decoder := xml.NewDecoder(bytes.NewReader(xmlBytes))
	activeStyle := ""
	level := int64(-1)
	for {
		token, err := decoder.Token()
		if err != nil {
			break
		}
		switch tok := token.(type) {
		case xml.StartElement:
			switch tok.Name.Local {
			case "titleStyle", "bodyStyle", "otherStyle":
				activeStyle = tok.Name.Local
				if styles[activeStyle] == nil {
					styles[activeStyle] = map[int64]pptxBulletKind{}
				}
			case "buChar":
				if activeStyle != "" && level >= 0 {
					styles[activeStyle][level] = pptxBulletChar
				}
			case "buAutoNum":
				if activeStyle != "" && level >= 0 {
					styles[activeStyle][level] = pptxBulletAutoNum
				}
			case "buNone":
				if activeStyle != "" && level >= 0 {
					styles[activeStyle][level] = pptxBulletOff
				}
			default:
				if parsed, ok := pptxStyleLevel(tok.Name.Local); ok {
					level = parsed
				}
			}
		case xml.EndElement:
			switch tok.Name.Local {
			case "titleStyle", "bodyStyle", "otherStyle":
				activeStyle, level = "", -1
			default:
				if _, ok := pptxStyleLevel(tok.Name.Local); ok {
					level = -1
				}
			}
		}
	}
	return styles
}

// handlePptxPicture 产出 p:pic 图片元素（P0 增强，见文件头注释）：pictureRID
// 经 slide 关系表定位 ppt/media 部件并内嵌为 data URI（关系缺失、读取失败或
// 超过 maxMediaDataURIBytes 时 URI 留空仍产出元素），prov 按形状几何生成。
func handlePptxPicture(doc *DoclingDocument, sh *pptxShape, group RefItem, slideInd int64, slideW, slideH float64, imageTargets map[string]string, reader *zip.Reader) {
	imageRef := &ImageRef{Mimetype: "application/octet-stream", Dpi: defaultImageDPI, Size: &ImageSize{}}
	if target, ok := imageTargets[sh.pictureRID]; ok {
		imageRef.Mimetype = ooxmlMediaMime(filepath.Ext(target))
		if raw, found := readPptxZipFile(reader, target); found {
			imageRef.URI = mediaToDataURI(filepath.Ext(target), raw)
			if config, _, err := image.DecodeConfig(bytes.NewReader(raw)); err == nil {
				imageRef.Size = &ImageSize{Width: float64(config.Width), Height: float64(config.Height)}
			}
		}
	}
	doc.AddPicture(imageRef,
		makePptxProv(sh.hasGeom(), sh.left, sh.top, sh.width, sh.height, slideInd, 0, slideW, slideH),
		&group)
}

// parsePptxPresentation 解析 presentation.xml：返回 sldSz 宽高（EMU）与
// sldIdLst 顺序的 slide r:id 引用列表。
func parsePptxPresentation(xmlBytes []byte) (float64, float64, []string) {
	decoder := xml.NewDecoder(bytes.NewReader(xmlBytes))
	var width, height float64
	var slideRefs []string
	for {
		token, err := decoder.Token()
		if err != nil {
			break // 含 io.EOF 与畸形 XML：已有内容照常返回
		}
		elem, ok := token.(xml.StartElement)
		if !ok {
			continue
		}
		switch elem.Name.Local {
		case "sldSz":
			// 页面尺寸：cx/cy 为无前缀属性（EMU）
			width = pptxAttrFloat(elem.Attr, "cx")
			height = pptxAttrFloat(elem.Attr, "cy")
		case "sldId":
			// slide 引用：r:id 为 relationships 命名空间的 id 属性
			if ref := pptxAttrRel(elem.Attr); ref != "" {
				slideRefs = append(slideRefs, ref)
			}
		}
	}
	return width, height, slideRefs
}

// parseOOXMLRelationships 解析 OOXML 关系文件（*_rels/*.rels），docx/pptx
// 共用（图片/备注/幻灯片关系均经此解析）：返回 relTypeSuffix（如 "/slide"、
// "/image"）结尾的关系的 Id → 部件路径映射；Target 以 "/" 开头按包内绝对
// 路径处理，否则相对 baseDir（关系文件所属部件的目录）解析。
func parseOOXMLRelationships(xmlBytes []byte, relTypeSuffix, baseDir string) map[string]string {
	targets := map[string]string{}
	if len(xmlBytes) == 0 {
		return targets
	}
	decoder := xml.NewDecoder(bytes.NewReader(xmlBytes))
	for {
		token, err := decoder.Token()
		if err != nil {
			break
		}
		elem, ok := token.(xml.StartElement)
		if !ok || elem.Name.Local != "Relationship" {
			continue
		}
		relType := pptxAttr(elem.Attr, "Type")
		id := pptxAttr(elem.Attr, "Id")
		target := pptxAttr(elem.Attr, "Target")
		if id == "" || target == "" || !strings.HasSuffix(relType, relTypeSuffix) {
			continue
		}
		if strings.HasPrefix(target, "/") {
			target = strings.TrimPrefix(target, "/")
		} else {
			target = path.Clean(path.Join(baseDir, target))
		}
		targets[id] = target
	}
	return targets
}

// parsePptxSlideShapes 用 token 流状态机解析 slide XML 的顶层形状集合。
// grpSp 分组形状不作为独立节点，其子形状自然拍平到 slide 层级
// （对齐源码 handle_groups：组内形状同样挂 slide 分组）；
// 分组自身的 xfrm 用于把子形状局部坐标递归换算到幻灯片坐标。
func parsePptxSlideShapes(xmlBytes []byte) []*pptxShape {
	decoder := xml.NewDecoder(bytes.NewReader(xmlBytes))
	var shapes []*pptxShape
	var (
		curShape     *pptxShape           // 当前收集中的形状（sp/graphicFrame/pic）
		curTable     *pptxTable           // 当前表格（a:tbl 内）
		curRow       []pptxCell           // 当前行已收齐的单元格
		curCell      *pptxCell            // 当前单元格（a:tc 内）
		curPara      *pptxPara            // 当前段落（a:p 内）
		inCell       bool                 // 是否处于表格单元格内（段落归属 cell 而非形状正文）
		inPPr        bool                 // 是否处于段落属性 a:pPr 内（其中的文本不收集）
		inLstStyle   bool                 // 是否处于文本框默认列表样式 a:lstStyle 内
		styleLevel   int64                // 当前 a:lvlNpPr 的层级（0 起，-1 表示不在默认样式中）
		inT          bool                 // 是否处于 a:t 文本节点内
		groupStack   []pptxGroupTransform // 外层到内层的组合变换栈
		inGroupProps bool                 // 当前是否解析组合自身的 p:grpSpPr
		order        int
	)
	styleLevel = -1

	// finishPara 段落收尾：cell 内段落并入单元格文本（多段落 \n 连接），
	// 形状正文段落追加到形状段落序列。
	finishPara := func() {
		if curPara == nil {
			return
		}
		p := *curPara
		curPara = nil
		if inCell && curCell != nil {
			if curCell.text != "" {
				curCell.text += "\n"
			}
			curCell.text += p.text
			return
		}
		if curShape != nil && curShape.kind == pptxShapeText {
			curShape.paras = append(curShape.paras, p)
		}
	}

	for {
		token, err := decoder.Token()
		if err != nil {
			break // 含 io.EOF 与畸形 XML：已解析的形状照常返回
		}
		switch tok := token.(type) {
		case xml.StartElement:
			switch tok.Name.Local {
			case "sp":
				// p:sp 文本形状（notesSlide 内也复用此解析，kind 语义不变）
				finishPara()
				order++
				curShape = &pptxShape{kind: pptxShapeText, order: order}
			case "graphicFrame":
				finishPara()
				order++
				curShape = &pptxShape{kind: pptxShapeFrame, order: order}
			case "pic":
				// p:pic 图片：记录形状（产出阶段经关系表解析 r:embed）
				finishPara()
				order++
				curShape = &pptxShape{kind: pptxShapePicture, order: order}
			case "blip":
				// a:blip 图片引用（r:embed 指向 media 关系），仅图片形状内收集
				if curShape != nil && curShape.kind == pptxShapePicture {
					curShape.pictureRID = xmlAttrNS(tok.Attr, officeRelNS, "embed")
				}
			case "chart":
				// 只记录关系引用；图表数据由共享 OOXML chart helper 在产出阶段接入。
				if curShape != nil && curShape.kind == pptxShapeFrame {
					curShape.chartRID = xmlAttrNS(tok.Attr, officeRelNS, "id")
				}
			case "cNvPr":
				if curShape != nil {
					curShape.name = pptxAttr(tok.Attr, "name")
					if curShape.officeObject != nil {
						curShape.officeObject.name = curShape.name
					}
				}
			case "relIds":
				if curShape != nil {
					curShape.officeObject = &officeObjectRecord{
						kind: "smartart", relationship: xmlAttrNS(tok.Attr, officeRelNS, "dm"), name: curShape.name,
					}
				}
			case "oleObj":
				if curShape != nil {
					curShape.officeObject = &officeObjectRecord{
						kind: "ole", relationship: xmlAttrNS(tok.Attr, officeRelNS, "id"),
						program: pptxAttr(tok.Attr, "progId"), name: curShape.name,
					}
				}
			case "prstTxWarp":
				if curShape != nil && curShape.kind == pptxShapeText {
					name := ""
					if curShape.officeObject != nil {
						name = curShape.officeObject.name
					}
					curShape.officeObject = &officeObjectRecord{kind: "wordart", name: name, geometry: pptxAttr(tok.Attr, "prst")}
				}
			case "grpSp":
				// 分组本身不建 Docling 节点，但保留变换供子形状换算到幻灯片坐标。
				finishPara()
				groupStack = append(groupStack, pptxGroupTransform{})
			case "grpSpPr":
				if len(groupStack) > 0 && curShape == nil {
					inGroupProps = true
				}
			case "ph":
				// 占位符声明：记录 type（title/ctrTitle/subTitle/body…）
				if curShape != nil {
					curShape.isPlaceholder = true
					curShape.phType = pptxAttr(tok.Attr, "type")
					curShape.phIdx = pptxAttr(tok.Attr, "idx")
				}
			case "off":
				// a:off 几何原点（EMU）；分组自身的 xfrm 此时无当前形状，被忽略
				if curShape != nil {
					curShape.left = pptxAttrFloat(tok.Attr, "x")
					curShape.top = pptxAttrFloat(tok.Attr, "y")
					curShape.hasOff = true
				} else if inGroupProps && len(groupStack) > 0 {
					g := &groupStack[len(groupStack)-1]
					g.offX, g.offY, g.hasOff = pptxAttrFloat(tok.Attr, "x"), pptxAttrFloat(tok.Attr, "y"), true
				}
			case "ext":
				// a:ext 几何尺寸（EMU）
				if curShape != nil {
					curShape.width = pptxAttrFloat(tok.Attr, "cx")
					curShape.height = pptxAttrFloat(tok.Attr, "cy")
					curShape.hasExt = true
				} else if inGroupProps && len(groupStack) > 0 {
					g := &groupStack[len(groupStack)-1]
					g.extX, g.extY, g.hasExt = pptxAttrFloat(tok.Attr, "cx"), pptxAttrFloat(tok.Attr, "cy"), true
				}
			case "chOff":
				if inGroupProps && len(groupStack) > 0 {
					g := &groupStack[len(groupStack)-1]
					g.childX, g.childY, g.hasChildOff = pptxAttrFloat(tok.Attr, "x"), pptxAttrFloat(tok.Attr, "y"), true
				}
			case "chExt":
				if inGroupProps && len(groupStack) > 0 {
					g := &groupStack[len(groupStack)-1]
					g.childW, g.childH, g.hasChildExt = pptxAttrFloat(tok.Attr, "cx"), pptxAttrFloat(tok.Attr, "cy"), true
				}
			case "tbl":
				// a:tbl 表格（graphicFrame 的 graphicData 内）
				curTable = &pptxTable{}
			case "tr":
				curRow = nil
			case "tc":
				// a:tc 单元格：gridSpan/rowSpan 缺省 1
				inCell = true
				curCell = &pptxCell{
					rowSpan: pptxAttrInt64(tok.Attr, "rowSpan", 1),
					colSpan: pptxAttrInt64(tok.Attr, "gridSpan", 1),
				}
			case "p":
				// a:p 段落（与 p: 命名空间元素不重名：其 local 均为 sp/sld 等专名）
				finishPara()
				curPara = &pptxPara{}
			case "pPr":
				// a:pPr 段落属性：lvl 为嵌套层级
				inPPr = true
				if curPara != nil {
					curPara.level = pptxAttrInt64(tok.Attr, "lvl", 0)
				}
			case "lstStyle":
				inLstStyle = true
			case "buChar":
				if inLstStyle && styleLevel >= 0 && curShape != nil {
					setPptxDefaultBullet(curShape, styleLevel, pptxBulletChar)
				} else if inPPr && curPara != nil {
					curPara.bullet = pptxBulletChar
				}
			case "buAutoNum":
				if inLstStyle && styleLevel >= 0 && curShape != nil {
					setPptxDefaultBullet(curShape, styleLevel, pptxBulletAutoNum)
				} else if inPPr && curPara != nil {
					curPara.bullet = pptxBulletAutoNum
				}
			case "buNone":
				if inLstStyle && styleLevel >= 0 && curShape != nil {
					setPptxDefaultBullet(curShape, styleLevel, pptxBulletOff)
				} else if inPPr && curPara != nil {
					curPara.bullet = pptxBulletOff
				}
			case "br":
				// a:br 软换行 → \n（任务定义的文本抽取口径）
				if curPara != nil && !inPPr {
					curPara.text += "\n"
				}
			case "tab":
				// a:tab 制表符 → \t
				if curPara != nil && !inPPr {
					curPara.text += "\t"
				}
			case "t":
				// a:t 文本运行
				inT = true
			default:
				if inLstStyle {
					if level, ok := pptxStyleLevel(tok.Name.Local); ok {
						styleLevel = level
					}
				}
			}
		case xml.EndElement:
			switch tok.Name.Local {
			case "sp", "graphicFrame", "pic":
				finishPara()
				if curShape != nil && curShape.kind != pptxShapeSkip {
					applyPptxGroupTransforms(curShape, groupStack)
					shapes = append(shapes, curShape)
				}
				curShape = nil
			case "tbl":
				if curShape != nil {
					curShape.table = curTable
				}
				curTable = nil
			case "tr":
				if len(curRow) > 0 && curTable != nil {
					curTable.rows = append(curTable.rows, curRow)
				}
				curRow = nil
			case "tc":
				if curCell != nil {
					curCell.text = strings.TrimSpace(curCell.text)
					curRow = append(curRow, *curCell)
				}
				curCell = nil
				inCell = false
			case "p":
				finishPara()
			case "pPr":
				inPPr = false
			case "lstStyle":
				inLstStyle = false
				styleLevel = -1
			case "grpSpPr":
				inGroupProps = false
			case "grpSp":
				if len(groupStack) > 0 {
					groupStack = groupStack[:len(groupStack)-1]
				}
			case "t":
				inT = false
			default:
				if inLstStyle {
					if _, ok := pptxStyleLevel(tok.Name.Local); ok {
						styleLevel = -1
					}
				}
			}
		case xml.CharData:
			// 仅收集 a:t 内文本（元素间格式空白不污染段落文本）
			if inT && curPara != nil && !inPPr {
				curPara.text += string(tok)
			}
		}
	}
	return shapes
}

// sortPptxShapes 形状按视觉位置排序（对齐源码 _iter_shapes_by_position）：
// 先按 top 升序（缺几何的用极大值殿后、组内保持原相对顺序），
// 相邻 top 差在容差内（链式比较前一个形状，行可跨多个容差步）归入同一行，
// 行内按 left 升序、left 同值按原顺序输出。
func sortPptxShapes(shapes []*pptxShape) []*pptxShape {
	const fallbackPos = float64(1 << 62)
	type shapeEntry struct {
		idx       int
		shape     *pptxShape
		top, left float64
	}
	entries := make([]shapeEntry, 0, len(shapes))
	for i, sh := range shapes {
		top, left := fallbackPos, fallbackPos
		if sh.hasGeom() {
			top, left = sh.top, sh.left
		}
		entries = append(entries, shapeEntry{idx: i, shape: sh, top: top, left: left})
	}
	sort.SliceStable(entries, func(a, b int) bool {
		if entries[a].top != entries[b].top {
			return entries[a].top < entries[b].top
		}
		return entries[a].idx < entries[b].idx
	})

	// 链式分行：与前一形状的 top 比较（非行首锚点），容差内的连续形状同一行
	var rows [][]shapeEntry
	var curRow []shapeEntry
	var prevTop float64
	for _, e := range entries {
		if len(curRow) == 0 || e.top-prevTop <= pptxShapeRowToleranceEMU {
			curRow = append(curRow, e)
		} else {
			rows = append(rows, curRow)
			curRow = []shapeEntry{e}
		}
		prevTop = e.top
	}
	if len(curRow) > 0 {
		rows = append(rows, curRow)
	}

	out := make([]*pptxShape, 0, len(shapes))
	for _, row := range rows {
		sort.SliceStable(row, func(a, b int) bool {
			if row[a].left != row[b].left {
				return row[a].left < row[b].left
			}
			return row[a].idx < row[b].idx
		})
		for _, e := range row {
			out = append(out, e.shape)
		}
	}
	return out
}

// handlePptxTextShape 产出文本形状的段落元素（对齐源码 _handle_text_elements）：
// prov 按 shape 级几何生成一次、各段落共用（charspan 为 shape 全文本长度）；
// title/ctrTitle 占位符 → title，其余 → paragraph；
// 列表段落（buChar/buAutoNum）按 lvl 组织层级列表分组，连续同组共享，
// 有序项 marker 自增；非列表段落中断当前列表组。
func handlePptxTextShape(doc *DoclingDocument, sh *pptxShape, group RefItem, slideInd int64, slideW, slideH float64) {
	// shape 级全文本为空白时整形状跳过（对齐源码 len(shape.text.strip())==0 分支）
	shapeText := strings.TrimSpace(pptxAllParaText(sh))
	if shapeText == "" {
		return
	}
	prov := makePptxProv(sh.hasGeom(), sh.left, sh.top, sh.width, sh.height,
		slideInd, int64(utf8.RuneCountInString(shapeText)), slideW, slideH)

	// 列表层级栈：stack[i] 对应段落 lvl=i 的列表组（lvl=0 挂 slide 分组，
	// lvl>0 的子组挂上一列表项下）；非列表段落清栈中断列表。
	type pptxListLevel struct {
		group    RefItem // 该层列表分组
		counter  int64   // 该层有序项计数器（新建组时归零）
		lastItem RefItem // 该层最后一个列表项（子列表的挂载父节点）
	}
	var stack []pptxListLevel

	isTitle := sh.isPlaceholder && (sh.phType == "title" || sh.phType == "ctrTitle")
	for _, para := range sh.paras {
		if strings.TrimSpace(para.text) == "" {
			continue // 空段落不产出（简化边界，见文件头注释）
		}
		bullet := para.bullet
		if bullet == pptxBulletNone {
			bullet = sh.defaultBullets[para.level]
		}
		if bullet != pptxBulletChar && bullet != pptxBulletAutoNum {
			// 非列表段落：中断列表组，按 label 产出普通文本
			stack = nil
			label := LabelParagraph
			if isTitle {
				label = LabelTitle
			}
			doc.AddText(label, para.text, prov, &group)
			continue
		}
		// 列表段落：lvl 与栈深对齐——深层新建子组挂上一列表项下，
		// 浅层截断栈回退复用旧组（同层连续段落共享组、计数器延续自增）
		lvl := para.level
		if lvl < 0 {
			lvl = 0
		}
		if int(len(stack)) > int(lvl)+1 {
			stack = stack[:int(lvl)+1]
		}
		for len(stack) <= int(lvl) {
			// 子列表挂上一列表项下；上层尚无列表项（如首段即深层）时回退 slide 分组
			var parent *RefItem
			if len(stack) == 0 || stack[len(stack)-1].lastItem.Kind != refTexts {
				parent = &group
			} else {
				p := stack[len(stack)-1].lastItem
				parent = &p
			}
			g := doc.AddListGroup("list", parent)
			stack = append(stack, pptxListLevel{group: g})
		}
		// 有序项 marker 从 "1." 自增（新建组后计数器已归零）；无序项 marker 为空
		lv := &stack[int(lvl)]
		enumerated := bullet == pptxBulletAutoNum
		marker := ""
		if enumerated {
			lv.counter++
			marker = fmt.Sprintf("%d.", lv.counter)
		}
		lv.lastItem = doc.AddListItem(lv.group, para.text, enumerated, marker, prov)
	}
}

// handlePptxOfficeObject 把 SmartArt、艺术字与 OLE 对象转换为 PictureItem。
// SmartArt 文本从 diagramData 关系部件提取；OLE 仅保留程序、关系和目标，
// 从不执行嵌入内容。
func handlePptxOfficeObject(doc *DoclingDocument, sh *pptxShape, group RefItem, slideInd int64, slideW, slideH float64, diagramTargets, oleTargets map[string]string, reader *zip.Reader) {
	if sh.officeObject == nil {
		return
	}
	record := *sh.officeObject
	if record.name == "" {
		record.name = sh.name
	}
	switch officeObjectClassName(record.kind) {
	case "smartart":
		record.target = diagramTargets[record.relationship]
		if data, ok := readPptxZipFile(reader, record.target); ok {
			record.text = collectDrawingMLText(data)
		}
	case "wordart":
		record.text = strings.TrimSpace(pptxAllParaText(sh))
	case "embedded_object":
		record.target = oleTargets[record.relationship]
	}
	textLen := int64(utf8.RuneCountInString(officeObjectCaption(record)))
	prov := makePptxProv(sh.hasGeom(), sh.left, sh.top, sh.width, sh.height, slideInd, textLen, slideW, slideH)
	addOfficeObjectPicture(doc, record, prov, &group, LayerBody)
}

// handlePptxTableShape 产出 graphicFrame 内的表格（对齐源码 _handle_tables）：
// num_rows/num_cols 按含跨列的全尺寸声明；空文本 cell 丢弃不产 cell；
// 首行 column_header；全空表格不建；prov 的 charspan 为 [0,0]。
// vMerge 跨行延续格无独立文本，经空文本丢弃规则自然滤除。
func handlePptxTableShape(doc *DoclingDocument, sh *pptxShape, group RefItem, slideInd int64, slideW, slideH float64, chartTargets map[string]string, reader *zip.Reader) {
	if sh.table == nil {
		// 图表 graphicFrame 复用共享 OOXML 图表解析器，锚点沿用 frame 几何。
		if target, ok := chartTargets[sh.chartRID]; ok {
			prov := makePptxProv(sh.hasGeom(), sh.left, sh.top, sh.width, sh.height, slideInd, 0, slideW, slideH)
			addOOXMLChartPictureFromPart(doc, reader, target, nil, prov, &group)
		}
		return
	}
	var cells []DoclingTableCell
	numRows := int64(len(sh.table.rows))
	numCols := int64(0)
	for r, row := range sh.table.rows {
		// 行宽 = 行内各格 gridSpan 之和，取最大值声明 num_cols
		rowWidth := int64(0)
		for _, c := range row {
			rowWidth += c.colSpan
		}
		if rowWidth > numCols {
			numCols = rowWidth
		}
		// 列号按 gridSpan 展开累计推进，保证 start 闭 end 开区间在网格上不重叠
		colIdx := int64(0)
		for _, c := range row {
			if strings.TrimSpace(c.text) != "" {
				cells = append(cells, DoclingTableCell{
					Text:              c.text,
					RowSpan:           c.rowSpan,
					ColSpan:           c.colSpan,
					StartRowOffsetIdx: int64(r),
					EndRowOffsetIdx:   int64(r) + c.rowSpan,
					StartColOffsetIdx: colIdx,
					EndColOffsetIdx:   colIdx + c.colSpan,
					ColumnHeader:      r == 0,
				})
			}
			colIdx += c.colSpan
		}
	}
	if len(cells) == 0 {
		return // 全空表格不建（对齐源码 len(tcells)>0 判定）
	}
	prov := makePptxProv(sh.hasGeom(), sh.left, sh.top, sh.width, sh.height,
		slideInd, 0, slideW, slideH)
	doc.AddTable(cells, numRows, numCols, prov, &group)
}

// handlePptxNotes 提取 slide 关联的备注文本（对齐源码 slide.has_notes_slide 分支）：
// 经 slide 关系表定位 notesSlide 部件，取 body 占位符文本（对齐
// notes_text_frame 语义），产出普通 text 元素挂 slide 分组；
// prov bbox 全 0（源码行为），charspan=[0, 文本长度]。
func handlePptxNotes(doc *DoclingDocument, reader *zip.Reader, slidePath string, group RefItem, slideInd int64) {
	// slide 关系文件：ppt/slides/_rels/slideN.xml.rels，Target 相对 slide 所在目录解析
	dir, name := path.Split(slidePath)
	relsXML, ok := readPptxZipFile(reader, path.Join(dir, "_rels", name+".rels"))
	if !ok {
		return
	}
	targets := parseOOXMLRelationships(relsXML, "/notesSlide", path.Clean(dir))
	if len(targets) == 0 {
		return
	}
	// 备注关系至多一条；存在多条时取任一即可
	var notesXML []byte
	var found bool
	for _, t := range targets {
		notesXML, found = readPptxZipFile(reader, t)
		if found {
			break
		}
	}
	if !found {
		return
	}
	notesText := parsePptxNotesText(notesXML)
	if strings.TrimSpace(notesText) == "" {
		return
	}
	prov := ProvenanceItem{
		PageNo:   slideInd + 1,
		BBox:     &DoclingBBox{},
		CharSpan: [2]int64{0, int64(utf8.RuneCountInString(notesText))},
	}
	ref := doc.AddText(LabelText, notesText, []ProvenanceItem{prov}, &group)
	doc.Texts[ref.Idx].ContentLayer = LayerNotes
}

// pptxComment 保存 PowerPoint 旧式或现代 comments 部件中的批注文本、协作
// 元数据和锚点坐标。
type pptxComment struct {
	id, parentID     string
	author           officeCommentAuthor
	created, status  string
	resolved         *bool
	assignedTo       []string
	startDate        string
	dueDate          string
	completion       string
	title            string
	text             string
	x, y             float64
	hasPos, isModern bool
}

// handlePptxComments 把幻灯片批注写入 NOTES 层，并把最近正文项的 comments
// 字段指向该批注。找不到正文目标时仍保留批注文本，避免协作信息丢失。
func handlePptxComments(doc *DoclingDocument, reader *zip.Reader, slidePath string, group RefItem, slideInd int64, slideRelsXML []byte, authors map[string]officeCommentAuthor) {
	targets := parseOOXMLRelationships(slideRelsXML, "/comments", path.Dir(slidePath))
	for _, commentsPath := range targets {
		commentsXML, ok := readPptxZipFile(reader, commentsPath)
		if !ok {
			continue
		}
		comments := parsePptxComments(commentsXML, authors)
		if len(comments) == 0 {
			continue
		}
		if comments[0].isModern {
			appendPptxModernComments(doc, comments, group, slideInd)
			continue
		}
		for _, comment := range comments {
			if strings.TrimSpace(comment.text) == "" {
				continue
			}
			target := nearestPptxCommentTarget(doc, slideInd+1, comment.x, comment.y, comment.hasPos)
			bbox := &DoclingBBox{CoordOrigin: CoordOriginBottomLeft}
			if comment.hasPos {
				bbox.L, bbox.T, bbox.R, bbox.B = comment.x, comment.y, comment.x, comment.y
			}
			prov := []ProvenanceItem{{
				PageNo:   slideInd + 1,
				BBox:     bbox,
				CharSpan: [2]int64{0, int64(utf8.RuneCountInString(comment.text))},
			}}
			commentRef := doc.AddText(LabelText, comment.text, prov, &group)
			doc.Texts[commentRef.Idx].ContentLayer = LayerNotes
			doc.Texts[commentRef.Idx].Meta = officeCommentMeta(officeCommentData{
				ID: comment.id, Author: comment.author, Created: comment.created,
			})
			if target != nil {
				appendPptxCommentRef(doc, *target, FineRef{RefItem: commentRef})
			}
		}
	}
}

// parsePptxCommentAuthors 从 presentation 关系读取旧式 commentAuthors 和
// Office 2021+ authors 部件。
func parsePptxCommentAuthors(reader *zip.Reader, presentationRelsXML []byte) map[string]officeCommentAuthor {
	authors := map[string]officeCommentAuthor{}
	for _, suffix := range []string{"/commentAuthors", "/authors"} {
		for _, target := range parseOOXMLRelationships(presentationRelsXML, suffix, path.Dir(pptxPresentationPath)) {
			data, ok := readPptxZipFile(reader, target)
			if !ok {
				continue
			}
			dec := xml.NewDecoder(bytes.NewReader(data))
			for {
				token, err := dec.Token()
				if err != nil {
					break
				}
				start, ok := token.(xml.StartElement)
				if !ok || (start.Name.Local != "cmAuthor" && start.Name.Local != "author") {
					continue
				}
				author := officeCommentAuthor{
					ID: xmlAttrVal(start.Attr, "id"), Name: xmlAttrVal(start.Attr, "name"),
					Initials: xmlAttrVal(start.Attr, "initials"), UserID: xmlAttrVal(start.Attr, "userId"),
					ProviderID: xmlAttrVal(start.Attr, "providerId"),
				}
				if author.ID != "" {
					authors[author.ID] = author
				}
			}
		}
	}
	return authors
}

// parsePptxComments 解析旧式 p:cmLst 或现代 p188:cmLst 批注集合。
func parsePptxComments(xmlBytes []byte, authors map[string]officeCommentAuthor) []pptxComment {
	if pptxModernCommentPart(xmlBytes) {
		return parsePptxModernComments(xmlBytes, authors)
	}
	return parsePptxLegacyComments(xmlBytes, authors)
}

// pptxModernCommentPart 根据根命名空间识别 Office 2021+ 批注部件。
func pptxModernCommentPart(xmlBytes []byte) bool {
	dec := xml.NewDecoder(bytes.NewReader(xmlBytes))
	for {
		token, err := dec.Token()
		if err != nil {
			return false
		}
		if start, ok := token.(xml.StartElement); ok {
			return start.Name.Local == "cmLst" && strings.Contains(start.Name.Space, "office/powerpoint/2018/8/main")
		}
	}
}

// parsePptxLegacyComments 保持旧式 p:cmLst 的输出和坐标关联行为。
func parsePptxLegacyComments(xmlBytes []byte, authors map[string]officeCommentAuthor) []pptxComment {
	decoder := xml.NewDecoder(bytes.NewReader(xmlBytes))
	var comments []pptxComment
	var current *pptxComment
	inText := false
	for {
		token, err := decoder.Token()
		if err != nil {
			break
		}
		switch tok := token.(type) {
		case xml.StartElement:
			switch tok.Name.Local {
			case "cm":
				authorID := xmlAttrVal(tok.Attr, "authorId")
				current = &pptxComment{
					id: xmlAttrVal(tok.Attr, "idx"), author: authors[authorID], created: xmlAttrVal(tok.Attr, "dt"),
				}
				if current.author.ID == "" {
					current.author.ID = authorID
				}
			case "pos":
				if current != nil {
					current.x = pptxAttrFloat(tok.Attr, "x")
					current.y = pptxAttrFloat(tok.Attr, "y")
					current.hasPos = true
				}
			case "text":
				inText = current != nil
			}
		case xml.CharData:
			if current != nil && inText {
				current.text += string(tok)
			}
		case xml.EndElement:
			switch tok.Name.Local {
			case "text":
				inText = false
			case "cm":
				if current != nil {
					current.text = strings.TrimSpace(current.text)
					comments = append(comments, *current)
				}
				current = nil
			}
		}
	}
	return comments
}

// pptxModernCommentXML 是现代 cm/reply 共享的 XML 子集。
type pptxModernCommentXML struct {
	ID         string `xml:"id,attr"`
	AuthorID   string `xml:"authorId,attr"`
	Created    string `xml:"created,attr"`
	Status     string `xml:"status,attr"`
	StartDate  string `xml:"startDate,attr"`
	DueDate    string `xml:"dueDate,attr"`
	AssignedTo string `xml:"assignedTo,attr"`
	Completion string `xml:"complete,attr"`
	Title      string `xml:"title,attr"`
	Pos        *struct {
		X float64 `xml:"x,attr"`
		Y float64 `xml:"y,attr"`
	} `xml:"pos"`
	TextBody struct {
		Paragraphs []struct {
			Runs []struct {
				Text string `xml:"t"`
			} `xml:"r"`
			Fields []struct {
				Text string `xml:"t"`
			} `xml:"fld"`
		} `xml:"p"`
	} `xml:"txBody"`
	Replies []pptxModernCommentXML `xml:"replyLst>reply"`
}

// text 返回现代批注 TextBody 的可见文字，段落以换行连接。
func (comment pptxModernCommentXML) text() string {
	paragraphs := make([]string, 0, len(comment.TextBody.Paragraphs))
	for _, paragraph := range comment.TextBody.Paragraphs {
		var text strings.Builder
		for _, run := range paragraph.Runs {
			text.WriteString(run.Text)
		}
		for _, field := range paragraph.Fields {
			text.WriteString(field.Text)
		}
		paragraphs = append(paragraphs, text.String())
	}
	return strings.TrimSpace(strings.Join(paragraphs, "\n"))
}

// parsePptxModernComments 解析现代批注及一层 replyLst 回复。
func parsePptxModernComments(xmlBytes []byte, authors map[string]officeCommentAuthor) []pptxComment {
	var root struct {
		Comments []pptxModernCommentXML `xml:"cm"`
	}
	if err := xml.Unmarshal(xmlBytes, &root); err != nil {
		return nil
	}
	var comments []pptxComment
	for _, item := range root.Comments {
		comment := makePptxModernComment(item, "", authors)
		if comment.text == "" {
			continue
		}
		if item.Pos != nil {
			comment.x, comment.y, comment.hasPos = item.Pos.X, item.Pos.Y, true
		}
		comments = append(comments, comment)
		for _, reply := range item.Replies {
			replyComment := makePptxModernComment(reply, item.ID, authors)
			if replyComment.text != "" {
				comments = append(comments, replyComment)
			}
		}
	}
	return comments
}

// makePptxModernComment 把现代 cm/reply XML 转为共享批注中间表示。
func makePptxModernComment(item pptxModernCommentXML, parentID string, authors map[string]officeCommentAuthor) pptxComment {
	author := authors[item.AuthorID]
	if author.ID == "" {
		author.ID = item.AuthorID
	}
	return pptxComment{
		id: item.ID, parentID: parentID, author: author, created: item.Created, status: item.Status,
		resolved: officeResolvedStatus(item.Status), assignedTo: strings.Fields(item.AssignedTo),
		startDate: item.StartDate, dueDate: item.DueDate, completion: item.Completion, title: item.Title,
		text: item.text(), isModern: true,
	}
}

// appendPptxModernComments 把现代批注根节点与回复统一挂到 comment_section。
func appendPptxModernComments(doc *DoclingDocument, comments []pptxComment, slideGroup RefItem, slideInd int64) {
	groups := map[string]RefItem{}
	for _, comment := range comments {
		if comment.parentID != "" {
			continue
		}
		group := GroupItem{
			Parent: &slideGroup, Children: []RefItem{}, ContentLayer: LayerNotes,
			Label: GroupLabelCommentSection, Name: "comment-slide" + strconv.FormatInt(slideInd+1, 10) + "-" + comment.id,
			Meta: officeCommentMeta(officeCommentData{
				ID: comment.id, Author: comment.author, Created: comment.created, Status: comment.status,
				Resolved: comment.resolved, AssignedTo: comment.assignedTo, StartDate: comment.startDate,
				DueDate: comment.dueDate, Completion: comment.completion, Title: comment.title,
			}),
		}
		doc.Groups = append(doc.Groups, group)
		groupRef := RefItem{Kind: refGroups, Idx: int64(len(doc.Groups) - 1)}
		doc.Groups[groupRef.Idx].SelfRef = groupRef.String()
		doc.appendChild(&slideGroup, groupRef)
		groups[comment.id] = groupRef
	}
	for _, comment := range comments {
		rootID := comment.id
		if comment.parentID != "" {
			rootID = comment.parentID
		}
		groupRef, ok := groups[rootID]
		if !ok {
			continue
		}
		bbox := &DoclingBBox{CoordOrigin: CoordOriginBottomLeft}
		if comment.hasPos {
			bbox.L, bbox.T, bbox.R, bbox.B = comment.x, comment.y, comment.x, comment.y
		}
		prov := []ProvenanceItem{{
			PageNo: slideInd + 1, BBox: bbox,
			CharSpan: [2]int64{0, int64(utf8.RuneCountInString(comment.text))},
		}}
		noteRef := doc.AddText(LabelText, comment.text, prov, &groupRef)
		doc.Texts[noteRef.Idx].ContentLayer = LayerNotes
		doc.Texts[noteRef.Idx].Meta = officeCommentMeta(officeCommentData{
			ID: comment.id, ParentID: comment.parentID, Author: comment.author,
			Created: comment.created, Status: comment.status, Resolved: comment.resolved,
			AssignedTo: comment.assignedTo, StartDate: comment.startDate, DueDate: comment.dueDate,
			Completion: comment.completion, Title: comment.title,
		})
		if comment.parentID == "" {
			target := nearestPptxCommentTarget(doc, slideInd+1, comment.x, comment.y, comment.hasPos)
			if target != nil {
				appendPptxCommentRef(doc, *target, FineRef{RefItem: groupRef})
			}
		}
	}
}

// nearestPptxCommentTarget 按批注锚点与正文项 bbox 中心距离选择最近目标。
func nearestPptxCommentTarget(doc *DoclingDocument, pageNo int64, x, y float64, hasPos bool) *RefItem {
	bestDistance := float64(1 << 62)
	var best *RefItem
	consider := func(ref RefItem, layer ContentLayer, prov []ProvenanceItem) {
		if layer != LayerBody || len(prov) == 0 || prov[0].PageNo != pageNo || prov[0].BBox == nil {
			return
		}
		distance := 0.0
		if hasPos {
			bbox := prov[0].BBox
			centerX, centerY := (bbox.L+bbox.R)/2, (bbox.T+bbox.B)/2
			dx, dy := centerX-x, centerY-y
			distance = dx*dx + dy*dy
		}
		if best == nil || distance < bestDistance {
			candidate := ref
			best = &candidate
			bestDistance = distance
		}
	}
	for i := range doc.Texts {
		consider(RefItem{Kind: refTexts, Idx: int64(i)}, doc.Texts[i].ContentLayer, doc.Texts[i].Prov)
	}
	for i := range doc.Tables {
		consider(RefItem{Kind: refTables, Idx: int64(i)}, doc.Tables[i].ContentLayer, doc.Tables[i].Prov)
	}
	for i := range doc.Pictures {
		consider(RefItem{Kind: refPictures, Idx: int64(i)}, doc.Pictures[i].ContentLayer, doc.Pictures[i].Prov)
	}
	return best
}

// appendPptxCommentRef 把批注引用写入支持 comments 字段的 DocItem。
func appendPptxCommentRef(doc *DoclingDocument, target RefItem, comment FineRef) {
	switch target.Kind {
	case refTexts:
		doc.Texts[target.Idx].Comments = append(doc.Texts[target.Idx].Comments, comment)
	case refTables:
		doc.Tables[target.Idx].Comments = append(doc.Tables[target.Idx].Comments, comment)
	case refPictures:
		doc.Pictures[target.Idx].Comments = append(doc.Pictures[target.Idx].Comments, comment)
	}
}

// parsePptxNotesText 提取 notesSlide 中 body 占位符形状的文本
// （无 body 占位符时返回空串，对齐 notes_text_frame 为 None 的行为）。
func parsePptxNotesText(xmlBytes []byte) string {
	var notesText string
	for _, sh := range parsePptxSlideShapes(xmlBytes) {
		if sh.isPlaceholder && sh.phType == "body" {
			notesText = pptxAllParaText(sh)
			break
		}
	}
	return notesText
}

// pptxShapeText 取形状全文本（段落间 \n 连接，对齐 python-pptx shape.text）。
func pptxAllParaText(sh *pptxShape) string {
	texts := make([]string, 0, len(sh.paras))
	for _, p := range sh.paras {
		texts = append(texts, p.text)
	}
	return strings.Join(texts, "\n")
}

// makePptxProv 生成元素来源证据（对齐源码 _generate_prov）：
// 有几何时 bbox=[l,t,l+w,t+h]（EMU 原始值），否则回退整页 (0,0,slideW,slideH)；
// 坐标原点 BOTTOMLEFT（pptx 页面坐标自底向上）；页号 1 起；
// charspan=[0,textLen]，表格/图片传 0 即 [0,0]。
func makePptxProv(hasGeom bool, left, top, width, height float64, slideInd, textLen int64, slideW, slideH float64) []ProvenanceItem {
	l, t, r, b := 0.0, 0.0, slideW, slideH
	if hasGeom {
		l, t = left, top
		r, b = left+width, top+height
	}
	return []ProvenanceItem{{
		PageNo:   slideInd + 1,
		BBox:     &DoclingBBox{L: l, T: t, R: r, B: b, CoordOrigin: CoordOriginBottomLeft},
		CharSpan: [2]int64{0, textLen},
	}}
}

// readPptxZipFile 按 zip 内路径读取部件内容；不存在或读取失败返回 (nil, false)。
func readPptxZipFile(reader *zip.Reader, name string) ([]byte, bool) {
	for _, f := range reader.File {
		if f.Name != name {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return nil, false
		}
		content, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return nil, false
		}
		return content, true
	}
	return nil, false
}

// pptxAttr 取无前缀属性值（缺省返回空串）。
func pptxAttr(attrs []xml.Attr, local string) string {
	for _, a := range attrs {
		if a.Name.Local == local && a.Name.Space == "" {
			return a.Value
		}
	}
	return ""
}

// pptxAttrFloat 取无前缀数值属性，缺省或非法返回 0。
func pptxAttrFloat(attrs []xml.Attr, local string) float64 {
	v, _ := strconv.ParseFloat(pptxAttr(attrs, local), 64)
	return v
}

// pptxAttrInt64 取无前缀整型属性，缺省或非法返回 def。
func pptxAttrInt64(attrs []xml.Attr, local string, def int64) int64 {
	if v, err := strconv.ParseInt(pptxAttr(attrs, local), 10, 64); err == nil {
		return v
	}
	return def
}

// pptxAttrRel 取 relationships 命名空间的 id 属性（r:id 引用）。
func pptxAttrRel(attrs []xml.Attr) string {
	for _, a := range attrs {
		if a.Name.Space == pptxOfficeRelNS && a.Name.Local == "id" {
			return a.Value
		}
	}
	return ""
}

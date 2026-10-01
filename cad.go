// cad.go 实现 CAD 图纸（DWG/DXF）输入解析为 DoclingDocument：以库依赖
// 方式引入 github.com/unitedrhino/go-cad（同进程纯 Go，不走 CLI 子进程），
// 流程为格式识别 → go-cad 解析 → 图框识别与渲染 → 文本按图框归属产出。
// 文档组织对齐图纸语义：
//   - 每个图框一页（页尺寸=该图框渲染 PNG 的像素尺寸），页内依次为
//     section 分组（组名=图名）、图名 section_header、整幅渲染 PNG 的
//     PictureItem、图框内文本（bbox 由世界坐标近似映射到页面像素）；
//   - go-cad RenderAllSheets 在有图框时首项固定为"整图全览"张，其内容
//     与逐图框张重复，且页号需与 SheetTexts 的图框序号对齐，故不作为
//     页面产出；
//   - 图框外文本（SheetText.Sheet==0）无对应图框页，参考 PDF 对页外
//     内容归首页的兜底惯例：prov 归首页（page_no=1 保证指向已登记
//     页面）、置于文档最前（body 直挂、不进图框分组）；
//   - 无图框图纸：DetectSheets 兜底返回的"整图"单张直接作为唯一页，
//     全部文本归该页（go-cad 此时全部文本 Sheet=0，与兜底单张同页）。
//
// 文本由矢量层直接提取（TEXT/MTEXT/ATTRIB，含 INSERT 块引用展开），
// 无需 OCR 兜底，PDFOptions 中的 OCR 钩子对 CAD 不生效。
package docling

import (
	"bytes"
	"fmt"
	"image"
	"math"
	"path/filepath"
	"strconv"
	"strings"

	cad "github.com/unitedrhino/go-cad"
)

// cadFormat 标识 CAD 字节流的容器家族（DWG 二进制 / DXF 文本或二进制）。
type cadFormat string

// CAD 容器家族取值。
const (
	cadFormatDWG cadFormat = "dwg"
	cadFormatDXF cadFormat = "dxf"
)

// dxfBinarySentinel 是 DXF 二进制变体的文件头哨兵，与 go-cad 内部
// DxfBinaryMagic 同一口径（go-cad 门面包未导出该常量，此处独立声明，
// 修改需与 go-cad 保持同步）。
var dxfBinarySentinel = []byte("AutoCAD Binary DXF\r\n\x1a\x00")

// ParseCAD 解析 CAD 图纸字节为 DoclingDocument（无文件名上下文的便捷
// 版）：MIME 与容器格式按字节特征识别，文档名回退 "cad"。
func ParseCAD(data []byte) (*DoclingDocument, error) {
	return ParseCADWithOptions(data, PDFOptions{})
}

// ParseCADWithOptions 解析 CAD 图纸（DWG/DXF）字节为多页 DoclingDocument：
// 每图框一页（页尺寸=渲染 PNG 像素尺寸），页内为图名标题、整幅渲染 PNG
// PictureItem 与图框内文本；图框外文本归首页并置于文档最前；无图框时
// 单页整图兜底。opt 复用 PDFOptions：仅 MIMEType（默认按格式取
// image/vnd.dwg / image/vnd.dxf）与 Filename（文档主干名来源）生效。
func ParseCADWithOptions(data []byte, opt PDFOptions) (*DoclingDocument, error) {
	format, err := detectCADFormat(data)
	if err != nil {
		return nil, err
	}
	if opt.MIMEType == "" {
		if format == cadFormatDWG {
			opt.MIMEType = "image/vnd.dwg"
		} else {
			opt.MIMEType = "image/vnd.dxf"
		}
	}
	var cdoc *cad.Document
	if format == cadFormatDWG {
		cdoc, err = cad.Parse(data)
	} else {
		cdoc, err = cad.ParseDXF(data)
	}
	if err != nil {
		return nil, fmt.Errorf("docling: CAD %s 解析失败: %w", format, err)
	}

	doc := NewDoclingDocument(cadDocumentName(opt.Filename))

	sheets := cad.DetectSheets(cdoc)
	// 兜底整图单张不是真实图框：剔除语义与 go-cad cad.SheetTexts 同口径，
	// 此时唯一渲染项即整图兜底页，全部文本归该页
	fallbackWhole := len(sheets) == 1 && sheets[0].Name == "整图"
	texts := cad.SheetTexts(cdoc)
	results, err := cad.RenderAllSheets(cdoc, cad.RenderOptions{}, "png")
	if err != nil {
		return nil, fmt.Errorf("docling: CAD 渲染失败: %w", err)
	}
	// 有图框时渲染清单首项是"整图全览"张（内容与逐图框重复），跳过使
	// 页号与 SheetTexts 的 1-based 图框序号严格对齐
	pages := results
	if !fallbackWhole && len(pages) > 0 {
		pages = pages[1:]
	}
	if len(pages) != len(sheets) {
		return nil, fmt.Errorf("docling: CAD 渲染张数 %d 与图框数 %d 不一致", len(pages), len(sheets))
	}

	// 图框外文本（仅真实图框场景存在）：归首页、文档最前，bbox 按首页
	// 图框盒近似映射（位置可能落在页面之外，保留真实映射不做钳制）
	if !fallbackWhole && len(pages) > 0 {
		for _, st := range texts {
			if st.Sheet != 0 {
				continue
			}
			prov := cadSheetTextProv(1, st, sheets[0].Box, float64(pages[0].Width), pageHeightOf(pages[0]))
			doc.AddText(LabelText, st.Text, prov, nil)
		}
	}

	for i, res := range pages {
		pageNo := int64(i + 1)
		height := pageHeightOf(res)
		doc.AddPage(pageNo, float64(res.Width), height)
		box := sheets[i].Box
		group := doc.AddSectionGroup(sheets[i].Name, nil)
		prov := []ProvenanceItem{{
			PageNo: pageNo,
			BBox:   &DoclingBBox{L: 0, T: 0, R: float64(res.Width), B: height, CoordOrigin: CoordOriginTopLeft},
		}}
		doc.AddHeading(1, sheets[i].Name, prov, &group)
		doc.AddPicture(&ImageRef{
			Mimetype: "image/png",
			Dpi:      defaultImageDPI,
			Size:     &ImageSize{Width: float64(res.Width), Height: height},
			URI:      mediaToDataURI(".png", res.Data),
		}, prov, &group)
		for _, st := range texts {
			sheetNo := st.Sheet
			if fallbackWhole {
				sheetNo = 1
			}
			if sheetNo != int(pageNo) {
				continue
			}
			doc.AddText(LabelText, st.Text, cadSheetTextProv(pageNo, st, box, float64(res.Width), height), &group)
		}
	}
	doc.Meta = &DocMeta{PageCount: len(pages)}
	return doc, nil
}

// pageHeightOf 取渲染结果的像素高度：SheetResult 只携带宽度，高度按图框
// 宽高比随内容变化，从 PNG 头解码是最可靠来源。go-cad 仅在 png.Encode
// 成功后返回数据，解码失败（产物损坏的不变量破坏）按 1 兜底，不让
// bbox 细节阻塞主流程。
func pageHeightOf(res cad.SheetResult) float64 {
	config, _, err := image.DecodeConfig(bytes.NewReader(res.Data))
	if err != nil || config.Height <= 0 {
		return 1
	}
	return float64(config.Height)
}

// cadDocumentName 从文件名提取文档主干名（go-cad 图名之外的文档级命名，
// 与 ParseByExt 链路 applyDocumentOrigin 的 stem 口径一致）；无文件名
// 或无主干时回退 "cad"。
func cadDocumentName(filename string) string {
	base := filepath.Base(strings.TrimSpace(filename))
	stem := strings.TrimSuffix(base, filepath.Ext(base))
	if strings.TrimSpace(stem) == "" {
		return "cad"
	}
	return stem
}

// cadSheetTextProv 把图纸文本的世界坐标近似映射为页面像素 bbox：渲染
// 视口在图框包围盒外扩 2% 边距（go-cad 内部口径，常量未导出），此处按
// 图框盒直接线性映射，bbox 存在边距级（约 2%）系统偏差，仅作检索定位
// 参考；Y 轴翻转到 Docling 默认 TOPLEFT 原点，插入点为基线左端。文本
// 宽度按 CJK 全角/半角字宽估算（真实字形宽依赖渲染字体链，无法精确
// 复原）。图框盒退化（零宽高/非有限值）时退化为仅页号。
func cadSheetTextProv(pageNo int64, st cad.SheetText, box [4]float64, pageW, pageH float64) []ProvenanceItem {
	worldW := box[2] - box[0]
	worldH := box[3] - box[1]
	if worldW <= 0 || worldH <= 0 || math.IsNaN(worldW) || math.IsInf(worldW, 0) ||
		math.IsNaN(st.X) || math.IsInf(st.X, 0) || math.IsNaN(st.Y) || math.IsInf(st.Y, 0) {
		return []ProvenanceItem{{PageNo: pageNo}}
	}
	scaleX := pageW / worldW
	scaleY := pageH / worldH
	// 异常字高（0/负/非有限）按图框世界高度的 1% 兜底，保证 bbox 可用
	textH := st.Height
	if textH <= 0 || math.IsNaN(textH) || math.IsInf(textH, 0) {
		textH = worldH * 0.01
	}
	px := (st.X - box[0]) * scaleX
	py := (box[3] - st.Y) * scaleY
	textW := cadTextWidthEstimate(st.Text) * textH * scaleY
	return []ProvenanceItem{{
		PageNo: pageNo,
		BBox: &DoclingBBox{
			L:           px,
			T:           py - textH*scaleY,
			R:           px + textW,
			B:           py,
			CoordOrigin: CoordOriginTopLeft,
		},
	}}
}

// cadTextWidthEstimate 按字符宽度估算文本显示宽度（以字高为单位）：
// CJK/全角字符计 1.0，其余计 0.55（拉丁比例字体的粗略中值）。
func cadTextWidthEstimate(text string) float64 {
	width := 0.0
	for _, r := range text {
		if r > 0x2E7F { // CJK 统一表意区起点之后按全角计（含兼容区/扩展 A）
			width++
			continue
		}
		width += 0.55
	}
	if width < 1 {
		width = 1
	}
	return width
}

// detectCADFormat 识别 CAD 容器格式：DWG 魔数（文件头 6 字节版本串）
// 优先——二进制格式前缀唯一可靠；未命中再看 DXF 二进制哨兵与 ASCII
// 特征（组码 0 + SECTION 起始段，或 999 注释行）。三者皆不匹配返回
// 明确错误，由调用方拒绝而非猜测分发。
func detectCADFormat(data []byte) (cadFormat, error) {
	if len(data) >= 6 && isDWGVersionMagic(string(data[:6])) {
		return cadFormatDWG, nil
	}
	if bytes.HasPrefix(data, dxfBinarySentinel) {
		return cadFormatDXF, nil
	}
	if looksLikeASCIIDXF(data) {
		return cadFormatDXF, nil
	}
	return "", fmt.Errorf("docling: 无法识别 CAD 格式：既非 DWG 版本串也非 DXF（ASCII/二进制）特征")
}

// isDWGVersionMagic 判断 6 字节文件头是否为 DWG 版本串：AC1002~AC1032
// 数字串覆盖 R2.5~R2018 全部主流容器（区间存在官方未使用的空洞，识别
// 从宽、解析从严）；更早版本为点分短串（AC1.2/AC1.40/AC1.50/AC2.10，
// 微 Cad 时代另有 MC0.0），串长不足 6 字节以 NUL 补齐，先剥离再前缀匹配。
func isDWGVersionMagic(head string) bool {
	if strings.HasPrefix(head, "AC10") {
		n, err := strconv.Atoi(head[2:6])
		return err == nil && n >= 1002 && n <= 1032
	}
	legacy := strings.TrimRight(head, "\x00")
	for _, prefix := range []string{"AC1.2", "AC1.4", "AC1.5", "AC2.1", "MC0.0"} {
		if strings.HasPrefix(legacy, prefix) {
			return true
		}
	}
	return false
}

// looksLikeASCIIDXF 判断字节流前部是否具备 ASCII DXF 特征：组码 0 行后
// 紧跟 SECTION 值行（任何 DXF 必以 SECTION 段起始），或出现 999 注释
// 组码行。只扫前 4KB/64 行，避免为明显非 DXF 的大文件做全文扫描；
// 先剥 UTF-8 BOM（部分工具导出 ASCII DXF 时带 BOM，会粘在首个组码
// 行行首导致匹配失败）。
func looksLikeASCIIDXF(data []byte) bool {
	head := bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))
	if len(head) > 4096 {
		head = head[:4096]
	}
	lines := strings.SplitN(string(head), "\n", 64)
	for i := 0; i+1 < len(lines); i++ {
		switch strings.TrimSpace(lines[i]) {
		case "0":
			if strings.TrimSpace(lines[i+1]) == "SECTION" {
				return true
			}
		case "999":
			return true
		}
	}
	return false
}

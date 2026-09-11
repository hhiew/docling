// image.go 实现图片格式输入（PNG/JPEG/BMP/WEBP）解析为 DoclingDocument：
// 魔数识别格式后构造单页文档（一页一图，ImageRef 内嵌 data URI）；
// 无 OCR 时仍产出包含像素尺寸/DPI 的 PictureItem；配置 OCRHook 或兼容的
// PageOCRHook 时，识别结果经清洗校验后由 ParseMarkdown 结构化并入文档。
package docparse

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"

	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/webp"
)

// defaultImageDPI 是图片未携带可解析物理分辨率时使用的屏幕基准 DPI。
const defaultImageDPI int64 = 72

// ParseImage 解析图片字节为 DoclingDocument（无 OCR 钩子的便捷版）。
func ParseImage(data []byte) (*DoclingDocument, error) {
	return ParseImageWithOptions(data, PDFOptions{})
}

// ParseImageWithOptions 解析图片字节为单页 DoclingDocument：魔数检测
// PNG/JPEG/BMP/WEBP（其余格式拒绝）；始终产出单页 PictureItem。新旧 OCR
// 钩子存在时以 pageNo=1 调用，失败或无效不影响图片文档返回。
func ParseImageWithOptions(data []byte, opt PDFOptions) (*DoclingDocument, error) {
	ext := detectImageExt(data)
	if ext == "" {
		return nil, fmt.Errorf("docparse: 不支持的图片格式（仅支持 PNG/JPEG/BMP/WEBP）")
	}
	config, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("docparse: 无法读取图片尺寸: %w", err)
	}
	if config.Width <= 0 || config.Height <= 0 {
		return nil, fmt.Errorf("docparse: 图片尺寸无效: %dx%d", config.Width, config.Height)
	}
	mimeType := ooxmlMediaMime(ext)
	if opt.MIMEType == "" {
		opt.MIMEType = mimeType
	}
	size := &ImageSize{Width: float64(config.Width), Height: float64(config.Height)}
	doc := NewDoclingDocument("image")
	doc.AddPage(1, size.Width, size.Height)
	prov := []ProvenanceItem{{
		PageNo: 1,
		BBox:   &DoclingBBox{L: 0, T: 0, R: size.Width, B: size.Height, CoordOrigin: CoordOriginTopLeft},
	}}
	doc.AddPicture(&ImageRef{
		Mimetype: mimeType,
		Dpi:      defaultImageDPI,
		Size:     size,
		URI:      imageToDataURI(ext, data),
	}, prov, nil)
	doc.Meta = &DocMeta{PageCount: 1}
	if !hasOCRHook(opt) {
		return doc, nil
	}
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

// imageToDataURI 把图片字节编码为 data URI（输入字节本已在内存，
// 不做 maxMediaDataURIBytes 上限保护；空字节返回空串）。
func imageToDataURI(ext string, data []byte) string {
	if len(data) == 0 {
		return ""
	}
	return "data:" + ooxmlMediaMime(ext) + ";base64," + base64.StdEncoding.EncodeToString(data)
}

// detectImageExt 按魔数识别图片格式并返回规范扩展名（".png"/".jpg"/
// ".bmp"/".webp"）；无法识别返回空串。
func detectImageExt(data []byte) string {
	switch {
	case len(data) >= 8 && bytes.Equal(data[:8], []byte("\x89PNG\r\n\x1a\n")):
		return ".png"
	case len(data) >= 3 && data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF:
		return ".jpg"
	case len(data) >= 2 && data[0] == 'B' && data[1] == 'M':
		return ".bmp"
	case len(data) >= 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WEBP":
		return ".webp"
	}
	return ""
}

// pdf_image_codecs.go 补齐 pdfcpu 不支持的 PDF 图片编码的纯 Go 解码：
//
//  1. JPEG 2000（/Filter /JPXDecode）：pdfcpu 会把原始 codestream 透传为
//     image/jp2 资产但不解码像素，这里用 go-jpeg2000 解码并重编码 PNG；
//  2. JBIG2（/Filter /JBIG2Decode）：pdfcpu 直接跳过整张图片，这里在单对象
//     提取失败后读取原始段流与可选 /JBIG2Globals 全局段，用 gobig2 解码为
//     灰度 PNG 并构造提取结果。
//
// 两条路径失败时均维持既有行为（透传资产或跳过图片），交由整页结构化视觉
// 兜底，不放大故障面。
package docparse

import (
	"bytes"
	"image/png"

	"github.com/dkrisman/gobig2"
	"github.com/mrjoshuak/go-jpeg2000"
	pdfcpumodel "github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
)

// decodePDFJPXImage 把 pdfcpu 透传的 JPEG 2000 图片解码为 PNG。仅处理
// image/jp2 资产；解码失败或像素超出安全上限时原样返回，不影响提取流程。
func decodePDFJPXImage(decoded pdfExtractedImage) pdfExtractedImage {
	if decoded.MIMEType != "image/jp2" || len(decoded.Data) == 0 {
		return decoded
	}
	source, err := jpeg2000.Decode(bytes.NewReader(decoded.Data))
	if err != nil {
		return decoded
	}
	bounds := source.Bounds()
	if bounds.Dx() <= 0 || bounds.Dy() <= 0 ||
		bounds.Dx() > pdfMaxExtractDimension || bounds.Dy() > pdfMaxExtractDimension {
		return decoded
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, source); err != nil {
		return decoded
	}
	decoded.Data = buf.Bytes()
	decoded.MIMEType = "image/png"
	return decoded
}

// decodePDFJBIG2Image 把 pdfcpu 透传的 JBIG2 图片解码为 PNG：段流即提取
// 资产字节，/JBIG2Globals 全局段从图片对象字典解引用。仅处理 image/jbig2
// 资产；解码失败或像素超出安全上限时原样返回，不影响提取流程。
func decodePDFJBIG2Image(context *pdfcpumodel.Context, objectNumber int, decoded pdfExtractedImage) pdfExtractedImage {
	if decoded.MIMEType != "image/jbig2" || len(decoded.Data) == 0 {
		return decoded
	}
	// 第三方解码链路可能 panic，兜底只增强不破坏。
	defer func() { _ = recover() }()
	var globals []byte
	if context != nil && context.Optimize != nil {
		if object := context.Optimize.ImageObjects[objectNumber]; object != nil && object.ImageDict != nil {
			if entry, _ := object.ImageDict.Find("JBIG2Globals"); entry != nil {
				if stream, _, err := context.DereferenceStreamDict(entry); err == nil && stream != nil {
					globals = stream.Raw
				}
			}
		}
	}
	decoder, err := gobig2.NewDecoderEmbedded(bytes.NewReader(decoded.Data), globals)
	if err != nil {
		return decoded
	}
	image, err := decoder.Decode()
	if err != nil {
		return decoded
	}
	bounds := image.Bounds()
	if bounds.Dx() <= 0 || bounds.Dy() <= 0 ||
		bounds.Dx() > pdfMaxExtractDimension || bounds.Dy() > pdfMaxExtractDimension {
		return decoded
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, image); err != nil {
		return decoded
	}
	decoded.Data = buf.Bytes()
	decoded.MIMEType = "image/png"
	return decoded
}

// pdf_smask.go 是低位深（bpc=1/2/4）软蒙版合成的编排层。pdfcpu 提取
// Image XObject 时只应用 bpc=8 的 SMask（源码标注 TODO），低位深软蒙版
// 被静默忽略、图片整体不透明；这里在 pdfcpu 提取结果之上解引用蒙版流，
// 由 internal/pdfenc 完成 alpha 展开与像素合成。
//
// 任何一步失败都保留原提取结果：软蒙版是增强信息，不能让图片整体丢失。
package docling

import (
	pdfcpumodel "github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	pdfcputypes "github.com/pdfcpu/pdfcpu/pkg/pdfcpu/types"

	"github.com/unitedrhino/docling/internal/pdfenc"
)

// applyPDFLowDepthSoftMask 对 pdfcpu 提取的图片应用低位深软蒙版。蒙版
// 缺失、bpc=8（pdfcpu 已处理）或任何解码失败时原样返回。
func applyPDFLowDepthSoftMask(context *pdfcpumodel.Context, objectNumber int, decoded pdfExtractedImage) pdfExtractedImage {
	if context == nil || context.Optimize == nil || len(decoded.Data) == 0 {
		return decoded
	}
	object := context.Optimize.ImageObjects[objectNumber]
	if object == nil || object.ImageDict == nil {
		return decoded
	}
	entry, _ := object.ImageDict.Find("SMask")
	if entry == nil {
		return decoded
	}
	// 第三方解码链路可能 panic，蒙版只增强不破坏，异常时保留原图。
	defer func() { _ = recover() }()
	smask, _, err := context.DereferenceStreamDict(entry)
	if err != nil || smask == nil {
		return decoded
	}
	bpc := smask.IntEntry("BitsPerComponent")
	if bpc == nil || (*bpc != 1 && *bpc != 2 && *bpc != 4) {
		return decoded
	}
	width, height := smask.IntEntry("Width"), smask.IntEntry("Height")
	if width == nil || height == nil || *width <= 0 || *height <= 0 ||
		*width > pdfMaxExtractDimension || *height > pdfMaxExtractDimension {
		return decoded
	}
	if err := smask.Decode(); err != nil {
		return decoded
	}
	alpha := pdfenc.DecodeSoftMaskAlpha(smask.Content, *width, *height, *bpc, pdfSoftMaskInverted(smask.Dict))
	if alpha == nil {
		return decoded
	}
	merged, ok := pdfenc.CompositeSoftMask(decoded.Data, alpha, *width, *height)
	if !ok {
		return decoded
	}
	decoded.Data = merged
	decoded.MIMEType = "image/png"
	return decoded
}

// pdfSoftMaskInverted 判断软蒙版 /Decode 数组是否要求像素反转（[1 0]）。
func pdfSoftMaskInverted(dict pdfcputypes.Dict) bool {
	array := dict.ArrayEntry("Decode")
	if len(array) == 0 || array[0] == nil {
		return false
	}
	first, ok := array[0].(pdfcputypes.Integer)
	return ok && int(first) == 1
}

// pdf_smask.go 补齐 pdfcpu 不支持的低位深（bpc=1/2/4）软蒙版合成。pdfcpu
// 提取 Image XObject 时只应用 bpc=8 的 SMask（源码标注 TODO），低位深软蒙版
// 被静默忽略、图片整体不透明；扫描章、手写签名和抠底照片常受影响。这里在
// pdfcpu 提取结果之上自行解码蒙版流并合成 alpha：
//
//  1. 从图片对象字典解引用 SMask 流，校验 bpc ∈ {1,2,4} 与尺寸安全上限；
//  2. 复用 pdfcpu 的流解码（覆盖 Flate/LZW/Predictor 等）得到蒙版原始像素；
//  3. 按 PDF 低位深行对齐展开为 8 位 alpha，值域归一化并处理 /Decode 反转；
//  4. 蒙版与图像尺寸不一致时最近邻缩放，叠加 alpha 后重新编码为 PNG。
//
// 任何一步失败都保留原提取结果：软蒙版是增强信息，不能让图片整体丢失。
package docparse

import (
	"bytes"
	"image"
	"image/color"
	"image/png"

	pdfcpumodel "github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	pdfcputypes "github.com/pdfcpu/pdfcpu/pkg/pdfcpu/types"
)

// applyPDFLowDepthSoftMask 对 pdfcpu 提取的图片应用低位深软蒙版。蒙版缺失、
// bpc=8（pdfcpu 已处理）或任何解码失败时原样返回。
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
	alpha := decodePDFSoftMaskAlpha(smask.Content, *width, *height, *bpc, pdfSoftMaskInverted(smask.Dict))
	if alpha == nil {
		return decoded
	}
	merged, ok := compositePDFSoftMask(decoded.Data, alpha, *width, *height)
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

// decodePDFSoftMaskAlpha 把低位深蒙版像素展开为 8 位 alpha：每行独立字节
// 对齐（行末 padding 不算像素），值域按 2^bpc-1 归一化，可选反转。像素字节
// 不足时返回 nil 表示蒙版损坏。
func decodePDFSoftMaskAlpha(raw []byte, width, height, bpc int, inverted bool) []uint8 {
	rowBytes := (bpc*width + 7) / 8
	if width <= 0 || height <= 0 || len(raw) < rowBytes*height {
		return nil
	}
	maxValue := uint16(1<<uint(bpc)) - 1
	alpha := make([]uint8, 0, width*height)
	for y := 0; y < height; y++ {
		row := raw[y*rowBytes : (y+1)*rowBytes]
		for x := 0; x < width; x++ {
			bitOffset := x * bpc
			shift := uint(8 - bpc - bitOffset%8)
			value := uint16((row[bitOffset/8] >> shift) & byte(maxValue))
			if inverted {
				value = maxValue - value
			}
			alpha = append(alpha, uint8(value*255/maxValue))
		}
	}
	return alpha
}

// compositePDFSoftMask 把蒙版 alpha（按蒙版自身尺寸布局）叠加到 PNG 图片：
// 非破坏合成（原 alpha 与蒙版相乘），尺寸不一致时最近邻采样，成功返回重新
// 编码的 PNG 字节。
func compositePDFSoftMask(data []byte, alpha []uint8, maskWidth, maskHeight int) ([]byte, bool) {
	source, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, false
	}
	bounds := source.Bounds()
	out := image.NewNRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	for y := 0; y < bounds.Dy(); y++ {
		maskY := maskHeight * y / bounds.Dy()
		for x := 0; x < bounds.Dx(); x++ {
			nc := color.NRGBAModel.Convert(source.At(bounds.Min.X+x, bounds.Min.Y+y)).(color.NRGBA)
			maskX := maskWidth * x / bounds.Dx()
			coverage := alpha[maskY*maskWidth+maskX]
			nc.A = uint8(uint16(nc.A) * uint16(coverage) / 255)
			out.SetNRGBA(x, y, nc)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, out); err != nil {
		return nil, false
	}
	return buf.Bytes(), true
}

// codecs.go 提供 PDF 内嵌图片的字节级编解码：JPEG 2000（JPXDecode）与
// JBIG2（JBIG2Decode）的纯 Go 解码，以及软蒙版的 alpha 展开与合成。
// 本包只做字节到像素的转换，不感知 PDF 对象模型；对象解引用、过滤器
// 校验与提取编排由根包完成。
package pdfenc

import (
	"bytes"
	"image"
	"image/color"
	"image/png"

	"github.com/dkrisman/gobig2"
	"github.com/mrjoshuak/go-jpeg2000"
)

// DecodeJPEG2000 把 JPEG 2000 codestream 解码并重编码为 PNG。解码失败、
// 像素为空或任一边超过 maxDimension 时返回 ok=false。
func DecodeJPEG2000(data []byte, maxDimension int) (pngData []byte, ok bool) {
	if len(data) == 0 {
		return nil, false
	}
	source, err := jpeg2000.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, false
	}
	bounds := source.Bounds()
	if bounds.Dx() <= 0 || bounds.Dy() <= 0 ||
		bounds.Dx() > maxDimension || bounds.Dy() > maxDimension {
		return nil, false
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, source); err != nil {
		return nil, false
	}
	return buf.Bytes(), true
}

// DecodeJBIG2 把 PDF 内嵌 JBIG2 段流解码为灰度 PNG。globals 是可选的
// /JBIG2Globals 全局段字节；解码失败或尺寸超限时返回 ok=false。
func DecodeJBIG2(stream, globals []byte, maxDimension int) (pngData []byte, ok bool) {
	if len(stream) == 0 {
		return nil, false
	}
	decoder, err := gobig2.NewDecoderEmbedded(bytes.NewReader(stream), globals)
	if err != nil {
		return nil, false
	}
	image, err := decoder.Decode()
	if err != nil {
		return nil, false
	}
	bounds := image.Bounds()
	if bounds.Dx() <= 0 || bounds.Dy() <= 0 ||
		bounds.Dx() > maxDimension || bounds.Dy() > maxDimension {
		return nil, false
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, image); err != nil {
		return nil, false
	}
	return buf.Bytes(), true
}

// DecodeSoftMaskAlpha 把低位深蒙版像素展开为 8 位 alpha：每行独立字节
// 对齐（行末 padding 不算像素），值域按 2^bpc-1 归一化，可选反转。像素
// 字节不足时返回 nil 表示蒙版损坏。
func DecodeSoftMaskAlpha(raw []byte, width, height, bpc int, inverted bool) []uint8 {
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

// CompositeSoftMask 把蒙版 alpha（按蒙版自身尺寸布局）叠加到 PNG 图片：
// 非破坏合成（原 alpha 与蒙版相乘），尺寸不一致时最近邻采样，成功返回
// 重新编码的 PNG 字节。
func CompositeSoftMask(data []byte, alpha []uint8, maskWidth, maskHeight int) ([]byte, bool) {
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

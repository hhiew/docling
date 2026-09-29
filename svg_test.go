// svg_test.go 验证 SVG 输入解析：文本提取顺序、危险内容消毒、尺寸解析、
// 纯图形 SVG 的 OCR 回退与 ParseByExt 注册。
package docling

import (
	"encoding/base64"
	"errors"
	"strings"
	"testing"
)

// sampleSVGText 常规带文本 SVG：标题、描述与两段 text（含 tspan）。
const sampleSVGText = `<?xml version="1.0" encoding="UTF-8"?>
<svg xmlns="http://www.w3.org/2000/svg" width="400" height="300" viewBox="0 0 400 300">
  <title>季度产量图</title>
  <desc>2026 年第三季度各月产量对比</desc>
  <rect x="10" y="20" width="380" height="260" fill="#f5f5f5"/>
  <text x="20" y="50" font-size="14">一月产量 <tspan font-weight="bold">1200</tspan> 件</text>
  <text x="20" y="80" font-size="14">二月产量 1500 件</text>
</svg>`

// sampleSVGMalicious 恶意 SVG：script 脚本、foreignObject 内嵌 HTML、
// onload 事件属性与外部 href 引用。
const sampleSVGMalicious = `<svg xmlns="http://www.w3.org/2000/svg" width="100" height="100" onload="alert(1)">
  <script>alert('xss')</script>
  <image href="https://evil.example.com/track.png" x="0" y="0" width="10" height="10"/>
  <a xlink:href="https://evil.example.com"><text x="1" y="10">外链文字</text></a>
  <foreignObject width="50" height="50"><body onload="steal()"><div>html</div></body></foreignObject>
  <text x="1" y="90">正常文字</text>
</svg>`

// sampleSVGPureGraphic 纯图形 SVG：无任何 text/title/desc。
const sampleSVGPureGraphic = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 200 100">
  <circle cx="50" cy="50" r="40" fill="red"/>
</svg>`

// TestParseSVGExtractsTextInOrder 验证文本条目按 title → desc → text 顺序
// 生成，tspan 字符并入所在 text，页尺寸取 width/height。
func TestParseSVGExtractsTextInOrder(t *testing.T) {
	doc, err := ParseSVG([]byte(sampleSVGText))
	if err != nil {
		t.Fatalf("ParseSVG: %v", err)
	}
	want := []struct {
		label DocItemLabel
		text  string
	}{
		{LabelTitle, "季度产量图"},
		{LabelCaption, "2026 年第三季度各月产量对比"},
		{LabelText, "一月产量 1200 件"},
		{LabelText, "二月产量 1500 件"},
	}
	if len(doc.Texts) != len(want) {
		t.Fatalf("texts = %+v", doc.Texts)
	}
	for i, expect := range want {
		if doc.Texts[i].Label != expect.label || doc.Texts[i].Text != expect.text {
			t.Fatalf("texts[%d] = {%s %q}, want {%s %q}", i,
				doc.Texts[i].Label, doc.Texts[i].Text, expect.label, expect.text)
		}
	}
	page := doc.Pages["1"]
	if page.Size == nil || page.Size.Width != 400 || page.Size.Height != 300 {
		t.Fatalf("page size wrong: %+v", page)
	}
}

// TestParseSVGPictureKeepsSanitizedCopy 验证 PictureItem 携带净化后 SVG 的
// data URI，且危险内容不出现在任何产出字段。
func TestParseSVGPictureKeepsSanitizedCopy(t *testing.T) {
	doc, err := ParseSVG([]byte(sampleSVGMalicious))
	if err != nil {
		t.Fatalf("ParseSVG: %v", err)
	}
	if len(doc.Pictures) != 1 || doc.Pictures[0].Image == nil {
		t.Fatalf("picture missing: %+v", doc.Pictures)
	}
	image := doc.Pictures[0].Image
	if image.Mimetype != "image/svg+xml" || image.Dpi != 96 ||
		!strings.HasPrefix(image.URI, "data:image/svg+xml;base64,") {
		t.Fatalf("image metadata wrong: %+v", image)
	}
	payload, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(image.URI, "data:image/svg+xml;base64,"))
	if err != nil {
		t.Fatalf("data URI not base64: %v", err)
	}
	clean := string(payload)
	for _, banned := range []string{"<script", "foreignObject", "onload", "evil.example.com", "alert"} {
		if strings.Contains(clean, banned) {
			t.Fatalf("sanitized copy still contains %q: %s", banned, clean)
		}
	}
	// 正常文字保留：<a> 元素合法（仅剥离其外部 xlink:href），包裹的文字不丢
	if !strings.Contains(clean, "正常文字") {
		t.Fatalf("sanitized copy lost normal text: %s", clean)
	}
	var sawNormal, sawLinked bool
	for _, txt := range doc.Texts {
		switch txt.Text {
		case "正常文字":
			sawNormal = true
		case "外链文字":
			sawLinked = true
		}
	}
	if !sawNormal || !sawLinked {
		t.Fatalf("texts lost legal content: normal=%v linked=%v texts=%+v", sawNormal, sawLinked, doc.Texts)
	}
}

// TestParseSVGRejectsNonSVGRoot 验证根元素不是 <svg> 与非 XML 输入被拒绝。
func TestParseSVGRejectsNonSVGRoot(t *testing.T) {
	for name, data := range map[string]string{
		"html root": `<html><body>x</body></html>`,
		"not xml":   "plain text",
		"empty":     "",
	} {
		if _, err := ParseSVG([]byte(data)); err == nil {
			t.Fatalf("%s should be rejected", name)
		}
	}
}

// TestParseSVGSizeFallback 验证尺寸解析：viewBox 回退与默认尺寸。
func TestParseSVGSizeFallback(t *testing.T) {
	doc, err := ParseSVG([]byte(sampleSVGPureGraphic))
	if err != nil {
		t.Fatalf("ParseSVG: %v", err)
	}
	if page := doc.Pages["1"]; page.Size == nil || page.Size.Width != 200 || page.Size.Height != 100 {
		t.Fatalf("viewBox fallback size wrong: %+v", page)
	}
	doc, err = ParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg"/>`))
	if err != nil {
		t.Fatalf("ParseSVG: %v", err)
	}
	if page := doc.Pages["1"]; page.Size == nil ||
		page.Size.Width != defaultSVGWidth || page.Size.Height != defaultSVGHeight {
		t.Fatalf("default size wrong: %+v", page)
	}
	// 单位换算：pt → px
	doc, err = ParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" width="72pt" height="1in"/>`))
	if err != nil {
		t.Fatalf("ParseSVG: %v", err)
	}
	if page := doc.Pages["1"]; page.Size == nil || page.Size.Width != 96 || page.Size.Height != 96 {
		t.Fatalf("unit conversion wrong: %+v", page)
	}
}

// TestParseSVGPureGraphicOCRHook 验证纯图形 SVG 触发 OCR 回退（MIME 为
// image/svg+xml、页号 1），识别文本经 Markdown 结构化并入；有内嵌文本时
// 不触发钩子。
func TestParseSVGPureGraphicOCRHook(t *testing.T) {
	calls := 0
	doc, err := ParseSVGWithOptions([]byte(sampleSVGPureGraphic), PDFOptions{
		Filename: "chart.svg",
		OCRHook: func(req OCRRequest) (string, error) {
			calls++
			if req.PageNo != 1 || req.MIMEType != "image/svg+xml" || req.Filename != "chart.svg" {
				t.Fatalf("OCR request wrong: %+v", req)
			}
			return "# 图表说明\n\n红色圆形示意图。", nil
		},
	})
	if err != nil {
		t.Fatalf("ParseSVGWithOptions: %v", err)
	}
	if calls != 1 {
		t.Fatalf("hook calls = %d, want 1", calls)
	}
	if !strings.Contains(doc.Text(), "红色圆形示意图。") {
		t.Fatalf("OCR text not merged: %q", doc.Text())
	}

	// 带文本 SVG 不触发钩子：文本已由规则提取，无需模型介入
	calls = 0
	doc, err = ParseSVGWithOptions([]byte(sampleSVGText), PDFOptions{
		OCRHook: func(OCRRequest) (string, error) { calls++; return "不应调用", nil },
	})
	if err != nil {
		t.Fatalf("ParseSVGWithOptions: %v", err)
	}
	if calls != 0 {
		t.Fatalf("hook should not run when text extracted, calls = %d", calls)
	}
}

// TestParseSVGHookFailureKeepsPicture 验证识别失败时保留图片项不报错。
func TestParseSVGHookFailureKeepsPicture(t *testing.T) {
	doc, err := ParseSVGWithOptions([]byte(sampleSVGPureGraphic), PDFOptions{
		PageOCRHook: func(int64, []byte) (string, error) { return "", errors.New("down") },
	})
	if err != nil {
		t.Fatalf("hook failure should not error: %v", err)
	}
	if len(doc.Pictures) != 1 || len(doc.Texts) != 0 {
		t.Fatalf("picture-only doc expected: pictures=%d texts=%d", len(doc.Pictures), len(doc.Texts))
	}
}

// TestParseByExtSVG 验证 ParseByExt 注册与 origin MIME 口径。
func TestParseByExtSVG(t *testing.T) {
	doc, err := ParseByExt("demo.svg", []byte(sampleSVGText))
	if err != nil {
		t.Fatalf("ParseByExt: %v", err)
	}
	if !strings.Contains(doc.Text(), "季度产量图") || len(doc.Pictures) != 1 {
		t.Fatalf("dispatch result wrong: texts=%+v pictures=%d", doc.Texts, len(doc.Pictures))
	}
	if doc.Origin == nil || doc.Origin.Mimetype != "image/svg+xml" {
		t.Fatalf("origin wrong: %+v", doc.Origin)
	}
	// content_list 派生包含 text 与 picture 项
	items := ToContentList(doc, SourceDocling)
	var sawText, sawPicture bool
	for _, item := range items {
		switch item.Type {
		case ItemTypeText:
			if strings.Contains(item.Text, "季度产量图") {
				sawText = true
			}
		case ItemTypeImage:
			sawPicture = true
		}
	}
	if !sawText || !sawPicture {
		t.Fatalf("content list wrong: text=%v picture=%v items=%+v", sawText, sawPicture, items)
	}
}

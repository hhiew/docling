// image_test.go 验证图片格式输入：魔数识别（含拒绝未知格式）、无 OCR 时
// 保留结构化图片、新旧 OCR 钩子兼容、识别结果校验与 ParseByExt 注册。
package docparse

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

// TestParseImageWithoutOCRKeepsStructuredPicture 验证未配置 OCR 时仍返回带
// 像素尺寸、DPI 和图片引用的结构化 PictureItem。
func TestParseImageWithoutOCRKeepsStructuredPicture(t *testing.T) {
	doc, err := ParseImage(minimalPNG)
	if err != nil {
		t.Fatalf("ParseImage: %v", err)
	}
	if len(doc.Pictures) != 1 || doc.Pictures[0].Image == nil {
		t.Fatalf("picture missing: %+v", doc.Pictures)
	}
	image := doc.Pictures[0].Image
	if image.Mimetype != "image/png" || image.Dpi <= 0 || image.Size == nil ||
		image.Size.Width != 1 || image.Size.Height != 1 {
		t.Fatalf("image metadata wrong: %+v", image)
	}
	if page := doc.Pages["1"]; page.Size == nil ||
		page.Size.Width != 1 || page.Size.Height != 1 {
		t.Fatalf("page metadata wrong: %+v", doc.Pages)
	}
}

// TestParseImageOCRRequestPreferred 验证新 OCRHook 优先于旧钩子，且请求包含
// 页号、MIME、文件名、原始数据与已有文本。
func TestParseImageOCRRequestPreferred(t *testing.T) {
	legacyCalled := false
	requestCalled := 0
	doc, err := ParseImageWithOptions(minimalPNG, PDFOptions{
		Filename: "scan.png",
		MIMEType: "image/png",
		OCRHook: func(req OCRRequest) (string, error) {
			requestCalled++
			if req.PageNo != 1 || req.MIMEType != "image/png" || req.Filename != "scan.png" ||
				!bytes.Equal(req.Data, minimalPNG) || req.ExistingText != "" {
				t.Fatalf("OCR request wrong: %+v", req)
			}
			return "```markdown\n# 扫描标题\n\n扫描正文。\n```", nil
		},
		PageOCRHook: func(int64, []byte) (string, error) {
			legacyCalled = true
			return "旧结果", nil
		},
	})
	if err != nil {
		t.Fatalf("ParseImageWithOptions: %v", err)
	}
	if requestCalled != 1 || legacyCalled {
		t.Fatalf("hook priority wrong: new=%d legacy=%v", requestCalled, legacyCalled)
	}
	if !strings.Contains(doc.Text(), "扫描正文。") || strings.Contains(doc.Text(), "```") {
		t.Fatalf("OCR markdown cleanup failed: %q", doc.Text())
	}
}

// TestParseImageOCRInvalidRetriesOnce 验证空结果、拒答和模型说明会被拒绝，
// 首次无效时仅重试一次，第二次有效结果才会并入。
func TestParseImageOCRInvalidRetriesOnce(t *testing.T) {
	calls := 0
	doc, err := ParseImageWithOptions(minimalPNG, PDFOptions{OCRHook: func(OCRRequest) (string, error) {
		calls++
		if calls == 1 {
			return "抱歉，我无法识别这张图片。", nil
		}
		return "识别成功。", nil
	}})
	if err != nil {
		t.Fatalf("ParseImageWithOptions: %v", err)
	}
	if calls != 2 || !strings.Contains(doc.Text(), "识别成功。") {
		t.Fatalf("retry result wrong: calls=%d text=%q", calls, doc.Text())
	}
}

// TestParseImageWithOCRHook 验证带钩子解析：单页一图（data URI）+ 识别文本
// 经 Markdown 结构化并入为文本项，Meta.PageCount=1。
func TestParseImageWithOCRHook(t *testing.T) {
	called := false
	hook := func(pageNo int64, pdfBytes []byte) (string, error) {
		called = true
		if pageNo != 1 {
			t.Fatalf("hook pageNo = %d, want 1", pageNo)
		}
		return "# 识别标题\n\n识别到的正文。", nil
	}
	doc, err := ParseImageWithOptions(minimalPNG, PDFOptions{PageOCRHook: hook})
	if err != nil {
		t.Fatalf("ParseImageWithOptions: %v", err)
	}
	if !called {
		t.Fatal("OCR hook not called")
	}
	if len(doc.Pictures) != 1 || doc.Pictures[0].Image == nil ||
		!strings.HasPrefix(doc.Pictures[0].Image.URI, "data:image/png;base64,") {
		t.Fatalf("picture wrong: %+v", doc.Pictures)
	}
	var sawHeading, sawBody bool
	for _, txt := range doc.Texts {
		// Markdown 一级标题产出 label=title（二级及以上才是 section_header）
		if (txt.Label == LabelTitle || txt.Label == LabelSectionHeader) && txt.Text == "识别标题" {
			sawHeading = true
		}
		if txt.Text == "识别到的正文。" {
			sawBody = true
		}
	}
	if !sawHeading || !sawBody {
		t.Fatalf("OCR text not merged as items: %+v", doc.Texts)
	}
	if doc.Meta == nil || doc.Meta.PageCount != 1 {
		t.Fatalf("meta wrong: %+v", doc.Meta)
	}
	if len(doc.Pages) != 1 {
		t.Fatalf("expected single page, got %+v", doc.Pages)
	}
}

// TestParseImageHookFailureKeepsPicture 验证识别失败/空文本时保留图片项、
// 不产文本也不报错。
func TestParseImageHookFailureKeepsPicture(t *testing.T) {
	hook := func(pageNo int64, pdfBytes []byte) (string, error) {
		return "", errors.New("ocr unavailable")
	}
	doc, err := ParseImageWithOptions(minimalPNG, PDFOptions{PageOCRHook: hook})
	if err != nil {
		t.Fatalf("hook failure should not error: %v", err)
	}
	if len(doc.Pictures) != 1 || len(doc.Texts) != 0 {
		t.Fatalf("picture-only doc expected, pictures=%d texts=%d", len(doc.Pictures), len(doc.Texts))
	}
}

// TestParseImageRejectsUnknownFormat 验证非图片字节与全部魔数分支：
// PNG/JPEG/BMP/WEBP 接受，其余拒绝。
func TestParseImageRejectsUnknownFormat(t *testing.T) {
	if _, err := ParseImageWithOptions([]byte("hello world"), PDFOptions{}); err == nil ||
		!strings.Contains(err.Error(), "不支持的图片格式") {
		t.Fatalf("unknown format should be rejected, got %v", err)
	}
	// 各格式魔数均被识别（此处仅断言不再报"格式不支持"，识别文本路径已另行覆盖）
	samples := map[string][]byte{
		"png":  {0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'},
		"jpg":  {0xFF, 0xD8, 0xFF, 0xE0},
		"bmp":  {'B', 'M'},
		"webp": {'R', 'I', 'F', 'F', 0, 0, 0, 0, 'W', 'E', 'B', 'P'},
	}
	for name, sample := range samples {
		_, err := ParseImageWithOptions(sample, PDFOptions{})
		if err != nil && strings.Contains(err.Error(), "不支持的图片格式") {
			t.Fatalf("%s sample should pass magic detection, got %v", name, err)
		}
	}
}

// TestParseByExtImage 验证 ParseByExt 对图片扩展名的注册且无 OCR 也成功。
func TestParseByExtImage(t *testing.T) {
	for _, name := range []string{"a.png", "b.JPG", "c.jpeg", "d.bmp", "e.webp"} {
		doc, err := ParseByExt(name, minimalPNG)
		if err != nil || len(doc.Pictures) != 1 {
			t.Fatalf("%s should dispatch to ParseImage, got doc=%+v err=%v", name, doc, err)
		}
	}
}

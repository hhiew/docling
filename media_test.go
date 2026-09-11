// media_test.go 验证 P0 能力补齐的 OOXML 媒体与元数据部分：
// mediaToDataURI（data URI 封装与 8MB 上限）、parseOOXMLCoreProps（core.xml
// 解析）、docx/pptx 图片提取（rId → media 部件内嵌 data URI + caption）与
// docx/pptx/xlsx/pdf 的 DocMeta 填充。zip 样例沿用 docx_test.go 的手工构造法。
package docparse

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"io"
	"sort"
	"strings"
	"testing"
)

// minimalPNG 是可解码的 1×1 PNG 图片样例，用于同时验证魔数与尺寸提取。
var minimalPNG, _ = base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=")

// testCorePropsXML docProps/core.xml 样例（DC/DCTERMS 命名空间齐全）。
const testCorePropsXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
	`<cp:coreProperties xmlns:cp="http://schemas.openxmlformats.org/package/2006/metadata/core-properties"` +
	` xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:dcterms="http://purl.org/dc/terms/"` +
	` xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">` +
	`<dc:title>平台接入手册</dc:title><dc:creator>DocParse</dc:creator>` +
	`<dc:subject>接入指南</dc:subject><dc:language>zh-CN</dc:language>` +
	`<dcterms:created xsi:type="dcterms:W3CDTF">2026-09-07T10:00:00Z</dcterms:created>` +
	`</cp:coreProperties>`

// mustZipEntriesForTest 把任意条目打包为 zip（二进制内容以 string 携带），
// 条目名排序保证产物稳定。
func mustZipEntriesForTest(t *testing.T, entries map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := newZipWriterForTest(t, &buf)
	names := make([]string, 0, len(entries))
	for name := range entries {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		writeZipEntryForTest(t, zw, name, entries[name])
	}
	closeZipWriterForTest(t, zw)
	return buf.Bytes()
}

// appendZipEntries 把 entries 追加为既有 zip 的新条目（xlsx 元数据测试用：
// 在 excelize 产物上补/替换 docProps/core.xml）；与 base 同名的条目以
// entries 为准（不再复制 base 原条目）。
func appendZipEntries(t *testing.T, base []byte, entries map[string]string) []byte {
	t.Helper()
	reader, err := zip.NewReader(bytes.NewReader(base), int64(len(base)))
	if err != nil {
		t.Fatalf("open base zip: %v", err)
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, f := range reader.File {
		if _, overridden := entries[f.Name]; overridden {
			continue // 同名条目以追加内容为准
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("open %s: %v", f.Name, err)
		}
		content, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			t.Fatalf("read %s: %v", f.Name, err)
		}
		w, err := zw.Create(f.Name)
		if err != nil {
			t.Fatalf("copy create %s: %v", f.Name, err)
		}
		if _, err := w.Write(content); err != nil {
			t.Fatalf("copy write %s: %v", f.Name, err)
		}
	}
	for name, content := range entries {
		writeZipEntryForTest(t, zw, name, content)
	}
	closeZipWriterForTest(t, zw)
	return buf.Bytes()
}

// TestMediaToDataURI 验证 data URI 封装：前缀/MIME 正确、base64 可逆；
// 空字节与超过 8MB 上限的图片返回空串（URI 留空保护）。
func TestMediaToDataURI(t *testing.T) {
	raw := []byte{0x89, 0x50, 0x4E, 0x47}
	uri := mediaToDataURI(".png", raw)
	if !strings.HasPrefix(uri, "data:image/png;base64,") {
		t.Fatalf("unexpected prefix: %q", uri)
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(uri, "data:image/png;base64,"))
	if err != nil || !bytes.Equal(decoded, raw) {
		t.Fatalf("base64 roundtrip failed: %v %q", err, decoded)
	}
	// 大小写与无点扩展名同样接受
	if got := mediaToDataURI("JPG", []byte{1}); !strings.HasPrefix(got, "data:image/jpeg;base64,") {
		t.Fatalf("jpeg mime wrong: %q", got)
	}
	// 未知扩展名回退 octet-stream
	if got := mediaToDataURI(".xyz", []byte{1}); !strings.HasPrefix(got, "data:application/octet-stream;base64,") {
		t.Fatalf("unknown ext mime wrong: %q", got)
	}
	if mediaToDataURI(".png", nil) != "" {
		t.Fatal("empty bytes should yield empty URI")
	}
	big := bytes.Repeat([]byte{0xFF}, maxMediaDataURIBytes+1)
	if uri := mediaToDataURI(".png", big); uri != "" {
		t.Fatalf("oversized image should skip URI, got len=%d", len(uri))
	}
}

// TestParseOOXMLCoreProps 验证 core.xml → DocMeta：全字段提取、空输入与
// 全空字段返回 nil。
func TestParseOOXMLCoreProps(t *testing.T) {
	meta := parseOOXMLCoreProps([]byte(testCorePropsXML))
	if meta == nil {
		t.Fatal("meta should not be nil")
	}
	if meta.Title != "平台接入手册" || meta.Author != "DocParse" || meta.Subject != "接入指南" ||
		meta.Language != "zh-CN" || meta.CreatedAt != "2026-09-07T10:00:00Z" {
		t.Fatalf("meta fields wrong: %+v", meta)
	}
	if parseOOXMLCoreProps(nil) != nil {
		t.Fatal("empty input should return nil")
	}
	if parseOOXMLCoreProps([]byte("<cp:coreProperties xmlns:cp=\"x\"/>")) != nil {
		t.Fatal("all-empty core props should return nil")
	}
}

// docxImageDocumentXMLRoot 带 DrawingML/relationships 命名声明的
// document.xml 外壳（图片用例用）。
func docxImageDocumentXMLRoot(body string) string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"` +
		` xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"` +
		` xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">` +
		`<w:body>` + body + `</w:body></w:document>`
}

// TestParseDocxExtractsPicture 验证 docx 图片提取：a:blip 的 r:embed 经
// document.xml.rels 定位 word/media 部件并内嵌 data URI，段落文本作 caption；
// 简化后 image Item 携带 ImgPath 与 ImageCaption。
func TestParseDocxExtractsPicture(t *testing.T) {
	body := `<w:p><w:r><w:t>系统架构图</w:t></w:r>` +
		`<w:drawing><a:blip r:embed="rId1"/></w:drawing></w:p>` +
		`<w:p><w:r><w:t>后续正文</w:t></w:r></w:p>`
	data := mustZipEntriesForTest(t, map[string]string{
		"word/document.xml": docxImageDocumentXMLRoot(body),
		"word/_rels/document.xml.rels": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
			`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
			`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="media/image1.png"/>` +
			`</Relationships>`,
		"word/media/image1.png": string(minimalPNG),
	})

	doc, err := ParseDocx(data)
	if err != nil {
		t.Fatalf("parse docx: %v", err)
	}
	if len(doc.Pictures) != 1 {
		t.Fatalf("expected 1 picture, got %+v", doc.Pictures)
	}
	pic := doc.Pictures[0]
	if pic.Image == nil || !strings.HasPrefix(pic.Image.URI, "data:image/png;base64,") {
		t.Fatalf("picture URI not embedded: %+v", pic.Image)
	}
	if pic.Image.Mimetype != "image/png" || pic.Image.Dpi != defaultImageDPI || pic.Image.Size == nil ||
		pic.Image.Size.Width != 1 || pic.Image.Size.Height != 1 {
		t.Fatalf("picture image ref incomplete: %+v", pic.Image)
	}
	if pic.Caption != "系统架构图" {
		t.Fatalf("picture caption = %q, want 系统架构图", pic.Caption)
	}
	// 正文文本不受影响（caption 文本不再重复产出 text 元素）
	for _, txt := range doc.Texts {
		if txt.Text == "系统架构图" {
			t.Fatalf("caption text should not emit a text item: %+v", doc.Texts)
		}
	}
	// 简化器视角：image Item 携带 ImgPath 与 ImageCaption
	items := ToContentList(doc, SourceGolight)
	var img *Item
	for i := range items {
		if items[i].Type == ItemTypeImage {
			img = &items[i]
		}
	}
	if img == nil {
		t.Fatalf("no image item in content list: %+v", items)
	}
	if !strings.HasPrefix(img.ImgPath, "data:image/png;base64,") || img.ImageCaption != "系统架构图" {
		t.Fatalf("image item wrong: %+v", img)
	}
}

// TestParseDocxOversizeImageSkipsURI 验证单图超过 8MB 上限时跳过 URI 填充
// （picture 元素仍产出，URI 留空）。
func TestParseDocxOversizeImageSkipsURI(t *testing.T) {
	bigPNG := append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte{0xFF}, maxMediaDataURIBytes+1)...)
	body := `<w:p><w:drawing><a:blip r:embed="rId1"/></w:drawing></w:p>`
	data := mustZipEntriesForTest(t, map[string]string{
		"word/document.xml": docxImageDocumentXMLRoot(body),
		"word/_rels/document.xml.rels": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
			`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
			`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="media/big.png"/>` +
			`</Relationships>`,
		"word/media/big.png": string(bigPNG),
	})

	doc, err := ParseDocx(data)
	if err != nil {
		t.Fatalf("parse docx: %v", err)
	}
	if len(doc.Pictures) != 1 {
		t.Fatalf("expected 1 picture, got %+v", doc.Pictures)
	}
	if doc.Pictures[0].Image == nil || doc.Pictures[0].Image.URI != "" {
		t.Fatalf("oversized image URI should stay empty: %+v", doc.Pictures[0].Image)
	}
}

// TestParseDocxDocMeta 验证 docx 读取 docProps/core.xml 填充 DocMeta；
// core.xml 缺失时 Meta 保持 nil。
func TestParseDocxDocMeta(t *testing.T) {
	body := `<w:p><w:r><w:t>正文</w:t></w:r></w:p>`
	withMeta := mustZipEntriesForTest(t, map[string]string{
		"word/document.xml": buildStructuredDocxXML(body),
		"docProps/core.xml": testCorePropsXML,
	})
	doc, err := ParseDocx(withMeta)
	if err != nil {
		t.Fatalf("parse docx: %v", err)
	}
	if doc.Meta == nil || doc.Meta.Title != "平台接入手册" || doc.Meta.Author != "DocParse" ||
		doc.Meta.CreatedAt != "2026-09-07T10:00:00Z" {
		t.Fatalf("docx meta wrong: %+v", doc.Meta)
	}
	// 无 core.xml：Meta 为 nil（不产空对象）
	doc2, err := ParseDocx(mustZipDocx(t, buildStructuredDocxXML(body)))
	if err != nil {
		t.Fatalf("parse docx: %v", err)
	}
	if doc2.Meta != nil {
		t.Fatalf("meta should stay nil without core.xml: %+v", doc2.Meta)
	}
}

// buildTestPPTXWithImage 构造带 p:pic 图片与可选 core.xml 的最小 pptx：
// 单 slide 单图片（rId2 → ../media/image1.png）。
func buildTestPPTXWithImage(t *testing.T, coreXML string) []byte {
	t.Helper()
	entries := map[string]string{
		"ppt/presentation.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
			`<p:presentation ` + pptxTestXMLNS + `>` +
			`<p:sldIdLst><p:sldId id="256" r:id="rId1"/></p:sldIdLst>` +
			`<p:sldSz cx="9144000" cy="6858000"/></p:presentation>`,
		"ppt/_rels/presentation.xml.rels": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
			`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
			`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide1.xml"/>` +
			`</Relationships>`,
		"ppt/slides/slide1.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
			`<p:sld ` + pptxTestXMLNS + `>` +
			`<p:cSld><p:spTree>` +
			`<p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr>` +
			`<p:grpSpPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="0" cy="0"/></a:xfrm></p:grpSpPr>` +
			`<p:pic>` +
			`<p:nvPicPr><p:cNvPr id="2" name="架构图"/><p:cNvPicPr/><p:nvPr/></p:nvPicPr>` +
			`<p:blipFill><a:blip r:embed="rId2"/></p:blipFill>` +
			`<p:spPr><a:xfrm><a:off x="838200" y="1825625"/><a:ext cx="3000000" cy="2000000"/></a:xfrm></p:spPr>` +
			`</p:pic>` +
			`</p:spTree></p:cSld></p:sld>`,
		"ppt/slides/_rels/slide1.xml.rels": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
			`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
			`<Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="../media/image1.png"/>` +
			`</Relationships>`,
		"ppt/media/image1.png": string(minimalPNG),
	}
	if coreXML != "" {
		entries["docProps/core.xml"] = coreXML
	}
	return mustZipEntriesForTest(t, entries)
}

// TestParsePPTXExtractsPicture 验证 pptx 图片提取：p:pic 的 r:embed 经 slide
// 关系表定位 ppt/media 部件内嵌 data URI，prov 按形状几何生成。
func TestParsePPTXExtractsPicture(t *testing.T) {
	doc, err := ParsePPTX(buildTestPPTXWithImage(t, ""))
	if err != nil {
		t.Fatalf("parse pptx: %v", err)
	}
	if len(doc.Pictures) != 1 {
		t.Fatalf("expected 1 picture, got %+v", doc.Pictures)
	}
	pic := doc.Pictures[0]
	if pic.Image == nil || !strings.HasPrefix(pic.Image.URI, "data:image/png;base64,") {
		t.Fatalf("picture URI not embedded: %+v", pic.Image)
	}
	if len(pic.Prov) != 1 || pic.Prov[0].PageNo != 1 || pic.Prov[0].BBox == nil ||
		pic.Prov[0].BBox.CoordOrigin != CoordOriginBottomLeft {
		t.Fatalf("picture prov wrong: %+v", pic.Prov)
	}
}

// TestParsePPTXDocMeta 验证 pptx 读取 core.xml 并填 PageCount（slide 数）；
// 无 core.xml 时仍以仅含 PageCount 的 Meta 产出。
func TestParsePPTXDocMeta(t *testing.T) {
	doc, err := ParsePPTX(buildTestPPTXWithImage(t, testCorePropsXML))
	if err != nil {
		t.Fatalf("parse pptx: %v", err)
	}
	if doc.Meta == nil || doc.Meta.Title != "平台接入手册" || doc.Meta.PageCount != 1 {
		t.Fatalf("pptx meta wrong: %+v", doc.Meta)
	}
	doc2, err := ParsePPTX(buildTestPPTXWithImage(t, ""))
	if err != nil {
		t.Fatalf("parse pptx: %v", err)
	}
	if doc2.Meta == nil || doc2.Meta.Title != "" || doc2.Meta.PageCount != 1 {
		t.Fatalf("pptx meta without core.xml wrong: %+v", doc2.Meta)
	}
}

// TestParseXLSXDocMeta 验证 xlsx 读取 core.xml 并填 PageCount（含隐藏表和图表工作表）。
func TestParseXLSXDocMeta(t *testing.T) {
	base := mustBuildTestXLSX(t, map[string][][]string{
		"参数表": {{"参数", "说明"}, {"timeout", "超时时间"}},
		"清单":  {{"项目", "数量"}, {"服务器", "2"}},
	})
	data := appendZipEntries(t, base, map[string]string{"docProps/core.xml": testCorePropsXML})
	doc, err := ParseXLSX(data)
	if err != nil {
		t.Fatalf("parse xlsx: %v", err)
	}
	if doc.Meta == nil || doc.Meta.Title != "平台接入手册" || doc.Meta.PageCount != 2 {
		t.Fatalf("xlsx meta wrong: %+v", doc.Meta)
	}
	// 无 core.xml 且有可见 sheet：Meta 仅携带 PageCount
	doc2, err := ParseXLSX(base)
	if err != nil {
		t.Fatalf("parse xlsx: %v", err)
	}
	if doc2.Meta == nil || doc2.Meta.Title != "" || doc2.Meta.PageCount != 2 {
		t.Fatalf("xlsx meta without core.xml wrong: %+v", doc2.Meta)
	}
}

// TestParsePDFDocMeta 验证 PDF 元数据尽力填充：仅可靠取到 PageCount。
func TestParsePDFDocMeta(t *testing.T) {
	data := buildTestPDF(t, []testPDFPage{
		{content: "BT /F1 10 Tf 72 700 Td (First page.) Tj ET\n"},
		{content: "BT /F1 10 Tf 72 700 Td (Second page.) Tj ET\n"},
	})
	doc, err := ParsePDF(data)
	if err != nil {
		t.Fatalf("ParsePDF: %v", err)
	}
	if doc.Meta == nil || doc.Meta.PageCount != 2 {
		t.Fatalf("pdf meta wrong: %+v", doc.Meta)
	}
	if doc.Meta.Title != "" || doc.Meta.Author != "" || doc.Meta.CreatedAt != "" {
		t.Fatalf("pdf should not fill unavailable meta fields: %+v", doc.Meta)
	}
}

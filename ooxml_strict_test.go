// ooxml_strict_test.go 验证 Strict OOXML 命名空间在纯 Go 路径中统一
// 归一化，确保 DOCX 公式、PPTX 幻灯片关系和 XLSX 工作表不会静默丢失。
package docparse

import (
	"archive/zip"
	"bytes"
	"io"
	"sort"
	"strings"
	"testing"
)

// TestParseStrictOOXMLFormats 验证三种 Office Open XML 格式从 ISO Strict
// URI 归一化后仍能提取各自的关键结构。
func TestParseStrictOOXMLFormats(t *testing.T) {
	t.Run("docx_formula", func(t *testing.T) {
		data := mustZipEntriesForTest(t, map[string]string{
			"_rels/.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
				`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>` +
				`</Relationships>`,
			"word/document.xml": buildStructuredDocxXML(`<w:p><w:r><w:t>严格正文</w:t></w:r>` +
				`<m:oMath><m:f><m:num><m:r><m:t>a</m:t></m:r></m:num>` +
				`<m:den><m:r><m:t>b</m:t></m:r></m:den></m:f></m:oMath></w:p>`),
		})
		doc, err := ParseDocx(mustMakeStrictOOXMLForTest(t, data))
		if err != nil {
			t.Fatalf("parse strict docx: %v", err)
		}
		// 当前 DOCX 语义把行内公式并入段落；这里验证 Strict 归一化没有
		// 丢失正文和 OMML 转换结果，不改变既有行内公式协议。
		if !strings.Contains(doc.Text(), "严格正文") || !strings.Contains(doc.Text(), `\frac{a}{b}`) {
			t.Fatalf("strict docx content missing: text=%q items=%+v", doc.Text(), doc.Texts)
		}
	})

	t.Run("pptx_slide_relationship", func(t *testing.T) {
		doc, err := ParsePPTX(mustMakeStrictOOXMLForTest(t, buildTestPPTX(t)))
		if err != nil {
			t.Fatalf("parse strict pptx: %v", err)
		}
		if !strings.Contains(doc.Text(), "演示文稿标题") || len(doc.Pages) != 2 {
			t.Fatalf("strict pptx slide content missing: pages=%d text=%q", len(doc.Pages), doc.Text())
		}
	})

	t.Run("xlsx_workbook", func(t *testing.T) {
		data := mustBuildTestXLSX(t, map[string][][]string{"数据": {{"名称", "值"}, {"温度", "23"}}})
		doc, err := ParseXLSX(mustMakeStrictOOXMLForTest(t, data))
		if err != nil {
			t.Fatalf("parse strict xlsx: %v", err)
		}
		if len(doc.Tables) == 0 || !strings.Contains(doc.Text(), "温度") {
			t.Fatalf("strict xlsx content missing: tables=%d text=%q", len(doc.Tables), doc.Text())
		}
	})
}

// TestNormalizeStrictOOXMLPackageSafety 验证普通 OOXML 不发生复制，并且
// Strict 包中的越界路径会在重写前被拒绝。
func TestNormalizeStrictOOXMLPackageSafety(t *testing.T) {
	t.Run("transitional_unchanged", func(t *testing.T) {
		data := mustZipEntriesForTest(t, map[string]string{
			"_rels/.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"></Relationships>`,
		})
		got, err := normalizeStrictOOXMLPackage(data)
		if err != nil {
			t.Fatalf("normalize transitional OOXML: %v", err)
		}
		if len(got) == 0 || &got[0] != &data[0] {
			t.Fatal("transitional OOXML should reuse the original byte slice")
		}
	})

	t.Run("strict_zip_slip_rejected", func(t *testing.T) {
		data := mustZipEntriesForTest(t, map[string]string{
			"_rels/.rels":          `<Relationships xmlns="http://purl.oclc.org/ooxml/package/relationships"></Relationships>`,
			"../word/document.xml": `<w:document xmlns:w="http://purl.oclc.org/ooxml/wordprocessingml/main"/>`,
		})
		if _, err := normalizeStrictOOXMLPackage(data); err == nil || !strings.Contains(err.Error(), "路径越界") {
			t.Fatalf("strict zip-slip error = %v", err)
		}
	})
}

// TestStrictOOXMLNamespaceOverrides 验证不能套用通用 2006 规则的官方映射。
func TestStrictOOXMLNamespaceOverrides(t *testing.T) {
	tests := map[string]string{
		"http://purl.oclc.org/ooxml/descriptions/base":                               "http://descriptions.openxmlformats.org/description/base",
		"http://purl.oclc.org/ooxml/officeDocument/relationships/metadata/thumbnail": "http://schemas.openxmlformats.org/package/2006/relationships/metadata/thumbnail",
		"http://purl.oclc.org/ooxml/wordprocessingml/main":                           "http://schemas.openxmlformats.org/wordprocessingml/2006/main",
	}
	for strictURI, want := range tests {
		if got := strictOOXMLNamespaceToTransitional(strictURI); got != want {
			t.Errorf("map %s = %s, want %s", strictURI, got, want)
		}
	}
}

// mustMakeStrictOOXMLForTest 把测试 OOXML 包中的常见 Transitional URI
// 改写为对应 Strict URI；二进制媒体保持原样。
func mustMakeStrictOOXMLForTest(t *testing.T, data []byte) []byte {
	t.Helper()
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("open OOXML fixture: %v", err)
	}
	var output bytes.Buffer
	writer := zip.NewWriter(&output)
	files := append([]*zip.File(nil), reader.File...)
	sort.SliceStable(files, func(i, j int) bool { return files[i].Name < files[j].Name })
	for _, file := range files {
		rc, err := file.Open()
		if err != nil {
			t.Fatalf("open OOXML entry %s: %v", file.Name, err)
		}
		payload, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			t.Fatalf("read OOXML entry %s: %v", file.Name, err)
		}
		if strings.HasSuffix(file.Name, ".xml") || strings.HasSuffix(file.Name, ".rels") {
			payload = []byte(transitionalToStrictOOXMLForTest(string(payload)))
		}
		entry, err := writer.Create(file.Name)
		if err != nil {
			t.Fatalf("create strict OOXML entry %s: %v", file.Name, err)
		}
		if _, err := entry.Write(payload); err != nil {
			t.Fatalf("write strict OOXML entry %s: %v", file.Name, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close strict OOXML fixture: %v", err)
	}
	return output.Bytes()
}

// transitionalToStrictOOXMLForTest 映射集成样例用到的标准命名空间。
func transitionalToStrictOOXMLForTest(value string) string {
	return strings.NewReplacer(
		"http://schemas.openxmlformats.org/package/2006/content-types", "http://purl.oclc.org/ooxml/package/content-types",
		"http://schemas.openxmlformats.org/package/2006/relationships", "http://purl.oclc.org/ooxml/package/relationships",
		"http://schemas.openxmlformats.org/officeDocument/2006/relationships", "http://purl.oclc.org/ooxml/officeDocument/relationships",
		"http://schemas.openxmlformats.org/officeDocument/2006/math", "http://purl.oclc.org/ooxml/officeDocument/math",
		"http://schemas.openxmlformats.org/wordprocessingml/2006/main", "http://purl.oclc.org/ooxml/wordprocessingml/main",
		"http://schemas.openxmlformats.org/presentationml/2006/main", "http://purl.oclc.org/ooxml/presentationml/main",
		"http://schemas.openxmlformats.org/spreadsheetml/2006/main", "http://purl.oclc.org/ooxml/spreadsheetml/main",
		"http://schemas.openxmlformats.org/drawingml/2006/spreadsheetDrawing", "http://purl.oclc.org/ooxml/drawingml/spreadsheetDrawing",
		"http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing", "http://purl.oclc.org/ooxml/drawingml/wordprocessingDrawing",
		"http://schemas.openxmlformats.org/drawingml/2006/chart", "http://purl.oclc.org/ooxml/drawingml/chart",
		"http://schemas.openxmlformats.org/drawingml/2006/diagram", "http://purl.oclc.org/ooxml/drawingml/diagram",
		"http://schemas.openxmlformats.org/drawingml/2006/main", "http://purl.oclc.org/ooxml/drawingml/main",
	).Replace(value)
}

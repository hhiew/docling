// official_schema_validation_test.go 提供可选的 Docling Core 1.10 官方模型验收：
// 用仓库内自构造 fixture 解析全部支持格式，再交给显式配置的 Python 解释器
// 调用 DoclingDocument.model_validate 校验。日常 Go 测试不依赖 Python。
package docling

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// officialSchemaValidationCase 是传给官方校验进程的单格式解析结果。
type officialSchemaValidationCase struct {
	Name     string          `json:"name"`
	Document json.RawMessage `json:"document"`
}

// validateDocumentWithDoclingCore110ForTest 在显式配置官方 Python 环境时，
// 校验单个测试文档；未配置时保持日常纯 Go 测试零外部依赖。
func validateDocumentWithDoclingCore110ForTest(t *testing.T, name string, doc *DoclingDocument) {
	t.Helper()
	python := strings.TrimSpace(os.Getenv("DOCLING_PYTHON"))
	if python == "" {
		return
	}
	data, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("序列化 %s 官方校验文档: %v", name, err)
	}
	validateDocumentsWithDoclingCore110(t, python, []officialSchemaValidationCase{{Name: name, Document: data}})
}

// decodeOfficialImageFixture 解码内嵌的最小图片 fixture。
func decodeOfficialImageFixture(t *testing.T, encoded string) []byte {
	t.Helper()
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatalf("解码图片 fixture: %v", err)
	}
	return data
}

// TestAllSupportedFormatsValidateWithDoclingCore110 验证全部支持格式的规范输出
// 都能被 Docling Core schema 1.10.0 的官方 Pydantic 模型接受。
// 设置 DOCLING_PYTHON 为已安装 docling-core 的 Python 解释器后启用。
func TestAllSupportedFormatsValidateWithDoclingCore110(t *testing.T) {
	python := strings.TrimSpace(os.Getenv("DOCLING_PYTHON"))
	if python == "" {
		t.Skip("未配置 DOCLING_PYTHON，跳过可选官方模型校验")
	}

	docx := mustZipDocx(t, buildStructuredDocxXML(
		`<w:p><w:r><w:t>Word 正文</w:t></w:r></w:p>`,
	))
	xlsx := mustBuildTestXLSX(t, map[string][][]string{
		"数据": {{"名称", "值"}, {"温度", "23"}},
	})
	pdf := buildTestPDF(t, []testPDFPage{{
		content: "BT /F1 10 Tf 72 720 Td (PDF body text.) Tj ET\n",
	}})
	jpeg := decodeOfficialImageFixture(t, "/9j/4AAQSkZJRgABAQAAAQABAAD/2wBDAAgGBgcGBQgHBwcJCQgKDBQNDAsLDBkSEw8UHRofHh0aHBwgJC4nICIsIxwcKDcpLDAxNDQ0Hyc5PTgyPC4zNDL/2wBDAQkJCQwLDBgNDRgyIRwhMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjIyMjL/wAARCAABAAEDASIAAhEBAxEB/8QAHwAAAQUBAQEBAQEAAAAAAAAAAAECAwQFBgcICQoL/8QAtRAAAgEDAwIEAwUFBAQAAAF9AQIDAAQRBRIhMUEGE1FhByJxFDKBkaEII0KxwRVS0fAkM2JyggkKFhcYGRolJicoKSo0NTY3ODk6Q0RFRkdISUpTVFVWV1hZWmNkZWZnaGlqc3R1dnd4eXqDhIWGh4iJipKTlJWWl5iZmqKjpKWmp6ipqrKztLW2t7i5usLDxMXGx8jJytLT1NXW19jZ2uHi4+Tl5ufo6erx8vP09fb3+Pn6/8QAHwEAAwEBAQEBAQEBAQAAAAAAAAECAwQFBgcICQoL/8QAtREAAgECBAQDBAcFBAQAAQJ3AAECAxEEBSExBhJBUQdhcRMiMoEIFEKRobHBCSMzUvAVYnLRChYkNOEl8RcYGRomJygpKjU2Nzg5OkNERUZHSElKU1RVVldYWVpjZGVmZ2hpanN0dXZ3eHl6goOEhYaHiImKkpOUlZaXmJmaoqOkpaanqKmqsrO0tba3uLm6wsPExcbHyMnK0tPU1dbX2Nna4uPk5ebn6Onq8vP09fb3+Pn6/9oADAMBAAIRAxEAPwD3+iiigD//2Q==")
	bmp := decodeOfficialImageFixture(t, "Qk06AAAAAAAAADYAAAAoAAAAAQAAAAEAAAABABgAAAAAAAQAAADEDgAAxA4AAAAAAAAAAAAA////AA==")
	webp := decodeOfficialImageFixture(t, "UklGRiQAAABXRUJQVlA4IBgAAAAwAQCdASoBAAEAAUAmJaQAA3AA/vz0AAA=")

	fixtures := []struct {
		name string
		data []byte
	}{
		{name: "sample.txt", data: []byte("纯文本正文。")},
		{name: "sample.md", data: []byte("# Markdown 标题\n\n正文。\n")},
		{name: "sample.docx", data: docx},
		{name: "sample.xlsx", data: xlsx},
		{name: "sample.csv", data: []byte("名称,值\n温度,23\n")},
		{name: "sample.pdf", data: pdf},
		{name: "sample.pptx", data: buildTestPPTX(t)},
		{name: "sample.html", data: []byte("<html><head><title>网页标题</title></head><body><h1>章节</h1><p>正文。</p></body></html>")},
		{name: "sample.asciidoc", data: []byte("= AsciiDoc 标题\n\n正文。\n")},
		{name: "sample.eml", data: buildMultipartEML(t)},
		{name: "sample.png", data: minimalPNG},
		{name: "sample.jpg", data: jpeg},
		{name: "sample.jpeg", data: jpeg},
		{name: "sample.bmp", data: bmp},
		{name: "sample.webp", data: webp},
	}

	cases := make([]officialSchemaValidationCase, 0, len(fixtures))
	for _, fixture := range fixtures {
		doc, err := ParseByExtWithOptions(fixture.name, fixture.data, ParseOptions{
			OriginURI:              "s3://docling-fixtures/" + fixture.name,
			DisablePopplerFallback: true,
		})
		if err != nil {
			t.Fatalf("解析 %s: %v", fixture.name, err)
		}
		data, err := json.Marshal(doc)
		if err != nil {
			t.Fatalf("序列化 %s: %v", fixture.name, err)
		}
		cases = append(cases, officialSchemaValidationCase{Name: fixture.name, Document: data})
	}
	validateDocumentsWithDoclingCore110(t, python, cases)
}

// validateDocumentsWithDoclingCore110 调用显式配置的 Python 官方模型校验一组
// Docling JSON；Python 仅用于可选验收，不进入生产路径。
func validateDocumentsWithDoclingCore110(t *testing.T, python string, cases []officialSchemaValidationCase) {
	t.Helper()
	payload, err := json.Marshal(cases)
	if err != nil {
		t.Fatalf("序列化官方校验输入: %v", err)
	}
	const validator = `
import json
import mimetypes
import sys

# Docling Core 1.10 通过宿主机 mimetypes.types_map 校验 MIME；部分 Linux
# 发行版能猜出 WebP，却只把它放在非严格映射中。显式登记标准 IANA 类型，
# 避免官方模型校验结果随宿主机 /etc/mime.types 漂移。
mimetypes.add_type("image/webp", ".webp", strict=True)

from docling_core.types.doc import DoclingDocument
from docling_core.types.doc.common.constants import CURRENT_VERSION

if CURRENT_VERSION != "1.10.0":
    raise RuntimeError(f"unexpected Docling schema version: {CURRENT_VERSION}")

for case in json.load(sys.stdin):
    try:
        DoclingDocument.model_validate(case["document"])
    except Exception as exc:
        raise RuntimeError(f'{case["name"]}: {exc}') from exc
`
	cmd := exec.Command(python, "-c", validator)
	cmd.Stdin = strings.NewReader(string(payload))
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Docling Core 1.10 官方模型校验失败: %v\n%s", err, output)
	}
}

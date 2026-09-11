package docling

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

// TestDualRunWithDoclingServe 是可选的集成对比测试（金标准双跑）：
// 设置 DOCPARSE_DOCLING_URL（docling-serve 地址）与 DOCPARSE_SAMPLE_DIR
// （样本目录，支持 pdf/docx/xlsx/pptx/html/adoc/md/csv/txt）后运行，
// 对同批样本分别用 docling 与 docling-serve 解析，输出逐样本的结构
// 一致性摘要（元素数量/label 分布/表格数），供人工核对复刻质量。
// 未设置环境变量时跳过——默认单测不依赖外部服务。
//
// 运行示例：
//
//	DOCPARSE_DOCLING_URL=http://<host>:8000 DOCPARSE_SAMPLE_DIR=./testdata/samples \
//	  go test ./docling/ -run TestDualRunWithDoclingServe -count=1 -v
func TestDualRunWithDoclingServe(t *testing.T) {
	serveURL := strings.TrimRight(os.Getenv("DOCPARSE_DOCLING_URL"), "/")
	sampleDir := os.Getenv("DOCPARSE_SAMPLE_DIR")
	if serveURL == "" || sampleDir == "" {
		t.Skip("DOCPARSE_DOCLING_URL / DOCPARSE_SAMPLE_DIR 未设置，跳过双跑对比（默认不依赖外部服务）")
	}
	entries, err := os.ReadDir(sampleDir)
	if err != nil {
		t.Fatalf("读取样本目录失败: %v", err)
	}
	client := &http.Client{Timeout: 300 * time.Second}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		data, err := os.ReadFile(filepath.Join(sampleDir, name))
		if err != nil {
			t.Errorf("读取样本 %s 失败: %v", name, err)
			continue
		}
		goSummary := dualRunParseLocal(t, name, data)
		docSummary, err := dualRunParseDocling(t, client, serveURL, name, data)
		if err != nil {
			t.Errorf("样本 %s docling 解析失败: %v", name, err)
			continue
		}
		t.Logf("样本 %-28s\n  go     : %s\n  docling: %s", name, goSummary, docSummary)
	}
}

// dualRunParseLocal 用 docling 本地解析并产出结构摘要。
func dualRunParseLocal(t *testing.T, name string, data []byte) string {
	t.Helper()
	doc, err := ParseByExt(name, data)
	if err != nil {
		return fmt.Sprintf("解析失败: %v", err)
	}
	return doclingSummaryJSON(doc)
}

// dualRunParseDocling 调 docling-serve /v1/convert/file 并产出结构摘要。
func dualRunParseDocling(t *testing.T, client *http.Client, serveURL, name string, data []byte) (string, error) {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	_ = writer.WriteField("to_formats", "json")
	_ = writer.WriteField("image_export_mode", "placeholder")
	part, err := writer.CreateFormFile("files", name)
	if err != nil {
		return "", err
	}
	if _, err := part.Write(data); err != nil {
		return "", err
	}
	if err := writer.Close(); err != nil {
		return "", err
	}
	req, err := http.NewRequest(http.MethodPost, serveURL+"/v1/convert/file", &body)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("status %d", resp.StatusCode)
	}
	var wrap struct {
		Document struct {
			JSONContent json.RawMessage `json:"json_content"`
		} `json:"document"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&wrap); err != nil {
		return "", err
	}
	doc, err := ParseDoclingDocument(wrap.Document.JSONContent)
	if err != nil {
		return "", err
	}
	return doclingSummaryJSON(doc), nil
}

// doclingSummaryJSON 产出结构摘要 JSON：元素数、label 分布、表格行列数
// （忽略文本内容差异，聚焦结构一致性）。
func doclingSummaryJSON(doc *DoclingDocument) string {
	labels := map[string]int{}
	for _, it := range doc.Texts {
		labels[string(it.Label)]++
	}
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	fmt.Fprintf(&b, `{"texts":%d,"tables":%d,"pictures":%d,"pages":%d,"labels":{`,
		len(doc.Texts), len(doc.Tables), len(doc.Pictures), len(doc.Pages))
	for i, k := range keys {
		if i > 0 {
			b.WriteString(",")
		}
		fmt.Fprintf(&b, `%q:%d`, k, labels[k])
	}
	b.WriteString("}")
	if len(doc.Tables) > 0 {
		fmt.Fprintf(&b, `,"table_dims":[`)
		for i, tb := range doc.Tables {
			if i > 0 {
				b.WriteString(",")
			}
			rows, cols := int64(0), int64(0)
			if tb.Data != nil {
				rows, cols = tb.Data.NumRows, tb.Data.NumCols
			}
			fmt.Fprintf(&b, `"%dx%d"`, rows, cols)
		}
		b.WriteString("]")
	}
	b.WriteString("}")
	return b.String()
}

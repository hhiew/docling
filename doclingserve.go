package docparse

// doclingserve.go —— docling-serve HTTP 服务统一入口。
// 把「调用 docling-serve 解析文档 → 得到 DoclingDocument」的能力下沉到本组件，
// 与 golight 各 ParseXxx 并列成为双引擎之一；上层业务只做结果形状转换。

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"time"
)

// DoclingServiceOptions docling-serve HTTP 服务连接与请求参数。
type DoclingServiceOptions struct {
	// ServiceURL docling-serve 服务地址（如 http://docling-parser:5001），必填
	ServiceURL string
	// Timeout 单次 HTTP 请求超时，<=0 时默认 120s（文档解析较慢）
	Timeout time.Duration
	// Retry 失败重试次数，<0 视为 0
	Retry int
	// ImageExportMode 图片导出模式：placeholder（默认）/ embedded / referenced
	ImageExportMode string
	// DoOCR 是否启用服务端 OCR，默认 true
	DoOCR *bool
	// TableMode 表格解析模式：fast（默认）/ accurate
	TableMode string
}

// DoclingServeResult docling-serve 解析结果摘要：除 DoclingDocument 外的派生信息。
type DoclingServeResult struct {
	// Pages 页尺寸摘要（页号从 1 起）
	Pages []DoclingPageInfo
	// ProcessingTime 服务端处理耗时（秒）
	ProcessingTime float64
	// RawJSON docling-serve 返回的原始 DoclingDocument JSON（可二次解析或归档）
	RawJSON json.RawMessage
}

func (o DoclingServiceOptions) timeout() time.Duration {
	if o.Timeout <= 0 {
		return 120 * time.Second
	}
	return o.Timeout
}

func (o DoclingServiceOptions) retry() int {
	if o.Retry < 0 {
		return 0
	}
	return o.Retry
}

func (o DoclingServiceOptions) imageExportMode() string {
	if strings.TrimSpace(o.ImageExportMode) == "" {
		return "placeholder"
	}
	return o.ImageExportMode
}

func (o DoclingServiceOptions) doOCR() string {
	if o.DoOCR != nil && !*o.DoOCR {
		return "false"
	}
	return "true"
}

func (o DoclingServiceOptions) tableMode() string {
	if strings.TrimSpace(o.TableMode) == "" {
		return "fast"
	}
	return o.TableMode
}

func (o DoclingServiceOptions) baseURL() string {
	return strings.TrimRight(strings.TrimSpace(o.ServiceURL), "/")
}

// DoclingPageInfo 单页尺寸信息（页号从 1 起）。
type DoclingPageInfo struct {
	PageIdx int64   `json:"page_idx"`
	Width   float64 `json:"width"`
	Height  float64 `json:"height"`
}

// HealthCheckDoclingService 检查 docling-serve 服务是否可达（启动预热与运行时探测）。
func HealthCheckDoclingService(ctx context.Context, opts DoclingServiceOptions) error {
	if opts.baseURL() == "" {
		return fmt.Errorf("docparse: docling service url is empty")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, opts.baseURL()+"/health", nil)
	if err != nil {
		return err
	}
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("docparse: docling health check returned status %d", resp.StatusCode)
	}
	return nil
}

// ParseWithDoclingService 把单个文件提交给 docling-serve 解析，返回统一 DoclingDocument。
// 与 golight 各 ParseXxx 并列：复杂版面/扫描件/中文正式文档建议走本入口（质量显著优于规则提取）。
func ParseWithDoclingService(ctx context.Context, docName string, data []byte, opts DoclingServiceOptions) (*DoclingDocument, *DoclingServeResult, error) {
	if opts.baseURL() == "" {
		return nil, nil, fmt.Errorf("docparse: docling service url is empty")
	}
	if len(data) == 0 {
		return nil, nil, fmt.Errorf("docparse: empty file data")
	}

	endpoint := opts.baseURL() + "/v1/convert/file"
	var lastErr error
	for attempt := 0; attempt <= opts.retry(); attempt++ {
		doc, result, err := doParseWithDoclingService(ctx, endpoint, docName, data, opts)
		if err == nil {
			return doc, result, nil
		}
		lastErr = err
		if attempt < opts.retry() {
			select {
			case <-ctx.Done():
				return nil, nil, ctx.Err()
			case <-time.After(time.Duration(attempt+1) * time.Second):
			}
		}
	}
	return nil, nil, fmt.Errorf("docparse: docling parse failed after %d retries: %w", opts.retry(), lastErr)
}

// doParseWithDoclingService 单次提交解析请求。
func doParseWithDoclingService(ctx context.Context, endpoint, docName string, data []byte, opts DoclingServiceOptions) (*DoclingDocument, *DoclingServeResult, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	_ = writer.WriteField("to_formats", "json")
	_ = writer.WriteField("image_export_mode", opts.imageExportMode())
	_ = writer.WriteField("do_ocr", opts.doOCR())
	_ = writer.WriteField("table_mode", opts.tableMode())

	part, err := writer.CreateFormFile("files", docName)
	if err != nil {
		return nil, nil, err
	}
	if _, err := part.Write(data); err != nil {
		return nil, nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, &body)
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{Timeout: opts.timeout()}
	httpResp, err := client.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer httpResp.Body.Close()

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, nil, err
	}
	if httpResp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("docparse: docling returned status %d: %s", httpResp.StatusCode, string(respBody))
	}

	// docling-serve /v1/convert/file 响应：{status, processing_time, document:{json_content}, errors}
	var serveResp struct {
		Status         string  `json:"status"`
		ProcessingTime float64 `json:"processing_time"`
		Document       struct {
			JSONContent json.RawMessage `json:"json_content"`
		} `json:"document"`
		Errors []string `json:"errors"`
	}
	if err := json.Unmarshal(respBody, &serveResp); err != nil {
		return nil, nil, err
	}
	if serveResp.Status != "success" {
		return nil, nil, fmt.Errorf("docparse: docling status=%s errors=%v", serveResp.Status, serveResp.Errors)
	}

	doc, err := ParseDoclingDocument(serveResp.Document.JSONContent)
	if err != nil {
		return nil, nil, err
	}
	return doc, &DoclingServeResult{
		Pages:          doclingServePages(doc),
		ProcessingTime: serveResp.ProcessingTime,
		RawJSON:        serveResp.Document.JSONContent,
	}, nil
}

// doclingServePages 从 DoclingDocument 派生页尺寸摘要（页号 1 起展示口径）。
func doclingServePages(doc *DoclingDocument) []DoclingPageInfo {
	out := make([]DoclingPageInfo, 0, len(doc.Pages))
	for _, pageNo := range doc.SortPageNos() {
		page, ok := doc.Pages[fmt.Sprintf("%d", pageNo)]
		if !ok || page.Size == nil {
			continue
		}
		out = append(out, DoclingPageInfo{PageIdx: pageNo, Width: page.Size.Width, Height: page.Size.Height})
	}
	return out
}

// openai.go 实现 OpenAI 兼容协议的多模态客户端：读取环境变量配置，把
// VisionRequest 组装为 chat/completions 请求（文本 + image_url data URI
// 部件），并兼容 string 与分段数组两种 message.content 返回形态。
package llmocr

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// 环境变量名：base_url / api_key / 模型名。
const (
	// EnvBaseURL 覆盖服务地址（默认 https://api.openai.com/v1）。
	EnvBaseURL = "OPENAI_BASE_URL"
	// EnvAPIKey 是鉴权密钥（必需）。
	EnvAPIKey = "OPENAI_API_KEY"
	// EnvModel 是多模态模型名（默认 gpt-4o）。
	EnvModel = "DOCLING_OCR_MODEL"
)

// 默认值。
const (
	DefaultBaseURL = "https://api.openai.com/v1"
	DefaultModel   = "gpt-4o"
	// DefaultTimeout 是单次识别调用的超时时间。
	DefaultTimeout = 120 * time.Second
)

// OpenAIConfig 是 OpenAIClient 的配置。
type OpenAIConfig struct {
	// BaseURL 是 API 根地址（不含 /chat/completions）。
	BaseURL string
	// APIKey 是 Bearer 鉴权密钥。
	APIKey string
	// Model 是多模态模型名。
	Model string
	// Timeout 是单次调用超时；0 用 DefaultTimeout。
	Timeout time.Duration
	// HTTPClient 覆盖默认 http.Client（测试注入用）；非 nil 时忽略 Timeout。
	HTTPClient *http.Client
}

// OpenAIClient 是 LLMClient 的 OpenAI 兼容实现。
type OpenAIClient struct {
	config OpenAIConfig
	client *http.Client
}

// NewOpenAIClientFromEnv 从环境变量构建客户端；OPENAI_API_KEY 未设置时返回错误。
func NewOpenAIClientFromEnv() (*OpenAIClient, error) {
	return NewOpenAIClient(OpenAIConfig{
		BaseURL: os.Getenv(EnvBaseURL),
		APIKey:  os.Getenv(EnvAPIKey),
		Model:   os.Getenv(EnvModel),
	})
}

// NewOpenAIClient 用显式配置构建客户端；BaseURL/Model 为空时取默认值，
// APIKey 为空返回错误。
func NewOpenAIClient(cfg OpenAIConfig) (*OpenAIClient, error) {
	if cfg.BaseURL == "" {
		cfg.BaseURL = DefaultBaseURL
	}
	if cfg.Model == "" {
		cfg.Model = DefaultModel
	}
	if cfg.APIKey == "" {
		return nil, errors.New("llmocr: api key is required (OPENAI_API_KEY)")
	}
	client := cfg.HTTPClient
	if client == nil {
		timeout := cfg.Timeout
		if timeout <= 0 {
			timeout = DefaultTimeout
		}
		client = &http.Client{Timeout: timeout}
	}
	return &OpenAIClient{config: cfg, client: client}, nil
}

// openaiChatRequest 是 chat/completions 请求体（仅本包用到的字段）。
type openaiChatRequest struct {
	Model    string          `json:"model"`
	Messages []openaiMessage `json:"messages"`
	Stream   bool            `json:"stream"`
}

// openaiMessage 是一条对话消息；Content 用 RawMessage 以兼容纯文本与分段数组。
type openaiMessage struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

// openaiTextPart 是 content 数组中的文本段。
type openaiTextPart struct {
	Type string `json:"type"` // "text"
	Text string `json:"text"`
}

// openaiImagePart 是 content 数组中的图片段（data URI）。
type openaiImagePart struct {
	Type     string `json:"type"` // "image_url"
	ImageURL struct {
		URL string `json:"url"`
	} `json:"image_url"`
}

// openaiChatResponse 是响应体（仅本包用到的字段）。
type openaiChatResponse struct {
	Choices []struct {
		Message struct {
			Content json.RawMessage `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

// Complete 实现 LLMClient：组装 messages 发送并提取首个 choice 的文本。
func (c *OpenAIClient) Complete(ctx context.Context, req VisionRequest) (string, error) {
	userParts := make([]json.RawMessage, 0, len(req.Images)+1)
	if prompt := strings.TrimSpace(req.Prompt); prompt != "" {
		raw, err := json.Marshal(openaiTextPart{Type: "text", Text: prompt})
		if err != nil {
			return "", err
		}
		userParts = append(userParts, raw)
	}
	for _, img := range req.Images {
		part := openaiImagePart{Type: "image_url"}
		part.ImageURL.URL = img.DataURI
		raw, err := json.Marshal(part)
		if err != nil {
			return "", err
		}
		userParts = append(userParts, raw)
	}

	messages := make([]openaiMessage, 0, 2)
	if sys := strings.TrimSpace(req.SystemPrompt); sys != "" {
		messages = append(messages, openaiMessage{Role: "system", Content: mustRawJSON(sys)})
	}
	userContent, err := json.Marshal(userParts)
	if err != nil {
		return "", err
	}
	messages = append(messages, openaiMessage{Role: "user", Content: userContent})

	body, err := json.Marshal(openaiChatRequest{Model: c.config.Model, Messages: messages})
	if err != nil {
		return "", err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		strings.TrimRight(c.config.BaseURL, "/")+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.config.APIKey)

	httpResp, err := c.client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("llmocr: request failed: %w", err)
	}
	defer httpResp.Body.Close()
	respBody, err := io.ReadAll(io.LimitReader(httpResp.Body, 32<<20))
	if err != nil {
		return "", fmt.Errorf("llmocr: read response failed: %w", err)
	}
	if httpResp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("llmocr: http %d: %s", httpResp.StatusCode, truncateRunes(strings.TrimSpace(string(respBody)), 500))
	}
	var parsed openaiChatResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return "", fmt.Errorf("llmocr: decode response failed: %w", err)
	}
	if len(parsed.Choices) == 0 {
		return "", errors.New("llmocr: response has no choices")
	}
	return decodeOpenAIContent(parsed.Choices[0].Message.Content)
}

// decodeOpenAIContent 兼容 message.content 的两种形态：纯字符串或分段数组
// （取其中 text 段拼接）。
func decodeOpenAIContent(raw json.RawMessage) (string, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return "", nil
	}
	if trimmed[0] == '"' {
		var s string
		if err := json.Unmarshal(trimmed, &s); err != nil {
			return "", err
		}
		return s, nil
	}
	var parts []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(trimmed, &parts); err != nil {
		return "", fmt.Errorf("llmocr: decode content failed: %w", err)
	}
	var b strings.Builder
	for _, p := range parts {
		b.WriteString(p.Text)
	}
	return b.String(), nil
}

// mustRawJSON 把字符串编码为 JSON RawMessage（输入恒为合法文本，忽略错误）。
func mustRawJSON(s string) json.RawMessage {
	raw, err := json.Marshal(s)
	if err != nil {
		return json.RawMessage(`""`)
	}
	return raw
}

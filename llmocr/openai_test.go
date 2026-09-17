// openai_test.go 覆盖 OpenAI 兼容客户端：请求组包（模型/鉴权/分段 content）、
// 两种 message.content 返回形态与错误传播。
package llmocr

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestOpenAIClientRequestShape 验证请求组包与 string content 解析。
func TestOpenAIClientRequestShape(t *testing.T) {
	var gotPath, gotAuth, gotBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		body, _ := io.ReadAll(r.Body)
		gotBody = string(body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"识别结果"}}]}`))
	}))
	defer server.Close()

	client, err := NewOpenAIClient(OpenAIConfig{BaseURL: server.URL, APIKey: "sk-test", Model: "vl-model"})
	if err != nil {
		t.Fatal(err)
	}
	got, err := client.Complete(context.Background(), VisionRequest{
		SystemPrompt: "系统",
		Prompt:       "识别这一页",
		Images:       []ImageInput{{MIMEType: "image/png", DataURI: "data:image/png;base64,QUJD"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got != "识别结果" {
		t.Fatalf("got=%q", got)
	}
	if gotPath != "/chat/completions" || gotAuth != "Bearer sk-test" {
		t.Fatalf("path=%s auth=%s", gotPath, gotAuth)
	}
	var parsed struct {
		Model    string `json:"model"`
		Messages []struct {
			Role    string          `json:"role"`
			Content json.RawMessage `json:"content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal([]byte(gotBody), &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed.Model != "vl-model" || len(parsed.Messages) != 2 {
		t.Fatalf("parsed=%+v body=%s", parsed, gotBody)
	}
	if !strings.Contains(gotBody, `"image_url":{"url":"data:image/png;base64,QUJD"}`) {
		t.Fatalf("body missing image part: %s", gotBody)
	}
}

// TestOpenAIClientArrayContent 验证分段数组 content 形态。
func TestOpenAIClientArrayContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":[{"type":"text","text":"部"},{"type":"text","text":"分"}]}}]}`))
	}))
	defer server.Close()
	client, _ := NewOpenAIClient(OpenAIConfig{BaseURL: server.URL, APIKey: "k"})
	got, err := client.Complete(context.Background(), VisionRequest{Prompt: "p", Images: []ImageInput{{DataURI: "data:image/png;base64,x"}}})
	if err != nil {
		t.Fatal(err)
	}
	if got != "部分" {
		t.Fatalf("got=%q", got)
	}
}

// TestOpenAIClientErrors 验证非 200 与缺 API Key 的错误传播。
func TestOpenAIClientErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte("rate limited"))
	}))
	defer server.Close()
	client, err := NewOpenAIClient(OpenAIConfig{BaseURL: server.URL, APIKey: "k"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.Complete(context.Background(), VisionRequest{Prompt: "p"}); err == nil || !strings.Contains(err.Error(), "429") {
		t.Fatalf("err=%v", err)
	}
	if _, err := NewOpenAIClient(OpenAIConfig{}); err == nil {
		t.Fatal("expect api key error")
	}
}

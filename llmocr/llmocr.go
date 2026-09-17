// Package llmocr 提供把任意 OpenAI 兼容多模态模型接入 docling 解析钩子的
// 适配层：OCRHook 负责扫描页/乱码页的逐字识别，PDFVisualHook 负责疑难页
// 的结构化视觉识别。本包只定义"发请求"的最小抽象（LLMClient）与两个钩子
// 构造器；结果校验（重试、乱码率、拒答、bbox 合法性）由 docling 库内置
// 完成，本包不重复实现。
package llmocr

import "context"

// ImageInput 是一次识别请求的输入部件：图片或单页 PDF 的 data URI。
type ImageInput struct {
	// MIMEType 是部件 MIME（image/png、application/pdf 等）。
	MIMEType string
	// DataURI 是完整 data URI（data:<mime>;base64,<payload>）。
	DataURI string
}

// VisionRequest 描述一次多模态识别调用。
type VisionRequest struct {
	// SystemPrompt 是系统提示；为空时不发送 system 消息。
	SystemPrompt string
	// Prompt 是用户提示词（识别要求、页号上下文、纠错反馈等）。
	Prompt string
	// Images 是按序排列的输入部件；通常为 1 个（单页 PDF 或图片）。
	Images []ImageInput
}

// LLMClient 是多模态模型调用的最小抽象；实现方负责鉴权、超时与协议格式。
type LLMClient interface {
	// Complete 发起一次识别调用，返回模型文本输出；网络/协议错误返回 error。
	Complete(ctx context.Context, req VisionRequest) (string, error)
}

// 默认提示词：与 docling 库的校验语义对齐（逐字输出、拒绝拒答、Markdown 结构）。
const (
	// DefaultOCRSystemPrompt 是 OCR 识别的系统提示。
	DefaultOCRSystemPrompt = "你是精确的文档识别引擎。只输出页面内容的 Markdown 表示：标题用 # 层级、表格用 GFM 管道表格、公式用 $...$ 或 $$...$$ LaTeX、列表保留层级。不总结、不解释、不改写、不添加 Markdown 代码围栏或任何说明文字。"
	// DefaultVisualSystemPrompt 是结构化视觉识别的系统提示。
	DefaultVisualSystemPrompt = "你是 PDF 版面结构识别引擎。严格按照用户提示词中的 JSON 协议输出，不使用 Markdown 围栏、不输出协议之外的任何文字。"
)

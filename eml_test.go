// eml_test.go 验证 EML 解析：Subject/引用头与 DocMeta、multipart 递归
// （text/plain + text/html + message/rfc822 嵌套）、base64 正文解码、
// 畸形输入报错与 ParseByExt 注册。
package docparse

import (
	"encoding/base64"
	"strings"
	"testing"
	"time"
)

// buildMultipartEML 构造 multipart/mixed 样例邮件：text/plain + text/html +
// 嵌套 message/rfc822 三部分；extraParts 追加到嵌套部分之前（base64 用例用）。
func buildMultipartEML(t *testing.T, extraParts ...string) []byte {
	t.Helper()
	subjectEnc := base64.StdEncoding.EncodeToString([]byte("项目周报"))
	forwardEnc := base64.StdEncoding.EncodeToString([]byte("转发周报"))
	var b strings.Builder
	b.WriteString("From: Zhang San <zhang@example.com>\r\n")
	b.WriteString("To: li@example.com, wang@example.com\r\n")
	b.WriteString("Subject: =?utf-8?B?" + subjectEnc + "?=\r\n")
	b.WriteString("Date: Mon, 07 Sep 2026 10:00:00 +0800\r\n")
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: multipart/mixed; boundary=\"BOUND\"\r\n")
	b.WriteString("\r\n")
	for _, part := range extraParts {
		b.WriteString(part)
	}
	b.WriteString("--BOUND\r\n")
	b.WriteString("Content-Type: text/plain; charset=utf-8\r\n\r\n")
	b.WriteString("本周进展：完成 P0 能力补齐。\r\n\r\n")
	b.WriteString("--BOUND\r\n")
	b.WriteString("Content-Type: text/html; charset=utf-8\r\n\r\n")
	b.WriteString("<p>风险说明：接口联调延期。</p>\r\n\r\n")
	b.WriteString("--BOUND\r\n")
	b.WriteString("Content-Type: message/rfc822\r\n\r\n")
	b.WriteString("From: bob@example.com\r\n")
	b.WriteString("Subject: =?utf-8?B?" + forwardEnc + "?=\r\n")
	b.WriteString("Content-Type: text/plain; charset=utf-8\r\n\r\n")
	b.WriteString("内层邮件正文。\r\n\r\n")
	b.WriteString("--BOUND--\r\n")
	return []byte(b.String())
}

// TestParseEMLEndToEnd 验证完整链路：Subject 标题项与 Meta（Title/Author/
// CreatedAt）、From/To/Date 键值文本项、三类 part 的正文并入与嵌套邮件递归，
// 以及 ParseByExt 对 .eml 的注册。
func TestParseEMLEndToEnd(t *testing.T) {
	doc, err := ParseEML(buildMultipartEML(t))
	if err != nil {
		t.Fatalf("ParseEML: %v", err)
	}
	// 标题项：Subject 解码为中文且产出 label=title 的首个元素
	if len(doc.Texts) == 0 || doc.Texts[0].Label != LabelTitle || doc.Texts[0].Text != "项目周报" {
		t.Fatalf("subject title wrong: %+v", doc.Texts)
	}
	// 引用头键值文本项
	joined := doc.Text()
	for _, want := range []string{
		"From: Zhang San <zhang@example.com>",
		"To: li@example.com, wang@example.com",
		"Date: Mon, 07 Sep 2026 10:00:00 +0800",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing header item %q in:\n%s", want, joined)
		}
	}
	// Meta：Author 取 From 显示名，CreatedAt 归一 RFC3339 UTC
	if doc.Meta == nil {
		t.Fatal("meta should not be nil")
	}
	if doc.Meta.Title != "项目周报" || doc.Meta.Subject != "项目周报" || doc.Meta.Author != "Zhang San" {
		t.Fatalf("meta wrong: %+v", doc.Meta)
	}
	wantTime, _ := time.Parse(time.RFC3339, "2026-09-07T02:00:00Z")
	gotTime, err := time.Parse(time.RFC3339, doc.Meta.CreatedAt)
	if err != nil || !gotTime.Equal(wantTime) {
		t.Fatalf("CreatedAt = %q, want 2026-09-07T02:00:00Z (%v)", doc.Meta.CreatedAt, err)
	}
	// 正文：text/plain 原文、text/html 提取文本、嵌套邮件 Subject 标题与正文
	for _, want := range []string{"本周进展：完成 P0 能力补齐。", "风险说明：接口联调延期。", "内层邮件正文。"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing body %q in:\n%s", want, joined)
		}
	}
	var sawForwardTitle bool
	for _, txt := range doc.Texts {
		if txt.Label == LabelTitle && txt.Text == "转发周报" {
			sawForwardTitle = true
		}
	}
	if !sawForwardTitle {
		t.Fatalf("nested eml subject title missing: %+v", doc.Texts)
	}
	// ParseByExt 注册 .eml
	if _, err := ParseByExt("邮件.eml", buildMultipartEML(t)); err != nil {
		t.Fatalf("ParseByExt .eml: %v", err)
	}
}

// TestParseEMLQuotedPrintableAndBase64Parts 验证叶子 part 的
// quoted-printable 与 base64 传输编码解码。
func TestParseEMLQuotedPrintableAndBase64Parts(t *testing.T) {
	qpPart := "--BOUND\r\nContent-Type: text/plain; charset=utf-8\r\n" +
		"Content-Transfer-Encoding: quoted-printable\r\n\r\n" +
		"进度=E6=8F=90=E7=A4=BA：已就绪。\r\n\r\n"
	b64Body := base64.StdEncoding.EncodeToString([]byte("附件说明：样例。"))
	b64Part := "--BOUND\r\nContent-Type: text/plain; charset=utf-8\r\n" +
		"Content-Transfer-Encoding: base64\r\n\r\n" +
		b64Body + "\r\n\r\n"
	doc, err := ParseEML(buildMultipartEML(t, qpPart, b64Part))
	if err != nil {
		t.Fatalf("ParseEML: %v", err)
	}
	joined := doc.Text()
	for _, want := range []string{"进度提示：已就绪。", "附件说明：样例。"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing decoded body %q in:\n%s", want, joined)
		}
	}
}

// TestParseEMLBinaryAttachmentNamed 验证二进制附件不解析负载，只按解码后的
// 文件名与 MIME 生成可追溯名称项。
func TestParseEMLBinaryAttachmentNamed(t *testing.T) {
	name := base64.StdEncoding.EncodeToString([]byte("项目附件.pdf"))
	binPart := "--BOUND\r\nContent-Type: application/octet-stream\r\n" +
		"Content-Disposition: attachment; filename=\"=?utf-8?B?" + name + "?=\"\r\n" +
		"Content-Transfer-Encoding: base64\r\n\r\n" +
		base64.StdEncoding.EncodeToString([]byte{0x00, 0x01, 0x02}) + "\r\n\r\n"
	doc, err := ParseEML(buildMultipartEML(t, binPart))
	if err != nil {
		t.Fatalf("ParseEML: %v", err)
	}
	if !strings.Contains(doc.Text(), "项目附件.pdf (application/octet-stream)") {
		t.Fatalf("decoded attachment name missing: %q", doc.Text())
	}
}

// TestParseEMLBodyCharset 验证正文按 Content-Type charset 解码常见旧编码。
func TestParseEMLBodyCharset(t *testing.T) {
	data := append([]byte("From: a@example.com\r\nSubject: charset\r\nContent-Type: text/plain; charset=windows-1252\r\n\r\ncaf"), 0xe9)
	doc, err := ParseEML(data)
	if err != nil {
		t.Fatalf("ParseEML: %v", err)
	}
	if !strings.Contains(doc.Text(), "café") {
		t.Fatalf("windows-1252 body not decoded: %q", doc.Text())
	}
}

// TestParseEMLPlainFallback 验证无 MIME 结构的纯文本邮件按 text/plain 兜底
// 解析正文。
func TestParseEMLPlainFallback(t *testing.T) {
	data := []byte("From: a@example.com\r\n" +
		"Subject: simple\r\n" +
		"\r\n" +
		"这是正文第一行。\r\n这是第二行。\r\n")
	doc, err := ParseEML(data)
	if err != nil {
		t.Fatalf("ParseEML: %v", err)
	}
	if !strings.Contains(doc.Text(), "这是正文第一行。") || !strings.Contains(doc.Text(), "这是第二行。") {
		t.Fatalf("plain body missing:\n%s", doc.Text())
	}
	if doc.Meta == nil || doc.Meta.Title != "simple" {
		t.Fatalf("meta wrong: %+v", doc.Meta)
	}
}

// TestParseEMLAlternativePicksBestRepresentation 验证 multipart/alternative
// 只解析最佳表示（multipart/related > text/html > text/plain），正文不重复。
func TestParseEMLAlternativePicksBestRepresentation(t *testing.T) {
	var b strings.Builder
	b.WriteString("From: a@example.com\r\n")
	b.WriteString("Subject: alt\r\n")
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: multipart/alternative; boundary=\"ALT\"\r\n\r\n")
	b.WriteString("--ALT\r\n")
	b.WriteString("Content-Type: text/plain; charset=utf-8\r\n\r\n")
	b.WriteString("纯文本版本的正文。\r\n\r\n")
	b.WriteString("--ALT\r\n")
	b.WriteString("Content-Type: text/html; charset=utf-8\r\n\r\n")
	b.WriteString("<p>富文本版本的正文。</p>\r\n\r\n")
	b.WriteString("--ALT--\r\n")
	doc, err := ParseEML([]byte(b.String()))
	if err != nil {
		t.Fatalf("ParseEML: %v", err)
	}
	joined := doc.Text()
	if !strings.Contains(joined, "富文本版本的正文。") {
		t.Fatalf("html representation missing:\n%s", joined)
	}
	if strings.Contains(joined, "纯文本版本的正文。") {
		t.Fatalf("plain representation should be skipped for alternative:\n%s", joined)
	}
}

// TestParseEMLMalformedHeader 验证非邮件输入返回 error。
func TestParseEMLMalformedHeader(t *testing.T) {
	if _, err := ParseEML([]byte("not an eml at all")); err == nil {
		t.Fatal("malformed eml should return error")
	}
}

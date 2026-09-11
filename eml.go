// eml.go 实现 EML（RFC 5322 邮件，含 MIME 多部分）解析为 DoclingDocument：
//   - 头解析：Subject 经 RFC 2047 解码后产出标题项并填 Meta.Title/Subject，
//     From 显示名填 Meta.Author、Date 填 Meta.CreatedAt（RFC3339 UTC），
//     From/To/Date 引用头以"键: 值"文本项产出；
//   - 正文按 MIME 递归拆解（深度上限 emlMaxDepth）：multipart 逐 part 递归，
//     text/html 复用 ParseHTML、text/plain 复用 ParseText、message/rfc822
//     附件递归 ParseEML，其余二进制附件跳过；
//   - Content-Transfer-Encoding 显式解码 base64 / quoted-printable，
//     字符集尽力转换（utf-8/us-ascii/iso-8859-1）；
//   - 各 part 解析失败仅跳过该 part（不阻断整体），顶层头解析失败返回 error。
package docling

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/mail"
	"strings"
	"time"

	"golang.org/x/text/encoding/htmlindex"
	"golang.org/x/text/transform"
)

// emlMaxDepth MIME 嵌套深度上限（multipart/message 递归层数），防御构造
// 恶意的深嵌套邮件；超过深度的部分跳过。
const emlMaxDepth = 16

// ParseEML 解析 EML 邮件（RFC 5322 头 + MIME 正文）为 DoclingDocument。
// 产出结构：Subject 标题项 → From/To/Date 引用头键值文本项 → 各 part 正文
// （按 MIME 顺序并入 body）；顶层头解析失败返回 error，正文 part 解析失败
// 时跳过该 part（不视为错误）。
func ParseEML(data []byte) (*DoclingDocument, error) {
	return parseEML(data, 0)
}

// parseEML ParseEML 的递归入口：depth 用于嵌套深度防护（message/rfc822
// 附件递归时 +1）。
func parseEML(data []byte, depth int) (*DoclingDocument, error) {
	if depth > emlMaxDepth {
		return nil, fmt.Errorf("docling: eml 嵌套深度超过 %d 层", emlMaxDepth)
	}
	msg, err := mail.ReadMessage(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("docling: 解析 eml 头失败: %w", err)
	}
	doc := NewDoclingDocument("eml")
	w := &emlWalker{doc: doc, depth: depth}
	w.parseHeaders(msg.Header)
	// 正文各 part 失败在 parsePart 内部消化：整体不因单个 part 失败而报错
	_ = w.parsePart(msg.Body, msg.Header, depth)
	return doc, nil
}

// emlWalker EML 解析状态：目标文档与当前递归深度。
type emlWalker struct {
	doc   *DoclingDocument
	depth int
}

// parseHeaders 解析邮件头：Subject 产出标题项并填 Meta.Title/Subject，
// From 显示名填 Meta.Author、Date 填 Meta.CreatedAt，From/To/Date 引用头
// 产出键值文本项。头值经 RFC 2047 编码头解码，解码失败保留原值。
func (w *emlWalker) parseHeaders(h mail.Header) {
	meta := &DocMeta{}
	subject := emlDecodeHeader(h.Get("Subject"))
	if subject != "" {
		meta.Title = subject
		meta.Subject = subject
		w.doc.AddTitle(subject, nil, nil)
	}
	if addrs, err := h.AddressList("From"); err == nil {
		names := make([]string, 0, len(addrs))
		for _, a := range addrs {
			if a.Name != "" {
				names = append(names, a.Name)
			} else {
				names = append(names, a.Address)
			}
		}
		meta.Author = strings.Join(names, ", ")
	}
	if date, err := h.Date(); err == nil {
		meta.CreatedAt = date.UTC().Format(time.RFC3339)
	}
	// 引用头以键值文本项产出，保留邮件可追溯性（谁发的/发给谁/何时）
	for _, kv := range []struct{ key, val string }{
		{"From", emlDecodeHeader(h.Get("From"))},
		{"To", emlDecodeHeader(h.Get("To"))},
		{"Date", emlDecodeHeader(h.Get("Date"))},
	} {
		if kv.val != "" {
			w.doc.AddText(LabelText, kv.key+": "+kv.val, nil, nil)
		}
	}
	if *meta != (DocMeta{}) {
		w.doc.Meta = meta
	}
}

// parsePart 处理单个 MIME part 的正文（header 携带该 part 的
// Content-Type/Content-Transfer-Encoding）：multipart 逐子 part 递归，
// message/rfc822 递归 parseEML，text/html / text/plain 复用对应解析器，
// 其余类型（二进制附件等）跳过。
// 参数 body 为该 part 的原始内容流（未解码），depth 为当前 MIME 深度。
func (w *emlWalker) parsePart(body io.Reader, header mail.Header, depth int) error {
	if depth > emlMaxDepth {
		return fmt.Errorf("docling: eml MIME 嵌套超过 %d 层", emlMaxDepth)
	}
	mediaType, params, err := mime.ParseMediaType(strings.TrimSpace(header.Get("Content-Type")))
	if err != nil {
		// Content-Type 缺失或畸形：按 text/plain 兜底（RFC 5322 默认语义）
		mediaType, params = "text/plain", nil
	}
	if strings.HasPrefix(mediaType, "multipart/") {
		boundary := params["boundary"]
		if boundary == "" {
			return fmt.Errorf("docling: eml multipart 缺少 boundary")
		}
		return w.walkMultipart(body, mediaType, boundary, depth)
	}
	// 附件负载不作为正文解析，只保留解码后的文件名和 MIME，避免 text/plain
	// 附件污染正文，同时保持邮件内容可追溯。
	if filename := emlAttachmentFilename(header, params); filename != "" {
		w.doc.AddText(LabelText, filename+" ("+mediaType+")", nil, nil)
		return nil
	}
	// 叶子 part：先按 CTE 解码，再按类型分派
	decoded := emlDecodeTransfer(body, header.Get("Content-Transfer-Encoding"))
	switch {
	case mediaType == "message/rfc822":
		subData, readErr := io.ReadAll(decoded)
		if readErr != nil {
			return readErr
		}
		sub, subErr := parseEML(subData, depth+1)
		if subErr != nil {
			return subErr
		}
		mergeOCRSubDocument(w.doc, sub, 0)
		return nil
	case mediaType == "text/html":
		subData, readErr := emlReadText(decoded, params["charset"])
		if readErr != nil {
			return readErr
		}
		sub, subErr := ParseHTML(subData)
		if subErr != nil {
			return subErr
		}
		mergeOCRSubDocument(w.doc, sub, 0)
		return nil
	case mediaType == "text/plain":
		subData, readErr := emlReadText(decoded, params["charset"])
		if readErr != nil {
			return readErr
		}
		sub, subErr := ParseText(subData)
		if subErr != nil {
			return subErr
		}
		mergeOCRSubDocument(w.doc, sub, 0)
		return nil
	default:
		// 其余类型（二进制附件等）跳过：知识库场景仅消费文本正文
		return nil
	}
}

// emlAttachmentFilename 按 Content-Disposition filename、Content-Type name
// 的顺序提取附件名，并解码 RFC 2047 编码字。
func emlAttachmentFilename(header mail.Header, contentTypeParams map[string]string) string {
	disposition, params, _ := mime.ParseMediaType(strings.TrimSpace(header.Get("Content-Disposition")))
	filename := strings.TrimSpace(params["filename"])
	if filename == "" && strings.EqualFold(disposition, "attachment") {
		filename = strings.TrimSpace(contentTypeParams["name"])
	}
	if filename == "" && !strings.EqualFold(disposition, "attachment") {
		return ""
	}
	if filename == "" {
		return "attachment"
	}
	return emlDecodeHeader(filename)
}

// emlReadText 读取 MIME 文本并按 charset 转为 UTF-8；未知字符集回退原始
// 字节，由下游文本清洗保证解析不中断。
func emlReadText(reader io.Reader, charset string) ([]byte, error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	charset = strings.TrimSpace(charset)
	if charset == "" || strings.EqualFold(charset, "utf-8") || strings.EqualFold(charset, "us-ascii") {
		return data, nil
	}
	encoding, err := htmlindex.Get(charset)
	if err != nil || encoding == nil {
		return data, nil
	}
	decoded, _, err := transform.Bytes(encoding.NewDecoder(), data)
	if err != nil {
		return data, nil
	}
	return decoded, nil
}

// walkMultipart 逐个解析 multipart 的子 part 并递归 parsePart；
// 单个子 part 解码/解析失败仅跳过该 part，继续处理后续 part。
// multipart/alternative 单独处理（各 part 为同一内容的不同表示，只取一份）。
func (w *emlWalker) walkMultipart(body io.Reader, mediaType, boundary string, depth int) error {
	if strings.EqualFold(mediaType, "multipart/alternative") {
		return w.walkAlternative(body, boundary, depth)
	}
	mr := multipart.NewReader(body, boundary)
	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		// 单个子 part 失败不阻断：跳过继续
		_ = w.parsePart(part, mail.Header(part.Header), depth+1)
	}
}

// walkAlternative 处理 multipart/alternative：各 part 是同一正文的不同保真度
// 表示（按惯例保真度递增排列），只解析"最佳"一份（multipart/related >
// text/html > text/plain），避免同一正文重复并入文档。
func (w *emlWalker) walkAlternative(body io.Reader, boundary string, depth int) error {
	mr := multipart.NewReader(body, boundary)
	var bestHeader mail.Header
	var bestBody []byte
	bestScore := -1
	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		data, readErr := io.ReadAll(part)
		if readErr != nil {
			continue // 单个表示读取失败：尝试下一个
		}
		score := 1 // 未知类型按 text/plain 同级兜底
		ct, _, ctErr := mime.ParseMediaType(part.Header.Get("Content-Type"))
		if ctErr == nil {
			switch {
			case strings.HasPrefix(ct, "multipart/"):
				score = 3 // multipart/related（HTML 正文 + 内嵌资源）保真度最高
			case ct == "text/html":
				score = 2
			}
		}
		if score >= bestScore {
			bestScore, bestHeader, bestBody = score, mail.Header(part.Header), data
		}
	}
	if bestScore < 0 {
		return nil
	}
	return w.parsePart(bytes.NewReader(bestBody), bestHeader, depth+1)
}

// emlDecodeTransfer 按 Content-Transfer-Encoding 包一层解码读取器：
// base64 / quoted-printable 显式解码，7bit/8bit/binary/未知保持原样。
func emlDecodeTransfer(r io.Reader, encoding string) io.Reader {
	switch strings.ToLower(strings.TrimSpace(encoding)) {
	case "base64":
		return base64.NewDecoder(base64.StdEncoding, r)
	case "quoted-printable":
		return quotedprintable.NewReader(r)
	default:
		return r
	}
}

// emlDecodeHeader 解码 RFC 2047 编码头（=?charset?B/Q?...?= 形态）；
// 无编码头或解码失败时返回原值。
func emlDecodeHeader(s string) string {
	if !strings.Contains(s, "=?") {
		return s
	}
	dec := &mime.WordDecoder{CharsetReader: emlCharsetReader}
	if out, err := dec.DecodeHeader(s); err == nil {
		return out
	}
	return s
}

// emlCharsetReader 使用 x/text HTML 字符集索引支持常见邮件头编码。
func emlCharsetReader(charset string, input io.Reader) (io.Reader, error) {
	charset = strings.TrimSpace(charset)
	if charset == "" || strings.EqualFold(charset, "utf-8") || strings.EqualFold(charset, "us-ascii") {
		return input, nil
	}
	encoding, err := htmlindex.Get(charset)
	if err != nil || encoding == nil {
		return nil, fmt.Errorf("docling: 不支持的邮件字符集 %s", charset)
	}
	return transform.NewReader(input, encoding.NewDecoder()), nil
}

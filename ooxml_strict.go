// ooxml_strict.go 实现 DOCX/PPTX/XLSX 共用的 Strict OOXML 命名空间
// 归一化。仅当根关系部件明确使用 ISO Strict URI 时才重写 XML；普通
// Transitional 包原样返回，媒体等二进制条目按压缩数据直接复制。
package docparse

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"regexp"
	"strings"
)

const (
	// strictOOXMLRootRelsMaxBytes 限制探测根关系部件时的解压读取量。
	strictOOXMLRootRelsMaxBytes = 64 << 10
	// strictOOXMLMaxMemberBytes 限制归一化包中单个条目的声明解压大小。
	strictOOXMLMaxMemberBytes = 512 << 20
	// strictOOXMLMaxTotalBytes 限制归一化包全部条目的声明解压大小总和。
	strictOOXMLMaxTotalBytes = 2 << 30
)

const (
	// strictOOXMLNamespacePrefix 是 ISO/IEC 29500 Strict URI 公共前缀。
	strictOOXMLNamespacePrefix = "http://purl.oclc.org/ooxml/"
	// transitionalOOXMLNamespaceHost 是 Transitional OOXML URI 主机前缀。
	transitionalOOXMLNamespaceHost = "http://schemas.openxmlformats.org/"
	// strictOOXMLRootRelationships 是 OPC 包入口关系文件路径。
	strictOOXMLRootRelationships = "_rels/.rels"
)

// strictOOXMLNamespacePattern 只匹配 URI 允许的稳定 ASCII 字符，避免
// 对 XML 正文中的任意文本做宽泛替换。
var strictOOXMLNamespacePattern = regexp.MustCompile(`http://purl\.oclc\.org/ooxml/[A-Za-z0-9_./-]+`)

// strictOOXMLNamespaceOverrides 保存无法用通用 2006 路径规则转换的 URI。
var strictOOXMLNamespaceOverrides = map[string]string{
	"http://purl.oclc.org/ooxml/descriptions/base":                               "http://descriptions.openxmlformats.org/description/base",
	"http://purl.oclc.org/ooxml/descriptions/full":                               "http://descriptions.openxmlformats.org/description/full",
	"http://purl.oclc.org/ooxml/officeDocument/relationships/customXml":          "http://schemas.openxmlformats.org/officeDocument/2006/customXml",
	"http://purl.oclc.org/ooxml/officeDocument/relationships/metadata/thumbnail": "http://schemas.openxmlformats.org/package/2006/relationships/metadata/thumbnail",
}

// normalizeStrictOOXMLPackage 检测并把 Strict OOXML 包转换为当前解析器及
// excelize 可消费的 Transitional URI。返回值在普通包上复用原始切片；
// Strict 包的 ZIP 路径或体积越界时返回错误，不尝试带风险的降级解析。
func normalizeStrictOOXMLPackage(data []byte) ([]byte, error) {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}
	strict, err := isStrictOOXMLPackage(reader)
	if err != nil || !strict {
		return data, err
	}

	var total uint64
	for _, file := range reader.File {
		if !isSafeOOXMLZipMember(file.Name) {
			return nil, fmt.Errorf("docparse: OOXML ZIP 路径越界: %s", file.Name)
		}
		if file.UncompressedSize64 > strictOOXMLMaxMemberBytes {
			return nil, fmt.Errorf("docparse: OOXML 部件过大: %s", file.Name)
		}
		total += file.UncompressedSize64
		if total > strictOOXMLMaxTotalBytes {
			return nil, fmt.Errorf("docparse: OOXML 解压总量过大")
		}
	}

	var output bytes.Buffer
	writer := zip.NewWriter(&output)
	for _, file := range reader.File {
		if !isOOXMLXMLPart(file.Name) {
			if err := writer.Copy(file); err != nil {
				_ = writer.Close()
				return nil, fmt.Errorf("docparse: 复制 OOXML 部件 %s: %w", file.Name, err)
			}
			continue
		}
		payload, err := readOOXMLZipFileLimited(file, strictOOXMLMaxMemberBytes)
		if err != nil {
			_ = writer.Close()
			return nil, fmt.Errorf("docparse: 读取 OOXML 部件 %s: %w", file.Name, err)
		}
		payload = strictOOXMLNamespacePattern.ReplaceAllFunc(payload, func(uri []byte) []byte {
			return []byte(strictOOXMLNamespaceToTransitional(string(uri)))
		})
		header := &zip.FileHeader{Name: file.Name, Method: zip.Deflate, Modified: file.Modified, Comment: file.Comment, NonUTF8: file.NonUTF8}
		header.SetMode(file.Mode())
		entry, err := writer.CreateHeader(header)
		if err != nil {
			_ = writer.Close()
			return nil, fmt.Errorf("docparse: 创建 OOXML 部件 %s: %w", file.Name, err)
		}
		if _, err := entry.Write(payload); err != nil {
			_ = writer.Close()
			return nil, fmt.Errorf("docparse: 写入 OOXML 部件 %s: %w", file.Name, err)
		}
	}
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("docparse: 完成 Strict OOXML 归一化: %w", err)
	}
	return output.Bytes(), nil
}

// isStrictOOXMLPackage 只读取根关系部件的有界前缀识别 Strict URI。
func isStrictOOXMLPackage(reader *zip.Reader) (bool, error) {
	for _, file := range reader.File {
		if file.Name != strictOOXMLRootRelationships {
			continue
		}
		payload, err := readOOXMLZipFileLimited(file, strictOOXMLRootRelsMaxBytes)
		if err != nil {
			return false, err
		}
		return bytes.Contains(payload, []byte(strictOOXMLNamespacePrefix)), nil
	}
	return false, nil
}

// readOOXMLZipFileLimited 有界读取 ZIP 条目，并拒绝声明或实际解压量超限。
func readOOXMLZipFileLimited(file *zip.File, limit uint64) ([]byte, error) {
	if file.UncompressedSize64 > limit {
		return nil, fmt.Errorf("声明大小超过 %d 字节", limit)
	}
	rc, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	payload, err := io.ReadAll(io.LimitReader(rc, int64(limit)+1))
	if err != nil {
		return nil, err
	}
	if uint64(len(payload)) > limit {
		return nil, fmt.Errorf("实际大小超过 %d 字节", limit)
	}
	return payload, nil
}

// isSafeOOXMLZipMember 判断条目路径始终位于包根目录内。
func isSafeOOXMLZipMember(name string) bool {
	normalized := strings.ReplaceAll(name, `\`, "/")
	if normalized == "" || strings.HasPrefix(normalized, "/") ||
		(len(normalized) > 1 && normalized[1] == ':') {
		return false
	}
	for _, part := range strings.Split(normalized, "/") {
		if part == ".." {
			return false
		}
	}
	return true
}

// isOOXMLXMLPart 判断需要命名空间改写的 XML 或关系部件。
func isOOXMLXMLPart(name string) bool {
	lower := strings.ToLower(name)
	return strings.HasSuffix(lower, ".xml") || strings.HasSuffix(lower, ".rels")
}

// strictOOXMLNamespaceToTransitional 把单个 Strict URI 映射到
// Transitional URI；常规路径在第一个段后插入 2006。
func strictOOXMLNamespaceToTransitional(uri string) string {
	if mapped, ok := strictOOXMLNamespaceOverrides[uri]; ok {
		return mapped
	}
	rest := strings.TrimPrefix(uri, strictOOXMLNamespacePrefix)
	rest = strings.ReplaceAll(rest, "extendedProperties", "extended-properties")
	rest = strings.ReplaceAll(rest, "customProperties", "custom-properties")
	segment, tail, found := strings.Cut(rest, "/")
	if !found {
		return transitionalOOXMLNamespaceHost + segment + "/2006"
	}
	return transitionalOOXMLNamespaceHost + segment + "/2006/" + tail
}

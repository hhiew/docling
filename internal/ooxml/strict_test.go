// strict_test.go 验证 Strict OOXML 命名空间到 Transitional 的映射规则。
package ooxml

import (
	"testing"
)

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

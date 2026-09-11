// unicode_test.go 验证 internal/pdfenc 的字符编码恢复单元：ToUnicode
// CMap 多字节进位、CID CMap 解析与标准 CJK 命名编码解码。
package pdfenc

import (
	"testing"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/encoding/korean"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/encoding/traditionalchinese"
)

// TestParsePDFToUnicodeCMapCarriesMultiByteRanges 验证 bfrange 的源编码与
// Unicode 目标均按大端整数完整进位，而不是只修改最后一个字节。
func TestParsePDFToUnicodeCMapCarriesMultiByteRanges(t *testing.T) {
	data := []byte(`/CIDInit /ProcSet findresource begin
12 dict begin begincmap
1 begincodespacerange
<0000> <FFFF>
endcodespacerange
1 beginbfrange
<00FF> <0100> <4E00>
endbfrange
endcmap end end`)
	cmap, ok := parsePDFToUnicodeCMap(data)
	if !ok {
		t.Fatal("parsePDFToUnicodeCMap 应接受合法多字节 CMap")
	}
	if got := cmap.Decode([]byte{0x00, 0xff, 0x01, 0x00}); got != "一丁" {
		t.Fatalf("decode=%q, want %q", got, "一丁")
	}
}

// TestParsePDFCIDCMap 验证 Encoding CMap 的逐字符和连续 CID 映射均能展开。
func TestParsePDFCIDCMap(t *testing.T) {
	data := []byte(`1 begincodespacerange
<00> <FF>
endcodespacerange
1 begincidchar
<20> 3
endcidchar
1 begincidrange
<41> <42> 5
endcidrange`)
	cmap, ok := ParseCIDCMap(data)
	if !ok || cmap.mappings[string([]byte{0x20})] != 3 ||
		cmap.mappings[string([]byte{0x41})] != 5 || cmap.mappings[string([]byte{0x42})] != 6 {
		t.Fatalf("CID CMap parse failed: %+v ok=%v", cmap, ok)
	}
	if len(cmap.ranges) != 1 || cmap.ranges[0].size != 1 {
		t.Fatalf("CID CMap codespace=%+v", cmap.ranges)
	}
}

// TestNamedPDFCMapDecoderCJKFamilies 验证标准简繁中、日、韩 CMap 名称均
// 路由到对应纯 Go 字符集，避免只修复单一中文样本。
func TestNamedPDFCMapDecoderCJKFamilies(t *testing.T) {
	tests := []struct {
		name     string
		encoding encoding.Encoding
		text     string
	}{
		{name: "GBK2K-H", encoding: simplifiedchinese.GB18030, text: "中文𠀀"},
		{name: "ETen-B5-H", encoding: traditionalchinese.Big5, text: "中文"},
		{name: "90ms-RKSJ-H", encoding: japanese.ShiftJIS, text: "日本"},
		{name: "EUC-H", encoding: japanese.EUCJP, text: "日本"},
		{name: "KSCms-UHC-H", encoding: korean.EUCKR, text: "한국"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			raw, err := test.encoding.NewEncoder().Bytes([]byte(test.text))
			if err != nil {
				t.Fatalf("encode fixture: %v", err)
			}
			decoder := NamedCMapDecoder(test.name)
			if decoder == nil {
				t.Fatalf("没有为 %s 创建解码器", test.name)
			}
			if got := decoder.Decode(raw); got != test.text {
				t.Fatalf("decoded=%q want=%q", got, test.text)
			}
		})
	}
}

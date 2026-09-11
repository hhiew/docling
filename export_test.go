package docparse

import (
	"strings"
	"testing"
)

// 构造含标题/段落/列表（嵌套+有序）/表格/代码/公式的示例文档（复用树构建 API）。
func buildExportSampleDoc() *DoclingDocument {
	doc := NewDoclingDocument("sample")
	h1 := doc.AddTitle("部署指南", nil, nil)
	h2 := doc.AddHeading(2, "环境要求", nil, &h1)
	doc.AddText(LabelText, "需要 Docker 24+。", nil, &h2)
	doc.AddCode("apt install docker", "bash", nil, &h2)
	g := doc.AddListGroup("", &h2)
	doc.AddListItem(g, "安装依赖", false, "", nil)
	doc.AddListItem(g, "启动服务", true, "1.", nil)
	h3 := doc.AddHeading(3, "端口说明", nil, &h2)
	cells := []DoclingTableCell{
		{StartRowOffsetIdx: 0, EndRowOffsetIdx: 1, StartColOffsetIdx: 0, EndColOffsetIdx: 1, Text: "端口", ColumnHeader: true},
		{StartRowOffsetIdx: 0, EndRowOffsetIdx: 1, StartColOffsetIdx: 1, EndColOffsetIdx: 2, Text: "用途", ColumnHeader: true},
		{StartRowOffsetIdx: 1, EndRowOffsetIdx: 2, StartColOffsetIdx: 0, EndColOffsetIdx: 1, Text: "8080"},
		{StartRowOffsetIdx: 1, EndRowOffsetIdx: 2, StartColOffsetIdx: 1, EndColOffsetIdx: 2, Text: "HTTP"},
	}
	doc.AddTable(cells, 2, 2, nil, &h3)
	doc.AddText(LabelText, "完成部署。", nil, &h3)
	doc.AddFormula("E=mc^2", nil, &h3)
	return doc
}

// ExportMarkdown：标题层级/段落/代码/列表/表格/公式全部还原。
func TestExportMarkdown(t *testing.T) {
	md := ExportMarkdown(buildExportSampleDoc())
	checks := []string{
		"# 部署指南",
		"## 环境要求",
		"需要 Docker 24+。",
		"```bash\napt install docker\n```",
		"- 安装依赖",
		"1. 启动服务",
		"### 端口说明",
		"| 端口 | 用途 |",
		"| 8080 | HTTP |",
		"$$E=mc^2$$",
		"完成部署。",
	}
	for _, want := range checks {
		if !strings.Contains(md, want) {
			t.Errorf("导出缺少 %q；实际：\n%s", want, md)
		}
	}
}

// 有序列表 Marker 缺省时按序生成；无序用 "-"。
func TestExportMarkdownListMarkers(t *testing.T) {
	doc := NewDoclingDocument("l")
	g := doc.AddListGroup("", nil)
	doc.AddListItem(g, "第一步", true, "", nil)
	doc.AddListItem(g, "第二步", true, "", nil)
	g2 := doc.AddListGroup("", nil)
	doc.AddListItem(g2, "选项甲", false, "", nil)
	md := ExportMarkdown(doc)
	if !strings.Contains(md, "1. 第一步") || !strings.Contains(md, "2. 第二步") {
		t.Errorf("有序列表序号生成错误:\n%s", md)
	}
	if !strings.Contains(md, "- 选项甲") {
		t.Errorf("无序列表符号错误:\n%s", md)
	}
}

// TextLevel 缺失时按树深度兜底（body 子节点为 1 级）。
func TestExportMarkdownLevelFallback(t *testing.T) {
	doc := NewDoclingDocument("d")
	h := doc.AddText(LabelSectionHeader, "无级别标题", nil, nil) // TextLevel=0
	doc.AddText(LabelText, "内容", nil, &h)
	md := ExportMarkdown(doc)
	if !strings.Contains(md, "# 无级别标题") { // body 子节点深度 1 → "#"
		t.Errorf("树深度兜底错误:\n%s", md)
	}
	_ = h
}

// 空文档导出安全。
func TestExportMarkdownEmpty(t *testing.T) {
	if got := ExportMarkdown(nil); got != "" {
		t.Errorf("nil doc 应导出空串，got %q", got)
	}
	if got := ExportMarkdown(NewDoclingDocument("empty")); strings.TrimSpace(got) != "" {
		t.Errorf("空文档应导出空内容，got %q", got)
	}
}

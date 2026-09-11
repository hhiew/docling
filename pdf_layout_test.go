// pdf_layout_test.go 验证 PDF XY-cut 阅读顺序的单栏保护、多栏排序与跨栏锚点语义。
package docling

import (
	"reflect"
	"testing"
)

// TestSortPDFPageLinesXYCutSingleColumnKeepsOrder 验证单栏页面不改变现有解析顺序。
func TestSortPDFPageLinesXYCutSingleColumnKeepsOrder(t *testing.T) {
	lines := []pdfLine{
		pdfLayoutTestLine("第一段", 50, 420, 760),
		pdfLayoutTestLine("第二段", 50, 390, 720),
		pdfLayoutTestLine("第三段", 75, 360, 680),
	}

	got := sortPDFPageLinesXYCut(lines)
	want := []string{"第一段", "第二段", "第三段"}
	if texts := pdfLayoutTestTexts(got); !reflect.DeepEqual(texts, want) {
		t.Fatalf("单栏顺序不应变化：got=%v want=%v", texts, want)
	}
}

// TestSortPDFPageLinesXYCutTwoColumns 验证双栏按左栏到底、再读右栏排序。
func TestSortPDFPageLinesXYCutTwoColumns(t *testing.T) {
	lines := []pdfLine{
		pdfLayoutTestLine("左一", 50, 250, 760),
		pdfLayoutTestLine("右一", 340, 540, 752),
		pdfLayoutTestLine("左二", 50, 245, 710),
		pdfLayoutTestLine("右二", 340, 535, 692),
		pdfLayoutTestLine("左三", 50, 230, 650),
		pdfLayoutTestLine("右三", 340, 525, 628),
	}

	got := sortPDFPageLinesXYCut(lines)
	want := []string{"左一", "左二", "左三", "右一", "右二", "右三"}
	if texts := pdfLayoutTestTexts(got); !reflect.DeepEqual(texts, want) {
		t.Fatalf("双栏顺序错误：got=%v want=%v", texts, want)
	}
}

// TestSortPDFPageLinesXYCutSpanningAnchors 验证跨栏标题和中部说明先锚定，再分别读取其上下栏区。
func TestSortPDFPageLinesXYCutSpanningAnchors(t *testing.T) {
	lines := []pdfLine{
		pdfLayoutTestLine("跨栏标题", 45, 545, 790),
		pdfLayoutTestLine("上左一", 50, 250, 750),
		pdfLayoutTestLine("上右一", 340, 540, 748),
		pdfLayoutTestLine("上左二", 50, 245, 715),
		pdfLayoutTestLine("上右二", 340, 535, 710),
		pdfLayoutTestLine("跨栏说明", 45, 545, 665),
		pdfLayoutTestLine("下左一", 50, 250, 620),
		pdfLayoutTestLine("下右一", 340, 540, 618),
		pdfLayoutTestLine("下左二", 50, 245, 575),
		pdfLayoutTestLine("下右二", 340, 535, 570),
	}

	got := sortPDFPageLinesXYCut(lines)
	want := []string{
		"跨栏标题",
		"上左一", "上左二", "上右一", "上右二",
		"跨栏说明",
		"下左一", "下左二", "下右一", "下右二",
	}
	if texts := pdfLayoutTestTexts(got); !reflect.DeepEqual(texts, want) {
		t.Fatalf("跨栏锚点顺序错误：got=%v want=%v", texts, want)
	}
}

// TestSortPDFPageLinesXYCutUnevenColumns 验证左右栏行距和基线不一致时仍按栏排序。
func TestSortPDFPageLinesXYCutUnevenColumns(t *testing.T) {
	lines := []pdfLine{
		pdfLayoutTestLine("左甲", 40, 238, 760),
		pdfLayoutTestLine("右甲", 335, 545, 739),
		pdfLayoutTestLine("左乙", 40, 225, 696),
		pdfLayoutTestLine("右乙", 335, 530, 674),
		pdfLayoutTestLine("左丙", 40, 242, 625),
		pdfLayoutTestLine("右丙", 335, 538, 588),
	}

	got := sortPDFPageLinesXYCut(lines)
	want := []string{"左甲", "左乙", "左丙", "右甲", "右乙", "右丙"}
	if texts := pdfLayoutTestTexts(got); !reflect.DeepEqual(texts, want) {
		t.Fatalf("非均匀双栏顺序错误：got=%v want=%v", texts, want)
	}
}

// TestSortPDFPageLinesXYCutThreeColumns 验证 XY-cut 递归切分三栏中文标准文档。
func TestSortPDFPageLinesXYCutThreeColumns(t *testing.T) {
	lines := []pdfLine{
		pdfLayoutTestLine("左一", 40, 180, 760), pdfLayoutTestLine("中一", 230, 370, 755), pdfLayoutTestLine("右一", 420, 560, 750),
		pdfLayoutTestLine("左二", 40, 180, 700), pdfLayoutTestLine("中二", 230, 370, 695), pdfLayoutTestLine("右二", 420, 560, 690),
		pdfLayoutTestLine("左三", 40, 180, 640), pdfLayoutTestLine("中三", 230, 370, 635), pdfLayoutTestLine("右三", 420, 560, 630),
	}
	got := sortPDFPageLinesXYCut(lines)
	want := []string{"左一", "左二", "左三", "中一", "中二", "中三", "右一", "右二", "右三"}
	if texts := pdfLayoutTestTexts(got); !reflect.DeepEqual(texts, want) {
		t.Fatalf("三栏递归顺序错误：got=%v want=%v", texts, want)
	}
}

// TestSortPDFPageLinesXYCutRejectsSequentialIndentation 验证普通段落的水平缩进不会被误切为多栏。
func TestSortPDFPageLinesXYCutRejectsSequentialIndentation(t *testing.T) {
	lines := []pdfLine{
		pdfLayoutTestLine("正文第一行", 50, 250, 760),
		pdfLayoutTestLine("正文第二行", 50, 245, 725),
		pdfLayoutTestLine("缩进说明一", 335, 520, 620),
		pdfLayoutTestLine("缩进说明二", 335, 515, 585),
	}

	got := sortPDFPageLinesXYCut(lines)
	want := []string{"正文第一行", "正文第二行", "缩进说明一", "缩进说明二"}
	if texts := pdfLayoutTestTexts(got); !reflect.DeepEqual(texts, want) {
		t.Fatalf("顺序段落不应按水平间隙切栏：got=%v want=%v", texts, want)
	}
}

// TestSortPDFLinesByPageXYCut 验证全文入口先按页号排序，并保持单页内的 XY-cut 结果。
func TestSortPDFLinesByPageXYCut(t *testing.T) {
	pageOneRight := pdfLayoutTestLine("第二页右", 340, 540, 710)
	pageOneRight.PageIdx = 1
	pageZeroBottom := pdfLayoutTestLine("第一页下", 50, 420, 680)
	pageZeroBottom.PageIdx = 0
	pageOneLeft := pdfLayoutTestLine("第二页左", 50, 250, 720)
	pageOneLeft.PageIdx = 1
	pageZeroTop := pdfLayoutTestLine("第一页上", 50, 420, 740)
	pageZeroTop.PageIdx = 0

	got := sortPDFLinesByPageXYCut([]pdfLine{pageOneRight, pageZeroBottom, pageOneLeft, pageZeroTop})
	want := []string{"第一页下", "第一页上", "第二页右", "第二页左"}
	if texts := pdfLayoutTestTexts(got); !reflect.DeepEqual(texts, want) {
		t.Fatalf("分页排序错误：got=%v want=%v", texts, want)
	}
}

// pdfLayoutTestLine 构造带有效矩形和常规字号的测试行。
func pdfLayoutTestLine(text string, minX, maxX, y float64) pdfLine {
	return pdfLine{
		Text:        text,
		MaxFontSize: 11,
		MinX:        minX,
		MaxX:        maxX,
		MinY:        y - 10,
		MaxY:        y,
	}
}

// pdfLayoutTestTexts 提取排序结果文本，简化断言。
func pdfLayoutTestTexts(lines []pdfLine) []string {
	texts := make([]string, 0, len(lines))
	for _, line := range lines {
		texts = append(texts, line.Text)
	}
	return texts
}

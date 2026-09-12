# examples — 样例与转换输出对照集

每个子目录对应一种受支持格式，包含两个文件：

| 文件 | 说明 |
|------|------|
| `sample.<ext>` | 最小典型样例源文件（可下载后直接投喂解析器） |
| `sample.expected.md` | `ParseByExtToMarkdown` 对该样例的完整期望输出 |

并排对照两个文件，即可直观了解该格式的转换效果。

## 样例类型

- `sample.<ext>`：最小典型样例，展示基础结构映射；
- `rich.docx` / `rich.pptx` / `rich-chart.xlsx`：复杂对象样例 —— SmartArt 流程图、
  艺术字、OLE 嵌入对象、OMML 公式（LaTeX 输出）、原生图表（数据表格 + SVG 语义预览）、
  母版继承与组合形状；
- `pdf/schmager-plateau10.pdf`、`xlsx/Book1.xlsx`：真实世界文档（学术论文、业务透视工作簿）。

## 回归测试

```bash
go test ./... -run TestExamplesGolden            # 严格逐字对比（CI/日常回归）
go test ./... -run TestExamplesGolden -update    # 解析行为变化后重建样例与期望输出
```

- 样例源文件由 [`examples_test.go`](../examples_test.go) 中的构造器生成（二进制格式
  pdf/docx/xlsx/pptx 由代码构造，保证可再生产），`-update` 会同时重建样例与期望输出。
- 常规模式下测试读取磁盘上的 `sample.<ext>` 解析后与 `sample.expected.md` 逐字对比；
  解析器输出发生任何变化都会在此暴露。

## 新增格式样例

1. 在 `examples_test.go` 的 `examplesSources()` 中追加一行（目录名、源文件名、构造器），
   代码构造的合成样例同时登记到 `syntheticExamplesFor`（真实样本不需要）；
2. 实现对应的构造函数；
3. 运行 `-update` 生成对照文件，检查 `sample.expected.md` 内容符合预期后一并提交。

图片格式（PNG/JPEG/BMP/WEBP）未收录：其 Markdown 输出依赖外部 OCR/视觉钩子，
无法给出确定性期望输出，见主 [README](../README.md) 的钩子章节。

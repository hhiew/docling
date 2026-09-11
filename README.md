# docparse — 纯 Go 多格式文档统一解析库

[English](README_EN.md) | 简体中文

`docparse` 把 PDF、Word、PPT、Excel/CSV、HTML、Markdown、AsciiDoc、EML 邮件、图片与纯文本等格式文档，解析为统一的 **DoclingDocument** 结构化模型（对齐 Docling Core `1.10.0` 官方协议），供知识库、RAG 切片、向量化与 AI 工具链消费。

设计目标：**纯 Go 生产基线，不依赖外部服务**；需要更高识别质量时，可按需挂载大模型 OCR 与结构化视觉钩子，按页增强。

## 特性

- **统一协议**：全部格式输出 Docling 兼容的 `DoclingDocument`（schema 1.10.0），并支持官方 Pydantic 模型自动化验收
- **纯 Go 解析**：默认引擎零外部服务依赖；PDF 字体/CMap 恢复、多栏阅读顺序、表格结构还原、Office 批注/公式/图表均为纯 Go 实现
- **大模型增强**：`OCRHook`（逐字文本）与 `PDFVisualHook`（带 bbox 的结构化 JSON）按页路由，内置质量信号、重试与强校验
- **多路导出**：Docling JSON / content_list / Markdown / HTML
- **知识库就绪**：层级分块（Hierarchical/Hybrid）与检索分块（content_list）内建

## 支持的格式

| 格式 | 入口函数 | 说明 |
|------|----------|------|
| 按扩展名自动分发 | `ParseByExt` / `ParseByExtWithOptions` | 不知道格式时的首选入口；自动补齐文档名、MIME、SHA-256 低 64 位哈希与 origin |
| PDF | `ParsePDF` / `ParsePDFWithOptions` | CropBox/UserUnit/Rotate 坐标归一化、词/行 bbox、ToUnicode/标准 CJK/自定义 Encoding CMap 字体恢复、内嵌位图（含 JPEG2000/JBIG2/软蒙版）、矩形/稀疏无边框表格、跨页续表、递归 XY-cut；可选 Poppler、OCR 与结构化视觉 Hook |
| Word | `ParseDocx` | Transitional/Strict OOXML、页眉页脚、现代批注回复链、脚注/尾注、修订与域、文本框、OMML 公式、图片、富表格、图表与复杂 Office 对象 |
| PPT | `ParsePPTX` | Transitional/Strict OOXML、notes、现代批注回复链、母版回退、组合元素、视觉顺序、图表与复杂 Office 对象 |
| Excel / CSV | `ParseXLSX` / `ParseCSV` | Transitional/Strict OOXML、隐藏表、threaded comments、原始公式、数据透视表、图片/形状/SmartArt/OLE、图表工作表、坐标顺序与结构化表格 |
| HTML | `ParseHTML` | title/首标题前 furniture、平铺标题、图片占位、行内格式/链接、富表格 |
| Markdown | `ParseMarkdown` / `ParseMarkdownTable` | 官方平铺标题、图片占位、格式/链接、原始 HTML 委托与 GFM 表格 |
| AsciiDoc | `ParseAsciiDoc` | 标题树、列表、字面/源码块、PictureItem 图片占位 |
| EML 邮件 | `ParseEML` | RFC 5322 头、正文择优、常见字符集、嵌套邮件与附件名称 |
| 图片 | `ParseImage` / `ParseImageWithOptions` | PNG/JPEG/BMP/WEBP 单页 PictureItem；无 OCR 也返回尺寸/DPI，配置后追加识别结构 |
| 纯文本 | `ParseText` | UTF-8 BOM 与控制字符清理兜底 |
| Docling JSON | `ParseDoclingDocument` | 解析 Docling 服务输出的 JSON（docling 引擎） |

## 快速上手

```bash
go get github.com/unitedrhino/docling
```

### 1. 按扩展名自动分发（推荐入口）

```go
import docparse "github.com/unitedrhino/docling"

doc, err := docparse.ParseByExt("报告.docx", data)
if err != nil { ... }
// DoclingDocument 没有 Items 字段；知识库使用 ToContentList 派生扁平内容。
items := docparse.ToContentList(doc, docparse.SourceGolight)
for _, it := range items {
    text := docparse.ItemToText(it) // 按类型提取文本（标题路径/表格/公式等）
}
```

需要记录来源 URI 或统一传入 PDF 兼容选项时，使用带选项入口：

```go
doc, err := docparse.ParseByExtWithOptions("报告.pdf", data, docparse.ParseOptions{
    OriginURI:       "s3://documents/报告.pdf",
    GarbageThreshold: 0.4,
    OCRHook: func(req docparse.OCRRequest) (string, error) {
        // req 含页号、MIME、文件名、原始数据和已有文本。
        return myOCR(req)
    },
})
```

### 2. 已知格式时单独解析

各格式提供独立入口，可直接调用：

```go
doc, err := docparse.ParseDocx(data)            // Word
doc, err := docparse.ParseMarkdown(mdData)      // Markdown
rows := docparse.ParseMarkdownTable(tableMd)    // 单独解析 Markdown 表格
```

### 3. 解析 + 导出 Markdown 一步到位

```go
md, err := docparse.ParseByExtToMarkdown("报告.pdf", data)
```

### 4. PDF OCR 与结构化视觉钩子

```go
doc, err := docparse.ParsePDFWithOptions(data, docparse.PDFOptions{
    // GarbageThreshold：页乱码率超过该值触发 OCR；0 = 仅扫描兜底页触发
    GarbageThreshold: 0.4,
    OCRHook: func(req docparse.OCRRequest) (string, error) {
        return myOCR(req)
    },
})
```

`OCRHook` 优先于兼容的 `PageOCRHook`。组件会清理代码围栏和模型说明，拒绝空结果、
拒答与明显乱码，首次无效时最多重试一次；已有文本页只采用乱码率更低的有效结果。
图片输入复用同一契约；未配置 OCR 时仍返回带尺寸、DPI 和 data URI 的 PictureItem。

复杂页可配置结构化视觉 Hook。默认仅在扫描、乱码、未恢复表格、公式密集或栏位歧义
时调用；`VisualAlways` 可显式覆盖图像型表格等无文本信号页面。模型返回的标签、bbox、
置信度下限、文本质量、标题层级、对象去重、表格拓扑和单元格 bbox 经强校验，无效时
重试一次；有效对象按 bbox 与规则正文去重合并，失败则保留纯 Go/Poppler/OCR 结果：

```go
doc, err := docparse.ParsePDFWithOptions(data, docparse.PDFOptions{
    OCRHook: myOCR,
    VisualHook: func(req docparse.PDFVisualRequest) (docparse.PDFVisualResult, error) {
        // req.Prompt 是内置严格 JSON 提示词；调用多模态模型后解码为返回结构。
        return myStructuredVisual(req)
    },
    MaxVisualPages: 20,
})
```

### 5. 内容列表与导出

```go
items := docparse.ToContentList(doc, docparse.SourceGolight) // 供知识库切片的内容列表
md := doc.ToMarkdown()                                       // 导出为 Markdown（等价 ExportMarkdown(doc)）
html := doc.ToHTML()                                         // 导出为完整 HTML 文档（等价 ExportHTML(doc)）

// 默认只导出 body；需要审阅页眉、备注等内容时显式选择内容层。
mdWithFurniture := doc.ToMarkdownWithOptions(docparse.ExportOptions{
    Layers: []docparse.ContentLayer{docparse.LayerBody, docparse.LayerFurniture},
})
```

图表遵循 Docling 1.10 协议，存放在 `pictures` 中并使用 `label=picture`；分类与
结构化数据分别位于 `meta.classification`、`meta.tabular_chart.chart_data`。
数据优先使用图表自带缓存；缓存缺失时，XLSX 从当前工作簿、DOCX/PPTX 从图表关系
指向的嵌入工作簿解析单元格公式。常见柱状、折线、面积、饼图、圆环、散点、雷达与
组合图会生成 960×540、96 DPI 的纯 Go SVG `ImageRef`；Markdown/HTML 导出器同时把
chart data 追加渲染为可检索表格。公式可解析普通 A1 区域、最终指向单一 A1 区域的
工作簿级/工作表级命名范围，以及 `TableName[Column]` 普通表格列结构化引用。
SVG 是语义预览，不承诺复刻 Office 的字体、主题、三维效果和动画；动态命名公式、
多区域/特殊项结构化引用及 Office 2016 扩展图表暂由缓存数据兜底。单个公式最多读取
十万个单元格，超限时安全降级。

SmartArt、艺术字、带文本形状和 OLE 同样遵循 `label=picture`：可见文字通过 caption
参与检索，关系目标等来源写入 `docparse__office_object_*` 扩展字段。OLE 载荷只记录，
不读取或执行。

### 6. 层级分块与知识库分块

```go
// 对齐官方 HierarchicalChunker：标题只进入上下文，列表组和表格保持结构，
// 不强制按长度切分；每个 chunk 携带官方 doc_items，以及便捷的
// doc refs、provenance 与 origin。
semanticChunks := docparse.HierarchicalChunks(doc)

// token 感知的超限切分 + 同标题 peer 合并。CountTokens 应与
// embedding 模型一致，可直接注入已有纯 Go tokenizer。
hybridChunks := docparse.HybridChunks(doc, docparse.HybridChunkOptions{
    MaxTokens:   512,
    CountTokens: embeddingTokenizer.Count,
})

// 知识库兼容策略：默认 900 rune、表格每段 100 行，并过滤页眉页脚/目录。
contentChunks := docparse.ChunkContentList(items, docparse.ContentChunkOptions{})
```

### 7. 文档来源与元数据

统一入口会填充官方 `origin`：`filename`、`mimetype`、`binary_hash`，以及可选
`uri`。`DocMeta` 按格式尽力填充：

```go
doc.Meta.Title     // 标题（dc:title / 邮件 Subject）
doc.Meta.Author    // 作者（dc:creator / 邮件 From 显示名）
doc.Meta.PageCount // 页数（PDF 页数 / pptx slide 数 / xlsx 全部 sheet 数）
```

## examples：样例与转换输出对照

[`examples/`](examples/) 目录为每种受支持格式提供一对文件：

- `sample.<ext>`：最小典型样例源文件
- `sample.expected.md`：`ParseByExtToMarkdown` 的完整期望输出

直接对照两个文件即可了解每种格式的转换效果；回归测试保证两者与解析器行为一致：

```bash
go test ./... -run TestExamplesGolden            # 严格逐字对比回归
go test ./... -run TestExamplesGolden -update    # 解析行为变化后一键重建对照集
```

用任意 HTTP/本地程序读取 `examples/` 下的样例文件并调用 `ParseByExtToMarkdown`，即可复现 `sample.expected.md` 的内容。

## Docling JSON 协议

- 输出固定为 `schema_name=DoclingDocument`、`version=1.10.0`。
- 标题层级使用官方 `level`；`text_level` 只兼容读取。
- 顶层始终包含 `body`、`furniture`、`groups`、`texts`、`pictures`、`tables`、
  `key_value_items`、`form_items` 与 `pages`；当前不提取的官方集合仍可无损往返。
- 内容层支持 `body`、`furniture`、`background`、`invisible`、`notes`，其中
  `furniture.content_layer` 固定为 `furniture`。
- 旧 `meta`、`caption`、`latex`、`annotations` 可读取；重新序列化时会归一化为
  官方字段、引用或节点级 `meta`，不再输出旧字段。

## 与 Docling 官方的关系与已知边界

协议层与 Docling Core `1.10.0` 完全对齐，可无损往返官方 JSON；解析质量层面：

- **纯 Go 强项**：文本型 PDF 的字体乱码恢复、结构化表格、多栏阅读顺序、Office 结构化对象（批注/公式/图表/修订），这些不依赖任何模型即可获得结构化输出
- **模型增强项**：扫描件 OCR、图片表格、复杂合并单元格、公式密集页、栏位歧义页 —— 配置 `OCRHook`/`VisualHook` 后按页自动增强
- **已知保守回退**（不伪造结果）：CCITT K>0（Group 3 二维）压缩、真实 ICC 色彩管理、软蒙版 Matte 语义、像素级版面分割与矢量图形语义 —— 相关内容保留原始信号并进入可选视觉链路
- 未配置任何模型时，纯 Go 链路依然输出完整的结构化文档；扫描件的文字识别需要配置 OCR

## 目录结构

```
docling/
├── docparse.go     # 统一模型与门面：Item、ParseByExt、ParseByExtToMarkdown、Item 工具
├── docling.go      # DoclingDocument 模型定义（TextItem/TableItem/PageItem/DocMeta 等）
├── doclingserve.go # Docling 服务解析（可选第二引擎）
├── contentlist.go  # ToContentList（知识库内容列表）
├── chunker.go      # HierarchicalChunks（官方层级语义分块）
├── hybrid_chunker.go # HybridChunks（token 感知切分、表头重复与 peer 合并）
├── content_chunk.go # ChunkContentList（知识库长度/表格/多模态策略）
├── export.go       # ExportMarkdown / ToMarkdown
├── export_html.go  # ExportHTML / ToHTML
├── media.go        # OOXML 共用：media 图片 data URI 封装、docProps/core.xml 元数据
├── ooxml_chart*.go # OOXML 图表：分类、公式回填、数据表格化与纯 Go SVG 预览
├── ooxml_strict.go # Strict OOXML 到 Transitional 命名空间的内存归一化
├── office_comment.go # OOXML 批注/回复链
├── office_object*.go # SmartArt/艺术字/形状/OLE 的语义映射与 SVG 预览
├── ocr.go          # OCRRequest/OCRHook、兼容 PageOCRHook 与质量回退
├── pdf*.go         # PDF 解析：文本/布局/表格/Unicode 恢复/图片解码/视觉路由/Poppler 可选
├── docx*.go pptx.go sheet.go # Word/PPT/Excel 解析
├── html.go markdown.go asciidoc.go eml.go image.go text.go table.go # 其余格式
├── examples/       # 各格式样例源文件与 Markdown 转换期望输出对照集
└── *_test.go       # 单元/基准/双引擎对比/官方 schema 验收测试
```

组件采用**单包 + 收窄的对外接口**：只有格式解析入口、统一模型与内容工具是对外 API；
文本清洗、表格转义/渲染、PDF 行分类等实现细节均为非导出符号。

## 依赖与致谢

- [pdfcpu](https://github.com/pdfcpu/pdfcpu)（Apache-2.0）— PDF 像素解码基础
- [excelize](https://github.com/qax-os/excelize)（BSD-3-Clause）— XLSX 读写与图表公式回填
- [ledongthuc/pdf](https://github.com/ledongthuc/pdf)（MIT）— PDF 文本坐标基础
- [go-jpeg2000](https://github.com/mrjoshuak/go-jpeg2000)（Apache-2.0）— 纯 Go JPEG 2000 解码
- [gobig2](https://github.com/dkrisman/gobig2)（Apache-2.0）— 纯 Go JBIG2 解码
- [goldmark](https://github.com/yuin/goldmark)（MIT）、[golang.org/x/net](https://pkg.go.dev/golang.org/x/net)、[golang.org/x/text](https://pkg.go.dev/golang.org/x/text)、[golang.org/x/image](https://pkg.go.dev/golang.org/x/image)
- [Docling](https://github.com/DS4SD/docling)（MIT）— 输出协议与分块语义的参考实现

## License

[MIT](LICENSE)

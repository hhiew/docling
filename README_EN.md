# docling — Pure-Go Document Parsing, Ready for Gen-AI

English | [简体中文](README.md)

> Feed any document to your LLM — a single pure-Go binary, 1–2 orders of magnitude faster, with optional LLM enhancement for the hard pages.

`docling` converts PDF, Word, PPT, Excel/CSV, HTML, Markdown, AsciiDoc, EML email, images and plain text into a unified **DoclingDocument** structured model (aligned with the Docling Core `1.10.0` official protocol) — a drop-in document ingestion layer for RAG chunking, embedding pipelines and agent toolchains.

> Naming note: this repository is a pure-Go implementation of the [Docling](https://github.com/docling-project/docling) protocol. The Go package is named `docling` and shares the same document model as the Python reference implementation.

## Why docling (Go)

The mainstream approach to modern document parsing is a Python model pipeline (e.g. Docling): high quality, but it needs a Python service, loads layout models, and can take tens of seconds per document. **docling (Go) makes a different engineering trade-off**:

- ⚡ **1–2 orders of magnitude faster**: the pure-Go rule engine handles text-based documents in milliseconds to seconds (benchmarks below); ingesting millions of documents no longer requires a GPU fleet
- 📦 **Single-binary deployment**: pure Go, zero Python, zero external services — `go get` and go; runs equally well in CI, on edge nodes and in embedded settings
- 🧠 **Modern hybrid architecture**: the rule engine covers structured content, while scanned pages, image tables and formula-dense pages are automatically routed to your LLM page by page (`OCRHook`/`PDFVisualHook`) — spend model tokens where they matter, not on every page
- 🔒 **Never fabricates**: content the parser is unsure about keeps its raw signal and falls back to vision, instead of guessing a structure
- 🤝 **Ecosystem compatible**: output is fully aligned with the official Docling protocol, so existing Docling downstream tooling plugs right in

## Architecture

<p align="center">
  <img src="assets/architecture.svg" alt="docling architecture" width="100%"/>
</p>

> The parser computes per-page quality signals (garbage ratio, table candidates, formula density,
> column ambiguity) and routes only the hard pages to your LLM. Model results are strictly
> validated and merged with rule text by geometry; failures keep the pure-Go output —
> **the model is spent only where it matters and never breaks existing results**.

## Benchmarks

Same machine, same real-world PDFs (Apache-2.0 test samples). `docling` (Go, no models) vs Docling `2.x` (CPU model pipeline, timed in-process after warm-up):

| Document | docling (Go) | Python Docling (CPU) | Speed-up |
|----------|----------|---------------|----------|
| Research paper (141 KB, 10 pages) | **0.31s** | 10.5s | **34×** |
| Technical book (1.2 MB, mixed text & images) | **0.73s** | 111.7s | **153×** |

```mermaid
xychart-beta
    title "PDF conversion time (seconds, lower is better)"
    x-axis ["Paper 141KB", "Book 1.2MB"]
    y-axis "Time (seconds)" 0 --> 120
    bar [10.5, 111.7]
    bar [0.31, 0.73]
```

> Bar series: Python Docling (CPU model pipeline) then docling (Go rule engine). The Python time includes layout/table model inference; the Go version loads no models.
>
> **An honest note on quality**: Docling's model pipeline is still the ceiling for complex layouts and image understanding. docling (Go)'s strategy is a rule-engine baseline plus LLM enhancement on the pages that need it — the two are complementary, not mutually exclusive.

## Parsing capability comparison

| Capability | docling (Go) | Python Docling | docling (Go) + LLM hooks |
|------|--------------------|----------------|----------------------|
| Text-based PDF words with coordinates | ✅ pure Go | ✅ | ✅ |
| Garbled-font recovery (CJK CMap / embedded fonts) | ✅ pure Go | ✅ | ✅ |
| Table recovery (ruled / borderless / cross-page merge) | ✅ pure Go | ✅ model | ✅ (image tables via vision) |
| Multi-column reading order (recursive XY-cut) | ✅ pure Go | ✅ model | ✅ |
| OCR for scanned pages | ➖ opt-in | ✅ built-in | ✅ OCRHook with any model |
| Image tables / formula-dense / ambiguous layouts | ➖ vision fallback | ✅ model | ✅ PDFVisualHook per page |
| Embedded image decoding (JPEG2000/JBIG2/soft masks) | ✅ pure Go | ✅ | ✅ |
| DOCX/PPTX/XLSX comments·formulas·revisions·charts·SmartArt | ✅ pure Go | ⚠️ partial | ✅ |
| Output protocol | ✅ Docling 1.10 official JSON | ✅ official | ✅ |
| Deployment | single binary | Python service + models | single binary + model API |

## Supported input formats

| Input format | docling (Go) | Python Docling |
|--------------|--------------|-----------------|
| PDF | ✅ | ✅ |
| Word (DOCX) | ✅ | ✅ |
| PPT (PPTX) | ✅ | ✅ |
| Excel (XLSX) / CSV | ✅ | ✅ |
| HTML | ✅ | ✅ |
| Markdown | ✅ | ✅ |
| AsciiDoc | ✅ | ❌ |
| EML email | ✅ | ✅ (also MSG) |
| Images PNG/JPEG/BMP/WEBP | ✅ | ✅ (also TIFF) |
| Plain text | ✅ | ✅ |
| Docling JSON (read back) | ✅ | ✅ |
| Audio transcription (WAV/MP3 ASR) | ❌ planned | ✅ |
| EPUB / Apple Pages | ❌ planned | ✅ |
| XML (XBRL/JATS/USPTO) / usda | ❌ planned | ✅ |
| LaTeX | ❌ planned | ✅ |
| ZIP archives | ❌ | ✅ |

## Supported output formats

| Output format | docling (Go) | Python Docling |
|---------------|--------------|-----------------|
| Markdown | ✅ | ✅ |
| HTML | ✅ | ✅ |
| DoclingDocument JSON (lossless) | ✅ | ✅ |
| content_list (flat retrieval list) | ✅ | ⚠️ similar |
| Hierarchical / Hybrid chunking | ✅ | ✅ |
| Knowledge-base chunking (length/table/multimodal strategy) | ✅ | ❌ |
| DocTags | ❌ planned | ✅ |
| Plain text | ✅ | ✅ |
| WebVTT (audio captions) | ❌ | ✅ |

## Supported Formats

| Format | Entry functions | Notes |
|--------|-----------------|-------|
| Auto-detect by extension | `ParseByExt` / `ParseByExtWithOptions` | Preferred entry when the format is unknown; fills in name, MIME, SHA-256 hash and origin |
| PDF | `ParsePDF` / `ParsePDFWithOptions` | Coordinate normalization, word/line bboxes, font recovery, embedded bitmaps, tables, cross-page merging, recursive XY-cut; optional Poppler, OCR and vision hooks |
| Word | `ParseDocx` | Transitional/Strict OOXML, comment threads, footnotes/endnotes, tracked changes & fields, OMML formulas, charts and SmartArt/OLE |
| PPT | `ParsePPTX` | Notes, comment threads, layout/master fallback, grouped shapes, charts and complex Office objects |
| Excel / CSV | `ParseXLSX` / `ParseCSV` | Hidden sheets, threaded comments, raw formulas, pivot tables, chart sheets |
| HTML | `ParseHTML` | Furniture, flattened headings, image placeholders, rich tables |
| Markdown | `ParseMarkdown` | Flattened headings, image placeholders, GFM tables |
| AsciiDoc | `ParseAsciiDoc` | Heading tree, lists, literal/source blocks |
| EML | `ParseEML` | RFC 5322 headers, best-body selection, common charsets, attachment names |
| Images | `ParseImage` | PNG/JPEG/BMP/WEBP; adds recognition results when OCR is configured |
| Plain text | `ParseText` | BOM and control-character cleanup |
| Docling JSON | `ParseDoclingDocument` | Parses JSON produced by a Docling service |

## Quick Start

```bash
go get github.com/unitedrhino/docling
```

```go
// Parse → Markdown in one line
md, err := docling.ParseByExtToMarkdown("report.pdf", data)

// Or step by step: get the structured model first
doc, err := docling.ParseByExt("report.docx", data)
items := docling.ToContentList(doc, docling.SourceGolight) // flat list for RAG
chunks := docling.HierarchicalChunks(doc)                   // official hierarchical semantics
md := doc.ToMarkdown()
html := doc.ToHTML()
```

### Attach an LLM: modern hybrid parsing

```go
doc, err := docling.ParsePDFWithOptions(data, docling.PDFOptions{
    GarbageThreshold: 0.4, // pages above this garbage ratio trigger OCR
    OCRHook: func(req docling.OCRRequest) (string, error) {
        return myLLMOCR(req) // plug in any OCR / multimodal model
    },
    VisualHook: func(req docling.PDFVisualRequest) (docling.PDFVisualResult, error) {
        // req.Prompt is the built-in strict JSON prompt; decode the model output into the struct.
        return myStructuredVision(req)
    },
    MaxVisualPages: 20,
})
```

Hook results are strictly validated (labels, confidence, bboxes, table topology, anti-refusal and
anti-repetition) with one automatic retry; valid objects are merged with rule text by geometry and
failures keep the pure-Go result — **model enhancement never breaks existing output**.

## examples: results showcase (source ↔ recognized Markdown)

[`examples/`](examples/) ships a "source file ↔ expected Markdown" pair per supported format,
all built from code and reproducible. Below are the actual results on complex objects:

### Word · rich.docx

Source (OMML formula, SmartArt process, WordArt and an OLE embedded object):

<p align="center">
  <img src="assets/examples/rich-docx.png" alt="rich.docx source" width="420"/>
</p>

Recognized Markdown (excerpt):

````markdown
能量换算关系:{E}^{2}=mc

![提交申请
技术评审
发布上线](data:image/svg+xml;base64,...)   ← SmartArt flow → semantic SVG, node text becomes searchable captions

![年度规划](data:image/svg+xml;base64,...)  ← WordArt

![Excel.Sheet.12](data:image/svg+xml;base64,...)  ← OLE embedded object (recorded, never executed)
````

### PPT · rich.pptx

Source (title, nested lists, column-span table and a native chart):

<p align="center">
  <img src="assets/examples/rich-pptx.png" alt="rich.pptx source" width="420"/>
</p>

Recognized Markdown (excerpt): title, lists and table restored item by item; the native chart
becomes both an SVG semantic preview and a searchable data table:

````markdown
| 表头A |  |
| --- | --- |
| 跨列内容 | 跨列内容 |

![](data:image/svg+xml;base64,...)   ← chart SVG preview

| 类别 | 销量 |
| --- | --- |
| 1月 | 120 |
| 2月 | 186 |
````

### Excel · rich-chart.xlsx

A workbook with a native line chart: the data sheet and the chart are restored as a searchable
table and an SVG preview respectively, with the chart's cell references fully preserved.

<p align="center">
  <img src="assets/examples/rich-chart.png" alt="rich-chart.xlsx source" width="420"/>
</p>

### Office · Microsoft official test documents (complex objects)

Samples taken from Microsoft's Open XML SDK official test assets (MIT licensed) — documents
produced by real Office 2007+ applications:

<p align="center">
  <img src="assets/examples/real-word-chart.png" alt="Word native charts" width="300"/>
  <img src="assets/examples/real-ppt-3dpie.png" alt="PPT 3D pie" width="300"/>
  <img src="assets/examples/real-xlsx-ole.png" alt="Excel OLE objects" width="300"/>
</p>

Recognized Markdown (excerpt): chart data from the **Word multi-series column charts** and the
**PPT 3D pie** is fully restored as searchable tables plus SVG previews; OLE embedded objects in
the Excel workbook are recorded by semantic classification (never executed):

````markdown
| 类别 | Series 1 | Series 2 | Series 3 |
| --- | --- | --- | --- |
| Category 1 | 4.3 | 2.4 | 2 |
| Category 2 | 2.5 | 4.4 |  |
| Category 3 | 3.5 | 1.8 | 3 |
| Category 4 | 4.5 | 2.8 | 5 |
````

### Real-world samples per format

| Format | Real sample | Source |
|--------|-------------|--------|
| PDF | 196-page master's thesis + 10-page two-column paper | Victoria University of Wellington (public thesis) |
| Word | official test document with 6 native charts | Microsoft Open XML SDK test assets (MIT) |
| PPT | official test deck with a 3D pie chart | Microsoft Open XML SDK test assets (MIT) |
| Excel | workbook with OLE embedded objects | Microsoft Open XML SDK test assets (MIT) |
| EML | real MIME multipart bounce mail | CPython stdlib test data (PSF licensed) |
| Plain text | The Adventures of Tom Sawyer, full book (380KB) | public domain (pdfcpu testdata) |
| CSV | x86 instruction set table (3,700+ rows) | golang.org/x/arch |
| Markdown | goldmark project README | yuin/goldmark (MIT) |
| AsciiDoc | lzip-go project CHANGELOG | sorairolake/lzip-go (CC-BY-4.0) |
| HTML | Go net/http package doc page (godoc) | Go official docs snapshot |

### PDF · real academic papers

A 196-page master's thesis, "Evaluating the GO Programming Language with Design Patterns"
(Victoria University of Wellington, 2010):

<p align="center">
  <img src="assets/examples/thesis.png" alt="thesis first page" width="380"/>
  <img src="assets/examples/gohotdraw.png" alt="GoHotDraw two-column paper" width="380"/>
</p>

Recognized Markdown (excerpt; full corpus at
[examples/pdf/design-patterns-thesis.expected.md](examples/pdf/design-patterns-thesis.expected.md)):

````markdown
## Evaluating the GO

## Programming Language with

## Design Patterns

by

### Frank Schmager

A thesis
submitted to the Victoria University ofWellington
...

### Abstract

GO is a newobject-oriented programming language developed at Google
by Rob Pike, Ken Thompson, and others. ...
````

The second one is a 10-page **two-column conference paper** by the same author
([gohotdraw-paper.pdf](examples/pdf/gohotdraw-paper.pdf), the GoHotDraw drawing framework),
demonstrating multi-column reading-order recovery. The two papers total 200+ pages and parse
in ~30s in pure Go — the golden regression doubles as a real-world stress test.

### Run it yourself (no external services needed)

```bash
git clone https://github.com/unitedrhino/docling
cd docling
go test ./... -run TestExamplesGolden -v   # verbatim regression over every sample, rich ones included
```

Or parse the complex samples directly in your own code:

```go
// go get github.com/unitedrhino/docling
data, _ := os.ReadFile("examples/docx/rich.docx")
md, err := docling.ParseByExtToMarkdown("rich.docx", data)
fmt.Println(md) // formula LaTeX, SmartArt flow SVG, WordArt & OLE classification at a glance
```


## Relationship to Docling

The protocol layer is fully aligned with Docling Core `1.10.0` and round-trips official JSON
losslessly; chunking mirrors the official HierarchicalChunker/HybridChunker. **Known conservative
boundaries** (results are never fabricated): CCITT K>0 compression, real ICC color management,
soft-mask Matte, pixel-level layout segmentation and vector graphics semantics keep their raw
signal and enter the optional vision path; text recognition for scanned pages requires OCR.

## Repository layout

```
docling/
├── docling.go           # facade: Item, ParseByExt, ParseByExtToMarkdown
├── docling_document.go  # DoclingDocument model structures (official protocol)
├── doclingserve.go      # Docling service parsing (optional second engine)
├── export*.go           # Markdown / HTML exporters
├── contentlist.go       # ToContentList (RAG content list)
├── chunker.go hybrid_chunker.go content_chunk.go # three chunking strategies
├── pdf*.go              # PDF: text/layout/tables/vision routing/image orchestration
├── docx*.go pptx.go sheet.go # Word / PPT / Excel parsing
├── html.go markdown.go asciidoc.go eml.go image.go text.go table.go
├── internal/pdfenc/     # PDF byte-encoding layer: ToUnicode/CMap/SFNT recovery, JPX/JBIG2 decoding, soft-mask alpha
├── internal/ooxml/      # Strict OOXML → Transitional normalization
├── examples/            # per-format sample sources paired with expected Markdown
└── *_test.go            # unit/end-to-end/official schema validation tests
```

## Dependencies & acknowledgements

- [pdfcpu](https://github.com/pdfcpu/pdfcpu) (Apache-2.0) — PDF pixel decoding foundation
- [excelize](https://github.com/qax-os/excelize) (BSD-3-Clause) — XLSX reading and chart formula backfill
- [ledongthuc/pdf](https://github.com/ledongthuc/pdf) (MIT) — PDF text coordinates foundation
- [go-jpeg2000](https://github.com/mrjoshuak/go-jpeg2000) (Apache-2.0) — pure Go JPEG 2000 decoding
- [gobig2](https://github.com/dkrisman/gobig2) (Apache-2.0) — pure Go JBIG2 decoding
- [goldmark](https://github.com/yuin/goldmark) (MIT), [golang.org/x/net](https://pkg.go.dev/golang.org/x/net), [golang.org/x/text](https://pkg.go.dev/golang.org/x/text), [golang.org/x/image](https://pkg.go.dev/golang.org/x/image)
- [Docling](https://github.com/docling-project/docling) (MIT) — reference for the output protocol and chunking semantics

## Community

Join us through any of these channels:

- Issues / PRs: this repository
- Scan the QR code to follow our WeChat official account for release notes and document-parsing deep dives — let's build together:

<p align="center">
  <img src="assets/wechat-official-account.jpg" alt="WeChat official account QR code" width="200"/>
</p>

## License

[MIT](LICENSE)

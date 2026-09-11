# docparse — Unified Multi-Format Document Parsing in Pure Go

English | [简体中文](README.md)

`docparse` converts PDF, Word, PPT, Excel/CSV, HTML, Markdown, AsciiDoc, EML email, images and plain text documents into a unified **DoclingDocument** structured model (aligned with the Docling Core `1.10.0` official protocol), ready for knowledge bases, RAG chunking, embedding pipelines and AI toolchains.

Design goals: **pure Go production baseline with zero external service dependencies**; when higher recognition quality is needed, LLM-based OCR and structured-vision hooks can be attached and applied page by page.

## Features

- **Unified protocol**: every format outputs a Docling-compatible `DoclingDocument` (schema 1.10.0), validated against the official Pydantic models
- **Pure Go parsing**: the default engine needs no external services; PDF font/CMap recovery, multi-column reading order, table structure recovery and Office comments/formulas/charts are all implemented in pure Go
- **LLM enhancement**: `OCRHook` (verbatim text) and `PDFVisualHook` (structured JSON with bboxes) are routed per page with built-in quality signals, retries and strict validation
- **Multiple exports**: Docling JSON / content_list / Markdown / HTML
- **RAG-ready**: hierarchical chunking (Hierarchical/Hybrid) and retrieval chunking (content_list) built in

## Supported Formats

| Format | Entry functions | Notes |
|--------|-----------------|-------|
| Auto-detect by extension | `ParseByExt` / `ParseByExtWithOptions` | Preferred entry when the format is unknown; fills in document name, MIME, low-64-bit SHA-256 hash and origin |
| PDF | `ParsePDF` / `ParsePDFWithOptions` | CropBox/UserUnit/Rotate normalization, word/line bboxes, ToUnicode/standard CJK/custom Encoding CMap font recovery, embedded bitmaps (incl. JPEG2000/JBIG2/soft masks), strict & borderless table recovery, cross-page table merging, recursive XY-cut; optional Poppler, OCR and structured-vision hooks |
| Word | `ParseDocx` | Transitional/Strict OOXML, headers/footers, modern comment threads, footnotes/endnotes, tracked changes & fields, text boxes, OMML formulas, images, rich tables, charts and complex Office objects |
| PPT | `ParsePPTX` | Transitional/Strict OOXML, notes, modern comment threads, layout/master fallback, grouped shapes, visual ordering, charts and complex Office objects |
| Excel / CSV | `ParseXLSX` / `ParseCSV` | Transitional/Strict OOXML, hidden sheets, threaded comments, raw formulas, pivot tables, images/shapes/SmartArt/OLE, chart sheets, stable ordering and structured tables |
| HTML | `ParseHTML` | Title/first-heading furniture, flattened headings, image placeholders, inline formatting/links, rich tables |
| Markdown | `ParseMarkdown` / `ParseMarkdownTable` | Official flattened headings, image placeholders, formatting/links, raw HTML delegation and GFM tables |
| AsciiDoc | `ParseAsciiDoc` | Heading tree, lists, literal/source blocks, PictureItem image placeholders |
| EML | `ParseEML` | RFC 5322 headers, best-body selection, common charsets, nested emails and attachment names |
| Images | `ParseImage` / `ParseImageWithOptions` | PNG/JPEG/BMP/WEBP single-page PictureItem; returns size/DPI without OCR, adds recognition results when configured |
| Plain text | `ParseText` | UTF-8 BOM and control-character cleanup fallback |
| Docling JSON | `ParseDoclingDocument` | Parses JSON produced by a Docling service (docling engine) |

## Quick Start

```bash
go get github.com/unitedrhino/docling
```

### 1. Auto-detect by extension (recommended entry)

```go
import docparse "github.com/unitedrhino/docling"

doc, err := docparse.ParseByExt("report.docx", data)
if err != nil { ... }
// DoclingDocument has no Items field; derive a flat list for knowledge bases.
items := docparse.ToContentList(doc, docparse.SourceGolight)
for _, it := range items {
    text := docparse.ItemToText(it) // extracts text per item type (heading path/table/formula...)
}
```

To record a source URI or pass PDF-specific options, use the options entry:

```go
doc, err := docparse.ParseByExtWithOptions("report.pdf", data, docparse.ParseOptions{
    OriginURI:       "s3://documents/report.pdf",
    GarbageThreshold: 0.4,
    OCRHook: func(req docparse.OCRRequest) (string, error) {
        // req carries page number, MIME, file name, raw data and existing text.
        return myOCR(req)
    },
})
```

### 2. Parse a known format directly

Each format exposes a standalone entry point:

```go
doc, err := docparse.ParseDocx(data)            // Word
doc, err := docparse.ParseMarkdown(mdData)      // Markdown
rows := docparse.ParseMarkdownTable(tableMd)    // parse a single Markdown table
```

### 3. Parse + export Markdown in one call

```go
md, err := docparse.ParseByExtToMarkdown("report.pdf", data)
```

### 4. PDF OCR and structured-vision hooks

```go
doc, err := docparse.ParsePDFWithOptions(data, docparse.PDFOptions{
    // GarbageThreshold: pages above this garbage ratio trigger OCR; 0 = scanned fallback pages only
    GarbageThreshold: 0.4,
    OCRHook: func(req docparse.OCRRequest) (string, error) {
        return myOCR(req)
    },
})
```

`OCRHook` takes precedence over the compatible `PageOCRHook`. The library strips code fences and
model disclaimers, rejects empty results, refusals and obvious garbage, and retries once when a
result is invalid; pages that already contain text only adopt OCR when it is cleaner. Image input
reuses the same contract; without OCR it still returns a PictureItem carrying size, DPI and a data URI.

Complex pages can attach a structured-vision hook. By default it is called only for scanned,
garbled, unrecovered-table, formula-dense or column-ambiguous pages; `VisualAlways` explicitly
covers image-only tables without text signals. Labels, bboxes, confidence floors, text quality,
heading levels, object dedup, table topology and cell bboxes from the model are strictly
validated and retried once; valid objects are merged with rule-based text by bbox, and failures
keep the pure Go/Poppler/OCR result:

```go
doc, err := docparse.ParsePDFWithOptions(data, docparse.PDFOptions{
    OCRHook: myOCR,
    VisualHook: func(req docparse.PDFVisualRequest) (docparse.PDFVisualResult, error) {
        // req.Prompt is the built-in strict JSON prompt; decode the model output into the result struct.
        return myStructuredVisual(req)
    },
    MaxVisualPages: 20,
})
```

### 5. Content lists and exports

```go
items := docparse.ToContentList(doc, docparse.SourceGolight) // flat list for knowledge bases
md := doc.ToMarkdown()                                       // export as Markdown (== ExportMarkdown(doc))
html := doc.ToHTML()                                         // export as a full HTML document (== ExportHTML(doc))

// Only body is exported by default; select layers to review headers and notes.
mdWithFurniture := doc.ToMarkdownWithOptions(docparse.ExportOptions{
    Layers: []docparse.ContentLayer{docparse.LayerBody, docparse.LayerFurniture},
})
```

Charts follow the Docling 1.10 protocol and live in `pictures` with `label=picture`; the
classification and structured data are stored in `meta.classification` and
`meta.tabular_chart.chart_data`. Chart data prefers the cached values embedded by Office; when
the cache is missing, XLSX reads the current workbook and DOCX/PPTX read the embedded workbook
referenced by the chart relationship. Common bar, line, area, pie, doughnut, scatter, radar and
combo charts render a 960×540, 96 DPI pure Go SVG `ImageRef`; the Markdown/HTML exporters also
append the chart data as a searchable table. Formulas resolve plain A1 ranges, workbook/sheet
scoped defined names that resolve to a single A1 range, and `TableName[Column]` structured
references. The SVG is a semantic preview and does not replicate Office fonts, themes, 3D effects
or animations; dynamic named formulas, multi-area/qualified structured references and Office 2016
chart extensions fall back to cached data. A single formula reads at most 100,000 cells.

SmartArt, WordArt, text-bearing shapes and OLE objects also follow `label=picture`: visible text
joins retrieval via captions and relationship targets are stored in the
`docparse__office_object_*` extension fields. OLE payloads are recorded only — never executed.

### 6. Hierarchical and knowledge-base chunking

```go
// Mirrors the official HierarchicalChunker: headings only update context, list groups and
// tables keep their structure, and no length trimming is applied; each chunk carries the
// official doc_items plus convenient doc refs, provenance and origin.
semanticChunks := docparse.HierarchicalChunks(doc)

// Token-aware splitting + same-heading peer merging. CountTokens should match the
// embedding model; plug in an existing pure Go tokenizer directly.
hybridChunks := docparse.HybridChunks(doc, docparse.HybridChunkOptions{
    MaxTokens:   512,
    CountTokens: embeddingTokenizer.Count,
})

// Knowledge-base compatible strategy: 900 runes by default, 100 table rows per segment,
// filtering page headers/footers and tables of contents.
contentChunks := docparse.ChunkContentList(items, docparse.ContentChunkOptions{})
```

### 7. Document origin and metadata

The unified entries fill the official `origin`: `filename`, `mimetype`, `binary_hash` and an
optional `uri`. `DocMeta` is filled best-effort per format:

```go
doc.Meta.Title     // title (dc:title / email Subject)
doc.Meta.Author    // author (dc:creator / email From display name)
doc.Meta.PageCount // pages (PDF pages / pptx slides / xlsx sheets)
```

## examples: samples paired with converted output

The [`examples/`](examples/) directory ships one pair of files per supported format:

- `sample.<ext>`: a minimal, typical source file
- `sample.expected.md`: the full expected output of `ParseByExtToMarkdown`

Comparing the two files shows exactly what each format converts to; regression tests keep both
in sync with the parser:

```bash
go test ./... -run TestExamplesGolden            # strict verbatim regression
go test ./... -run TestExamplesGolden -update    # rebuild the corpus after parser changes
```

Read any sample from `examples/` and call `ParseByExtToMarkdown` to reproduce the contents of
its `sample.expected.md`.

## Docling JSON protocol

- Output is fixed to `schema_name=DoclingDocument`, `version=1.10.0`.
- Heading levels use the official `level`; `text_level` is only read for compatibility.
- The top level always contains `body`, `furniture`, `groups`, `texts`, `pictures`, `tables`,
  `key_value_items`, `form_items` and `pages`; official collections not yet extracted still
  round-trip losslessly.
- Content layers support `body`, `furniture`, `background`, `invisible` and `notes`, with
  `furniture.content_layer` fixed to `furniture`.
- Legacy `meta`, `caption`, `latex` and `annotations` are readable; on re-serialization they are
  normalized into official fields, references or node-level `meta`.

## Relationship to Docling and known boundaries

The protocol layer is fully aligned with Docling Core `1.10.0` and round-trips official JSON
losslessly. On parsing quality:

- **Pure Go strengths**: font/garbled-text recovery for text-based PDFs, structured tables,
  multi-column reading order and structured Office objects (comments/formulas/charts/revisions)
  — none of these need a model to produce structured output
- **Model-enhanced**: OCR for scanned pages, image tables, complex merged cells, formula-dense
  pages and column-ambiguous pages — attach `OCRHook`/`VisualHook` and they are enhanced per page
- **Known conservative fallbacks** (results are never fabricated): CCITT K>0 (Group 3 2D)
  compression, real ICC color management, soft-mask Matte semantics, pixel-level layout
  segmentation and vector graphics semantics — such content keeps its raw signal and enters the
  optional vision path
- Without any model configured, the pure Go path still emits fully structured documents; text
  recognition for scanned pages requires OCR

## Repository layout

```
docling/
├── docparse.go     # unified model and facade: Item, ParseByExt, ParseByExtToMarkdown, item tools
├── docling.go      # DoclingDocument model (TextItem/TableItem/PageItem/DocMeta etc.)
├── doclingserve.go # Docling service parsing (optional second engine)
├── contentlist.go  # ToContentList (knowledge-base content list)
├── chunker.go      # HierarchicalChunks (official hierarchical semantics)
├── hybrid_chunker.go # HybridChunks (token-aware splitting, header repetition, peer merging)
├── content_chunk.go # ChunkContentList (knowledge-base length/table/multimodal strategy)
├── export.go       # ExportMarkdown / ToMarkdown
├── export_html.go  # ExportHTML / ToHTML
├── media.go        # OOXML shared: media data URIs, docProps/core.xml metadata
├── ooxml_chart*.go # OOXML charts: classification, formula backfill, tabular data, pure Go SVG preview
├── ooxml_strict.go # in-memory Strict → Transitional OOXML normalization
├── office_comment.go # OOXML comments and reply chains
├── office_object*.go # SmartArt/WordArt/shapes/OLE semantic mapping and SVG preview
├── ocr.go          # OCRRequest/OCRHook, compatible PageOCRHook and quality fallback
├── pdf*.go         # PDF parsing: text/layout/tables/Unicode recovery/image decoding/vision routing/optional Poppler
├── docx*.go pptx.go sheet.go # Word/PPT/Excel parsing
├── html.go markdown.go asciidoc.go eml.go image.go text.go table.go # remaining formats
├── examples/       # per-format sample sources paired with expected Markdown output
└── *_test.go       # unit/benchmark/dual-engine/official schema validation tests
```

The package follows a **single package + narrow public interface** design: only the format
entries, unified model and content tools are public API; text cleanup, table escaping/rendering
and PDF line classification are unexported implementation details.

## Dependencies & acknowledgements

- [pdfcpu](https://github.com/pdfcpu/pdfcpu) (Apache-2.0) — PDF pixel decoding foundation
- [excelize](https://github.com/qax-os/excelize) (BSD-3-Clause) — XLSX reading and chart formula backfill
- [ledongthuc/pdf](https://github.com/ledongthuc/pdf) (MIT) — PDF text coordinates foundation
- [go-jpeg2000](https://github.com/mrjoshuak/go-jpeg2000) (Apache-2.0) — pure Go JPEG 2000 decoding
- [gobig2](https://github.com/dkrisman/gobig2) (Apache-2.0) — pure Go JBIG2 decoding
- [goldmark](https://github.com/yuin/goldmark) (MIT), [golang.org/x/net](https://pkg.go.dev/golang.org/x/net), [golang.org/x/text](https://pkg.go.dev/golang.org/x/text), [golang.org/x/image](https://pkg.go.dev/golang.org/x/image)
- [Docling](https://github.com/DS4SD/docling) (MIT) — reference for the output protocol and chunking semantics

## License

[MIT](LICENSE)

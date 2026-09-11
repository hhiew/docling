// docling.go 定义 DoclingDocument 的 Go 结构体与文档树构建 API，
// 序列化形态与 docling-core 2.93.0（schema version 1.10.0）保持同构，
// 作为 docling 通用组件的"详细 JSON"输出协议：
// 各解析器（markdown/docx/pdf/xlsx/pptx/html 等）产出本结构，
// 再由 contentlist.go 的 ToContentList 统一简化为知识库 content_list。
//
// 协议要点（源自 docling-core 源码核对）：
//   - 引用一律为 {"$ref": "#/texts/0"} 形式（RefItem）；
//   - texts 为八类文本元素的 Union，Go 侧用单结构体 + label 分派专属字段表达；
//   - prov（ProvenanceItem）为列表，bbox 默认 TOPLEFT 坐标原点；
//   - 表格 cell 行列偏移 start 闭 end 开，grid 为序列化时现算的 computed 字段；
//   - pages 键为字符串化的页号（Python int 键序列化产物）。
package docling

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// Docling schema 固定值（docling_core/types/doc/common/constants.py）。
const (
	// DoclingSchemaName schema 固定字面量。
	DoclingSchemaName = "DoclingDocument"
	// DoclingSchemaVersion 当前 schema 版本。
	DoclingSchemaVersion = "1.10.0"
)

// DocItemLabel 元素细粒度分类，取值对齐 docling-core DocItemLabel 的常用子集。
type DocItemLabel string

// DocItemLabel 常用取值。
const (
	LabelTitle              DocItemLabel = "title"
	LabelSectionHeader      DocItemLabel = "section_header"
	LabelText               DocItemLabel = "text"
	LabelParagraph          DocItemLabel = "paragraph"
	LabelListItem           DocItemLabel = "list_item"
	LabelCode               DocItemLabel = "code"
	LabelFormula            DocItemLabel = "formula"
	LabelCaption            DocItemLabel = "caption"
	LabelFootnote           DocItemLabel = "footnote"
	LabelReference          DocItemLabel = "reference"
	LabelHandwrittenText    DocItemLabel = "handwritten_text"
	LabelEmptyValue         DocItemLabel = "empty_value"
	LabelFieldKey           DocItemLabel = "field_key"
	LabelFieldValue         DocItemLabel = "field_value"
	LabelFieldHint          DocItemLabel = "field_hint"
	LabelFieldHeading       DocItemLabel = "field_heading"
	LabelMarker             DocItemLabel = "marker"
	LabelCheckboxSelected   DocItemLabel = "checkbox_selected"
	LabelCheckboxUnselected DocItemLabel = "checkbox_unselected"
	LabelTable              DocItemLabel = "table"
	LabelPicture            DocItemLabel = "picture"
	// LabelChart 仅用于兼容旧 Docling 文档；新图表必须使用 LabelPicture
	// 并通过 PictureMeta.Classification 标注图表类型。
	LabelChart          DocItemLabel = "chart"
	LabelForm           DocItemLabel = "form"
	LabelKeyValueRegion DocItemLabel = "key_value_region"
	LabelGradingScale   DocItemLabel = "grading_scale"
	LabelFieldRegion    DocItemLabel = "field_region"
	LabelFieldItem      DocItemLabel = "field_item"
	// 版式家具与目录细分类（docling-core 枚举存在；PDF 后端用于
	// 页眉/页脚跨页指纹与目录点线区域的行级标注）。
	LabelPageHeader    DocItemLabel = "page_header"
	LabelPageFooter    DocItemLabel = "page_footer"
	LabelDocumentIndex DocItemLabel = "document_index"
)

// GroupLabel 分组节点类型，对齐 docling-core GroupLabel 常用子集。
type GroupLabel string

// GroupLabel 常用取值。
const (
	GroupLabelUnspecified    GroupLabel = "unspecified"
	GroupLabelList           GroupLabel = "list"
	GroupLabelOrderedList    GroupLabel = "ordered_list" // 仅兼容官方已弃用取值
	GroupLabelChapter        GroupLabel = "chapter"
	GroupLabelSection        GroupLabel = "section"
	GroupLabelSheet          GroupLabel = "sheet"
	GroupLabelSlide          GroupLabel = "slide"
	GroupLabelFormArea       GroupLabel = "form_area"
	GroupLabelKeyValueArea   GroupLabel = "key_value_area"
	GroupLabelCommentSection GroupLabel = "comment_section"
	GroupLabelInline         GroupLabel = "inline"
	GroupLabelPictureArea    GroupLabel = "picture_area"
)

// ContentLayer 内容层：正文/版式家具/不可见/备注，对齐 docling-core ContentLayer。
type ContentLayer string

// ContentLayer 取值。
const (
	LayerBody       ContentLayer = "body"
	LayerFurniture  ContentLayer = "furniture"
	LayerBackground ContentLayer = "background"
	LayerInvisible  ContentLayer = "invisible"
	LayerNotes      ContentLayer = "notes"
)

// CoordOrigin bbox 坐标原点：TOPLEFT（默认）/ BOTTOMLEFT（pptx 等页面坐标系使用）。
type CoordOrigin string

// CoordOrigin 取值。
const (
	CoordOriginTopLeft    CoordOrigin = "TOPLEFT"
	CoordOriginBottomLeft CoordOrigin = "BOTTOMLEFT"
)

// docRefKind 引用指向的顶层集合，决定 $ref 路径段。
type docRefKind string

// 顶层集合名。
const (
	refBody      docRefKind = "body"
	refFurniture docRefKind = "furniture"
	refGroups    docRefKind = "groups"
	refTexts     docRefKind = "texts"
	refTables    docRefKind = "tables"
	refPictures  docRefKind = "pictures"
)

// RefItem 文档树内引用，JSON 形态为 {"$ref": "#/texts/0"}；body 恒为 "#/body"。
type RefItem struct {
	Kind docRefKind `json:"-"`
	Idx  int64      `json:"-"`
}

// FineRef 是可选携带字符区间的细粒度引用，用于评论等注释关系。
type FineRef struct {
	RefItem
	Range *[2]int64 `json:"range,omitempty"`
}

// MarshalJSON 输出 Docling 的 {$ref, range?} 形态。
func (r FineRef) MarshalJSON() ([]byte, error) {
	payload := map[string]any{"$ref": r.RefItem.String()}
	if r.Range != nil {
		payload["range"] = r.Range
	}
	return json.Marshal(payload)
}

// UnmarshalJSON 解析 Docling 的细粒度引用。
func (r *FineRef) UnmarshalJSON(data []byte) error {
	var payload struct {
		Ref   string    `json:"$ref"`
		Range *[2]int64 `json:"range"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return err
	}
	refData, err := json.Marshal(map[string]string{"$ref": payload.Ref})
	if err != nil {
		return err
	}
	if err := r.RefItem.UnmarshalJSON(refData); err != nil {
		return err
	}
	r.Range = payload.Range
	return nil
}

// MarshalJSON 序列化为 {"$ref": "#/texts/0"} 形态。
func (r RefItem) MarshalJSON() ([]byte, error) {
	if r.Kind == refBody {
		return []byte(`{"$ref":"#/body"}`), nil
	}
	if r.Kind == refFurniture {
		return []byte(`{"$ref":"#/furniture"}`), nil
	}
	return json.Marshal(struct {
		Ref string `json:"$ref"`
	}{Ref: fmt.Sprintf("#/%s/%d", r.Kind, r.Idx)})
}

// UnmarshalJSON 解析 "#/texts/0" 形式的引用。
func (r *RefItem) UnmarshalJSON(data []byte) error {
	var raw struct {
		Ref string `json:"$ref"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	ref := strings.TrimPrefix(raw.Ref, "#/")
	if ref == "body" {
		r.Kind = refBody
		r.Idx = 0
		return nil
	}
	if ref == "furniture" {
		r.Kind = refFurniture
		r.Idx = 0
		return nil
	}
	kind, idx, ok := strings.Cut(ref, "/")
	if !ok {
		return fmt.Errorf("docling: 非法 $ref: %q", raw.Ref)
	}
	r.Kind = docRefKind(kind)
	var n int64
	if _, err := fmt.Sscanf(idx, "%d", &n); err != nil {
		return fmt.Errorf("docling: 非法 $ref 下标: %q", raw.Ref)
	}
	r.Idx = n
	return nil
}

// String 输出 cref 字符串形式（#/texts/0），便于日志与调试。
func (r RefItem) String() string {
	if r.Kind == refBody {
		return "#/body"
	}
	if r.Kind == refFurniture {
		return "#/furniture"
	}
	return fmt.Sprintf("#/%s/%d", r.Kind, r.Idx)
}

// DoclingBBox 页面/页内坐标边界框；坐标原点由 CoordOrigin 决定（默认 TOPLEFT）。
type DoclingBBox struct {
	L           float64     `json:"l"`
	T           float64     `json:"t"`
	R           float64     `json:"r"`
	B           float64     `json:"b"`
	CoordOrigin CoordOrigin `json:"coord_origin"`
}

// MarshalJSON 为未显式设置坐标原点的旧对象补齐 Docling 默认 TOPLEFT。
func (b DoclingBBox) MarshalJSON() ([]byte, error) {
	type alias DoclingBBox
	if b.CoordOrigin == "" {
		b.CoordOrigin = CoordOriginTopLeft
	}
	return json.Marshal(alias(b))
}

// ProvenanceItem 元素来源证据：页号 + 页内 bbox + 字符偏移区间（0 起，[start,end)）。
// 一个元素跨行/跨页时可有多段。
type ProvenanceItem struct {
	PageNo   int64        `json:"page_no"`
	BBox     *DoclingBBox `json:"bbox"`
	CharSpan [2]int64     `json:"charspan"`
}

// MarshalJSON 为旧来源记录补齐官方必需的 bbox 对象。
func (p ProvenanceItem) MarshalJSON() ([]byte, error) {
	type alias ProvenanceItem
	if p.BBox == nil {
		p.BBox = &DoclingBBox{CoordOrigin: CoordOriginTopLeft}
	}
	return json.Marshal(alias(p))
}

// DocumentOrigin 文档来源信息（mimetype/文件名等）。
type DocumentOrigin struct {
	Mimetype   string `json:"mimetype"`
	BinaryHash uint64 `json:"binary_hash"`
	Filename   string `json:"filename"`
	URI        string `json:"uri,omitempty"`
}

// UnmarshalJSON 兼容 Docling 接受的十六进制字符串 binary_hash 输入。
func (o *DocumentOrigin) UnmarshalJSON(data []byte) error {
	var payload struct {
		Mimetype   string          `json:"mimetype"`
		BinaryHash json.RawMessage `json:"binary_hash"`
		Filename   string          `json:"filename"`
		URI        string          `json:"uri"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return err
	}
	var hash uint64
	if err := json.Unmarshal(payload.BinaryHash, &hash); err != nil {
		var encoded string
		if stringErr := json.Unmarshal(payload.BinaryHash, &encoded); stringErr != nil {
			return fmt.Errorf("docling: 非法 binary_hash: %w", err)
		}
		encoded = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(encoded)), "0x")
		if len(encoded) > 16 {
			encoded = encoded[len(encoded)-16:]
		}
		parsed, parseErr := strconv.ParseUint(encoded, 16, 64)
		if parseErr != nil {
			return fmt.Errorf("docling: 非法十六进制 binary_hash: %w", parseErr)
		}
		hash = parsed
	}
	*o = DocumentOrigin{Mimetype: payload.Mimetype, BinaryHash: hash, Filename: payload.Filename, URI: payload.URI}
	return nil
}

// ScriptPosition 表示文本相对基线的位置。
type ScriptPosition string

// ScriptPosition 取值对齐 docling-core Script。
const (
	ScriptBaseline ScriptPosition = "baseline"
	ScriptSub      ScriptPosition = "sub"
	ScriptSuper    ScriptPosition = "super"
)

// Formatting 表示单个文本元素的统一样式。
type Formatting struct {
	Bold          bool           `json:"bold"`
	Italic        bool           `json:"italic"`
	Underline     bool           `json:"underline"`
	Strikethrough bool           `json:"strikethrough"`
	Script        ScriptPosition `json:"script"`
}

// MarshalJSON 为旧对象补齐官方默认 baseline。
func (f Formatting) MarshalJSON() ([]byte, error) {
	type alias Formatting
	if f.Script == "" {
		f.Script = ScriptBaseline
	}
	return json.Marshal(alias(f))
}

// BaseMeta 保存节点级官方元数据及命名空间扩展字段。
// 使用 RawMessage 保证当前解析器尚不理解的官方或自定义字段可无损往返。
type BaseMeta map[string]json.RawMessage

// TextItem texts[] 元素：单结构体覆盖 title/section_header/text/paragraph/
// list_item/code/formula/checkbox 等八类 Union，专属字段按 label 填充。
type TextItem struct {
	SelfRef      string            `json:"self_ref"`
	Parent       *RefItem          `json:"parent,omitempty"`
	Children     []RefItem         `json:"children"`
	ContentLayer ContentLayer      `json:"content_layer"`
	Meta         BaseMeta          `json:"meta,omitempty"`
	Label        DocItemLabel      `json:"label"`
	Prov         []ProvenanceItem  `json:"prov"`
	Source       []json.RawMessage `json:"source,omitempty"`
	Comments     []FineRef         `json:"comments,omitempty"`
	Orig         string            `json:"orig"`
	Text         string            `json:"text"`
	// —— 以下为按 label 分派的专属字段 ——
	TextLevel    int64       `json:"level,omitempty"`         // section_header：标题层级（1 起）
	Enumerated   *bool       `json:"enumerated,omitempty"`    // list_item：是否有序
	Marker       string      `json:"marker,omitempty"`        // list_item：序号标记（"1."/"·"）
	CodeLanguage string      `json:"code_language,omitempty"` // code：代码语言
	Hyperlink    string      `json:"hyperlink,omitempty"`     // 行内链接目标
	Formatting   *Formatting `json:"formatting,omitempty"`    // 统一文本样式
	Latex        string      `json:"-"`                       // formula：旧版 schema 的独立 latex 字段（仅兼容读取）
	Captions     []RefItem   `json:"captions,omitempty"`      // code：标题说明引用
	References   []RefItem   `json:"references,omitempty"`    // code：相关内容引用
	Footnotes    []RefItem   `json:"footnotes,omitempty"`     // code：脚注引用
	Image        *ImageRef   `json:"image,omitempty"`         // code：可选渲染图
}

// MarshalJSON 为各文本 Union 补齐官方 label 专属默认字段。
func (t TextItem) MarshalJSON() ([]byte, error) {
	type alias TextItem
	if t.ContentLayer == "" {
		t.ContentLayer = LayerBody
	}
	if t.Children == nil {
		t.Children = []RefItem{}
	}
	t.Prov = nonNilProvenance(t.Prov)
	if t.Label == LabelSectionHeader && t.TextLevel <= 0 {
		t.TextLevel = 1
	}
	if t.Label == LabelListItem {
		if t.Enumerated == nil {
			value := false
			t.Enumerated = &value
		}
		if t.Marker == "" {
			t.Marker = "-"
		}
	}
	if t.Label == LabelCode {
		t.CodeLanguage = canonicalCodeLanguage(t.CodeLanguage)
		if t.Captions == nil {
			t.Captions = []RefItem{}
		}
		if t.References == nil {
			t.References = []RefItem{}
		}
		if t.Footnotes == nil {
			t.Footnotes = []RefItem{}
		}
	}
	data, err := json.Marshal(alias(t))
	if err != nil || t.Label != LabelCode {
		return data, err
	}
	// omitempty 避免普通 TextItem 出现 FloatingItem 专属字段；code 分支
	// 再显式补齐官方空引用集合。
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, err
	}
	emptyList := json.RawMessage(`[]`)
	for _, field := range []string{"captions", "references", "footnotes"} {
		if _, exists := payload[field]; !exists {
			payload[field] = emptyList
		}
	}
	return json.Marshal(payload)
}

// canonicalCodeLanguage 把常见 Markdown 语言标记转换为 Docling 枚举值。
func canonicalCodeLanguage(language string) string {
	normalized := strings.ToLower(strings.TrimSpace(language))
	if normalized == "" {
		return "unknown"
	}
	aliases := map[string]string{
		"ada": "Ada", "awk": "Awk", "bash": "Bash", "sh": "Bash", "shell": "Bash",
		"bc": "bc", "c": "C", "c#": "C#", "csharp": "C#", "c++": "C++", "cpp": "C++",
		"cmake": "CMake", "cobol": "COBOL", "css": "CSS", "ceylon": "Ceylon",
		"clojure": "Clojure", "crystal": "Crystal", "cuda": "Cuda", "cython": "Cython",
		"d": "D", "dart": "Dart", "dc": "dc", "dockerfile": "Dockerfile", "doclang": "DocLang",
		"elixir": "Elixir", "erlang": "Erlang", "fortran": "FORTRAN", "forth": "Forth",
		"go": "Go", "golang": "Go", "html": "HTML", "haskell": "Haskell", "haxe": "Haxe",
		"java": "Java", "javascript": "JavaScript", "js": "JavaScript", "json": "JSON",
		"julia": "Julia", "kotlin": "Kotlin", "latex": "Latex", "lisp": "Lisp", "lua": "Lua",
		"matlab": "Matlab", "moonscript": "MoonScript", "nim": "Nim", "ocaml": "OCaml",
		"objectivec": "ObjectiveC", "objective-c": "ObjectiveC", "octave": "Octave",
		"php": "PHP", "pascal": "Pascal", "perl": "Perl", "prolog": "Prolog",
		"python": "Python", "py": "Python", "racket": "Racket", "ruby": "Ruby", "rust": "Rust",
		"sml": "SML", "sql": "SQL", "scala": "Scala", "scheme": "Scheme", "swift": "Swift",
		"tikz": "Tikz", "typescript": "TypeScript", "ts": "TypeScript", "unknown": "unknown",
		"visualbasic": "VisualBasic", "visual-basic": "VisualBasic", "xml": "XML",
		"yaml": "YAML", "yml": "YAML",
	}
	if canonical, ok := aliases[normalized]; ok {
		return canonical
	}
	return "unknown"
}

// UnmarshalJSON 兼容旧 text_level/latex，同时以官方 level/text 作为规范字段。
func (t *TextItem) UnmarshalJSON(data []byte) error {
	type alias TextItem
	var payload struct {
		alias
		LegacyLevel int64  `json:"text_level"`
		LegacyLatex string `json:"latex"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return err
	}
	*t = TextItem(payload.alias)
	if t.TextLevel == 0 {
		t.TextLevel = payload.LegacyLevel
	}
	t.Latex = payload.LegacyLatex
	if t.Text == "" && payload.LegacyLatex != "" {
		t.Text = payload.LegacyLatex
	}
	return nil
}

// GroupItem groups[] 分组节点：标题隐式 section、列表 group、sheet/slide 组等。
type GroupItem struct {
	SelfRef      string       `json:"self_ref"`
	Parent       *RefItem     `json:"parent,omitempty"`
	Children     []RefItem    `json:"children"`
	ContentLayer ContentLayer `json:"content_layer"`
	Meta         BaseMeta     `json:"meta,omitempty"`
	Label        GroupLabel   `json:"label"`
	Name         string       `json:"name"`
}

// DoclingTableCell 表格单元格；行列偏移 start 闭 end 开（end=普通单元格 start+1）。
type DoclingTableCell struct {
	BBox              *DoclingBBox `json:"bbox,omitempty"`
	RowSpan           int64        `json:"row_span"`
	ColSpan           int64        `json:"col_span"`
	StartRowOffsetIdx int64        `json:"start_row_offset_idx"`
	EndRowOffsetIdx   int64        `json:"end_row_offset_idx"`
	StartColOffsetIdx int64        `json:"start_col_offset_idx"`
	EndColOffsetIdx   int64        `json:"end_col_offset_idx"`
	Text              string       `json:"text"`
	ColumnHeader      bool         `json:"column_header"`
	RowHeader         bool         `json:"row_header"`
	RowSection        bool         `json:"row_section"`
	Fillable          bool         `json:"fillable"`
	Ref               *RefItem     `json:"ref,omitempty"`
}

// MarshalJSON 为旧表格单元格补齐官方默认跨度。
func (c DoclingTableCell) MarshalJSON() ([]byte, error) {
	type alias DoclingTableCell
	if c.RowSpan <= 0 {
		c.RowSpan = 1
	}
	if c.ColSpan <= 0 {
		c.ColSpan = 1
	}
	return json.Marshal(alias(c))
}

// TableOrientation 表示表格在页面中的逆时针旋转角度。
type TableOrientation string

// TableOrientation 取值对齐 docling-core Orientation。
const (
	TableOrientation0   TableOrientation = "rot_0"
	TableOrientation90  TableOrientation = "rot_90"
	TableOrientation180 TableOrientation = "rot_180"
	TableOrientation270 TableOrientation = "rot_270"
)

// TableData 表格数据；grid 为序列化时由 table_cells 按 offset 现算的 computed 字段。
type TableData struct {
	TableCells  []DoclingTableCell `json:"table_cells"`
	NumRows     int64              `json:"num_rows"`
	NumCols     int64              `json:"num_cols"`
	Orientation TableOrientation   `json:"orientation"`
}

// MarshalJSON 输出 table_cells/num_rows/num_cols 并现算 grid 矩阵，
// 对齐 docling-core TableData.grid computed field 的序列化行为。
func (t TableData) MarshalJSON() ([]byte, error) {
	type alias TableData
	if t.TableCells == nil {
		t.TableCells = []DoclingTableCell{}
	}
	if t.Orientation == "" {
		t.Orientation = TableOrientation0
	}
	grid := make([][]*DoclingTableCell, 0, t.NumRows)
	for rowIndex := int64(0); rowIndex < t.NumRows; rowIndex++ {
		row := make([]*DoclingTableCell, 0, t.NumCols)
		for colIndex := int64(0); colIndex < t.NumCols; colIndex++ {
			row = append(row, &DoclingTableCell{
				RowSpan:           1,
				ColSpan:           1,
				StartRowOffsetIdx: rowIndex,
				EndRowOffsetIdx:   rowIndex + 1,
				StartColOffsetIdx: colIndex,
				EndColOffsetIdx:   colIndex + 1,
			})
		}
		grid = append(grid, row)
	}
	for i := range t.TableCells {
		c := &t.TableCells[i]
		for r := c.StartRowOffsetIdx; r < c.EndRowOffsetIdx && r < t.NumRows; r++ {
			for col := c.StartColOffsetIdx; col < c.EndColOffsetIdx && col < t.NumCols; col++ {
				grid[r][col] = c
			}
		}
	}
	return json.Marshal(struct {
		alias
		Grid [][]*DoclingTableCell `json:"grid"`
	}{alias(t), grid})
}

// TableItem tables[] 表格元素。
type TableItem struct {
	SelfRef      string            `json:"self_ref"`
	Parent       *RefItem          `json:"parent,omitempty"`
	Children     []RefItem         `json:"children"`
	ContentLayer ContentLayer      `json:"content_layer"`
	Meta         BaseMeta          `json:"meta,omitempty"`
	Label        DocItemLabel      `json:"label"`
	Prov         []ProvenanceItem  `json:"prov"`
	Source       []json.RawMessage `json:"source,omitempty"`
	Comments     []FineRef         `json:"comments,omitempty"`
	Captions     []RefItem         `json:"captions"`
	References   []RefItem         `json:"references"`
	Footnotes    []RefItem         `json:"footnotes"`
	Image        *ImageRef         `json:"image,omitempty"`
	Annotations  []json.RawMessage `json:"annotations"`
	Caption      string            `json:"-"` // 旧版字符串 caption，仅兼容读取
	Data         *TableData        `json:"data"`
}

// UnmarshalJSON 兼容旧版字符串 caption。
func (t *TableItem) UnmarshalJSON(data []byte) error {
	type alias TableItem
	var payload struct {
		alias
		Caption string `json:"caption"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return err
	}
	*t = TableItem(payload.alias)
	t.Caption = payload.Caption
	return nil
}

// ImageSize 图片像素尺寸。
type ImageSize struct {
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

// ImageRef 图片资产引用：内嵌为 base64 data URI，也可为 http(s)/路径引用。
type ImageRef struct {
	Mimetype string     `json:"mimetype"`
	Dpi      int64      `json:"dpi"`
	Size     *ImageSize `json:"size"`
	URI      string     `json:"uri"`
}

// MarshalJSON 为旧图片引用补齐官方必需的 size 对象。
func (r ImageRef) MarshalJSON() ([]byte, error) {
	type alias ImageRef
	if r.Size == nil {
		r.Size = &ImageSize{}
	}
	return json.Marshal(alias(r))
}

// PredictionMeta 是带可选置信度和来源的预测元数据基类。
type PredictionMeta struct {
	Confidence *float64 `json:"confidence,omitempty"`
	CreatedBy  string   `json:"created_by,omitempty"`
}

// PictureClassificationPrediction 表示图片分类的一项预测。
type PictureClassificationPrediction struct {
	PredictionMeta
	ClassName string `json:"class_name"`
}

// PictureClassificationMetaField 保存图片分类预测列表。
type PictureClassificationMetaField struct {
	Predictions []PictureClassificationPrediction `json:"predictions"`
}

// TabularChartMetaField 保存图表标题和表格化数据。
type TabularChartMetaField struct {
	PredictionMeta
	Title     string     `json:"title,omitempty"`
	ChartData *TableData `json:"chart_data"`
}

// PictureMeta 保存图片专属官方元数据，并无损保留命名空间扩展字段。
type PictureMeta struct {
	Classification *PictureClassificationMetaField `json:"classification,omitempty"`
	TabularChart   *TabularChartMetaField          `json:"tabular_chart,omitempty"`
	Description    json.RawMessage                 `json:"description,omitempty"`
	Molecule       json.RawMessage                 `json:"molecule,omitempty"`
	Code           json.RawMessage                 `json:"code,omitempty"`
	Extra          map[string]json.RawMessage      `json:"-"`
}

// MarshalJSON 合并标准字段与命名空间扩展字段。
func (m PictureMeta) MarshalJSON() ([]byte, error) {
	payload := make(map[string]json.RawMessage, len(m.Extra)+5)
	for key, value := range m.Extra {
		payload[key] = value
	}
	appendValue := func(key string, value any) error {
		data, err := json.Marshal(value)
		if err != nil {
			return err
		}
		payload[key] = data
		return nil
	}
	if m.Classification != nil {
		if err := appendValue("classification", m.Classification); err != nil {
			return nil, err
		}
	}
	if m.TabularChart != nil {
		if err := appendValue("tabular_chart", m.TabularChart); err != nil {
			return nil, err
		}
	}
	for key, value := range map[string]json.RawMessage{
		"description": m.Description,
		"molecule":    m.Molecule,
		"code":        m.Code,
	} {
		if len(value) > 0 && string(value) != "null" {
			payload[key] = value
		}
	}
	return json.Marshal(payload)
}

// UnmarshalJSON 解析标准图片元数据并保存未知扩展字段。
func (m *PictureMeta) UnmarshalJSON(data []byte) error {
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(data, &payload); err != nil {
		return err
	}
	m.Extra = map[string]json.RawMessage{}
	for key, value := range payload {
		switch key {
		case "classification":
			if err := json.Unmarshal(value, &m.Classification); err != nil {
				return err
			}
		case "tabular_chart":
			if err := json.Unmarshal(value, &m.TabularChart); err != nil {
				return err
			}
		case "description":
			m.Description = value
		case "molecule":
			m.Molecule = value
		case "code":
			m.Code = value
		default:
			m.Extra[key] = value
		}
	}
	return nil
}

// PictureItem pictures[] 图片元素（Go 各后端暂不产出，结构先行）。
type PictureItem struct {
	SelfRef      string            `json:"self_ref"`
	Parent       *RefItem          `json:"parent,omitempty"`
	Children     []RefItem         `json:"children"`
	ContentLayer ContentLayer      `json:"content_layer"`
	Meta         *PictureMeta      `json:"meta,omitempty"`
	Label        DocItemLabel      `json:"label"`
	Prov         []ProvenanceItem  `json:"prov"`
	Source       []json.RawMessage `json:"source,omitempty"`
	Comments     []FineRef         `json:"comments,omitempty"`
	Captions     []RefItem         `json:"captions"`
	References   []RefItem         `json:"references"`
	Footnotes    []RefItem         `json:"footnotes"`
	Annotations  []json.RawMessage `json:"annotations"`
	Caption      string            `json:"-"` // 旧版字符串 caption，仅兼容读取
	Image        *ImageRef         `json:"image,omitempty"`
}

// UnmarshalJSON 兼容旧版字符串 caption，并保留官方图片元数据。
func (p *PictureItem) UnmarshalJSON(data []byte) error {
	type alias PictureItem
	var payload struct {
		alias
		Caption string `json:"caption"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return err
	}
	*p = PictureItem(payload.alias)
	p.Caption = payload.Caption
	return nil
}

// PageItem 单页信息；pages 以字符串化页号为键。
type PageItem struct {
	Size   *ImageSize `json:"size"`
	PageNo int64      `json:"page_no"`
	Image  *ImageRef  `json:"image,omitempty"`
}

// MarshalJSON 为旧页面记录补齐官方必需的 size 对象。
func (p PageItem) MarshalJSON() ([]byte, error) {
	type alias PageItem
	if p.Size == nil {
		p.Size = &ImageSize{}
	}
	return json.Marshal(alias(p))
}

// UnmarshalJSON 兼容旧版 docling-serve 的平铺页结构 {"width":..,"height":..}
// （新版为 {"size":{"width","height"},"page_no":N}）。
func (p *PageItem) UnmarshalJSON(data []byte) error {
	type alias PageItem
	var a alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	if a.Size == nil {
		var legacy struct {
			Width  float64 `json:"width"`
			Height float64 `json:"height"`
		}
		if err := json.Unmarshal(data, &legacy); err == nil && (legacy.Width > 0 || legacy.Height > 0) {
			a.Size = &ImageSize{Width: legacy.Width, Height: legacy.Height}
		}
	}
	*p = PageItem(a)
	return nil
}

// DocMeta 文档级元数据：各解析器尽力填充（OOXML docProps/core.xml、
// PDF 页数、EML 邮件头等），取不到的字段留空不序列化。
type DocMeta struct {
	Title     string `json:"title,omitempty"`     // 文档标题（dc:title / 邮件 Subject）
	Author    string `json:"author,omitempty"`    // 作者（dc:creator / 邮件 From 显示名）
	Subject   string `json:"subject,omitempty"`   // 主题（dc:subject / 邮件 Subject）
	Language  string `json:"language,omitempty"`  // 语言（dc:language）
	PageCount int    `json:"pageCount,omitempty"` // 页数（PDF 页数 / pptx slide 数 / xlsx 可见 sheet 数）
	CreatedAt string `json:"createdAt,omitempty"` // 创建时间（dcterms:created / 邮件 Date，RFC3339 UTC）
}

// DoclingDocument 顶层文档结构（详细 JSON 协议），与 docling-core
// DoclingDocument.export_to_dict() 产物同构。
type DoclingDocument struct {
	SchemaName    string              `json:"schema_name"`
	Version       string              `json:"version"`
	Name          string              `json:"name"`
	Origin        *DocumentOrigin     `json:"origin,omitempty"`
	Meta          *DocMeta            `json:"-"`
	Body          *GroupItem          `json:"body"`
	Furniture     *GroupItem          `json:"furniture"`
	Groups        []GroupItem         `json:"groups"`
	Texts         []TextItem          `json:"texts"`
	Pictures      []PictureItem       `json:"pictures"`
	Tables        []TableItem         `json:"tables"`
	KeyValueItems []json.RawMessage   `json:"key_value_items"`
	FormItems     []json.RawMessage   `json:"form_items"`
	FieldRegions  []json.RawMessage   `json:"field_regions,omitempty"`
	FieldItems    []json.RawMessage   `json:"field_items,omitempty"`
	Pages         map[string]PageItem `json:"pages"`
}

// UnmarshalJSON 兼容读取旧版顶层 meta；该内部元数据不会再写入 Docling JSON。
func (d *DoclingDocument) UnmarshalJSON(data []byte) error {
	type alias DoclingDocument
	var current alias
	if err := json.Unmarshal(data, &current); err != nil {
		return err
	}
	var legacy struct {
		Meta *DocMeta `json:"meta"`
	}
	if err := json.Unmarshal(data, &legacy); err != nil {
		return err
	}
	*d = DoclingDocument(current)
	d.Meta = legacy.Meta
	return nil
}

// MarshalJSON 在输出前复制并规范化文档，确保旧字段只读兼容且不修改调用方对象。
func (d DoclingDocument) MarshalJSON() ([]byte, error) {
	canonical := cloneDoclingDocument(d)
	normalizeDoclingDocument(&canonical)
	type alias DoclingDocument
	return json.Marshal(alias(canonical))
}

// cloneDoclingDocument 复制规范化过程可能修改的集合与嵌套对象。
func cloneDoclingDocument(doc DoclingDocument) DoclingDocument {
	if doc.Body != nil {
		body := *doc.Body
		body.Children = append([]RefItem(nil), doc.Body.Children...)
		body.Meta = cloneBaseMeta(doc.Body.Meta)
		doc.Body = &body
	}
	if doc.Furniture != nil {
		furniture := *doc.Furniture
		furniture.Children = append([]RefItem(nil), doc.Furniture.Children...)
		furniture.Meta = cloneBaseMeta(doc.Furniture.Meta)
		doc.Furniture = &furniture
	}
	doc.Groups = append([]GroupItem(nil), doc.Groups...)
	for i := range doc.Groups {
		doc.Groups[i].Children = append([]RefItem(nil), doc.Groups[i].Children...)
		doc.Groups[i].Meta = cloneBaseMeta(doc.Groups[i].Meta)
	}
	doc.Texts = append([]TextItem(nil), doc.Texts...)
	for i := range doc.Texts {
		doc.Texts[i].Children = append([]RefItem(nil), doc.Texts[i].Children...)
		doc.Texts[i].Prov = append([]ProvenanceItem(nil), doc.Texts[i].Prov...)
		doc.Texts[i].Meta = cloneBaseMeta(doc.Texts[i].Meta)
	}
	doc.Tables = append([]TableItem(nil), doc.Tables...)
	for i := range doc.Tables {
		doc.Tables[i].Children = append([]RefItem(nil), doc.Tables[i].Children...)
		doc.Tables[i].Prov = append([]ProvenanceItem(nil), doc.Tables[i].Prov...)
		doc.Tables[i].Captions = append([]RefItem(nil), doc.Tables[i].Captions...)
		doc.Tables[i].References = append([]RefItem(nil), doc.Tables[i].References...)
		doc.Tables[i].Footnotes = append([]RefItem(nil), doc.Tables[i].Footnotes...)
		doc.Tables[i].Annotations = append([]json.RawMessage(nil), doc.Tables[i].Annotations...)
		doc.Tables[i].Meta = cloneBaseMeta(doc.Tables[i].Meta)
		doc.Tables[i].Data = cloneTableData(doc.Tables[i].Data)
	}
	doc.Pictures = append([]PictureItem(nil), doc.Pictures...)
	for i := range doc.Pictures {
		doc.Pictures[i].Children = append([]RefItem(nil), doc.Pictures[i].Children...)
		doc.Pictures[i].Prov = append([]ProvenanceItem(nil), doc.Pictures[i].Prov...)
		doc.Pictures[i].Captions = append([]RefItem(nil), doc.Pictures[i].Captions...)
		doc.Pictures[i].References = append([]RefItem(nil), doc.Pictures[i].References...)
		doc.Pictures[i].Footnotes = append([]RefItem(nil), doc.Pictures[i].Footnotes...)
		doc.Pictures[i].Annotations = append([]json.RawMessage(nil), doc.Pictures[i].Annotations...)
		doc.Pictures[i].Meta = clonePictureMeta(doc.Pictures[i].Meta)
	}
	doc.KeyValueItems = append([]json.RawMessage(nil), doc.KeyValueItems...)
	doc.FormItems = append([]json.RawMessage(nil), doc.FormItems...)
	doc.FieldRegions = append([]json.RawMessage(nil), doc.FieldRegions...)
	doc.FieldItems = append([]json.RawMessage(nil), doc.FieldItems...)
	if doc.Pages != nil {
		pages := make(map[string]PageItem, len(doc.Pages))
		for pageNo, page := range doc.Pages {
			pages[pageNo] = page
		}
		doc.Pages = pages
	}
	return doc
}

// cloneBaseMeta 复制节点元数据映射。
func cloneBaseMeta(meta BaseMeta) BaseMeta {
	if meta == nil {
		return nil
	}
	copyMeta := make(BaseMeta, len(meta))
	for key, value := range meta {
		copyMeta[key] = value
	}
	return copyMeta
}

// cloneTableData 复制可能由兼容归一化修改的表格数据。
func cloneTableData(data *TableData) *TableData {
	if data == nil {
		return nil
	}
	copyData := *data
	copyData.TableCells = append([]DoclingTableCell(nil), data.TableCells...)
	return &copyData
}

// clonePictureMeta 复制图片元数据及图表数据。
func clonePictureMeta(meta *PictureMeta) *PictureMeta {
	if meta == nil {
		return nil
	}
	copyMeta := *meta
	if meta.Classification != nil {
		classification := *meta.Classification
		classification.Predictions = append([]PictureClassificationPrediction(nil), meta.Classification.Predictions...)
		copyMeta.Classification = &classification
	}
	if meta.TabularChart != nil {
		chart := *meta.TabularChart
		chart.ChartData = cloneTableData(meta.TabularChart.ChartData)
		copyMeta.TabularChart = &chart
	}
	if meta.Extra != nil {
		copyMeta.Extra = make(map[string]json.RawMessage, len(meta.Extra))
		for key, value := range meta.Extra {
			copyMeta.Extra[key] = value
		}
	}
	return &copyMeta
}

// NewDoclingDocument 创建空文档，body/furniture 初始化对齐 docling-core
// （name="_root_"，self_ref="#/body"/"#/furniture"）。
func NewDoclingDocument(name string) *DoclingDocument {
	body := &GroupItem{
		SelfRef:      "#/body",
		Children:     []RefItem{},
		ContentLayer: LayerBody,
		Label:        GroupLabelUnspecified,
		Name:         "_root_",
	}
	return &DoclingDocument{
		SchemaName:    DoclingSchemaName,
		Version:       DoclingSchemaVersion,
		Name:          name,
		Body:          body,
		Furniture:     &GroupItem{SelfRef: "#/furniture", Children: []RefItem{}, ContentLayer: LayerFurniture, Label: GroupLabelUnspecified, Name: "_root_"},
		Groups:        []GroupItem{},
		Texts:         []TextItem{},
		Pictures:      []PictureItem{},
		Tables:        []TableItem{},
		KeyValueItems: []json.RawMessage{},
		FormItems:     []json.RawMessage{},
		FieldRegions:  nil,
		FieldItems:    nil,
		Pages:         map[string]PageItem{},
	}
}

// bodyRef 返回指向 body 的引用。
func (d *DoclingDocument) bodyRef() RefItem { return RefItem{Kind: refBody} }

// furnitureRef 返回指向 furniture 根节点的引用。
func (d *DoclingDocument) furnitureRef() RefItem { return RefItem{Kind: refFurniture} }

// addPage 登记页面尺寸（页号从 1 起）。
func (d *DoclingDocument) AddPage(pageNo int64, width, height float64) {
	d.Pages[fmt.Sprintf("%d", pageNo)] = PageItem{
		Size:   &ImageSize{Width: width, Height: height},
		PageNo: pageNo,
	}
}

// AddText 追加文本元素并挂到 parent（nil 则挂 body），返回元素自身引用。
func (d *DoclingDocument) AddText(label DocItemLabel, text string, prov []ProvenanceItem, parent *RefItem) RefItem {
	item := TextItem{
		Parent:       d.parentOrBody(parent),
		Children:     []RefItem{},
		ContentLayer: LayerBody,
		Label:        label,
		Prov:         nonNilProvenance(prov),
		Orig:         text,
		Text:         text,
	}
	d.Texts = append(d.Texts, item)
	ref := RefItem{Kind: refTexts, Idx: int64(len(d.Texts) - 1)}
	d.Texts[ref.Idx].SelfRef = ref.String()
	d.appendChild(parent, ref)
	return ref
}

// AddTitle 追加文档主标题（label=title）。
func (d *DoclingDocument) AddTitle(text string, prov []ProvenanceItem, parent *RefItem) RefItem {
	return d.AddText(LabelTitle, text, prov, parent)
}

// AddHeading 追加章节标题（label=section_header，level 从 1 起）。
func (d *DoclingDocument) AddHeading(level int64, text string, prov []ProvenanceItem, parent *RefItem) RefItem {
	ref := d.AddText(LabelSectionHeader, text, prov, parent)
	d.Texts[ref.Idx].TextLevel = level
	return ref
}

// AddCode 追加代码块（label=code，可携带语言标记）。
func (d *DoclingDocument) AddCode(text, language string, prov []ProvenanceItem, parent *RefItem) RefItem {
	ref := d.AddText(LabelCode, text, prov, parent)
	d.Texts[ref.Idx].CodeLanguage = language
	return ref
}

// AddFormula 追加公式（label=formula，text 存 LaTeX 源码）。
func (d *DoclingDocument) AddFormula(text string, prov []ProvenanceItem, parent *RefItem) RefItem {
	return d.AddText(LabelFormula, text, prov, parent)
}

// AddCheckbox 追加复选框（选中/未选中），并清除文本中的 ☐☑☒ 前缀符号。
func (d *DoclingDocument) AddCheckbox(checked bool, text string, prov []ProvenanceItem, parent *RefItem) RefItem {
	label := LabelCheckboxUnselected
	if checked {
		label = LabelCheckboxSelected
	}
	text = strings.TrimLeft(text, "☐☑☒ \t")
	return d.AddText(label, text, prov, parent)
}

// AddListGroup 追加列表分组（label=list），返回分组引用。
func (d *DoclingDocument) AddListGroup(name string, parent *RefItem) RefItem {
	item := GroupItem{
		Parent:       d.parentOrBody(parent),
		Children:     []RefItem{},
		ContentLayer: LayerBody,
		Label:        GroupLabelList,
		Name:         name,
	}
	d.Groups = append(d.Groups, item)
	ref := RefItem{Kind: refGroups, Idx: int64(len(d.Groups) - 1)}
	d.Groups[ref.Idx].SelfRef = ref.String()
	d.appendChild(parent, ref)
	return ref
}

// AddListItem 向列表分组追加列表项（label=list_item）。
func (d *DoclingDocument) AddListItem(group RefItem, text string, enumerated bool, marker string, prov []ProvenanceItem) RefItem {
	enumeratedFlag := enumerated
	ref := d.AddText(LabelListItem, text, prov, &group)
	d.Texts[ref.Idx].Enumerated = &enumeratedFlag
	d.Texts[ref.Idx].Marker = marker
	return ref
}

// AddTable 追加表格元素（cells 的行列偏移须 start 闭 end 开）。
func (d *DoclingDocument) AddTable(cells []DoclingTableCell, numRows, numCols int64, prov []ProvenanceItem, parent *RefItem) RefItem {
	if cells == nil {
		cells = []DoclingTableCell{}
	}
	item := TableItem{
		Parent:       d.parentOrBody(parent),
		Children:     []RefItem{},
		ContentLayer: LayerBody,
		Label:        LabelTable,
		Prov:         nonNilProvenance(prov),
		Captions:     []RefItem{},
		References:   []RefItem{},
		Footnotes:    []RefItem{},
		Annotations:  []json.RawMessage{},
		Data:         &TableData{TableCells: cells, NumRows: numRows, NumCols: numCols},
	}
	d.Tables = append(d.Tables, item)
	ref := RefItem{Kind: refTables, Idx: int64(len(d.Tables) - 1)}
	d.Tables[ref.Idx].SelfRef = ref.String()
	d.appendChild(parent, ref)
	return ref
}

// AddPicture 追加图片元素。
func (d *DoclingDocument) AddPicture(img *ImageRef, prov []ProvenanceItem, parent *RefItem) RefItem {
	item := PictureItem{
		Parent:       d.parentOrBody(parent),
		Children:     []RefItem{},
		ContentLayer: LayerBody,
		Label:        LabelPicture,
		Prov:         nonNilProvenance(prov),
		Captions:     []RefItem{},
		References:   []RefItem{},
		Footnotes:    []RefItem{},
		Annotations:  []json.RawMessage{},
		Image:        img,
	}
	d.Pictures = append(d.Pictures, item)
	ref := RefItem{Kind: refPictures, Idx: int64(len(d.Pictures) - 1)}
	d.Pictures[ref.Idx].SelfRef = ref.String()
	d.appendChild(parent, ref)
	return ref
}

// nonNilProvenance 保证官方必需的空列表稳定输出为 [] 而不是 null。
func nonNilProvenance(prov []ProvenanceItem) []ProvenanceItem {
	if prov == nil {
		return []ProvenanceItem{}
	}
	return prov
}

// AddSectionGroup 追加隐式 section 分组（标题层级树中用于跳级补位，
// 对齐 msword_backend 的 SECTION group 与 html_backend 的跳级行为）。
func (d *DoclingDocument) AddSectionGroup(name string, parent *RefItem) RefItem {
	item := GroupItem{
		Parent:       d.parentOrBody(parent),
		Children:     []RefItem{},
		ContentLayer: LayerBody,
		Label:        GroupLabelSection,
		Name:         name,
	}
	d.Groups = append(d.Groups, item)
	ref := RefItem{Kind: refGroups, Idx: int64(len(d.Groups) - 1)}
	d.Groups[ref.Idx].SelfRef = ref.String()
	d.appendChild(parent, ref)
	return ref
}

// addGroupNode 内部通用分组追加（AddListGroup/AddSectionGroup 共用）。

// parentOrBody parent 为空时回退到 body 引用。
func (d *DoclingDocument) parentOrBody(parent *RefItem) *RefItem {
	if parent == nil {
		r := d.bodyRef()
		return &r
	}
	p := *parent
	return &p
}

// appendChild 把子引用挂到 parent 节点的 children（nil 则挂 body）。
func (d *DoclingDocument) appendChild(parent *RefItem, child RefItem) {
	target := d.parentOrBody(parent)
	switch target.Kind {
	case refBody:
		d.Body.Children = append(d.Body.Children, child)
	case refFurniture:
		if d.Furniture != nil {
			d.Furniture.Children = append(d.Furniture.Children, child)
		}
	case refGroups:
		if target.Idx >= 0 && target.Idx < int64(len(d.Groups)) {
			d.Groups[target.Idx].Children = append(d.Groups[target.Idx].Children, child)
		}
	case refTexts:
		if target.Idx >= 0 && target.Idx < int64(len(d.Texts)) {
			d.Texts[target.Idx].Children = append(d.Texts[target.Idx].Children, child)
		}
	case refTables:
		if target.Idx >= 0 && target.Idx < int64(len(d.Tables)) {
			d.Tables[target.Idx].Children = append(d.Tables[target.Idx].Children, child)
		}
	case refPictures:
		if target.Idx >= 0 && target.Idx < int64(len(d.Pictures)) {
			d.Pictures[target.Idx].Children = append(d.Pictures[target.Idx].Children, child)
		}
	}
}

// SortPageNos 返回升序页号列表（pages 键为字符串化数字）。
func (d *DoclingDocument) SortPageNos() []int64 {
	nums := make([]int64, 0, len(d.Pages))
	for k := range d.Pages {
		var n int64
		if _, err := fmt.Sscanf(k, "%d", &n); err == nil {
			nums = append(nums, n)
		}
	}
	sort.Slice(nums, func(i, j int) bool { return nums[i] < nums[j] })
	return nums
}

// Text 聚合全文档纯文本（按阅读顺序拼接各元素文本），
// 与 ToContentList 后 JoinItemTexts 的结果一致，供调用方取得兜底纯文本。
func (d *DoclingDocument) Text() string {
	return JoinItemTexts(ToContentList(d, ""))
}

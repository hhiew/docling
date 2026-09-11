// hybrid_chunker.go 在层级语义分块之上实现 token 感知的二次切分与合并。
// 调用方可注入与向量模型一致的纯 Go tokenizer 计数函数；未注入时使用
// rune 数作为保守上限，不引入 Python、模型文件或外部服务依赖。
package docling

import (
	"encoding/json"
	"strings"
)

const (
	// DefaultHybridMaxTokens 是 HybridChunks 未显式配置时的 token 上限。
	DefaultHybridMaxTokens = 512
	// hybridChunkDelimiter 对齐 Docling 同标题 peer 合并时的段落分隔。
	hybridChunkDelimiter = "\n\n"
)

// HybridChunkOptions 控制 token 感知分块。
type HybridChunkOptions struct {
	// MaxTokens 是每个“标题上下文 + 正文”的 token 上限；非正数使用 512。
	MaxTokens int
	// CountTokens 返回文本的 token 数，应与下游 embedding 模型一致；nil 时按 rune 数估算。
	CountTokens func(text string) int
	// DisablePeerMerge 关闭同标题下相邻小块的贪心合并。
	DisablePeerMerge bool
	// DisableTableHeaderRepeat 关闭大表切分后的 Markdown 表头重复。
	DisableTableHeaderRepeat bool
}

// normalizedHybridChunkOptions 是已填充默认值的内部选项。
type normalizedHybridChunkOptions struct {
	maxTokens         int              // maxTokens 是上下文化后的硬上限。
	countTokens       func(string) int // countTokens 是已具备异常值回退的计数函数。
	mergePeers        bool             // mergePeers 表示是否合并同标题相邻块。
	repeatTableHeader bool             // repeatTableHeader 表示是否重复表头。
}

// HybridChunks 先调用 HierarchicalChunks 生成文档语义块，再按用户
// tokenizer 切分超限块，最后尽量合并具有相同标题的相邻小块。
func HybridChunks(doc *DoclingDocument, options HybridChunkOptions) []DocChunk {
	base := HierarchicalChunks(doc)
	if len(base) == 0 {
		return nil
	}
	normalized := normalizeHybridChunkOptions(options)
	refined := make([]DocChunk, 0, len(base))
	for _, chunk := range base {
		refined = append(refined, splitHybridChunk(chunk, normalized)...)
	}
	if !normalized.mergePeers {
		return refined
	}
	return mergeHybridPeerChunks(refined, normalized)
}

// ContextualizeDocChunk 把标题路径与正文组合为 embedding 模型应消费的文本。
func ContextualizeDocChunk(chunk DocChunk) string {
	parts := make([]string, 0, len(chunk.Meta.Headings)+1)
	for _, heading := range chunk.Meta.Headings {
		if heading = strings.TrimSpace(heading); heading != "" {
			parts = append(parts, heading)
		}
	}
	if chunk.Text != "" {
		parts = append(parts, chunk.Text)
	}
	return strings.Join(parts, "\n")
}

// normalizeHybridChunkOptions 填充默认预算和保守计数器。
func normalizeHybridChunkOptions(options HybridChunkOptions) normalizedHybridChunkOptions {
	maxTokens := options.MaxTokens
	if maxTokens <= 0 {
		maxTokens = DefaultHybridMaxTokens
	}
	counter := options.CountTokens
	if counter == nil {
		counter = func(text string) int { return len([]rune(text)) }
	}
	safeCounter := func(text string) int {
		count := counter(text)
		if count < 0 {
			return 0
		}
		if count == 0 && text != "" {
			return len([]rune(text))
		}
		return count
	}
	return normalizedHybridChunkOptions{
		maxTokens: maxTokens, countTokens: safeCounter,
		mergePeers: !options.DisablePeerMerge, repeatTableHeader: !options.DisableTableHeaderRepeat,
	}
}

// splitHybridChunk 保留完整元数据地切分一个超限层级块。
func splitHybridChunk(chunk DocChunk, options normalizedHybridChunkOptions) []DocChunk {
	chunk = cloneHybridDocChunk(chunk)
	if options.countTokens(ContextualizeDocChunk(chunk)) <= options.maxTokens {
		return []DocChunk{chunk}
	}
	// 官方 HybridChunker 将标题也计入预算；若标题本身已占满预算，
	// 则对该超长块去掉标题上下文，优先保证正文不丢且硬上限成立。
	emptyBody := cloneHybridDocChunk(chunk)
	emptyBody.Text = ""
	if options.countTokens(ContextualizeDocChunk(emptyBody)) >= options.maxTokens {
		chunk.Meta.Headings = nil
	}
	if options.repeatTableHeader && hybridChunkContainsTable(chunk) {
		if segments := splitHybridTableText(chunk, options); len(segments) > 1 {
			return segments
		}
	}
	texts := splitHybridText(chunk.Text, func(candidate string) bool {
		probe := chunk
		probe.Text = candidate
		return options.countTokens(ContextualizeDocChunk(probe)) <= options.maxTokens
	})
	return hybridChunksWithTexts(chunk, texts)
}

// splitHybridTableText 按 Markdown 行切分单表块，每段重复表头，表前说明只保留在首段。
func splitHybridTableText(chunk DocChunk, options normalizedHybridChunkOptions) []DocChunk {
	lines := strings.Split(chunk.Text, "\n")
	separator := -1
	for index, line := range lines {
		trimmed := strings.TrimSpace(line)
		if index > 0 && strings.HasPrefix(trimmed, "|") && strings.Contains(trimmed, "---") {
			separator = index
			break
		}
	}
	if separator <= 0 || !strings.HasPrefix(strings.TrimSpace(lines[separator-1]), "|") || separator+1 >= len(lines) {
		return nil
	}
	headerStart := separator - 1
	preamble := strings.TrimSpace(strings.Join(lines[:headerStart], "\n"))
	header := strings.Join(lines[headerStart:separator+1], "\n")
	bodyLines := lines[separator+1:]
	makeText := func(first bool, body []string) string {
		prefix := header
		if first && preamble != "" {
			prefix = preamble + "\n\n" + header
		}
		if len(body) == 0 {
			return prefix
		}
		return prefix + "\n" + strings.Join(body, "\n")
	}
	fits := func(text string) bool {
		probe := chunk
		probe.Text = text
		return options.countTokens(ContextualizeDocChunk(probe)) <= options.maxTokens
	}
	if !fits(makeText(false, nil)) || (preamble != "" && !fits(makeText(true, nil))) {
		return nil
	}
	var texts []string
	current := make([]string, 0)
	first := true
	for _, line := range bodyLines {
		candidate := append(append([]string(nil), current...), line)
		if fits(makeText(first, candidate)) {
			current = candidate
			continue
		}
		if len(current) > 0 {
			texts = append(texts, makeText(first, current))
			first = false
			current = nil
		}
		if fits(makeText(first, []string{line})) {
			current = []string{line}
			continue
		}
		// 单行也超限时在行内做硬切；表头仍重复，文本不丢失。
		lineParts := splitHybridText(line, func(part string) bool { return fits(makeText(first, []string{part})) })
		for _, part := range lineParts {
			texts = append(texts, makeText(first, []string{part}))
			first = false
		}
	}
	if len(current) > 0 {
		texts = append(texts, makeText(first, current))
	}
	return hybridChunksWithTexts(chunk, texts)
}

// splitHybridText 先以段落、换行和句末为自然边界贪心切分，单元仍超限时按 rune 二分。
func splitHybridText(text string, fits func(string) bool) []string {
	if text == "" {
		return nil
	}
	if fits(text) {
		return []string{text}
	}
	units := hybridSemanticUnits(text)
	parts := make([]string, 0, len(units))
	current := ""
	for _, unit := range units {
		if fits(current + unit) {
			current += unit
			continue
		}
		if current != "" {
			parts = append(parts, current)
			current = ""
		}
		for unit != "" && !fits(unit) {
			prefix, rest := largestHybridPrefix(unit, fits)
			if prefix == "" {
				runes := []rune(unit)
				prefix, rest = string(runes[:1]), string(runes[1:])
			}
			parts = append(parts, prefix)
			unit = rest
		}
		current = unit
	}
	if current != "" {
		parts = append(parts, current)
	}
	return parts
}

// hybridSemanticUnits 保留原始字符地按句末和换行切出自然单元。
func hybridSemanticUnits(text string) []string {
	var units []string
	start := 0
	runes := []rune(text)
	for index, value := range runes {
		boundary := value == '\n' || strings.ContainsRune("。！？；.!?;", value)
		if !boundary {
			continue
		}
		end := index + 1
		if end > start {
			units = append(units, string(runes[start:end]))
			start = end
		}
	}
	if start < len(runes) {
		units = append(units, string(runes[start:]))
	}
	return units
}

// largestHybridPrefix 二分查找满足 token 上限的最长 rune 前缀。
func largestHybridPrefix(text string, fits func(string) bool) (prefix, rest string) {
	runes := []rune(text)
	low, high, best := 1, len(runes), 0
	for low <= high {
		middle := low + (high-low)/2
		if fits(string(runes[:middle])) {
			best = middle
			low = middle + 1
		} else {
			high = middle - 1
		}
	}
	return string(runes[:best]), string(runes[best:])
}

// hybridChunksWithTexts 为切分后文本深拷贝相同来源元数据。
func hybridChunksWithTexts(base DocChunk, texts []string) []DocChunk {
	chunks := make([]DocChunk, 0, len(texts))
	for _, text := range texts {
		if text == "" {
			continue
		}
		chunk := cloneHybridDocChunk(base)
		chunk.Text = text
		chunks = append(chunks, chunk)
	}
	return chunks
}

// mergeHybridPeerChunks 贪心合并标题上下文一致且组合后不超限的相邻块。
func mergeHybridPeerChunks(chunks []DocChunk, options normalizedHybridChunkOptions) []DocChunk {
	if len(chunks) < 2 {
		return chunks
	}
	result := make([]DocChunk, 0, len(chunks))
	current := cloneHybridDocChunk(chunks[0])
	for _, next := range chunks[1:] {
		if !equalHybridHeadings(current.Meta.Headings, next.Meta.Headings) {
			result = append(result, current)
			current = cloneHybridDocChunk(next)
			continue
		}
		candidate := mergeHybridChunks(current, next)
		if options.countTokens(ContextualizeDocChunk(candidate)) > options.maxTokens {
			result = append(result, current)
			current = cloneHybridDocChunk(next)
			continue
		}
		current = candidate
	}
	return append(result, current)
}

// mergeHybridChunks 合并两个 peer 的文本、引用和页面来源。
func mergeHybridChunks(left, right DocChunk) DocChunk {
	merged := cloneHybridDocChunk(left)
	merged.Text = left.Text + hybridChunkDelimiter + right.Text
	merged.Meta.DocItems = append(merged.Meta.DocItems, cloneHybridRawMessages(right.Meta.DocItems)...)
	merged.Meta.DocRefs = append(merged.Meta.DocRefs, right.Meta.DocRefs...)
	merged.Meta.Provenance = append(merged.Meta.Provenance, cloneChunkProvenance(right.Meta.Provenance)...)
	return merged
}

// equalHybridHeadings 按顺序比较标题上下文。
func equalHybridHeadings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

// hybridChunkContainsTable 通过引用与官方 doc_items 双重判定单表块。
func hybridChunkContainsTable(chunk DocChunk) bool {
	for _, ref := range chunk.Meta.DocRefs {
		if strings.HasPrefix(ref, "#/tables/") {
			return true
		}
	}
	for _, raw := range chunk.Meta.DocItems {
		var header struct {
			Label string `json:"label"`
		}
		if json.Unmarshal(raw, &header) == nil && header.Label == string(LabelTable) {
			return true
		}
	}
	return false
}

// cloneHybridDocChunk 深拷贝 Hybrid 流程会追加的切片和来源对象。
func cloneHybridDocChunk(chunk DocChunk) DocChunk {
	chunk.Meta.DocItems = cloneHybridRawMessages(chunk.Meta.DocItems)
	chunk.Meta.DocRefs = append([]string(nil), chunk.Meta.DocRefs...)
	chunk.Meta.Headings = append([]string(nil), chunk.Meta.Headings...)
	chunk.Meta.Provenance = cloneChunkProvenance(chunk.Meta.Provenance)
	chunk.Meta.Origin = cloneDocumentOrigin(chunk.Meta.Origin)
	return chunk
}

// cloneHybridRawMessages 深拷贝 JSON 元素，避免调用方修改分块时共享底层字节。
func cloneHybridRawMessages(values []json.RawMessage) []json.RawMessage {
	if len(values) == 0 {
		return []json.RawMessage{}
	}
	result := make([]json.RawMessage, len(values))
	for index, value := range values {
		result[index] = append(json.RawMessage(nil), value...)
	}
	return result
}

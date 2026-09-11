// office_collaboration_test.go 验证 DOCX、XLSX 与 PPTX 的现代协作批注、
// 回复链、作者信息和解决状态均可在纯 Go 解析路径中保留。
package docling

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"sort"
	"testing"
)

// commentMetaStringForTest 读取批注节点的字符串元数据。
func commentMetaStringForTest(t *testing.T, meta BaseMeta, key string) string {
	t.Helper()
	var value string
	if err := json.Unmarshal(meta[key], &value); err != nil {
		t.Fatalf("decode comment meta %s: %v, raw=%s", key, err, meta[key])
	}
	return value
}

// commentMetaBoolForTest 读取批注节点的布尔元数据。
func commentMetaBoolForTest(t *testing.T, meta BaseMeta, key string) bool {
	t.Helper()
	var value bool
	if err := json.Unmarshal(meta[key], &value); err != nil {
		t.Fatalf("decode comment meta %s: %v, raw=%s", key, err, meta[key])
	}
	return value
}

// validateOfficeCollaborationDocumentForTest 在显式配置官方 Python 环境时，
// 额外确认带协作批注的文档仍满足 Docling Core 1.10 模型。
func validateOfficeCollaborationDocumentForTest(t *testing.T, name string, doc *DoclingDocument) {
	t.Helper()
	validateDocumentWithDoclingCore110ForTest(t, name, doc)
}

// mustRewriteOOXMLForTest 在已有 OOXML zip 上覆盖或追加测试部件。
func mustRewriteOOXMLForTest(t *testing.T, base []byte, overrides map[string]string) []byte {
	t.Helper()
	reader, err := zip.NewReader(bytes.NewReader(base), int64(len(base)))
	if err != nil {
		t.Fatalf("open base OOXML: %v", err)
	}
	var buf bytes.Buffer
	writer := zip.NewWriter(&buf)
	for _, file := range reader.File {
		if _, replaced := overrides[file.Name]; replaced {
			continue
		}
		rc, err := file.Open()
		if err != nil {
			t.Fatalf("open OOXML entry %s: %v", file.Name, err)
		}
		payload, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			t.Fatalf("read OOXML entry %s: %v", file.Name, err)
		}
		entry, err := writer.Create(file.Name)
		if err != nil {
			t.Fatalf("copy OOXML entry %s: %v", file.Name, err)
		}
		if _, err := entry.Write(payload); err != nil {
			t.Fatalf("write OOXML entry %s: %v", file.Name, err)
		}
	}
	names := make([]string, 0, len(overrides))
	for name := range overrides {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatalf("create OOXML override %s: %v", name, err)
		}
		if _, err := entry.Write([]byte(overrides[name])); err != nil {
			t.Fatalf("write OOXML override %s: %v", name, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close rewritten OOXML: %v", err)
	}
	return buf.Bytes()
}

// TestParseDocxCommentRepliesAndMetadata 验证 commentsExtended.xml 把回复归入
// 同一 comment_section，并保留作者、时间、父批注与已解决状态。
func TestParseDocxCommentRepliesAndMetadata(t *testing.T) {
	documentXML := `<?xml version="1.0" encoding="UTF-8"?>` +
		`<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">` +
		`<w:body><w:p><w:commentRangeStart w:id="7"/><w:r><w:t>需要确认</w:t></w:r>` +
		`<w:commentRangeEnd w:id="7"/><w:r><w:commentReference w:id="7"/></w:r></w:p>` +
		`</w:body></w:document>`
	commentsXML := `<?xml version="1.0" encoding="UTF-8"?>` +
		`<w:comments xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" ` +
		`xmlns:w14="http://schemas.microsoft.com/office/word/2010/wordml">` +
		`<w:comment w:id="7" w:author="审核员" w:initials="SH" w:date="2026-09-09T08:00:00Z">` +
		`<w:p w14:paraId="AAA11111"><w:r><w:t>请补充依据</w:t></w:r></w:p></w:comment>` +
		`<w:comment w:id="8" w:author="撰写人" w:initials="ZX" w:date="2026-09-09T08:05:00Z">` +
		`<w:p w14:paraId="BBB22222"><w:r><w:t>依据已补充</w:t></w:r></w:p></w:comment>` +
		`</w:comments>`
	commentsExtendedXML := `<?xml version="1.0" encoding="UTF-8"?>` +
		`<w15:commentsEx xmlns:w15="http://schemas.microsoft.com/office/word/2012/wordml">` +
		`<w15:commentEx w15:paraId="AAA11111" w15:done="1"/>` +
		`<w15:commentEx w15:paraId="BBB22222" w15:paraIdParent="AAA11111"/>` +
		`</w15:commentsEx>`
	data := mustZipEntriesForTest(t, map[string]string{
		"word/document.xml":         documentXML,
		"word/comments.xml":         commentsXML,
		"word/commentsExtended.xml": commentsExtendedXML,
	})

	doc, err := ParseDocx(data)
	if err != nil {
		t.Fatalf("parse docx collaborative comments: %v", err)
	}
	if len(doc.Groups) != 1 || doc.Groups[0].Label != GroupLabelCommentSection || len(doc.Groups[0].Children) != 2 {
		t.Fatalf("comment thread group = %+v, want one group with root and reply", doc.Groups)
	}
	if len(doc.Texts) != 3 || doc.Texts[1].Text != "请补充依据" || doc.Texts[2].Text != "依据已补充" {
		t.Fatalf("comment thread texts = %+v", doc.Texts)
	}
	root, reply := doc.Texts[1], doc.Texts[2]
	if commentMetaStringForTest(t, root.Meta, commentMetaAuthor) != "审核员" ||
		commentMetaStringForTest(t, root.Meta, commentMetaInitials) != "SH" ||
		commentMetaStringForTest(t, root.Meta, commentMetaCreated) != "2026-09-09T08:00:00Z" ||
		!commentMetaBoolForTest(t, root.Meta, commentMetaResolved) {
		t.Fatalf("root comment metadata = %+v", root.Meta)
	}
	if commentMetaStringForTest(t, reply.Meta, commentMetaParentID) != "7" ||
		commentMetaStringForTest(t, reply.Meta, commentMetaAuthor) != "撰写人" {
		t.Fatalf("reply metadata = %+v", reply.Meta)
	}
	if len(doc.Texts[0].Comments) != 1 || doc.Texts[0].Comments[0].RefItem.String() != doc.Groups[0].SelfRef {
		t.Fatalf("body comment thread reference = %+v", doc.Texts[0].Comments)
	}
	validateOfficeCollaborationDocumentForTest(t, "collaboration.docx", doc)
}

// TestParseXLSXThreadedComments 验证 Excel 365 threaded comments 的人员、
// 回复链、解决状态和单元格锚点均转换为 notes/comment_section。
func TestParseXLSXThreadedComments(t *testing.T) {
	base := mustBuildTestXLSX(t, map[string][][]string{"协作": {{"指标", "数值"}, {"温度", "20"}}})
	data := mustRewriteOOXMLForTest(t, base, map[string]string{
		"xl/worksheets/_rels/sheet1.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
			`<Relationship Id="rIdThread" Type="http://schemas.microsoft.com/office/2017/10/relationships/threadedComment" Target="../threadedComments/threadedComment1.xml"/>` +
			`</Relationships>`,
		"xl/persons/person.xml": `<personList xmlns="http://schemas.microsoft.com/office/spreadsheetml/2018/threadedcomments">` +
			`<person id="{PERSON-1}" displayName="审核员" userId="review@example.com" providerId="PeoplePicker"/>` +
			`<person id="{PERSON-2}" displayName="撰写人" userId="writer@example.com" providerId="PeoplePicker"/>` +
			`</personList>`,
		"xl/threadedComments/threadedComment1.xml": `<ThreadedComments xmlns="http://schemas.microsoft.com/office/spreadsheetml/2018/threadedcomments">` +
			`<threadedComment ref="A2" dT="2026-09-09T09:00:00Z" personId="{PERSON-1}" id="{COMMENT-1}" done="1"><text>@撰写人 数值是否准确</text>` +
			`<mentions><mention mentionpersonId="{PERSON-2}" mentionId="{MENTION-1}" startIndex="0" length="4"/></mentions></threadedComment>` +
			`<threadedComment ref="A2" dT="2026-09-09T09:05:00Z" personId="{PERSON-2}" id="{COMMENT-2}" parentId="{COMMENT-1}"><text>已经复核</text></threadedComment>` +
			`</ThreadedComments>`,
	})

	doc, err := ParseXLSX(data)
	if err != nil {
		t.Fatalf("parse xlsx threaded comments: %v", err)
	}
	var thread *GroupItem
	for i := range doc.Groups {
		if doc.Groups[i].Label == GroupLabelCommentSection {
			thread = &doc.Groups[i]
			break
		}
	}
	if thread == nil || len(thread.Children) != 2 {
		t.Fatalf("xlsx comment thread = %+v", thread)
	}
	root := doc.Texts[thread.Children[0].Idx]
	reply := doc.Texts[thread.Children[1].Idx]
	if root.Text != "@撰写人 数值是否准确" || commentMetaStringForTest(t, root.Meta, commentMetaAuthor) != "审核员" ||
		commentMetaStringForTest(t, root.Meta, commentMetaUserID) != "review@example.com" ||
		!commentMetaBoolForTest(t, root.Meta, commentMetaResolved) {
		t.Fatalf("xlsx root comment = %+v", root)
	}
	var mentions []officeCommentMention
	if err := json.Unmarshal(root.Meta[commentMetaMentions], &mentions); err != nil || len(mentions) != 1 ||
		mentions[0].PersonID != "{PERSON-2}" || mentions[0].Start != 0 || mentions[0].Length != 4 {
		t.Fatalf("xlsx comment mentions = %+v, err=%v", mentions, err)
	}
	if reply.Text != "已经复核" || commentMetaStringForTest(t, reply.Meta, commentMetaParentID) != "{COMMENT-1}" {
		t.Fatalf("xlsx reply comment = %+v", reply)
	}
	if len(doc.Tables) != 1 || len(doc.Tables[0].Comments) != 1 || doc.Tables[0].Comments[0].RefItem.String() != thread.SelfRef {
		t.Fatalf("xlsx table thread reference missing: tables=%+v thread=%+v", doc.Tables, thread)
	}
	validateOfficeCollaborationDocumentForTest(t, "collaboration.xlsx", doc)
}

// TestParsePPTXModernCommentReplies 验证 PowerPoint 现代批注的 author part、
// replyLst、状态和坐标关联，无需 LibreOffice 或 Python。
func TestParsePPTXModernCommentReplies(t *testing.T) {
	data := mustZipEntriesForTest(t, map[string]string{
		"ppt/presentation.xml": `<p:presentation ` + pptxTestXMLNS + `><p:sldIdLst><p:sldId id="256" r:id="rId1"/></p:sldIdLst><p:sldSz cx="1000" cy="800"/></p:presentation>`,
		"ppt/_rels/presentation.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
			`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="slides/slide1.xml"/>` +
			`<Relationship Id="rIdAuthors" Type="http://schemas.microsoft.com/office/2018/10/relationships/authors" Target="authors/author1.xml"/>` +
			`</Relationships>`,
		"ppt/slides/slide1.xml": `<p:sld ` + pptxTestXMLNS + `><p:cSld><p:spTree>` +
			`<p:sp><p:nvSpPr><p:cNvPr id="2" name="正文"/><p:cNvSpPr/><p:nvPr/></p:nvSpPr>` +
			`<p:spPr><a:xfrm><a:off x="200" y="300"/><a:ext cx="100" cy="50"/></a:xfrm></p:spPr>` +
			`<p:txBody><a:bodyPr/><a:p><a:r><a:t>需要评审</a:t></a:r></a:p></p:txBody></p:sp>` +
			`</p:spTree></p:cSld></p:sld>`,
		"ppt/slides/_rels/slide1.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
			`<Relationship Id="rIdComments" Type="http://schemas.microsoft.com/office/2018/10/relationships/comments" Target="../comments/comment1.xml"/>` +
			`</Relationships>`,
		"ppt/authors/author1.xml": `<p188:authorLst xmlns:p188="http://schemas.microsoft.com/office/powerpoint/2018/8/main">` +
			`<p188:author id="{AUTHOR-1}" name="审核员" initials="SH" userId="review@example.com" providerId="PeoplePicker"/>` +
			`<p188:author id="{AUTHOR-2}" name="撰写人" initials="ZX" userId="writer@example.com" providerId="PeoplePicker"/>` +
			`</p188:authorLst>`,
		"ppt/comments/comment1.xml": `<p188:cmLst xmlns:p188="http://schemas.microsoft.com/office/powerpoint/2018/8/main" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">` +
			`<p188:cm id="{COMMENT-1}" authorId="{AUTHOR-1}" created="2026-09-09T10:00:00Z" status="resolved" ` +
			`assignedTo="{AUTHOR-2}" startDate="2026-09-09T00:00:00Z" dueDate="2026-09-10T00:00:00Z" complete="100%" title="版式评审">` +
			`<p188:unknownAnchor/><p188:pos x="220" y="310"/><p188:replyLst>` +
			`<p188:reply id="{COMMENT-2}" authorId="{AUTHOR-2}" created="2026-09-09T10:05:00Z" status="active">` +
			`<p188:txBody><a:bodyPr/><a:p><a:r><a:t>已经调整</a:t></a:r></a:p></p188:txBody></p188:reply>` +
			`</p188:replyLst><p188:txBody><a:bodyPr/><a:p><a:r><a:t>请调整此处</a:t></a:r></a:p></p188:txBody></p188:cm>` +
			`</p188:cmLst>`,
	})

	doc, err := ParsePPTX(data)
	if err != nil {
		t.Fatalf("parse pptx modern comments: %v", err)
	}
	var body *TextItem
	var thread *GroupItem
	for i := range doc.Texts {
		if doc.Texts[i].Text == "需要评审" {
			body = &doc.Texts[i]
		}
	}
	for i := range doc.Groups {
		if doc.Groups[i].Label == GroupLabelCommentSection {
			thread = &doc.Groups[i]
		}
	}
	if body == nil || thread == nil || len(thread.Children) != 2 {
		t.Fatalf("pptx body/thread missing: body=%+v groups=%+v", body, doc.Groups)
	}
	root := doc.Texts[thread.Children[0].Idx]
	reply := doc.Texts[thread.Children[1].Idx]
	if root.Text != "请调整此处" || commentMetaStringForTest(t, root.Meta, commentMetaAuthor) != "审核员" ||
		!commentMetaBoolForTest(t, root.Meta, commentMetaResolved) {
		t.Fatalf("pptx root comment = %+v", root)
	}
	if commentMetaStringForTest(t, root.Meta, commentMetaCompletion) != "100%" ||
		commentMetaStringForTest(t, root.Meta, commentMetaTitle) != "版式评审" {
		t.Fatalf("pptx task comment metadata = %+v", root.Meta)
	}
	if reply.Text != "已经调整" || commentMetaStringForTest(t, reply.Meta, commentMetaParentID) != "{COMMENT-1}" ||
		commentMetaStringForTest(t, reply.Meta, commentMetaAuthor) != "撰写人" {
		t.Fatalf("pptx reply comment = %+v", reply)
	}
	if len(body.Comments) != 1 || body.Comments[0].RefItem.String() != thread.SelfRef {
		t.Fatalf("pptx body thread reference = %+v", body.Comments)
	}
	validateOfficeCollaborationDocumentForTest(t, "collaboration.pptx", doc)
}

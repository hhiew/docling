// xlsx_collaboration.go 解析 Excel 365 threaded comments 与 persons 部件，
// 将回复链转换为 notes 层 comment_section，并保留单元格锚点和协作元数据。
package docparse

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"path"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"
)

// xlsxThreadedComment 保存一个 Excel 现代批注或回复。
type xlsxThreadedComment struct {
	ID       string
	ParentID string
	Cell     string
	PersonID string
	Created  string
	Resolved *bool
	Text     string
	Mentions []officeCommentMention
}

// parseXLSXCommentAuthors 从 workbook 的 person 关系或标准部件路径读取人员表。
func parseXLSXCommentAuthors(reader *zip.Reader) map[string]officeCommentAuthor {
	authors := map[string]officeCommentAuthor{}
	if reader == nil {
		return authors
	}
	personPath := "xl/persons/person.xml"
	relsXML, _ := readZipFileBytes(reader, "xl/_rels/workbook.xml.rels")
	for _, target := range parseOOXMLRelationships(relsXML, "/person", "xl") {
		personPath = target
		break
	}
	data, _ := readZipFileBytes(reader, personPath)
	dec := xml.NewDecoder(bytes.NewReader(data))
	for {
		token, err := dec.Token()
		if err != nil {
			return authors
		}
		start, ok := token.(xml.StartElement)
		if !ok || start.Name.Local != "person" {
			continue
		}
		author := officeCommentAuthor{
			ID: xmlAttrVal(start.Attr, "id"), Name: xmlAttrVal(start.Attr, "displayName"),
			UserID: xmlAttrVal(start.Attr, "userId"), ProviderID: xmlAttrVal(start.Attr, "providerId"),
		}
		if author.ID != "" {
			authors[author.ID] = author
		}
	}
}

// parseXLSXThreadedComments 读取一个 worksheet 对应的 threadedComment 部件。
func parseXLSXThreadedComments(reader *zip.Reader, sheetPart string) []xlsxThreadedComment {
	if reader == nil || sheetPart == "" {
		return nil
	}
	relsPath := path.Join(path.Dir(sheetPart), "_rels", path.Base(sheetPart)+".rels")
	relsXML, _ := readZipFileBytes(reader, relsPath)
	targets := parseOOXMLRelationships(relsXML, "/threadedComment", path.Dir(sheetPart))
	var comments []xlsxThreadedComment
	for _, target := range targets {
		data, _ := readZipFileBytes(reader, target)
		comments = append(comments, parseXLSXThreadedCommentPart(data)...)
	}
	return comments
}

// parseXLSXThreadedCommentPart 解析 ThreadedComments XML，未知扩展安全忽略。
func parseXLSXThreadedCommentPart(data []byte) []xlsxThreadedComment {
	dec := xml.NewDecoder(bytes.NewReader(data))
	var comments []xlsxThreadedComment
	for {
		token, err := dec.Token()
		if err != nil {
			return comments
		}
		start, ok := token.(xml.StartElement)
		if !ok || start.Name.Local != "threadedComment" {
			continue
		}
		raw, err := collectSubTree(dec, start)
		if err != nil {
			return comments
		}
		comment := xlsxThreadedComment{
			ID: xmlAttrVal(start.Attr, "id"), ParentID: xmlAttrVal(start.Attr, "parentId"),
			Cell: xmlAttrVal(start.Attr, "ref"), PersonID: xmlAttrVal(start.Attr, "personId"),
			Created: xmlAttrVal(start.Attr, "dT"), Resolved: officeResolvedStatus(xmlAttrVal(start.Attr, "done")),
		}
		comment.Text, comment.Mentions = parseXLSXThreadedCommentBody(raw)
		if comment.ID != "" && strings.TrimSpace(comment.Text) != "" {
			comments = append(comments, comment)
		}
	}
}

// parseXLSXThreadedCommentBody 提取现代批注正文及 @mention 字符范围。
func parseXLSXThreadedCommentBody(raw []byte) (string, []officeCommentMention) {
	dec := xml.NewDecoder(bytes.NewReader(raw))
	text := ""
	var mentions []officeCommentMention
	for {
		token, err := dec.Token()
		if err != nil {
			return strings.TrimSpace(text), mentions
		}
		switch value := token.(type) {
		case xml.StartElement:
			switch value.Name.Local {
			case "text":
				var part string
				if err := dec.DecodeElement(&part, &value); err == nil {
					text += part
				}
			case "mention":
				start, _ := strconv.ParseInt(xmlAttrVal(value.Attr, "startIndex"), 10, 64)
				length, _ := strconv.ParseInt(xmlAttrVal(value.Attr, "length"), 10, 64)
				mentions = append(mentions, officeCommentMention{
					PersonID: xmlAttrVal(value.Attr, "mentionpersonId"), MentionID: xmlAttrVal(value.Attr, "mentionId"),
					Start: start, Length: length,
				})
			}
		}
	}
}

// appendXLSXThreadedComments 把现代批注按根批注分组，并把表格关联到整个回复链。
func appendXLSXThreadedComments(doc *DoclingDocument, reader *zip.Reader, part xlsxSheetPart, pageNo int64, parent RefItem, layer ContentLayer, width, height float64) (float64, float64, map[string]bool) {
	comments := parseXLSXThreadedComments(reader, part.path)
	commentedCells := map[string]bool{}
	if len(comments) == 0 {
		return width, height, commentedCells
	}
	authors := parseXLSXCommentAuthors(reader)
	byID := make(map[string]xlsxThreadedComment, len(comments))
	for _, comment := range comments {
		byID[comment.ID] = comment
		if comment.Cell != "" {
			commentedCells[comment.Cell] = true
		}
	}
	rootID := func(id string) string {
		origin := id
		seen := map[string]bool{}
		for id != "" && !seen[id] {
			comment, exists := byID[id]
			if !exists {
				return origin
			}
			seen[id] = true
			if comment.ParentID == "" {
				return id
			}
			id = comment.ParentID
		}
		return origin
	}
	groups := map[string]RefItem{}
	for _, comment := range comments {
		root := rootID(comment.ID)
		if _, exists := groups[root]; exists {
			continue
		}
		rootComment := byID[root]
		group := GroupItem{
			Parent: &parent, Children: []RefItem{}, ContentLayer: LayerNotes,
			Label: GroupLabelCommentSection, Name: "comment-" + root,
			Meta: officeCommentMeta(officeCommentData{
				ID: rootComment.ID, Author: authors[rootComment.PersonID], Created: rootComment.Created,
				Resolved: rootComment.Resolved, Cell: rootComment.Cell,
			}),
		}
		if layer == LayerInvisible {
			group.ContentLayer = LayerInvisible
		}
		doc.Groups = append(doc.Groups, group)
		groupRef := RefItem{Kind: refGroups, Idx: int64(len(doc.Groups) - 1)}
		doc.Groups[groupRef.Idx].SelfRef = groupRef.String()
		doc.appendChild(&parent, groupRef)
		groups[root] = groupRef
	}
	linkedRoots := map[string]bool{}
	for _, comment := range comments {
		groupRef := groups[rootID(comment.ID)]
		cell := comment.Cell
		if cell == "" && comment.ParentID != "" {
			cell = byID[rootID(comment.ID)].Cell
		}
		col, row, err := excelCellCoordinates(cell)
		var prov []ProvenanceItem
		if err == nil {
			prov = []ProvenanceItem{xlsxCellProvenance(pageNo, row, col, row+1, col+1)}
			if float64(col+1) > width {
				width = float64(col + 1)
			}
			if float64(row+1) > height {
				height = float64(row + 1)
			}
		}
		noteRef := doc.AddText(LabelText, strings.TrimSpace(comment.Text), prov, &groupRef)
		doc.Texts[noteRef.Idx].ContentLayer = doc.Groups[groupRef.Idx].ContentLayer
		doc.Texts[noteRef.Idx].Meta = officeCommentMeta(officeCommentData{
			ID: comment.ID, ParentID: comment.ParentID, Author: authors[comment.PersonID],
			Created: comment.Created, Resolved: comment.Resolved, Cell: cell, Mentions: comment.Mentions,
		})
		root := rootID(comment.ID)
		if linkedRoots[root] || err != nil {
			continue
		}
		for tableIndex := range doc.Tables {
			if xlsxTableContainsCell(&doc.Tables[tableIndex], pageNo, row, col) {
				doc.Tables[tableIndex].Comments = append(doc.Tables[tableIndex].Comments, FineRef{RefItem: groupRef})
				linkedRoots[root] = true
				break
			}
		}
	}
	return width, height, commentedCells
}

// excelCellCoordinates 把 A1 坐标转换为 0 起行列，非法坐标返回错误。
func excelCellCoordinates(cell string) (int, int, error) {
	col, row, err := excelize.CellNameToCoordinates(cell)
	return col - 1, row - 1, err
}

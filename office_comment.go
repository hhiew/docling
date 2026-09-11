// office_comment.go 定义 Office 协作批注在 Docling 节点 meta 中的稳定扩展键，
// 并集中完成作者、回复、时间和解决状态的 JSON 编码。
package docling

import (
	"encoding/json"
	"strings"
)

const (
	commentMetaID         = "docling__comment_id"          // 批注或回复的源文件标识。
	commentMetaParentID   = "docling__comment_parent_id"   // 回复所指向的父批注标识。
	commentMetaAuthorID   = "docling__comment_author_id"   // OOXML 作者/人员标识。
	commentMetaAuthor     = "docling__comment_author"      // 作者显示名称。
	commentMetaInitials   = "docling__comment_initials"    // 作者缩写。
	commentMetaUserID     = "docling__comment_user_id"     // Office 协作用户标识。
	commentMetaProviderID = "docling__comment_provider_id" // 人员信息提供方标识。
	commentMetaCreated    = "docling__comment_created"     // OOXML 原始创建时间。
	commentMetaStatus     = "docling__comment_status"      // active/resolved/closed 等源状态。
	commentMetaResolved   = "docling__comment_resolved"    // 归一化后的是否已解决状态。
	commentMetaCell       = "docling__comment_cell"        // XLSX 批注锚定单元格。
	commentMetaMentions   = "docling__comment_mentions"    // XLSX 批注内提及人员及字符范围。
	commentMetaAssignedTo = "docling__comment_assigned_to" // PPTX 任务型批注的负责人列表。
	commentMetaStartDate  = "docling__comment_start_date"  // PPTX 任务型批注开始时间。
	commentMetaDueDate    = "docling__comment_due_date"    // PPTX 任务型批注截止时间。
	commentMetaCompletion = "docling__comment_completion"  // PPTX 任务型批注完成比例。
	commentMetaTitle      = "docling__comment_title"       // PPTX 任务型批注标题。
)

// officeCommentAuthor 保存 OOXML 人员部件中的协作身份信息。
type officeCommentAuthor struct {
	ID         string
	Name       string
	Initials   string
	UserID     string
	ProviderID string
}

// officeCommentMention 保存 threaded comment 内一次 @ 提及的人员与字符范围。
type officeCommentMention struct {
	PersonID  string `json:"person_id"`
	MentionID string `json:"mention_id"`
	Start     int64  `json:"start"`
	Length    int64  `json:"length"`
}

// officeCommentData 是三种 Office 后端共享的批注元数据中间表示。
type officeCommentData struct {
	ID         string
	ParentID   string
	Author     officeCommentAuthor
	Created    string
	Status     string
	Resolved   *bool
	Cell       string
	Mentions   []officeCommentMention
	AssignedTo []string
	StartDate  string
	DueDate    string
	Completion string
	Title      string
}

// officeCommentMeta 把协作批注中间表示编码为可随 Docling JSON 无损往返的 meta。
func officeCommentMeta(comment officeCommentData) BaseMeta {
	meta := BaseMeta{}
	putString := func(key, value string) {
		if value == "" {
			return
		}
		data, err := json.Marshal(value)
		if err == nil {
			meta[key] = data
		}
	}
	putString(commentMetaID, comment.ID)
	putString(commentMetaParentID, comment.ParentID)
	putString(commentMetaAuthorID, comment.Author.ID)
	putString(commentMetaAuthor, comment.Author.Name)
	putString(commentMetaInitials, comment.Author.Initials)
	putString(commentMetaUserID, comment.Author.UserID)
	putString(commentMetaProviderID, comment.Author.ProviderID)
	putString(commentMetaCreated, comment.Created)
	putString(commentMetaStatus, comment.Status)
	putString(commentMetaCell, comment.Cell)
	putString(commentMetaStartDate, comment.StartDate)
	putString(commentMetaDueDate, comment.DueDate)
	putString(commentMetaCompletion, comment.Completion)
	putString(commentMetaTitle, comment.Title)
	if comment.Resolved != nil {
		if data, err := json.Marshal(*comment.Resolved); err == nil {
			meta[commentMetaResolved] = data
		}
	}
	if len(comment.Mentions) > 0 {
		if data, err := json.Marshal(comment.Mentions); err == nil {
			meta[commentMetaMentions] = data
		}
	}
	if len(comment.AssignedTo) > 0 {
		if data, err := json.Marshal(comment.AssignedTo); err == nil {
			meta[commentMetaAssignedTo] = data
		}
	}
	return meta
}

// officeResolvedStatus 把 Office 状态字符串归一化为布尔值；未知状态不输出。
func officeResolvedStatus(status string) *bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "resolved", "closed", "done", "1", "true", "on":
		value := true
		return &value
	case "active", "open", "0", "false", "off":
		value := false
		return &value
	default:
		return nil
	}
}

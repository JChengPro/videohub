package moderation

import (
	"backend/internal/mq"
	"backend/internal/video"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Handler struct {
	db      *gorm.DB
	preview func(*gin.Context, *video.Video) error
}

func NewHandler(db *gorm.DB, preview func(*gin.Context, *video.Video) error) *Handler {
	return &Handler{db: db, preview: preview}
}

func (h *Handler) List(c *gin.Context) {
	var input struct {
		Status string `json:"status"`
		Query  string `json:"query"`
		Offset int    `json:"offset"`
		Limit  int    `json:"limit"`
	}
	if c.ShouldBindJSON(&input) != nil {
		c.JSON(400, gin.H{"error": "查询参数错误"})
		return
	}
	if input.Status == "" {
		input.Status = video.VideoStatusPendingReview
	}
	switch input.Status {
	case video.VideoStatusPendingReview, video.VideoStatusPublished, video.VideoStatusRejected, video.VideoStatusProcessing, video.VideoStatusFailed, video.VideoStatusDeleted:
	default:
		c.JSON(400, gin.H{"error": "不支持的状态"})
		return
	}
	if input.Offset < 0 || utf8.RuneCountInString(input.Query) > 100 {
		c.JSON(400, gin.H{"error": "查询参数错误"})
		return
	}
	if input.Limit <= 0 || input.Limit > 50 {
		input.Limit = 20
	}
	query := h.db.WithContext(c.Request.Context()).Model(&video.Video{}).Where("status = ?", input.Status)
	if q := strings.TrimSpace(input.Query); q != "" {
		query = query.Where("LOCATE(?, title) > 0 OR LOCATE(?, username) > 0", q, q)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		c.JSON(500, gin.H{"error": "无法读取审核列表"})
		return
	}
	items := []video.Video{}
	order := "id DESC"
	if input.Status == video.VideoStatusPendingReview {
		order = "id ASC"
	}
	if err := query.Order(order).Offset(input.Offset).Limit(input.Limit).Find(&items).Error; err != nil {
		c.JSON(500, gin.H{"error": "无法读取审核列表"})
		return
	}
	// List responses intentionally omit media URLs; detail grants a short preview ticket.
	for i := range items {
		items[i].PlayURL = ""
		items[i].CoverURL = ""
	}
	c.JSON(200, gin.H{"items": items, "total": total})
}

func (h *Handler) Detail(c *gin.Context) {
	var input video.DetailRequest
	if c.ShouldBindJSON(&input) != nil || input.ID == 0 {
		c.JSON(400, gin.H{"error": "视频编号无效"})
		return
	}
	var target video.Video
	if h.db.WithContext(c.Request.Context()).First(&target, input.ID).Error != nil {
		c.JSON(404, gin.H{"error": "视频不存在"})
		return
	}
	if h.preview != nil && target.Status != video.VideoStatusDeleted {
		if err := h.preview(c, &target); err != nil {
			c.JSON(500, gin.H{"error": "无法加载视频预览"})
			return
		}
	}
	reviews := []Review{}
	if err := h.db.WithContext(c.Request.Context()).Where("video_id = ?", target.ID).Order("id DESC").Find(&reviews).Error; err != nil {
		c.JSON(500, gin.H{"error": "无法读取审核记录"})
		return
	}
	c.JSON(200, gin.H{"video": target, "reviews": reviews})
}

var errConflict = errors.New("video already reviewed or withdrawn")

func (h *Handler) Decide(c *gin.Context) {
	var input struct {
		ID       uint   `json:"id"`
		Decision string `json:"decision"`
		Reason   string `json:"reason"`
	}
	if c.ShouldBindJSON(&input) != nil || input.ID == 0 || (input.Decision != "approve" && input.Decision != "reject") {
		c.JSON(400, gin.H{"error": "审核参数错误"})
		return
	}
	input.Reason = strings.TrimSpace(input.Reason)
	if utf8.RuneCountInString(input.Reason) > 500 || (input.Decision == "reject" && input.Reason == "") {
		c.JSON(400, gin.H{"error": "驳回时必须填写原因，最多 500 字"})
		return
	}
	err := h.staffTransaction(c, false, func(tx *gorm.DB, admin Staff) error {
		var target video.Video
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&target, input.ID).Error; err != nil {
			return err
		}
		if target.Status != video.VideoStatusPendingReview {
			return errConflict
		}
		status := video.VideoStatusRejected
		updates := map[string]any{"review_reason": input.Reason}
		now := time.Now().Truncate(time.Millisecond)
		if input.Decision == "approve" {
			status = video.VideoStatusPublished
			updates["published_at"] = now
		}
		updates["status"] = status
		if err := tx.Model(&target).Updates(updates).Error; err != nil {
			return err
		}
		if err := tx.Create(&Review{VideoID: target.ID, ReviewerID: admin.ID, ReviewerName: admin.Username, Decision: input.Decision, Reason: input.Reason}).Error; err != nil {
			return err
		}
		if status == video.VideoStatusPublished {
			eventID := video.NewEventID("video_published")
			event := mq.VideoPublishedEvent{EventID: eventID, EventType: "video_published", VideoID: target.ID, AuthorID: target.AuthorID, Title: target.Title, PlayObjectKey: target.PlayObjectKey, CoverObjectKey: target.CoverObjectKey, CreateTime: now.UnixMilli()}
			msg, err := video.NewOutboxMsg(mq.VideoPublishedQueueName, eventID, event, event.EventType, target.ID, target.AuthorID, target.Title)
			if err != nil {
				return err
			}
			if err := tx.Create(msg).Error; err != nil {
				return err
			}
		}
		content := "你的作品《" + target.Title + "》已通过审核"
		if status == video.VideoStatusRejected {
			content = "你的作品未通过审核：" + input.Reason
		}
		if utf8.RuneCountInString(content) > 500 {
			content = string([]rune(content)[:500])
		}
		eventID := video.NewEventID("notification_review")
		event := mq.NotificationEvent{EventID: eventID, ReceiverID: target.AuthorID, ActorID: 0, Type: "review", TargetType: "submission", TargetID: target.ID, Content: content, DedupKey: fmt.Sprintf("review:%d", target.ID)}
		msg, err := video.NewOutboxMsg(mq.NotificationQueueName, eventID, event, "notification_review", target.ID, target.AuthorID, target.Title)
		if err != nil {
			return err
		}
		return tx.Create(msg).Error
	})
	if errors.Is(err, errConflict) {
		c.JSON(409, gin.H{"error": "该视频已被审核或撤回，请刷新列表"})
		return
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(404, gin.H{"error": "视频不存在"})
		return
	}
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(200, gin.H{"message": "审核结果已保存"})
}

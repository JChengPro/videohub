package video

import (
	"backend/internal/mq"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// /video/publish 发布视频时写 videos 表
func (r *Repository) Create(ctx context.Context, video *Video) error {
	return r.db.WithContext(ctx).Create(video).Error
}

func newOutboxMsg(queueName string, eventID string, payload any, eventType string, videoID, authorID uint, title string) (*OutboxMsg, error) {
	if queueName == "" {
		return nil, errors.New("outbox queue_name is required")
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return &OutboxMsg{
		EventID:   eventID,
		EventType: eventType,
		QueueName: queueName,
		Payload:   string(b),
		VideoID:   videoID,
		AuthorID:  authorID,
		Title:     title,
		Status:    "pending",
	}, nil
}

// 生成唯一的eventID
func newEventID(eventType string) string {
	return fmt.Sprintf("%s:%d", eventType, time.Now().UnixNano())
}

// NewEventID 暴露给其他业务包，用于生成与视频事件相同格式的 Outbox 事件 ID。
func NewEventID(eventType string) string {
	return newEventID(eventType)
}

// NewOutboxMsg 暴露统一的 Outbox 构造逻辑，供关注等其他业务事务复用。
func NewOutboxMsg(queueName string, eventID string, payload any, eventType string, videoID, authorID uint, title string) (*OutboxMsg, error) {
	return newOutboxMsg(queueName, eventID, payload, eventType, videoID, authorID, title)
}

// CreateWithOutbox = 发布视频时使用的事务方法。   先不直接发 MQ，而是把“要发的消息”可靠地写进 MySQL。
func (r *Repository) CreateWithOutbox(ctx context.Context, video *Video) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(video).Error; err != nil {
			return err
		}

		eventID := newEventID("video_published")

		event := mq.VideoPublishedEvent{
			EventID:        eventID,
			EventType:      "video_published",
			VideoID:        video.ID,
			AuthorID:       video.AuthorID,
			Title:          video.Title,
			PlayObjectKey:  video.PlayObjectKey,
			CoverObjectKey: video.CoverObjectKey,
			CreateTime:     video.CreateTime.UnixMilli(),
		}
		msg, err := newOutboxMsg(mq.VideoPublishedQueueName, eventID, event, event.EventType, video.ID, video.AuthorID, video.Title)
		if err != nil {
			return err
		}
		if err := tx.Create(msg).Error; err != nil {
			return err
		}
		return nil
	})
}

func (r *Repository) DeleteWithOutbox(ctx context.Context, video *Video) error {
	if video == nil {
		return errors.New("video is nil")
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 使用状态删除保留视频记录，便于审计、恢复以及后续清理实际文件
		result := tx.Model(&Video{}).
			Where("id = ? AND status = ?", video.ID, VideoStatusPublished).
			Update("status", VideoStatusDeleted)

		if result.Error != nil {
			return result.Error
		}

		// RowsAffected 为 0，表示视频已经删除或不处于 published 状态。
		// 此时不重复创建 video_deleted Outbox 消息。
		if result.RowsAffected == 0 {
			return nil
		}

		eventID := newEventID("video_deleted")

		event := mq.VideoPublishedEvent{
			EventID:        eventID,
			EventType:      "video_deleted",
			VideoID:        video.ID,
			AuthorID:       video.AuthorID,
			Title:          video.Title,
			PlayObjectKey:  video.PlayObjectKey,
			CoverObjectKey: video.CoverObjectKey,
		}
		msg, err := newOutboxMsg(mq.VideoPublishedQueueName, eventID, event, event.EventType, video.ID, video.AuthorID, video.Title)
		if err != nil {
			return err
		}
		return tx.Create(msg).Error
	})
}

// /video/detail 查询视频详情
func (r *Repository) FindByID(ctx context.Context, id uint) (*Video, error) {
	var video Video
	if err := r.db.WithContext(ctx).First(&video, id).Error; err != nil {
		return nil, err
	}
	return &video, nil
}

// FindPublishedByID 只查询允许公开展示和互动的视频
func (r *Repository) FindPublishedByID(ctx context.Context, id uint) (*Video, error) {
	var video Video
	if err := r.db.WithContext(ctx).Where("id = ? AND status = ?", id, VideoStatusPublished).First(&video).Error; err != nil {
		return nil, err
	}
	return &video, nil
}

// ExistPublishedByID 判断视频是否存在且允许互动。
func (r *Repository) ExistPublishedByID(ctx context.Context, videoID uint) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&Video{}).
		Where("id = ? AND status = ?", videoID, VideoStatusPublished).
		Count(&count).
		Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// lockPublishedVideo 在事务中锁定已发布视频，防止互动过程中视频被并发删除
func findPublishedVideoForUpdate(tx *gorm.DB, videoID uint) (*Video, error) {
	var video Video

	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Select("id", "author_id", "title").
		Where("id = ? AND status = ?", videoID, VideoStatusPublished).
		First(&video).Error; err != nil {
		return nil, err
	}
	return &video, nil
}

func lockPublishedVideo(tx *gorm.DB, videoID uint) error {
	_, err := findPublishedVideoForUpdate(tx, videoID)
	return err
}

// /video/listByAuthor 查询作者视频列表
func (r *Repository) ListByAuthorID(ctx context.Context, authorID uint) ([]Video, error) {
	var videos []Video
	// 个人主页不会展示已删除视频
	if err := r.db.WithContext(ctx).Where("author_id = ? AND status = ?", authorID, VideoStatusPublished).Order("create_time desc").Find(&videos).Error; err != nil {
		return nil, err
	}
	return videos, nil
}

func (r *Repository) CommentCount(ctx context.Context, videoID uint) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&Comment{}).
		Where("video_id = ?", videoID).
		Count(&count).
		Error
	return count, err
}

func (r *Repository) CommentCounts(ctx context.Context, videoIDs []uint) (map[uint]int64, error) {
	counts := make(map[uint]int64, len(videoIDs))
	if len(videoIDs) == 0 {
		return counts, nil
	}

	var rows []struct {
		VideoID uint  `gorm:"column:video_id"`
		Count   int64 `gorm:"column:count"`
	}
	err := r.db.WithContext(ctx).
		Model(&Comment{}).
		Select("video_id, COUNT(*) AS count").
		Where("video_id IN ?", videoIDs).
		Group("video_id").
		Scan(&rows).
		Error
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		counts[row.VideoID] = row.Count
	}
	return counts, nil
}

// 已经无用 点赞事务版里暂时不需要通过 videoRepo.ChangeLikesCount() 来更新点赞数 点赞数更新直接写在 like_repo.go 的事务里
func (r *Repository) ChangeLikesCount(ctx context.Context, videoID uint, delta int64) error {
	return r.db.WithContext(ctx).Model(&Video{}).
		Where("id=?", videoID).UpdateColumn("likes_count", gorm.Expr("GREATEST(likes_count + ?,0)", delta)).Error
}

func (r *Repository) DeleteByID(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&Video{}, id).Error
}

func (r *Repository) ChangePopularity(ctx context.Context, videoID uint, delta int64) error {
	return r.db.WithContext(ctx).
		Model(&Video{}).Where("id = ?", videoID).
		UpdateColumn("popularity", gorm.Expr("GREATEST(popularity + ?, 0)", delta)).Error
}

// ChangePopularityOnce 按 event_id 做消费幂等
// 同一个 event_id + consumerName 只能成功更新一次 popularity
func (r *Repository) ChangePopularityOnce(ctx context.Context, eventID string, consumerName string, videoID uint, delta int64) (bool, error) {
	if eventID == "" || consumerName == "" || videoID == 0 || delta == 0 {
		return false, errors.New("event_id, consumer_name, video_id and delta are required")
	}

	processed := false

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 1. 先插入消费记录，唯一键冲突表示这个消费已经处理过该事件
		if err := tx.Create(&ConsumedEvent{
			EventID:      eventID,
			ConsumerName: consumerName,
		}).Error; err != nil {
			var mysqlErr *mysql.MySQLError
			if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
				processed = false
				return nil
			}
			return err
		}

		// 2. 只有消费记录插入成功，才能更新视频热度
		// 这一步和上面的消费记录必须在同一个事务中
		result := tx.Model(&Video{}).Where("id = ? AND status = ?", videoID, VideoStatusPublished).
			UpdateColumn("popularity", gorm.Expr("GREATEST(popularity + ?, 0)", delta))

		if result.Error != nil {
			return result.Error
		}

		if result.RowsAffected == 0 {
			// 视频已经删除，该延迟事件不需要继续处理。
			// 返回 nil 提交 consumed_events，避免 RabbitMQ 无限重试。
			processed = false
			return nil
		}

		processed = true
		return nil
	})
	return processed, err
}

// GetPopularity 查询视频当前的最终热度值。
// Worker 用它把 Redis 热榜覆盖成 MySQL 的最终值，避免重复消费导致 Redis 分数重复累加。
func (r *Repository) GetPopularity(ctx context.Context, videoID uint) (int64, error) {
	var popularity int64
	// Pluck 是只查某一个字段
	result := r.db.WithContext(ctx).Model(&Video{}).Where("id = ? AND status = ?", videoID, VideoStatusPublished).Pluck("popularity", &popularity)

	if result.Error != nil {
		return 0, result.Error
	}
	if result.RowsAffected == 0 {
		return 0, gorm.ErrRecordNotFound
	}

	return popularity, nil
}

// 现在发布视频时已经会往 outbox_msgs 表插入  但是还没有任何代码去读取它、发送 MQ、修改状态。
func (r *Repository) ListPendingOutbox(ctx context.Context, limit int) ([]OutboxMsg, error) {
	if limit <= 0 {
		limit = 20
	}

	var messages []OutboxMsg
	err := r.db.WithContext(ctx).Where("status = ?", "pending").Order("create_time asc").Limit(limit).Find(&messages).Error
	return messages, err
}

func (r *Repository) TryMarkOutboxPublishing(ctx context.Context, id uint) (bool, error) {
	result := r.db.WithContext(ctx).Model(&OutboxMsg{}).Where("id = ? AND status = ?", id, "pending").Update("status", "publishing")
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected == 1, nil
}

// MQ 发送成功后，把这条 outbox 消息标记为 published。
func (r *Repository) MarkOutboxPublished(ctx context.Context, id uint) error {
	//只有被当前 poller 抢到、处于 publishing 的消息，才能改成 published。
	return r.db.WithContext(ctx).Model(&OutboxMsg{}).
		Where("id = ? AND status = ?", id, "publishing").
		Updates(map[string]any{
			"status":     "published",
			"last_error": "",
		}).Error
}

// MQ 发送失败时，不删除消息，也不改成 published。只记录失败次数和失败原因。这样下次 poller 还能继续扫描 pending 消息重试。
func (r *Repository) RecordOutboxPublishFailure(ctx context.Context, id uint, publishErr error) error {
	message := ""
	if publishErr != nil {
		message = strings.TrimSpace(publishErr.Error())
	}
	if len(message) > 500 {
		message = message[:500]
	}

	// 只有当前状态是 publishing 的消息，发送失败后才能退回 pending。
	return r.db.WithContext(ctx).Model(&OutboxMsg{}).
		Where("id = ? AND status = ?", id, "publishing").
		Updates(map[string]any{
			"status":      "pending",
			"retry_count": gorm.Expr("retry_count + 1"),
			"last_error":  message,
		}).Error
}

func (r *Repository) ResetStalePublishingOutbox(ctx context.Context, timeout time.Duration) error {
	if timeout <= 0 {
		timeout = time.Minute
	}

	deadline := time.Now().Add(-timeout)

	//找出 update_time 早于 11:59:00（时间点，这个之前的说明存在一分钟了） 的 publishing 消息
	return r.db.WithContext(ctx).Model(&OutboxMsg{}).
		Where("status = ? AND update_time < ?", "publishing", deadline).
		Update("status", "pending").Error
}

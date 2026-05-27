package video

import (
	"backend/internal/mq"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
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

func newOutboxMsg(queueName string, payload any, eventType string, videoID, authorID uint, title string) (*OutboxMsg, error) {
	if queueName == "" {
		return nil, errors.New("outbox queue_name is required")
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return &OutboxMsg{
		EventType: eventType,
		QueueName: queueName,
		Payload:   string(b),
		VideoID:   videoID,
		AuthorID:  authorID,
		Title:     title,
		Status:    "pending",
	}, nil
}

// CreateWithOutbox = 发布视频时使用的事务方法。   先不直接发 MQ，而是把“要发的消息”可靠地写进 MySQL。
func (r *Repository) CreateWithOutbox(ctx context.Context, video *Video) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(video).Error; err != nil {
			return err
		}

		event := mq.VideoPublishedEvent{
			EventType:  "video_published",
			VideoID:    video.ID,
			AuthorID:   video.AuthorID,
			Title:      video.Title,
			CreateTime: video.CreateTime.UnixMilli(),
		}
		msg, err := newOutboxMsg(mq.VideoPublishedQueueName, event, event.EventType, video.ID, video.AuthorID, video.Title)
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
		if err := tx.Delete(&Video{}, video.ID).Error; err != nil {
			return err
		}

		event := mq.VideoPublishedEvent{
			EventType: "video_deleted",
			VideoID:   video.ID,
			AuthorID:  video.AuthorID,
			Title:     video.Title,
		}
		msg, err := newOutboxMsg(mq.VideoPublishedQueueName, event, event.EventType, video.ID, video.AuthorID, video.Title)
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

// /video/listByAuthor 查询作者视频列表
func (r *Repository) ListByAuthorID(ctx context.Context, authorID uint) ([]Video, error) {
	var videos []Video
	if err := r.db.WithContext(ctx).Where("author_id = ?", authorID).Order("create_time desc").Find(&videos).Error; err != nil {
		return nil, err
	}
	return videos, nil
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

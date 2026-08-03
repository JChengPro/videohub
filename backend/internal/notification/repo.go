package notification

import (
	"context"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// CreateOnce 使用 dedup_key 唯一索引保证重复 MQ 消息不会生成重复通知。
// 返回值表示本次是否真正插入，实时推送只针对新通知发送。
func (r *Repository) CreateOnce(ctx context.Context, n *Notification) (bool, error) {
	result := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{DoNothing: true}).
		Create(n)
	return result.RowsAffected > 0, result.Error
}

func (r *Repository) List(ctx context.Context, receiverID uint, notificationType string, limit int, beforeID uint) ([]Notification, error) {
	var notifications []Notification
	query := r.db.WithContext(ctx).
		Model(&Notification{}).
		Select("notifications.*, accounts.username AS actor_username").
		Joins("LEFT JOIN accounts ON accounts.id = notifications.actor_id").
		Where("notifications.receiver_id = ?", receiverID).
		Order("notifications.id DESC").
		Limit(limit)

	if notificationType != "" {
		query = query.Where("notifications.type = ?", notificationType)
	}
	if beforeID > 0 {
		query = query.Where("notifications.id < ?", beforeID)
	}

	return notifications, query.Find(&notifications).Error
}

func (r *Repository) UnreadCount(ctx context.Context, receiverID uint) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&Notification{}).
		Where("receiver_id = ? AND is_read = ?", receiverID, false).
		Count(&count).Error
	return count, err
}

func (r *Repository) MarkRead(ctx context.Context, receiverID, id uint) error {
	now := time.Now()
	return r.db.WithContext(ctx).
		Model(&Notification{}).
		Where("id = ? AND receiver_id = ?", id, receiverID).
		Updates(map[string]any{"is_read": true, "read_time": &now}).Error
}

func (r *Repository) MarkAllRead(ctx context.Context, receiverID uint) error {
	now := time.Now()
	return r.db.WithContext(ctx).
		Model(&Notification{}).
		Where("receiver_id = ? AND is_read = ?", receiverID, false).
		Updates(map[string]any{"is_read": true, "read_time": &now}).Error
}

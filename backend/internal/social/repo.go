package social

import (
	"context"

	"backend/internal/account"
	"backend/internal/mq"
	"backend/internal/video"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Follow(ctx context.Context, relation *Social) error {
	return r.db.WithContext(ctx).Create(relation).Error
}

// FollowWithNotificationOutbox 将关注关系和通知事件放在同一个事务中。
func (r *Repository) FollowWithNotificationOutbox(ctx context.Context, relation *Social) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(relation)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return nil
		}

		eventID := video.NewEventID("notification_social_followed")
		event := mq.NotificationEvent{
			EventID:    eventID,
			Type:       "follow",
			ReceiverID: relation.VloggerID,
			ActorID:    relation.FollowerID,
			TargetType: "account",
			TargetID:   relation.FollowerID,
			Content:    "关注了你",
			DedupKey:   eventID,
		}
		msg, err := video.NewOutboxMsg(
			mq.NotificationQueueName,
			eventID,
			event,
			"notification_social_followed",
			0,
			relation.FollowerID,
			"",
		)
		if err != nil {
			return err
		}
		return tx.Create(msg).Error
	})
}

func (r *Repository) Unfollow(ctx context.Context, followerID uint, vloggerID uint) error {
	return r.db.WithContext(ctx).
		Where("follower_id = ? AND vlogger_id = ?", followerID, vloggerID).
		Delete(&Social{}).
		Error
}

func (r *Repository) IsFollowing(ctx context.Context, followerID uint, vloggerID uint) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).
		Model(&Social{}).
		Where("follower_id = ? AND vlogger_id = ?", followerID, vloggerID).
		Count(&count).
		Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// ListFollowers：查“谁关注了我”   vlogger_id：被关注者
func (r *Repository) ListFollowers(ctx context.Context, vloggerID uint) ([]account.Account, error) {
	var followers []account.Account
	if err := r.db.WithContext(ctx).
		Model(&account.Account{}).
		Joins("JOIN socials ON socials.follower_id = accounts.id").
		Where("socials.vlogger_id = ?", vloggerID).
		Find(&followers).
		Error; err != nil {
		return nil, err
	}
	return followers, nil
}

// ListFollowing：查“我关注了谁”
func (r *Repository) ListFollowing(ctx context.Context, followerID uint) ([]account.Account, error) {
	var following []account.Account
	if err := r.db.WithContext(ctx).
		Model(&account.Account{}).
		Joins("JOIN socials ON socials.vlogger_id = accounts.id").
		Where("socials.follower_id = ?", followerID).
		Find(&following).
		Error; err != nil {
		return nil, err
	}
	return following, nil
}

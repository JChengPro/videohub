package video

import (
	"backend/internal/mq"
	"context"

	"gorm.io/gorm"
)

type LikeRepository struct {
	db *gorm.DB
}

func NewLikeRepository(db *gorm.DB) *LikeRepository {
	return &LikeRepository{db: db}
}

func (r *LikeRepository) Create(ctx context.Context, like *Like) error {
	return r.db.WithContext(ctx).Create(like).Error
}

/*
Delete(&Like{}) 删除的是啥？

	它删除的是 likes 表里的点赞关系记录，不是删除 videos.likes_count。
	对应 SQL 大概是：
	DELETE FROM likes
	WHERE video_id = ? AND account_id = ?;
	比如：
	video_id = 1
	account_id = 3

	意思是：
	删除“用户 3 点赞了视频 1”这条记录
	它不会自动修改：
	videos.likes_count
	所以取消点赞完整逻辑应该是两步：

	1. 删除 likes 表中的点赞关系
	2. videos.likes_count - 1
*/
func (r *LikeRepository) Delete(ctx context.Context, videoID uint, accountID uint) error {
	return r.db.WithContext(ctx).
		Where("video_id=? AND account_id=?", videoID, accountID).
		Delete(&Like{}).Error
}

func (r *LikeRepository) IsLiked(ctx context.Context, videoID uint, accountID uint) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&Like{}).
		Where("video_id = ? AND account_id = ?", videoID, accountID).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// 用户点赞和数据库视频点赞数量的增加是同一个事务，一组数据库操作，要么全部成功，要么全部失败
func (r *LikeRepository) LikeWithTx(ctx context.Context, like *Like) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(like).Error; err != nil {
			return err
		}
		if err := tx.Model(&Video{}).Where("id = ?", like.VideoID).
			UpdateColumn("likes_count", gorm.Expr("GREATEST(likes_count + ?,0)", 1)).Error; err != nil {
			return err
		}
		return nil
	})
}

func (r *LikeRepository) LikeWithTxAndOutbox(ctx context.Context, like *Like) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(like).Error; err != nil {
			return err
		}
		if err := tx.Model(&Video{}).Where("id = ?", like.VideoID).
			UpdateColumn("likes_count", gorm.Expr("GREATEST(likes_count + ?,0)", 1)).Error; err != nil {
			return err
		}

		event := mq.LikeEvent{
			EventType: "like_created",
			VideoID:   like.VideoID,
			AccountID: like.AccountID,
		}
		msg, err := newOutboxMsg(mq.LikeQueueName, event, event.EventType, like.VideoID, like.AccountID, "")
		if err != nil {
			return err
		}
		return tx.Create(msg).Error
	})
}

func (r *LikeRepository) UnlikeWithTx(ctx context.Context, videoID, accountID uint) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("video_id = ? AND account_id = ?", videoID, accountID).Delete(&Like{}).Error; err != nil {
			return err
		}
		if err := tx.Model(&Video{}).Where("id = ?", videoID).
			UpdateColumn("likes_count", gorm.Expr("GREATEST(likes_count + ?,0)", -1)).Error; err != nil {
			return err
		}
		return nil
	})
}

func (r *LikeRepository) UnlikeWithTxAndOutbox(ctx context.Context, videoID, accountID uint) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("video_id = ? AND account_id = ?", videoID, accountID).Delete(&Like{}).Error; err != nil {
			return err
		}
		if err := tx.Model(&Video{}).Where("id = ?", videoID).
			UpdateColumn("likes_count", gorm.Expr("GREATEST(likes_count + ?,0)", -1)).Error; err != nil {
			return err
		}

		event := mq.LikeEvent{
			EventType: "like_deleted",
			VideoID:   videoID,
			AccountID: accountID,
		}
		msg, err := newOutboxMsg(mq.LikeQueueName, event, event.EventType, videoID, accountID, "")
		if err != nil {
			return err
		}
		return tx.Create(msg).Error
	})
}

// 查询用户点过赞所以的视频
func (r *LikeRepository) ListLikedVideos(ctx context.Context, accountID uint) ([]Video, error) {
	var videos []Video

	if err := r.db.WithContext(ctx).
		Model(&Video{}).Joins("JOIN likes ON likes.video_id = videos.id").
		Where("likes.account_id = ?", accountID).Order("likes.created_at desc").Find(&videos).Error; err != nil {
		return nil, err
	}
	return videos, nil
}

func (r *LikeRepository) LikedVideoIDs(ctx context.Context, accountID uint, videoIDs []uint) (map[uint]bool, error) {
	if accountID == 0 || len(videoIDs) == 0 {
		return nil, nil
	}
	var liked []uint
	if err := r.db.WithContext(ctx).Model(&Like{}).
		Where("account_id = ? AND video_id IN ?", accountID, videoIDs).
		Pluck("video_id", &liked).Error; err != nil {
		return nil, err
	}
	set := make(map[uint]bool, len(liked))
	for _, id := range liked {
		set[id] = true
	}
	return set, nil
}

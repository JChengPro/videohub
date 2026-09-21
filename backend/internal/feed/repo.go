package feed

import (
	"context"
	"time"

	"backend/internal/video"

	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) PublishedIDs(ctx context.Context, ids []uint) (map[uint]bool, error) {
	var found []uint
	err := r.db.WithContext(ctx).Model(&video.Video{}).Where("id IN ? AND status = ?", ids, video.VideoStatusPublished).Pluck("id", &found).Error
	result := make(map[uint]bool, len(found))
	for _, id := range found {
		result[id] = true
	}
	return result, err
}

// 不是传统的分页逻辑，而是给我某个时间点之前的最新 N 条数据
func (r *Repository) ListLatest(ctx context.Context, limit int, latestBefore time.Time) ([]*video.Video, error) {
	var videos []*video.Video

	query := r.db.WithContext(ctx).
		Model(&video.Video{}).
		Where("status = ?", video.VideoStatusPublished).
		Order("published_at DESC")

	if !latestBefore.IsZero() {
		query = query.Where("published_at < ?", latestBefore)
	}

	if err := query.Limit(limit).Find(&videos).Error; err != nil {
		return nil, err
	}

	return videos, nil
}

func (r *Repository) ListFollowing(ctx context.Context, accountID uint, limit int, before int64) ([]*video.Video, error) {
	var videos []*video.Video

	query := r.db.WithContext(ctx).
		Model(&video.Video{}).
		Joins("JOIN socials ON socials.vlogger_id = videos.author_id").
		Where("socials.follower_id = ? AND videos.status = ?", accountID, video.VideoStatusPublished).
		Order("videos.published_at desc").
		Limit(limit)

	if before > 0 {
		query = query.Where("videos.published_at < FROM_UNIXTIME(? / 1000)", before)
	}

	if err := query.Find(&videos).Error; err != nil {
		return nil, err
	}

	return videos, nil
}

func (r *Repository) ListHot(ctx context.Context, limit int) ([]*video.Video, error) {
	var videos []*video.Video
	if err := r.db.WithContext(ctx).
		Where("status = ?", video.VideoStatusPublished).
		Order("popularity desc, create_time desc, id desc").Limit(limit).
		Find(&videos).Error; err != nil {
		return nil, err
	}
	return videos, nil
}

func (r *Repository) GetByIDs(ctx context.Context, ids []uint) ([]*video.Video, error) {
	var videos []*video.Video
	if len(ids) == 0 {
		return videos, nil
	}

	if err := r.db.WithContext(ctx).
		Model(&video.Video{}).
		Where("id IN ? AND status = ?", ids, video.VideoStatusPublished).
		Find(&videos).Error; err != nil {
		return nil, err
	}

	return videos, nil
}

func (r *Repository) ListByLikesCount(ctx context.Context, limit int, likesCountBefore int64, idBefore uint) ([]*video.Video, error) {
	var videos []*video.Video
	query := r.db.WithContext(ctx).
		Where("status = ?", video.VideoStatusPublished).
		Order("likes_count desc, id desc").Limit(limit)
	if likesCountBefore > 0 || idBefore > 0 {
		query = query.Where("(likes_count, id) < (?, ?)", likesCountBefore, idBefore)
	}
	if err := query.Find(&videos).Error; err != nil {
		return nil, err
	}
	return videos, nil
}

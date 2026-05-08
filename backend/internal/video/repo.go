package video

import (
	"context"

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

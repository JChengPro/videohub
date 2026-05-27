package video

import (
	"backend/internal/mq"
	"context"

	"gorm.io/gorm"
)

type CommentRepository struct {
	db *gorm.DB
}

func NewCommentRepository(db *gorm.DB) *CommentRepository {
	return &CommentRepository{db: db}
}

func (r *CommentRepository) Create(ctx context.Context, comment *Comment) error {
	return r.db.WithContext(ctx).Create(comment).Error
}

func (r *CommentRepository) CreateWithOutbox(ctx context.Context, comment *Comment) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(comment).Error; err != nil {
			return err
		}

		event := mq.CommentEvent{
			EventType: "comment_published",
			CommentID: comment.ID,
			VideoID:   comment.VideoID,
			AuthorID:  comment.AuthorID,
			Username:  comment.Username,
			Content:   comment.Content,
		}
		msg, err := newOutboxMsg(mq.CommentQueueName, event, event.EventType, comment.VideoID, comment.AuthorID, "")
		if err != nil {
			return err
		}
		return tx.Create(msg).Error
	})
}

func (r *CommentRepository) ListByVideoID(ctx context.Context, videoID uint) ([]Comment, error) {
	var comments []Comment
	if err := r.db.WithContext(ctx).Where("video_id = ?", videoID).Order("created_at desc").Find(&comments).Error; err != nil {
		return nil, err
	}
	return comments, nil
}

// 通过CommentID查找对应的评论
func (r *CommentRepository) FindByID(ctx context.Context, commentID uint) (*Comment, error) {
	var comment Comment
	if err := r.db.WithContext(ctx).First(&comment, commentID).Error; err != nil {
		return nil, err
	}
	return &comment, nil
}

func (r *CommentRepository) Delete(ctx context.Context, commentID uint) error {
	return r.db.WithContext(ctx).Delete(&Comment{}, commentID).Error
}

func (r *CommentRepository) DeleteWithOutbox(ctx context.Context, comment *Comment) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(&Comment{}, comment.ID).Error; err != nil {
			return err
		}

		event := mq.CommentEvent{
			EventType: "comment_deleted",
			CommentID: comment.ID,
			VideoID:   comment.VideoID,
			AuthorID:  comment.AuthorID,
			Username:  comment.Username,
			Content:   comment.Content,
		}
		msg, err := newOutboxMsg(mq.CommentQueueName, event, event.EventType, comment.VideoID, comment.AuthorID, "")
		if err != nil {
			return err
		}
		return tx.Create(msg).Error
	})
}

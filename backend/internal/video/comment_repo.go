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

		// 锁定已发布视频，防止发表评论过程中视频被并发删除。
		targetVideo, err := findPublishedVideoForUpdate(tx, comment.VideoID)
		if err != nil {
			return err
		}

		if err := tx.Create(comment).Error; err != nil {
			return err
		}

		// 文件顶部写的是同一个 package 名，就属于同一个包。同一个包里的函数、结构体、变量可以互相使用，不
		// 要求写在同一个文件里。
		eventID := newEventID("comment_published")

		event := mq.CommentEvent{
			EventID:   eventID,
			EventType: "comment_published",
			CommentID: comment.ID,
			VideoID:   comment.VideoID,
			AuthorID:  comment.AuthorID,
			Username:  comment.Username,
			Content:   comment.Content,
		}
		msg, err := newOutboxMsg(mq.CommentQueueName, eventID, event, event.EventType, comment.VideoID, comment.AuthorID, "")
		if err != nil {
			return err
		}
		if err := tx.Create(msg).Error; err != nil {
			return err
		}

		if targetVideo.AuthorID == comment.AuthorID {
			return nil
		}

		notificationEventID := newEventID("notification_comment_published")
		notificationEvent := mq.NotificationEvent{
			EventID:    notificationEventID,
			Type:       "comment",
			ReceiverID: targetVideo.AuthorID,
			ActorID:    comment.AuthorID,
			TargetType: "video",
			TargetID:   comment.VideoID,
			Content:    comment.Content,
			DedupKey:   notificationEventID,
		}
		notificationMsg, err := newOutboxMsg(
			mq.NotificationQueueName,
			notificationEventID,
			notificationEvent,
			"notification_comment_published",
			comment.VideoID,
			comment.AuthorID,
			targetVideo.Title,
		)
		if err != nil {
			return err
		}
		return tx.Create(notificationMsg).Error
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

		eventID := newEventID("comment_deleted")

		event := mq.CommentEvent{
			EventID:   eventID,
			EventType: "comment_deleted",
			CommentID: comment.ID,
			VideoID:   comment.VideoID,
			AuthorID:  comment.AuthorID,
			Username:  comment.Username,
			Content:   comment.Content,
		}
		msg, err := newOutboxMsg(mq.CommentQueueName, eventID, event, event.EventType, comment.VideoID, comment.AuthorID, "")
		if err != nil {
			return err
		}
		return tx.Create(msg).Error
	})
}

package video

import (
	"backend/internal/mq"
	"context"
	"errors"
	"log"
	"strings"
)

type CommentService struct {
	repo   *CommentRepository
	rabbit *mq.RabbitMQ
}

func NewCommentService(repo *CommentRepository, rabbit *mq.RabbitMQ) *CommentService {
	return &CommentService{repo: repo, rabbit: rabbit}
}

func (s *CommentService) Publish(ctx context.Context, comment *Comment) error {
	if comment == nil {
		return errors.New("comment is nil")
	}

	comment.Content = strings.TrimSpace(comment.Content)

	if comment.VideoID == 0 || comment.AuthorID == 0 {
		return errors.New("video_id and author_id are required")
	}
	if comment.Content == "" {
		return errors.New("content is required")
	}
	if len(comment.Content) > 500 {
		return errors.New("content is too long")
	}

	// 同步写入 DB，拿到 ID 后立即返回给前端
	if err := s.repo.Create(ctx, comment); err != nil {
		return err
	}

	// 异步：热度更新交给 Worker
	if s.rabbit != nil {
		event := mq.CommentEvent{
			EventType: "comment_published",
			VideoID:   comment.VideoID,
			AuthorID:  comment.AuthorID,
		}
		if err := s.rabbit.DeclareQueue(mq.CommentQueueName); err != nil {
			log.Printf("declare comment queue failed: %v", err)
		} else if err := s.rabbit.PublishJSON(ctx, mq.CommentQueueName, event); err != nil {
			log.Printf("publish comment event failed: %v", err)
		}
	}
	return nil
}

func (s *CommentService) ListByVideoID(ctx context.Context, videoID uint) ([]Comment, error) {
	if videoID == 0 {
		return nil, errors.New("video_id is required")
	}
	return s.repo.ListByVideoID(ctx, videoID)
}

func (s *CommentService) Delete(ctx context.Context, commentID uint, accountID uint) error {
	if commentID == 0 || accountID == 0 {
		return errors.New("comment_id and account_id are required")
	}

	comment, err := s.repo.FindByID(ctx, commentID)
	if err != nil {
		return err
	}
	if comment.AuthorID != accountID {
		return errors.New("unauthorized")
	}

	// 同步删除
	if err := s.repo.Delete(ctx, commentID); err != nil {
		return err
	}

	// 异步：热度更新交给 Worker
	if s.rabbit != nil {
		event := mq.CommentEvent{
			EventType: "comment_deleted",
			VideoID:   comment.VideoID,
		}
		if err := s.rabbit.DeclareQueue(mq.CommentQueueName); err != nil {
			log.Printf("declare comment queue failed: %v", err)
		} else if err := s.rabbit.PublishJSON(ctx, mq.CommentQueueName, event); err != nil {
			log.Printf("publish comment delete event failed: %v", err)
		}
	}

	return nil
}

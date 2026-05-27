package video

import (
	"backend/internal/mq"
	"context"
	"errors"
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

	// 同步写入 DB，并在同一个事务里写 outbox，后续由 poller 可靠投递 MQ。
	if err := s.repo.CreateWithOutbox(ctx, comment); err != nil {
		return err
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

	// 同步删除，并在同一个事务里写 outbox，后续由 poller 可靠投递 MQ。
	if err := s.repo.DeleteWithOutbox(ctx, comment); err != nil {
		return err
	}

	return nil
}

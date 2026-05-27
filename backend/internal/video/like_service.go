package video

import (
	"backend/internal/mq"
	"context"
	"errors"
)

type LikeService struct {
	likeRepo  *LikeRepository
	videoRepo *Repository
	rabbit    *mq.RabbitMQ
}

func NewLikeService(likeRepo *LikeRepository, videoRepo *Repository, rabbit *mq.RabbitMQ) *LikeService {
	return &LikeService{likeRepo: likeRepo, videoRepo: videoRepo, rabbit: rabbit}
}

func (s *LikeService) Like(ctx context.Context, videoID uint, accountID uint) error {
	if videoID == 0 || accountID == 0 {
		return errors.New("video_id and account_id are required")
	}

	liked, err := s.likeRepo.IsLiked(ctx, videoID, accountID)
	if err != nil {
		return err
	}
	if liked {
		return nil
	}

	// 同步写入 DB，并在同一个事务里写 outbox，后续由 poller 可靠投递 MQ。
	if err := s.likeRepo.LikeWithTxAndOutbox(ctx, &Like{
		VideoID:   videoID,
		AccountID: accountID,
	}); err != nil {
		return err
	}

	return nil
}

func (s *LikeService) Unlike(ctx context.Context, videoID, accountID uint) error {
	if videoID == 0 || accountID == 0 {
		return errors.New("video_id and account_id are required")
	}

	liked, err := s.likeRepo.IsLiked(ctx, videoID, accountID)
	if err != nil {
		return err
	}
	if !liked {
		return nil
	}

	// 同步写入 DB，并在同一个事务里写 outbox，后续由 poller 可靠投递 MQ。
	if err := s.likeRepo.UnlikeWithTxAndOutbox(ctx, videoID, accountID); err != nil {
		return err
	}

	return nil
}

func (s *LikeService) IsLiked(ctx context.Context, videoID, accountID uint) (bool, error) {
	if videoID == 0 || accountID == 0 {
		return false, errors.New("video_id and account_id are required")
	}
	return s.likeRepo.IsLiked(ctx, videoID, accountID)
}

func (s *LikeService) ListLikedVideos(ctx context.Context, accountID uint) ([]Video, error) {
	if accountID == 0 {
		return nil, errors.New("account_id is requred")
	}
	return s.likeRepo.ListLikedVideos(ctx, accountID)
}

package video

import (
	"backend/internal/mq"
	"backend/internal/storage"
	"context"
	"errors"
)

type LikeService struct {
	likeRepo  *LikeRepository
	videoRepo *Repository
	rabbit    *mq.RabbitMQ
	storage   storage.Storage
}

func NewLikeService(likeRepo *LikeRepository, videoRepo *Repository, rabbit *mq.RabbitMQ, fileStorage storage.Storage) *LikeService {
	return &LikeService{likeRepo: likeRepo, videoRepo: videoRepo, rabbit: rabbit, storage: fileStorage}
}

func (s *LikeService) Like(ctx context.Context, videoID uint, accountID uint) (LikeStateResponse, error) {
	if videoID == 0 || accountID == 0 {
		return LikeStateResponse{}, errors.New("video_id and account_id are required")
	}

	exists, err := s.videoRepo.ExistPublishedByID(ctx, videoID)
	if err != nil {
		return LikeStateResponse{}, err
	}
	if !exists {
		return LikeStateResponse{}, errors.New("video not found")
	}

	// 同步写入 DB，并在同一个事务里写 outbox，后续由 poller 可靠投递 MQ。
	likesCount, err := s.likeRepo.LikeWithTxAndOutbox(ctx, &Like{
		VideoID:   videoID,
		AccountID: accountID,
	})
	if err != nil {
		return LikeStateResponse{}, err
	}
	return LikeStateResponse{IsLiked: true, LikesCount: likesCount}, nil
}

func (s *LikeService) Unlike(ctx context.Context, videoID, accountID uint) (LikeStateResponse, error) {
	if videoID == 0 || accountID == 0 {
		return LikeStateResponse{}, errors.New("video_id and account_id are required")
	}

	exists, err := s.videoRepo.ExistPublishedByID(ctx, videoID)
	if err != nil {
		return LikeStateResponse{}, err
	}
	if !exists {
		return LikeStateResponse{}, errors.New("video not found")
	}

	// 同步写入 DB，并在同一个事务里写 outbox，后续由 poller 可靠投递 MQ。
	likesCount, err := s.likeRepo.UnlikeWithTxAndOutbox(ctx, videoID, accountID)
	if err != nil {
		return LikeStateResponse{}, err
	}
	return LikeStateResponse{IsLiked: false, LikesCount: likesCount}, nil
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
	videos, err := s.likeRepo.ListLikedVideos(ctx, accountID)
	if err != nil {
		return nil, err
	}
	for i := range videos {
		if err := RefreshAccessURLs(ctx, s.storage, &videos[i]); err != nil {
			return nil, err
		}
	}
	return videos, nil
}

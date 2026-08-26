package video

import (
	"backend/internal/cache"
	"backend/internal/storage"
	"context"
	"errors"
	"fmt"
	"log"
)

type LikeService struct {
	likeRepo  *LikeRepository
	videoRepo *Repository
	storage   storage.Storage
	cache     *cache.Client
}

func NewLikeService(likeRepo *LikeRepository, videoRepo *Repository, fileStorage storage.Storage, cacheClient *cache.Client) *LikeService {
	return &LikeService{likeRepo: likeRepo, videoRepo: videoRepo, storage: fileStorage, cache: cacheClient}
}

func (s *LikeService) invalidateVideoCaches(ctx context.Context, videoID uint) {
	if s.cache == nil || videoID == 0 {
		return
	}
	if err := s.cache.Del(
		ctx,
		fmt.Sprintf("video:detail:id=%d", videoID),
		fmt.Sprintf("video:entity:%d", videoID),
	); err != nil {
		// The database transaction is already committed. Keep the request successful;
		// the worker will retry cache invalidation from the outbox event.
		log.Printf("invalidate like caches failed: video_id=%d err=%v", videoID, err)
	}
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
	s.invalidateVideoCaches(ctx, videoID)
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
	s.invalidateVideoCaches(ctx, videoID)
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

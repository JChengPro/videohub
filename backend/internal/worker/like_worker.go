package worker

import (
	"backend/internal/cache"
	"backend/internal/mq"
	"backend/internal/video"
	"context"
	"fmt"
	"log"
)

type LikeWorker struct {
	videoRepo *video.Repository
	cache     *cache.Client
}

func NewLikeWorker(videoRepo *video.Repository, cacheClient *cache.Client) *LikeWorker {
	return &LikeWorker{
		videoRepo: videoRepo,
		cache:     cacheClient,
	}
}

func (w *LikeWorker) deleteVideoDetailCache(ctx context.Context, videoID uint) error {
	if w.cache == nil || videoID == 0 {
		return nil
	}
	cacheKey := fmt.Sprintf("video:detail:id=%d", videoID)
	return w.cache.Del(ctx, cacheKey)
}

func (w *LikeWorker) updateHotRanking(ctx context.Context, videoID uint, delta int64) error {
	if w.cache == nil || videoID == 0 || delta == 0 {
		return nil
	}
	return w.cache.ZIncrBy(ctx, "feed:hot:zset", videoID, delta)
}

func (w *LikeWorker) HandleLikeCreated(ctx context.Context, event mq.LikeEvent) error {
	log.Printf("handle like created: video_id=%d", event.VideoID)

	if event.VideoID == 0 {
		return nil
	}
	if err := w.videoRepo.ChangePopularity(ctx, event.VideoID, 1); err != nil {
		return err
	}
	if err := w.updateHotRanking(ctx, event.VideoID, 1); err != nil {
		return err
	}
	return w.deleteVideoDetailCache(ctx, event.VideoID)
}

func (w *LikeWorker) HandleLikeDeleted(ctx context.Context, event mq.LikeEvent) error {
	log.Printf("handle like deleted: video_id=%d", event.VideoID)

	if event.VideoID == 0 {
		return nil
	}
	if err := w.videoRepo.ChangePopularity(ctx, event.VideoID, -1); err != nil {
		return err
	}
	if err := w.updateHotRanking(ctx, event.VideoID, -1); err != nil {
		return err
	}
	return w.deleteVideoDetailCache(ctx, event.VideoID)
}

package worker

import (
	"backend/internal/cache"
	"backend/internal/mq"
	"backend/internal/video"
	"context"
	"errors"
	"log"
)

type CommentWorker struct {
	videoRepo *video.Repository
	cache     *cache.Client
}

func NewCommentWorker(videoRepo *video.Repository, cacheClient *cache.Client) *CommentWorker {
	return &CommentWorker{videoRepo: videoRepo, cache: cacheClient}
}

func (w *CommentWorker) HandleCommentPublished(ctx context.Context, event mq.CommentEvent) error {
	log.Printf("handle comment published: video_id=%d", event.VideoID)

	if event.VideoID == 0 {
		return errors.New("video_id is required")
	}
	if err := w.videoRepo.ChangePopularity(ctx, event.VideoID, 2); err != nil {
		return err
	}
	return w.updateHotRanking(ctx, event.VideoID, 2)
}

func (w *CommentWorker) HandleCommentDeleted(ctx context.Context, event mq.CommentEvent) error {
	log.Printf("handle comment deleted: video_id=%d", event.VideoID)

	if event.VideoID == 0 {
		return errors.New("video_id is required")
	}
	if err := w.videoRepo.ChangePopularity(ctx, event.VideoID, -2); err != nil {
		return err
	}
	return w.updateHotRanking(ctx, event.VideoID, -2)
}

func (w *CommentWorker) updateHotRanking(ctx context.Context, videoID uint, delta int64) error {
	if w.cache == nil || videoID == 0 || delta == 0 {
		return nil
	}
	return w.cache.ZIncrBy(ctx, "feed:hot:zset", videoID, delta)
}

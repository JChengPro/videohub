package worker

import (
	"backend/internal/cache"
	"backend/internal/mq"
	"backend/internal/video"
	"context"
	"errors"
	"gorm.io/gorm"
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
	// 评论发布会增加视频热度，必须按 event_id 做幂等。
	// 同一条 comment_published 消息重复投递时，只允许第一次更新 MySQL popularity。
	processed, err := w.videoRepo.ChangePopularityOnce(ctx, event.EventID, "comment_popularity", event.VideoID, 2)
	if err != nil {
		return err
	}
	if !processed {
		// 重复发布事件不再增加 MySQL 热度，但仍继续修复 Redis。
		log.Printf("comment publish event already processed, retry redis sync: event_id=%s", event.EventID)
	}

	return w.updateHotRankingFromDB(ctx, event.VideoID)
}

func (w *CommentWorker) HandleCommentDeleted(ctx context.Context, event mq.CommentEvent) error {
	log.Printf("handle comment deleted: video_id=%d", event.VideoID)

	if event.VideoID == 0 {
		return errors.New("video_id is required")
	}
	// 评论删除会减少视频热度，也必须按 event_id 做幂等。
	processed, err := w.videoRepo.ChangePopularityOnce(ctx, event.EventID, "comment_popularity", event.VideoID, -2)
	if err != nil {
		return err
	}
	if !processed {
		// 重复删除事件不再减少 MySQL 热度，但仍继续修复 Redis。
		log.Printf("comment delete event already processed, retry redis sync: event_id=%s", event.EventID)
	}

	return w.updateHotRankingFromDB(ctx, event.VideoID)
}

// updateHotRankingFromDB 用 MySQL 当前 popularity 覆盖 Redis 热榜分数。
// 评论事件重复消费时，Redis 最终分数仍以 MySQL 为准。
func (w *CommentWorker) updateHotRankingFromDB(ctx context.Context, videoID uint) error {
	if w.cache == nil || videoID == 0 {
		return nil
	}

	popularity, err := w.videoRepo.GetPopularity(ctx, videoID)

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return w.cache.ZRem(ctx, "feed:hot:zset", videoID)
	}

	if err != nil {
		return err
	}

	return w.cache.ZAddScore(ctx, "feed:hot:zset", videoID, popularity)
}

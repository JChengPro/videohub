package worker

import (
	"backend/internal/cache"
	"backend/internal/mq"
	"context"
	"fmt"
	"log"

	"github.com/redis/go-redis/v9"
)

type VideoWorker struct {
	cache *cache.Client
}

func NewVideoWorker(cacheClient *cache.Client) *VideoWorker {
	return &VideoWorker{
		cache: cacheClient,
	}
}

func (w *VideoWorker) HandleVideoPublished(ctx context.Context, event mq.VideoPublishedEvent) error {
	log.Printf("handle video event: event_type=%s video_id=%d author_id=%d title=%s",
		event.EventType,
		event.VideoID,
		event.AuthorID,
		event.Title,
	)

	if w.cache == nil {
		return nil
	}

	timelineKey := "feed:global_timeline"
	switch event.EventType {
	case "video_published":
		if event.CreateTime > 0 {
			if err := w.cache.ZAdd(ctx, timelineKey, redis.Z{
				Score:  float64(event.CreateTime),
				Member: fmt.Sprintf("%d", event.VideoID),
			}); err != nil {
				return err
			}
			if err := w.cache.ZRemRangeByRank(ctx, timelineKey, 0, -1001); err != nil {
				log.Printf("trim feed global timeline failed: %v", err)
			}
		}
	case "video_deleted":
		if err := w.cache.ZRem(ctx, timelineKey, event.VideoID); err != nil {
			return err
		}
	}

	// 删除 feed latest 缓存是幂等操作：重复删除同一批 key，最终结果仍然是缓存不存在
	keys, err := w.cache.ScanKeys(ctx, "feed:latest:*")
	if err != nil {
		return err
	}

	if len(keys) > 0 {
		if err := w.cache.Del(ctx, keys...); err != nil {
			return err
		}
	}

	log.Printf("deleted feed latest cache keys: %d", len(keys))
	return nil
}

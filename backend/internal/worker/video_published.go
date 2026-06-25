package worker

import (
	"backend/internal/cache"
	"backend/internal/mq"
	"backend/internal/storage"
	"context"
	"fmt"
	"log"

	"github.com/redis/go-redis/v9"
)

type VideoWorker struct {
	cache       *cache.Client
	fileStorage storage.Storage
}

func NewVideoWorker(cacheClient *cache.Client, fileStorage storage.Storage) *VideoWorker {
	return &VideoWorker{
		cache:       cacheClient,
		fileStorage: fileStorage,
	}
}

func (w *VideoWorker) HandleVideoPublished(ctx context.Context, event mq.VideoPublishedEvent) error {
	log.Printf("handle video event: event_type=%s video_id=%d author_id=%d title=%s",
		event.EventType,
		event.VideoID,
		event.AuthorID,
		event.Title,
	)

	// 实际文件删除不依赖 Redis，Redis 不可用时也必须继续执行。
	if event.EventType == "video_deleted" && w.fileStorage != nil {
		if event.PlayObjectKey != "" {
			if err := w.fileStorage.Delete(ctx, event.PlayObjectKey); err != nil {
				return err
			}
		}
		if event.CoverObjectKey != "" {
			if err := w.fileStorage.Delete(ctx, event.CoverObjectKey); err != nil {
				return err
			}
		}
	}

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

		// API 删除视频后同步清缓存，是为了让删除立即生效。
		// Worker 收到 Outbox 投递的删除事件后再次清理，是为了兜底

		// 从最新视频时间线中移除已经删除视频
		if err := w.cache.ZRem(ctx, timelineKey, event.VideoID); err != nil {
			return err
		}

		// 从热榜中移除已删除视频
		if err := w.cache.ZRem(ctx, "feed:hot:zset", event.VideoID); err != nil {
			return err
		}

		// 删除视频详情缓存和视频实体缓存
		// Del 本身具有幂等性，key 不存在时重复删除也不会报错
		if err := w.cache.Del(
			ctx,
			fmt.Sprintf("video:detail:id=%d", event.VideoID),
			fmt.Sprintf("video:entity:%d", event.VideoID),
		); err != nil {
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

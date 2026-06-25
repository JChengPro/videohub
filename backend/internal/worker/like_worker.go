package worker

import (
	"backend/internal/cache"
	"backend/internal/mq"
	"backend/internal/video"
	"context"
	"errors"
	"fmt"
	"gorm.io/gorm"
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

func (w *LikeWorker) deleteVideoCaches(ctx context.Context, videoID uint) error {
	if w.cache == nil || videoID == 0 {
		return nil
	}
	// 详情接口和视频流使用不同的缓存 key，点赞数变化后两者都需要失效。
	return w.cache.Del(
		ctx,
		fmt.Sprintf("video:detail:id=%d", videoID),
		fmt.Sprintf("video:entity:%d", videoID),
	)
}

// updateHotRankingFromDB 用 MySQL 当前 popularity 覆盖 Redis 热榜分数。
// 这样即使 MQ 重复投递，Redis 也不会因为 ZINCRBY 被重复累加。
func (w *LikeWorker) updateHotRankingFromDB(ctx context.Context, videoID uint) error {
	if w.cache == nil || videoID == 0 {
		return nil
	}

	popularity, err := w.videoRepo.GetPopularity(ctx, videoID)

	if errors.Is(err, gorm.ErrRecordNotFound) {
		// 视频已删除，确保它不会被延迟消息重新加入热榜。
		return w.cache.ZRem(ctx, "feed:hot:zset", videoID)
	}

	if err != nil {
		return err
	}

	return w.cache.ZAddScore(ctx, "feed:hot:zset", videoID, popularity)
}

func (w *LikeWorker) HandleLikeCreated(ctx context.Context, event mq.LikeEvent) error {
	log.Printf("handle like created: video_id=%d", event.VideoID)

	if event.VideoID == 0 {
		return nil
	}

	// Mysql 热度更新必须按 event_id 做幂等
	// 如果同一条 MQ 消息重复投递，ChangePopularityOnce 就会返回 processed=false
	processed, err := w.videoRepo.ChangePopularityOnce(ctx, event.EventID, "like_popularity", event.VideoID, 1)
	if err != nil {
		return err
	}
	if !processed {
		// MySQL 已经处理过该事件，但 Redis 上一次可能更新失败。
		// 因此这里只跳过 MySQL 更新，仍继续使用 MySQL 最终值修复 Redis。
		log.Printf("like event already processed, retry redis sync: event_id=%s", event.EventID)
	}

	if err := w.updateHotRankingFromDB(ctx, event.VideoID); err != nil {
		return err
	}
	return w.deleteVideoCaches(ctx, event.VideoID)
}

func (w *LikeWorker) HandleLikeDeleted(ctx context.Context, event mq.LikeEvent) error {
	log.Printf("handle like deleted: video_id=%d", event.VideoID)

	if event.VideoID == 0 {
		return nil
	}

	// 取消点赞事件也必须按 event_id 做幂等，避免重复消费导致热度重复减少。
	processed, err := w.videoRepo.ChangePopularityOnce(ctx, event.EventID, "like_popularity", event.VideoID, -1)
	if err != nil {
		return err
	}
	if !processed {
		// 重复事件不再修改 MySQL，但仍继续同步 Redis 热榜和删除详情缓存。
		log.Printf("unlike event already processed, retry redis sync: event_id=%s", event.EventID)
	}

	if err := w.updateHotRankingFromDB(ctx, event.VideoID); err != nil {
		return err
	}
	return w.deleteVideoCaches(ctx, event.VideoID)
}

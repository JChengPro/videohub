package worker

import (
	"backend/internal/cache"
	"backend/internal/mq"
	"context"
	"log"
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
	log.Printf("handle video published: video_id=%d author_id=%d title=%s",
		event.VideoID,
		event.AuthorID,
		event.Title,
	)

	if w.cache == nil {
		return nil
	}

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

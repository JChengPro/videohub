package video

import (
	"backend/internal/cache"
	"context"
	"encoding/json"
	"fmt"
	"time"
)

const processingProgressTTL = 24 * time.Hour

type ProcessingProgress struct {
	VideoID   uint      `json:"video_id"`
	Stage     string    `json:"stage"`
	Progress  int       `json:"progress"`
	UpdatedAt time.Time `json:"updated_at"`
}

func processingProgressKey(videoID uint) string {
	return fmt.Sprintf("video:processing:%d", videoID)
}

func SaveProcessingProgress(ctx context.Context, client *cache.Client, videoID uint, stage string, progress int) error {
	if client == nil || videoID == 0 {
		return nil
	}
	if progress < 0 {
		progress = 0
	}
	if progress > 100 {
		progress = 100
	}
	payload, err := json.Marshal(ProcessingProgress{VideoID: videoID, Stage: stage, Progress: progress, UpdatedAt: time.Now()})
	if err != nil {
		return err
	}
	return client.Set(ctx, processingProgressKey(videoID), string(payload), processingProgressTTL)
}

func LoadProcessingProgress(ctx context.Context, client *cache.Client, videoID uint) (ProcessingProgress, error) {
	if client == nil || videoID == 0 {
		return ProcessingProgress{}, nil
	}
	value, err := client.Get(ctx, processingProgressKey(videoID))
	if err != nil {
		return ProcessingProgress{}, err
	}
	var progress ProcessingProgress
	if err := json.Unmarshal([]byte(value), &progress); err != nil {
		return ProcessingProgress{}, err
	}
	return progress, nil
}

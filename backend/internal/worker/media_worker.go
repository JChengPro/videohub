package worker

import (
	"backend/internal/cache"
	"backend/internal/media"
	"backend/internal/mq"
	"backend/internal/storage"
	"backend/internal/video"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"path/filepath"
)

const maxMediaProcessingAttempts = 3

type MediaWorker struct {
	repo        *video.Repository
	cache       *cache.Client
	fileStorage storage.Storage
}

func NewMediaWorker(repo *video.Repository, cacheClient *cache.Client, fileStorage storage.Storage) *MediaWorker {
	return &MediaWorker{repo: repo, cache: cacheClient, fileStorage: fileStorage}
}

// Handle returns retry=true only for a temporary failure that still has attempts left.
func (w *MediaWorker) Handle(ctx context.Context, event mq.VideoProcessingEvent) (retry bool, err error) {
	if w == nil || w.repo == nil || w.fileStorage == nil {
		return false, errors.New("media worker dependencies are unavailable")
	}
	if event.EventType != "video_processing_requested" || event.VideoID == 0 {
		return false, errors.New("invalid media processing event")
	}

	target, err := w.repo.BeginProcessingAttempt(ctx, event.VideoID, maxMediaProcessingAttempts)
	if errors.Is(err, video.ErrProcessingAttemptsExhausted) {
		_ = w.repo.MarkProcessingFailed(ctx, event.VideoID, err)
		_ = video.SaveProcessingProgress(ctx, w.cache, event.VideoID, "failed", 0)
		return false, err
	}
	if err != nil {
		return true, err
	}

	// A redelivered message after the database commit only needs to finish original cleanup.
	if target.Status == video.VideoStatusPublished || target.Status == video.VideoStatusPendingReview || target.Status == video.VideoStatusRejected {
		if err := w.cleanupOriginal(ctx, target); err != nil {
			return true, err
		}
		return false, nil
	}
	if target.Status != video.VideoStatusProcessing {
		return false, nil
	}

	err = w.processAttempt(ctx, target)
	if err == nil {
		return false, nil
	}

	_ = w.repo.RecordProcessingError(ctx, target.ID, err)
	if errors.Is(err, media.ErrInvalidMedia) || target.ProcessingAttempts >= maxMediaProcessingAttempts {
		_ = w.repo.MarkProcessingFailed(ctx, target.ID, err)
		_ = video.SaveProcessingProgress(ctx, w.cache, target.ID, "failed", 0)
		w.cleanupGeneratedObjects(ctx, target)
		return false, err
	}
	_ = video.SaveProcessingProgress(ctx, w.cache, target.ID, "retrying", 0)
	return true, err
}

func (w *MediaWorker) processAttempt(ctx context.Context, target *video.Video) error {
	if target.OriginalObjectKey == "" {
		return fmt.Errorf("%w: original object key is empty", media.ErrInvalidMedia)
	}
	workDir, err := os.MkdirTemp("", fmt.Sprintf("videohub-media-%d-*", target.ID))
	if err != nil {
		return err
	}
	defer os.RemoveAll(workDir)

	if err := video.SaveProcessingProgress(ctx, w.cache, target.ID, "downloading", 2); err != nil {
		log.Printf("save media progress failed: video_id=%d err=%v", target.ID, err)
	}
	ext := filepath.Ext(target.OriginalObjectKey)
	if ext == "" {
		ext = ".input"
	}
	inputPath := filepath.Join(workDir, "original"+ext)
	if err := w.downloadToFile(ctx, target.OriginalObjectKey, inputPath); err != nil {
		return fmt.Errorf("download original: %w", err)
	}

	if err := video.SaveProcessingProgress(ctx, w.cache, target.ID, "probing", 5); err != nil {
		log.Printf("save media progress failed: video_id=%d err=%v", target.ID, err)
	}
	info, err := media.Probe(ctx, inputPath)
	if err != nil {
		return err
	}

	outputPath := filepath.Join(workDir, "playback.mp4")
	if err := video.SaveProcessingProgress(ctx, w.cache, target.ID, "transcoding", 10); err != nil {
		log.Printf("save media progress failed: video_id=%d err=%v", target.ID, err)
	}
	err = media.Transcode(ctx, inputPath, outputPath, info.DurationMillis, func(percent int) {
		overall := 10 + percent*70/100
		if progressErr := video.SaveProcessingProgress(ctx, w.cache, target.ID, "transcoding", overall); progressErr != nil {
			log.Printf("save transcode progress failed: video_id=%d err=%v", target.ID, progressErr)
		}
	})
	if err != nil {
		return err
	}
	outputInfo, err := media.Probe(ctx, outputPath)
	if err != nil {
		return fmt.Errorf("probe transcoded output: %w", err)
	}

	if err := video.SaveProcessingProgress(ctx, w.cache, target.ID, "generating_covers", 82); err != nil {
		log.Printf("save media progress failed: video_id=%d err=%v", target.ID, err)
	}
	coverPaths, err := media.GenerateCandidateCovers(ctx, inputPath, filepath.Join(workDir, "covers"), info.DurationMillis)
	if err != nil {
		return err
	}

	playObjectKey, coverObjectKeys := mediaOutputKeys(target.AuthorID, target.ID)
	if err := video.SaveProcessingProgress(ctx, w.cache, target.ID, "uploading", 90); err != nil {
		log.Printf("save media progress failed: video_id=%d err=%v", target.ID, err)
	}
	if err := uploadFile(ctx, w.fileStorage, playObjectKey, outputPath); err != nil {
		return fmt.Errorf("upload playback: %w", err)
	}
	for index, coverPath := range coverPaths {
		if err := uploadFile(ctx, w.fileStorage, coverObjectKeys[index], coverPath); err != nil {
			return fmt.Errorf("upload cover %d: %w", index+1, err)
		}
	}

	selectedCover := target.CoverObjectKey
	if selectedCover == "" {
		selectedCover = coverObjectKeys[1]
	}
	target.PlayObjectKey = playObjectKey
	target.CoverObjectKey = selectedCover
	target.CoverCandidates = coverObjectKeys
	target.VideoCodec = outputInfo.VideoCodec
	target.AudioCodec = outputInfo.AudioCodec
	target.FormatName = "mp4"
	target.Width = outputInfo.Width
	target.Height = outputInfo.Height
	target.DurationMillis = outputInfo.DurationMillis

	completed, err := w.repo.CompleteProcessingWithOutbox(ctx, target)
	if err != nil {
		return fmt.Errorf("complete media processing: %w", err)
	}
	if !completed {
		current, readErr := w.repo.FindByID(ctx, target.ID)
		if readErr != nil {
			return readErr
		}
		if current.Status == video.VideoStatusDeleted {
			w.cleanupGeneratedObjects(ctx, target)
			return w.cleanupOriginal(ctx, target)
		}
		return nil
	}
	if err := video.SaveProcessingProgress(ctx, w.cache, target.ID, "completed", 100); err != nil {
		log.Printf("save completed progress failed: video_id=%d err=%v", target.ID, err)
	}
	return w.cleanupOriginal(ctx, target)
}

func (w *MediaWorker) downloadToFile(ctx context.Context, objectKey, filename string) error {
	reader, err := w.fileStorage.Open(ctx, objectKey)
	if err != nil {
		return err
	}
	defer reader.Close()
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(file, reader)
	closeErr := file.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func (w *MediaWorker) cleanupOriginal(ctx context.Context, target *video.Video) error {
	if target.OriginalObjectKey == "" {
		return nil
	}
	if err := w.fileStorage.Delete(ctx, target.OriginalObjectKey); err != nil {
		return fmt.Errorf("delete original object: %w", err)
	}
	return w.repo.ClearOriginalObjectKey(ctx, target.ID, target.OriginalObjectKey)
}

func (w *MediaWorker) cleanupGeneratedObjects(ctx context.Context, target *video.Video) {
	playObjectKey, coverObjectKeys := mediaOutputKeys(target.AuthorID, target.ID)
	_ = w.fileStorage.Delete(ctx, playObjectKey)
	for _, objectKey := range coverObjectKeys {
		_ = w.fileStorage.Delete(ctx, objectKey)
	}
	if target.CoverObjectKey != "" {
		_ = w.fileStorage.Delete(ctx, target.CoverObjectKey)
	}
}

func mediaOutputKeys(authorID, videoID uint) (string, []string) {
	playObjectKey := path.Join("videos", fmt.Sprintf("%d", authorID), "processed", fmt.Sprintf("%d", videoID), "playback.mp4")
	coverBase := path.Join("covers", fmt.Sprintf("%d", authorID), "generated", fmt.Sprintf("%d", videoID))
	covers := []string{
		path.Join(coverBase, "cover_1.jpg"),
		path.Join(coverBase, "cover_2.jpg"),
		path.Join(coverBase, "cover_3.jpg"),
	}
	return playObjectKey, covers
}

func uploadFile(ctx context.Context, fileStorage storage.Storage, objectKey, filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()
	return fileStorage.Upload(ctx, objectKey, file)
}

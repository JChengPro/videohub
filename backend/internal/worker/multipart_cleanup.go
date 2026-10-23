package worker

import (
	"context"
	"log"
	"time"

	"backend/internal/storage"
	"backend/internal/video"
)

// StartMultipartCleanup aborts OSS uploads with no successful activity for two hours.
func StartMultipartCleanup(ctx context.Context, repo *video.Repository, fileStorage storage.Storage, interval time.Duration) {
	multipartStore, ok := fileStorage.(storage.MultipartStorage)
	if !ok {
		log.Println("multipart cleanup disabled: current storage is not OSS")
		return
	}
	if interval <= 0 {
		interval = 5 * time.Minute
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				RunMultipartCleanupOnce(ctx, repo, multipartStore)
			}
		}
	}()
}

// RunMultipartCleanupOnce executes one cleanup batch. Exporting the single-run
// operation keeps the scheduler small and makes the state transition testable.
func RunMultipartCleanupOnce(ctx context.Context, repo *video.Repository, store storage.MultipartStorage) {
	tasks, err := repo.ListExpiredMultiparts(ctx, 50)
	if err != nil {
		log.Printf("list expired multipart uploads failed: %v", err)
		return
	}
	for _, task := range tasks {
		claimed, claimErr := repo.TryMarkMultipartAborting(ctx, task.ID, 0, true)
		if claimErr != nil || !claimed {
			if claimErr != nil {
				log.Printf("claim expired multipart upload failed: task_id=%s err=%v", task.ID, claimErr)
			}
			continue
		}
		if abortErr := store.AbortMultipart(ctx, task.ObjectKey, task.UploadID); abortErr != nil {
			_ = repo.RestoreMultipartAfterAbortFailure(context.Background(), task.ID, abortErr)
			log.Printf("abort expired multipart upload failed: task_id=%s err=%v", task.ID, abortErr)
			continue
		}
		if markErr := repo.MarkMultipartExpired(ctx, task.ID, "上传超过2小时未完成，临时分片已清理"); markErr != nil {
			log.Printf("mark multipart upload expired failed: task_id=%s err=%v", task.ID, markErr)
		}
	}
}

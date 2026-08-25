package video

import (
	"backend/internal/cache"
	"backend/internal/storage"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Service 做业务校验
type Service struct {
	repo         *Repository
	cache        *cache.Client
	fileStorage  storage.Storage
	cacheTTL     time.Duration
	detailLoadMu sync.Mutex //video detail cache miss 之后的重建过程  > 同一时间，只有一个 goroutine 可以负责“查 MySQL 并把详情缓存写回 Redis”。
}

func NewService(repo *Repository, cacheClient *cache.Client, fileStorage storage.Storage) *Service {
	return &Service{
		repo:        repo,
		cache:       cacheClient,
		fileStorage: fileStorage,
		cacheTTL:    5 * time.Minute,
	}
}

// 核心逻辑是：先写 MySQL，成功后再发 MQ。
func (s *Service) Publish(ctx context.Context, video *Video) error {
	if video == nil {
		return errors.New("video is null")
	}
	video.Title = strings.TrimSpace(video.Title)
	video.OriginalObjectKey = strings.TrimSpace(video.OriginalObjectKey)
	video.CoverObjectKey = strings.TrimSpace(video.CoverObjectKey)

	if video.Title == "" {
		return errors.New("title is required")
	}
	if video.OriginalObjectKey == "" {
		return errors.New("play_object_key is required")
	}
	ownerVideoPrefix := fmt.Sprintf("videos/%d/", video.AuthorID)
	if !strings.HasPrefix(video.OriginalObjectKey, ownerVideoPrefix) {
		return errors.New("play_object_key does not belong to current account")
	}
	if video.CoverObjectKey != "" {
		ownerCoverPrefix := fmt.Sprintf("covers/%d/", video.AuthorID)
		if !strings.HasPrefix(video.CoverObjectKey, ownerCoverPrefix) {
			return errors.New("cover_object_key does not belong to current account")
		}
	}

	video.Status = VideoStatusProcessing
	// 私有 OSS 签名 URL 会过期，数据库只保存稳定 ObjectKey。
	video.PlayURL = ""
	video.CoverURL = ""
	video.PlayObjectKey = ""

	//使用outbox解决数据库和mq的双写一致性问题
	if err := s.repo.CreateProcessingWithOutbox(ctx, video); err != nil {
		return err
	}
	// Redis progress is observational; a temporary Redis failure must not turn a
	// successfully committed video task into a misleading HTTP error.
	_ = SaveProcessingProgress(ctx, s.cache, video.ID, "queued", 0)
	return nil
}

func (s *Service) ProcessingStatus(ctx context.Context, videoID, accountID uint) (*ProcessingStatusResponse, error) {
	if videoID == 0 || accountID == 0 {
		return nil, errors.New("video id and account id are required")
	}
	target, err := s.repo.FindByID(ctx, videoID)
	if err != nil {
		return nil, err
	}
	if target.AuthorID != accountID {
		return nil, errors.New("unauthorized")
	}

	response := &ProcessingStatusResponse{
		VideoID:  target.ID,
		Status:   target.Status,
		Attempts: target.ProcessingAttempts,
		Error:    target.ProcessingError,
	}
	if target.Status == VideoStatusPublished {
		response.Stage = "completed"
		response.Progress = 100
		if err := RefreshAccessURLs(ctx, s.fileStorage, target); err != nil {
			return nil, err
		}
		response.PlayURL = target.PlayURL
		response.CoverURL = target.CoverURL
		for _, objectKey := range target.CoverCandidates {
			url, urlErr := s.fileStorage.URL(ctx, objectKey, time.Hour)
			if urlErr != nil {
				return nil, urlErr
			}
			response.CandidateCoverURLs = append(response.CandidateCoverURLs, url)
		}
		return response, nil
	}
	if target.Status == VideoStatusFailed {
		response.Stage = "failed"
		return response, nil
	}

	progress, progressErr := LoadProcessingProgress(ctx, s.cache, videoID)
	if progressErr == nil && progress.VideoID != 0 {
		response.Stage = progress.Stage
		response.Progress = progress.Progress
	} else {
		response.Stage = "queued"
	}
	return response, nil
}

func (s *Service) SelectCandidateCover(ctx context.Context, videoID, accountID uint, index int) error {
	if videoID == 0 || accountID == 0 {
		return errors.New("video id and account id are required")
	}
	target, err := s.repo.FindByID(ctx, videoID)
	if err != nil {
		return err
	}
	if target.AuthorID != accountID {
		return errors.New("unauthorized")
	}
	if index < 0 || index >= len(target.CoverCandidates) {
		return errors.New("invalid cover candidate index")
	}
	if err := s.repo.SelectCandidateCover(ctx, videoID, accountID, target.CoverCandidates[index]); err != nil {
		return err
	}
	return s.deleteDetailCache(ctx, videoID)
}

func (s *Service) Detail(ctx context.Context, id uint) (*Video, error) {
	if id == 0 {
		return nil, errors.New("video id is required")
	}

	cacheKey := fmt.Sprintf("video:detail:id=%d", id)

	if s.cache != nil {
		cached, err := s.cache.Get(ctx, cacheKey)
		if err == nil {
			var video Video
			if err := json.Unmarshal([]byte(cached), &video); err == nil {
				if err := RefreshAccessURLs(ctx, s.fileStorage, &video); err != nil {
					return nil, err
				}
				if err := s.refreshCommentCount(ctx, &video); err != nil {
					return nil, err
				}
				return &video, nil
			}
		}
	}

	if s.cache == nil {
		video, err := s.repo.FindPublishedByID(ctx, id)
		if err != nil {
			return nil, err
		}
		if err := RefreshAccessURLs(ctx, s.fileStorage, video); err != nil {
			return nil, err
		}
		if err := s.refreshCommentCount(ctx, video); err != nil {
			return nil, err
		}
		return video, nil
	}
	s.detailLoadMu.Lock()
	defer s.detailLoadMu.Unlock()

	//拿到锁之后再查一次缓存，防止别的请求已经回填好了
	cached, err := s.cache.Get(ctx, cacheKey)
	if err == nil {
		var video Video
		if err := json.Unmarshal([]byte(cached), &video); err == nil {
			if err := RefreshAccessURLs(ctx, s.fileStorage, &video); err != nil {
				return nil, err
			}
			if err := s.refreshCommentCount(ctx, &video); err != nil {
				return nil, err
			}
			return &video, nil
		}
	}

	video, err := s.repo.FindPublishedByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if s.cache != nil {
		if b, err := json.Marshal(video); err == nil {
			_ = s.cache.Set(ctx, cacheKey, string(b), s.cacheTTL)
		}
	}
	if err := RefreshAccessURLs(ctx, s.fileStorage, video); err != nil {
		return nil, err
	}
	if err := s.refreshCommentCount(ctx, video); err != nil {
		return nil, err
	}

	return video, nil
}

func (s *Service) ListByAuthor(ctx context.Context, authorID uint) ([]Video, error) {
	if authorID == 0 {
		return nil, errors.New("author_id is required")
	}
	videos, err := s.repo.ListByAuthorID(ctx, authorID)
	if err != nil {
		return nil, err
	}
	ids := make([]uint, len(videos))
	for i := range videos {
		ids[i] = videos[i].ID
	}
	commentCounts, err := s.repo.CommentCounts(ctx, ids)
	if err != nil {
		return nil, err
	}
	for i := range videos {
		videos[i].CommentsCount = commentCounts[videos[i].ID]
		if err := RefreshAccessURLs(ctx, s.fileStorage, &videos[i]); err != nil {
			return nil, err
		}
	}
	return videos, nil
}

func (s *Service) refreshCommentCount(ctx context.Context, target *Video) error {
	count, err := s.repo.CommentCount(ctx, target.ID)
	if err != nil {
		return err
	}
	target.CommentsCount = count
	return nil
}

func (s *Service) deleteDetailCache(ctx context.Context, videoID uint) error {
	if s.cache == nil || videoID == 0 {
		return nil
	}
	cacheKey := fmt.Sprintf("video:detail:id=%d", videoID)
	return s.cache.Del(ctx, cacheKey)
}

func (s *Service) Delete(ctx context.Context, videoID uint, accountID uint) error {
	if videoID == 0 || accountID == 0 {
		return errors.New("video id and account_id are required")
	}

	video, err := s.repo.FindByID(ctx, videoID)
	if err != nil {
		return err
	}

	if video.AuthorID != accountID {
		return errors.New("unauthorized")
	}

	if err := s.repo.DeleteWithOutbox(ctx, video); err != nil {
		return err
	}

	if err := s.deleteDetailCache(ctx, videoID); err != nil {
		return err
	}
	if err := s.deleteHotRanking(ctx, videoID); err != nil {
		return err
	}

	return nil
}

// 删除视频后还要删除ZSet里面的热榜
func (s *Service) deleteHotRanking(ctx context.Context, videoID uint) error {
	if s.cache == nil || videoID == 0 {
		return nil
	}
	return s.cache.ZRem(ctx, "feed:hot:zset", videoID)
}

// MergeChunks 把临时分片合并后上传到当前存储实现。
func (s *Service) MergeChunks(ctx context.Context, fileID string, fileExt string, accountID uint) (string, string, error) {
	if err := validateUploadFileID(fileID); err != nil {
		return "", "", err
	}
	ext, err := normalizeVideoExtension(fileExt)
	if err != nil {
		return "", "", err
	}
	chunkDir := filepath.Join(".run", "uploads", "chunks", fileID)
	entries, err := os.ReadDir(chunkDir)
	if err != nil {
		return "", "", fmt.Errorf("chunks not found: %w", err)
	}
	totalChunks := len(entries)

	tempFile, err := os.CreateTemp("", "videohub-merge-*"+ext)
	if err != nil {
		return "", "", fmt.Errorf("create temp file failed: %w", err)
	}
	tempPath := tempFile.Name()
	defer os.Remove(tempPath)
	defer tempFile.Close()

	date := time.Now().Format("20060102")
	objectKey := path.Join("videos", fmt.Sprintf("%d", accountID), date, randHex(16)+ext)

	// 分片按编号顺序写入临时文件。
	for i := 0; i < totalChunks; i++ {
		chunkPath := filepath.Join(chunkDir, fmt.Sprintf("%d", i))
		data, err := os.ReadFile(chunkPath)
		if err != nil {
			return "", "", fmt.Errorf("chunk %d missing: %w", i, err)
		}
		if _, err := tempFile.Write(data); err != nil {
			return "", "", fmt.Errorf("write chunk %d failed: %w", i, err)
		}
	}

	if _, err := tempFile.Seek(0, io.SeekStart); err != nil {
		return "", "", fmt.Errorf("seek merged file failed: %w", err)
	}
	if err := s.fileStorage.Upload(ctx, objectKey, tempFile); err != nil {
		return "", "", fmt.Errorf("upload merged file failed: %w", err)
	}

	playURL, err := s.fileStorage.URL(ctx, objectKey, time.Hour)
	if err != nil {
		return "", "", err
	}

	_ = os.RemoveAll(chunkDir)
	return playURL, objectKey, nil
}

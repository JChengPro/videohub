package video

import (
	"backend/internal/cache"
	"backend/internal/mq"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

// Service 做业务校验
type Service struct {
	repo         *Repository
	cache        *cache.Client
	rabbit       *mq.RabbitMQ
	cacheTTL     time.Duration
	detailLoadMu sync.Mutex //video detail cache miss 之后的重建过程  > 同一时间，只有一个 goroutine 可以负责“查 MySQL 并把详情缓存写回 Redis”。
}

func NewService(repo *Repository, cacheClient *cache.Client, rabbit *mq.RabbitMQ) *Service {
	return &Service{
		repo:     repo,
		cache:    cacheClient,
		rabbit:   rabbit,
		cacheTTL: 5 * time.Minute,
	}
}

// 核心逻辑是：先写 MySQL，成功后再发 MQ。
func (s *Service) Publish(ctx context.Context, video *Video) error {
	if video == nil {
		return errors.New("video is null")
	}
	video.Title = strings.TrimSpace(video.Title)
	video.PlayURL = strings.TrimSpace(video.PlayURL)
	video.CoverURL = strings.TrimSpace(video.CoverURL)

	if video.Title == "" {
		return errors.New("title is required")
	}
	if video.PlayURL == "" {
		return errors.New("play_url is required")
	}
	if video.CoverURL == "" {
		return errors.New("cover_url is required")
	}

	//使用outbox解决数据库和mq的双写一致性问题
	return s.repo.CreateWithOutbox(ctx, video)
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
				return &video, nil
			}
		}
	}

	if s.cache == nil {
		return s.repo.FindByID(ctx, id)
	}
	s.detailLoadMu.Lock()
	defer s.detailLoadMu.Unlock()

	//拿到锁之后再查一次缓存，防止别的请求已经回填好了
	cached, err := s.cache.Get(ctx, cacheKey)
	if err == nil {
		var video Video
		if err := json.Unmarshal([]byte(cached), &video); err == nil {
			return &video, nil
		}
	}

	video, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if s.cache != nil {
		if b, err := json.Marshal(video); err == nil {
			_ = s.cache.Set(ctx, cacheKey, string(b), s.cacheTTL)
		}
	}

	return video, nil
}

func (s *Service) ListByAuthor(ctx context.Context, authorID uint) ([]Video, error) {
	if authorID == 0 {
		return nil, errors.New("author_id is required")
	}
	return s.repo.ListByAuthorID(ctx, authorID)
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

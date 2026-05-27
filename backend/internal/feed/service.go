package feed

import (
	rediscache "backend/internal/cache"
	"backend/internal/video"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strconv"
	"sync"
	"time"

	localcache "github.com/patrickmn/go-cache"
	"github.com/redis/go-redis/v9"
	"golang.org/x/sync/singleflight"
)

type Service struct {
	repo         *Repository
	cache        *rediscache.Client
	likeRepo     *video.LikeRepository //新增，用于查 is_liked
	localcache   *localcache.Cache
	cacheTTL     time.Duration
	requestGroup singleflight.Group //requestGroup 用来管理“哪些请求是同一个请求”。
}

func NewService(repo *Repository, cacheClient *rediscache.Client, likeRepo *video.LikeRepository) *Service {
	return &Service{
		repo:       repo,
		cache:      cacheClient,
		likeRepo:   likeRepo,
		localcache: localcache.New(3*time.Second, 5*time.Second),
		cacheTTL:   24 * time.Hour,
	}
}

func (s *Service) GetVideoByIDs(ctx context.Context, videoIDs []uint) ([]*video.Video, error) {
	if len(videoIDs) == 0 {
		return []*video.Video{}, nil
	}

	videoMap := make(map[uint]*video.Video, len(videoIDs))
	missedL1 := make([]uint, 0, len(videoIDs))
	for _, id := range videoIDs {
		cacheKey := fmt.Sprintf("video:entity:%d", id)
		if s.localcache != nil {
			if v, found := s.localcache.Get(cacheKey); found {
				if data, ok := v.(video.Video); ok {
					safeCopy := data
					videoMap[id] = &safeCopy
					continue
				}
			}
		}
		missedL1 = append(missedL1, id)
	}

	if len(missedL1) == 0 {
		return buildOrderedResult(videoIDs, videoMap), nil
	}

	missedL2 := make([]uint, 0, len(missedL1))
	if s.cache != nil {
		cacheKeys := make([]string, len(missedL1))
		for i, id := range missedL1 {
			cacheKeys[i] = fmt.Sprintf("video:entity:%d", id)
		}

		cacheCtx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
		results, err := s.cache.MGet(cacheCtx, cacheKeys...)
		cancel()

		if err == nil {
			for i, res := range results {
				id := missedL1[i]
				if res != nil {
					if str, ok := res.(string); ok {
						var v video.Video
						if err := json.Unmarshal([]byte(str), &v); err == nil {
							videoMap[id] = &v
							if s.localcache != nil {
								s.localcache.Set(cacheKeys[i], v, 5*time.Second)
							}
							continue
						}
					}
				}
				missedL2 = append(missedL2, id)
			}
		} else {
			missedL2 = missedL1
			log.Printf("feed video entity MGet failed, fallback to MySQL: %v", err)
		}
	} else {
		missedL2 = missedL1
	}

	if len(missedL2) == 0 {
		return buildOrderedResult(videoIDs, videoMap), nil
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	for _, id := range missedL2 {
		wg.Add(1)
		go func(videoID uint) {
			defer wg.Done()

			sfKey := fmt.Sprintf("sf:entity:%d", videoID)
			v, err, _ := s.requestGroup.Do(sfKey, func() (interface{}, error) {
				videoList, err := s.repo.GetByIDs(ctx, []uint{videoID})
				if err != nil || len(videoList) == 0 {
					return nil, err
				}

				safeCopy := *videoList[0]
				cacheKey := fmt.Sprintf("video:entity:%d", safeCopy.ID)
				if s.cache != nil {
					if b, err := json.Marshal(safeCopy); err == nil {
						go func(k string, b []byte) {
							setCtx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
							defer cancel()
							_ = s.cache.SetBytes(setCtx, k, b, time.Hour)
						}(cacheKey, b)
					}
				}
				return videoList[0], nil
			})

			if err == nil && v != nil {
				safeCopy := *(v.(*video.Video))
				mu.Lock()
				videoMap[videoID] = &safeCopy
				mu.Unlock()
				if s.localcache != nil {
					s.localcache.Set(fmt.Sprintf("video:entity:%d", safeCopy.ID), safeCopy, 5*time.Second)
				}
			}
		}(id)
	}
	wg.Wait()

	return buildOrderedResult(videoIDs, videoMap), nil
}

func (s *Service) listLatestFromDB(ctx context.Context, limit int, latestBefore time.Time, accountID uint) (ListLatestResponse, error) {
	reqTime := int64(0)
	if !latestBefore.IsZero() {
		reqTime = latestBefore.UnixMilli()
	}
	sfKey := fmt.Sprintf("sf:fallback:listLatest:%d:%d", limit, reqTime)
	v, err, _ := s.requestGroup.Do(sfKey, func() (interface{}, error) {
		return s.repo.ListLatest(ctx, limit, latestBefore)
	})
	if err != nil {
		return ListLatestResponse{}, err
	}

	videos := v.([]*video.Video)
	var nextTime int64
	if len(videos) > 0 {
		nextTime = videos[len(videos)-1].CreateTime.UnixMilli()
	}
	return ListLatestResponse{
		VideoList: s.toFeedVideoItems(ctx, videos, accountID),
		HasMore:   len(videos) == limit,
		NextTime:  nextTime,
	}, nil
}

func (s *Service) ListLatest(ctx context.Context, limit int, latestBefore time.Time, accountID uint) (ListLatestResponse, error) {
	if limit <= 0 {
		return ListLatestResponse{}, errors.New("limit must be positive")
	}
	if limit > 50 {
		limit = 50
	}

	if s.cache == nil {
		return s.listLatestFromDB(ctx, limit, latestBefore, accountID)
	}

	zsetTail, err := s.cache.ZRangeWithScores(ctx, "feed:global_timeline", 0, 0)
	if err != nil {
		return ListLatestResponse{}, err
	}

	if len(zsetTail) == 0 {
		v, err, _ := s.requestGroup.Do("sf:fallback:global_timeline_rebuild", func() (interface{}, error) {
			dbVideos, err := s.repo.ListLatest(ctx, 1000, time.Time{})
			if err != nil {
				return nil, err
			}
			if len(dbVideos) == 0 {
				return "EMPTY_DB", nil
			}

			bgCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			zElements := make([]redis.Z, 0, len(dbVideos))
			for _, vid := range dbVideos {
				zElements = append(zElements, redis.Z{
					Score:  float64(vid.CreateTime.UnixMilli()),
					Member: fmt.Sprintf("%d", vid.ID),
				})
			}
			return "SUCCESS", s.cache.ZAdd(bgCtx, "feed:global_timeline", zElements...)
		})
		if err != nil {
			return ListLatestResponse{}, err
		}
		if v == "EMPTY_DB" {
			return ListLatestResponse{VideoList: []FeedVideoItem{}, HasMore: false}, nil
		}
		return s.ListLatest(ctx, limit, latestBefore, accountID)
	}

	watermark := int64(zsetTail[0].Score)
	reqTime := time.Now().UnixMilli()
	if !latestBefore.IsZero() {
		reqTime = latestBefore.UnixMilli()
	}

	var baseVideos []*video.Video
	if reqTime <= watermark {
		sfKey := fmt.Sprintf("sf:cold:listLatest:%d:%d", limit, reqTime)
		v, err, _ := s.requestGroup.Do(sfKey, func() (interface{}, error) {
			return s.repo.ListLatest(ctx, limit, latestBefore)
		})
		if err != nil {
			return ListLatestResponse{}, err
		}
		baseVideos = v.([]*video.Video)
	} else {
		maxScore := "+inf"
		if !latestBefore.IsZero() {
			maxScore = fmt.Sprintf("%d", reqTime-1)
		}

		videoIDsStr, err := s.cache.ZRevRangeByScore(ctx, "feed:global_timeline", maxScore, "-inf", 0, int64(limit))
		if err != nil {
			return ListLatestResponse{}, err
		}

		videoIDs := make([]uint, 0, len(videoIDsStr))
		for _, idStr := range videoIDsStr {
			id, err := strconv.ParseUint(idStr, 10, 64)
			if err == nil {
				videoIDs = append(videoIDs, uint(id))
			}
		}

		if len(videoIDs) > 0 {
			baseVideos, err = s.GetVideoByIDs(ctx, videoIDs)
			if err != nil {
				return ListLatestResponse{}, err
			}
		}

		if len(baseVideos) < limit {
			remainLimit := limit - len(baseVideos)
			var coldCursor time.Time
			if len(baseVideos) > 0 {
				coldCursor = baseVideos[len(baseVideos)-1].CreateTime
			} else {
				coldCursor = latestBefore
			}

			sfKey := fmt.Sprintf("sf:stitch:listLatest:%d:%d", remainLimit, coldCursor.UnixMilli())
			v, err, _ := s.requestGroup.Do(sfKey, func() (interface{}, error) {
				return s.repo.ListLatest(ctx, remainLimit, coldCursor)
			})
			if err == nil {
				baseVideos = append(baseVideos, v.([]*video.Video)...)
			}
		}
	}

	var nextTime int64
	if len(baseVideos) > 0 {
		nextTime = baseVideos[len(baseVideos)-1].CreateTime.UnixMilli()
	}

	return ListLatestResponse{
		VideoList: s.toFeedVideoItems(ctx, baseVideos, accountID),
		HasMore:   len(baseVideos) == limit,
		NextTime:  nextTime,
	}, nil
}

func (s *Service) ListFollowing(ctx context.Context, accountID uint, limit int, latestTime int64) (ListByFollowingResponse, error) {
	if accountID == 0 {
		return ListByFollowingResponse{}, errors.New("account_id is required")
	}
	if limit <= 0 {
		return ListByFollowingResponse{}, errors.New("limit must be positive")
	}
	if limit > 50 {
		limit = 50
	}

	videos, err := s.repo.ListFollowing(ctx, accountID, limit+1, latestTime)
	if err != nil {
		return ListByFollowingResponse{}, err
	}

	hasMore := len(videos) > limit
	if hasMore {
		videos = videos[:limit]
	}

	var nextTime int64
	if len(videos) > 0 {
		nextTime = videos[len(videos)-1].CreateTime.UnixMilli()
	}

	return ListByFollowingResponse{
		VideoList: s.toFeedVideoItems(ctx, videos, accountID),
		HasMore:   hasMore,
		NextTime:  nextTime,
	}, nil
}

func (s *Service) ListByPopularity(ctx context.Context, limit int, asOf int64, offset int, accountID uint) (ListByPopularityResponse, error) {
	if limit <= 0 {
		return ListByPopularityResponse{}, errors.New("limit must be positive")
	}
	if limit > 50 {
		limit = 50
	}
	if s.cache != nil {
		members, err := s.cache.ZRevRange(ctx, "feed:hot:zset", 0, int64(limit-1))
		if err == nil && len(members) > 0 {
			ids := make([]uint, 0, len(members))
			for _, member := range members {
				id64, err := strconv.ParseUint(member, 10, 64)
				if err != nil {
					continue
				}
				ids = append(ids, uint(id64))
			}
			if len(ids) > 0 {
				videos, err := s.repo.GetByIDs(ctx, ids)
				if err == nil {
					videoMap := make(map[uint]*video.Video, len(videos))
					for _, v := range videos {
						videoMap[v.ID] = v
					}
					ordered := make([]*video.Video, 0, len(ids))
					for _, id := range ids {
						if v, ok := videoMap[id]; ok {
							ordered = append(ordered, v)
						}
					}
					return ListByPopularityResponse{
						VideoList:  s.toFeedVideoItems(ctx, ordered, accountID),
						AsOf:       asOf,
						NextOffset: offset + limit,
						HasMore:    len(ordered) >= limit,
					}, nil
				}
			}
		}
	}
	videos, err := s.repo.ListHot(ctx, limit)
	if err != nil {
		return ListByPopularityResponse{}, err
	}

	return ListByPopularityResponse{
		VideoList:  s.toFeedVideoItems(ctx, videos, accountID),
		AsOf:       asOf,
		NextOffset: offset + limit,
		HasMore:    len(videos) >= limit,
	}, nil
}

func (s *Service) ListByLikesCount(ctx context.Context, limit int, likesCountBefore int64, idBefore uint, accountID uint) (ListLikesCountResponse, error) {
	if limit <= 0 {
		return ListLikesCountResponse{}, errors.New("limit must be positive")
	}
	if limit > 50 {
		limit = 50
	}

	videos, err := s.repo.ListByLikesCount(ctx, limit+1, likesCountBefore, idBefore)
	if err != nil {
		return ListLikesCountResponse{}, err
	}

	hasMore := len(videos) > limit
	if hasMore {
		videos = videos[:limit]
	}

	resp := ListLikesCountResponse{
		VideoList: s.toFeedVideoItems(ctx, videos, accountID),
		HasMore:   hasMore,
	}

	if len(videos) > 0 {
		last := videos[len(videos)-1]
		lc := last.LikesCount
		id := last.ID
		resp.NextLikesCountBefore = &lc
		resp.NextIDBefore = &id
	}

	return resp, nil
}

func (s *Service) toFeedVideoItems(ctx context.Context, videos []*video.Video, accountID uint) []FeedVideoItem {
	if len(videos) == 0 {
		return nil
	}

	ids := make([]uint, len(videos))
	for i, v := range videos {
		ids[i] = v.ID
	}

	likedSet, _ := s.likeRepo.LikedVideoIDs(ctx, accountID, ids)

	items := make([]FeedVideoItem, len(videos))
	for i, v := range videos {
		items[i] = FeedVideoItem{
			ID:          v.ID,
			Author:      FeedAuthor{ID: v.AuthorID, Username: v.Username},
			Title:       v.Title,
			Description: v.Description,
			PlayURL:     v.PlayURL,
			CoverURL:    v.CoverURL,
			CreateTime:  v.CreateTime.UnixMilli(),
			LikesCount:  v.LikesCount,
			IsLiked:     likedSet[v.ID],
		}
	}
	return items
}

func buildOrderedResult(orderedIDs []uint, dataMap map[uint]*video.Video) []*video.Video {
	res := make([]*video.Video, 0, len(orderedIDs))
	for _, id := range orderedIDs {
		if v, ok := dataMap[id]; ok && v != nil {
			res = append(res, v)
		}
	}
	return res
}

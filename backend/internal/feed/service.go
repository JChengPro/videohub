package feed

import (
	"backend/internal/cache"
	"backend/internal/video"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"
)

type Service struct {
	repo  *Repository
	cache *cache.Client
	likeRepo *video.LikeRepository  //新增，用于查 is_liked
}

func NewService(repo *Repository, cacheClient *cache.Client, likeRepo *video.LikeRepository) *Service {
	return &Service{repo: repo, cache: cacheClient,likeRepo: likeRepo}
}

func (s *Service) ListLatest(ctx context.Context, limit int, latestTime int64, accountID uint) (ListLatestResponse, error) {
	if limit <= 0 {
		return ListLatestResponse{}, errors.New("limit must be positive")
	}
	if limit > 50 {
		limit = 50
	}

	cachekey := fmt.Sprintf("feed:latest:limit=%d:latestTime=%d", limit, latestTime)

	if s.cache != nil && latestTime == 0 {
		if cached, err := s.cache.Get(ctx, cachekey); err == nil {
			var resp ListLatestResponse
			if err := json.Unmarshal([]byte(cached), &resp); err == nil {
				return resp, nil
			}
		}
	}

	videos, err := s.repo.ListLatest(ctx, limit+1, latestTime)
	if err != nil {
		return ListLatestResponse{}, err
	}
	hasmore := len(videos) > limit
	if hasmore {
		videos = videos[:limit]
	}
	var nextTime int64
	if len(videos) > 0 {
		nextTime = videos[len(videos)-1].CreateTime.UnixMilli()
	}
	resp := ListLatestResponse{
		VideoList: s.toFeedVideoItems(ctx, videos, accountID),
		HasMore:   hasmore,
		NextTime:  nextTime,
	}

	if s.cache != nil && latestTime == 0 {
		if b, err := json.Marshal(resp); err == nil {
			_ = s.cache.Set(ctx, cachekey, string(b), 30*time.Second)
		}
	}
	return resp, nil
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
					videoMap := make(map[uint]video.Video, len(videos))
					for _, v := range videos {
						videoMap[v.ID] = v
					}
					ordered := make([]video.Video, 0, len(ids))
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

func (s *Service) toFeedVideoItems(ctx context.Context, videos []video.Video, accountID uint) []FeedVideoItem {
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
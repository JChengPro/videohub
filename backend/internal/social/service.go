package social

import (
	"backend/internal/account"
	"context"
	"errors"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Follow(ctx context.Context, followerID, vloggerID uint) error {
	if followerID == 0 || vloggerID == 0 {
		return errors.New("follower_id and vlogger_id are required")
	}
	if followerID == vloggerID {
		return errors.New("cannot follow yourself")
	}

	following, err := s.repo.IsFollowing(ctx, followerID, vloggerID)
	if err != nil {
		return err
	}
	if following {
		return nil
	}

	return s.repo.Follow(ctx, &Social{
		FollowerID: followerID,
		VloggerID:  vloggerID,
	})
}

func (s *Service) Unfollow(ctx context.Context, followerID, vloggerID uint) error {
	if followerID == 0 || vloggerID == 0 {
		return errors.New("follower_id and vlogger_id are required")
	}
	if followerID == vloggerID {
		return errors.New("cannot unfollow yourself")
	}

	following, err := s.repo.IsFollowing(ctx, followerID, vloggerID)
	if err != nil {
		return err
	}
	if !following {
		return nil
	}

	return s.repo.Unfollow(ctx, followerID, vloggerID)
}

func (s *Service) ListFollowers(ctx context.Context, vloggerID uint) ([]account.Account, error) {
	if vloggerID == 0 {
		return nil, errors.New("vlogger_id is required")
	}
	return s.repo.ListFollowers(ctx, vloggerID)
}

func (s *Service) ListFollowing(ctx context.Context, followerID uint) ([]account.Account, error) {
	if followerID == 0 {
		return nil, errors.New("follower_id is required")
	}
	return s.repo.ListFollowing(ctx, followerID)
}

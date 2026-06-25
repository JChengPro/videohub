package social

import (
	"backend/internal/account"
	"context"
	"errors"
)

type Service struct {
	repo        *Repository
	accountRepo *account.Repository
}

func NewService(repo *Repository, accountRepo *account.Repository) *Service {
	return &Service{repo: repo, accountRepo: accountRepo}
}

func (s *Service) ensureAccountExists(ctx context.Context, accountID uint) error {
	exists, err := s.accountRepo.ExistsByID(ctx, accountID)
	if err != nil {
		return err
	}
	if !exists {
		return errors.New("account does not exist")
	}
	return nil
}

func (s *Service) Follow(ctx context.Context, followerID, vloggerID uint) error {
	if followerID == 0 || vloggerID == 0 {
		return errors.New("follower_id and vlogger_id are required")
	}
	if followerID == vloggerID {
		return errors.New("cannot follow yourself")
	}
	if err := s.ensureAccountExists(ctx, vloggerID); err != nil {
		return err
	}

	following, err := s.repo.IsFollowing(ctx, followerID, vloggerID)
	if err != nil {
		return err
	}
	if following {
		return nil
	}

	return s.repo.FollowWithNotificationOutbox(ctx, &Social{
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
	if err := s.ensureAccountExists(ctx, vloggerID); err != nil {
		return nil, err
	}
	return s.repo.ListFollowers(ctx, vloggerID)
}

func (s *Service) ListFollowing(ctx context.Context, followerID uint) ([]account.Account, error) {
	if followerID == 0 {
		return nil, errors.New("follower_id is required")
	}
	if err := s.ensureAccountExists(ctx, followerID); err != nil {
		return nil, err
	}
	return s.repo.ListFollowing(ctx, followerID)
}

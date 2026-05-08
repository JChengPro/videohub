package worker

import (
	"backend/internal/mq"
	"backend/internal/social"
	"context"
	"log"
)

type SocialWorker struct {
	repo *social.Repository
}

func NewSocialWorker(repo *social.Repository) *SocialWorker {
	return &SocialWorker{
		repo: repo,
	}
}

func (w *SocialWorker) HandleSocialFollowed(ctx context.Context, event mq.SocialEvent) error {
	log.Printf("handle social followed: follower_id=%d vlogger_id=%d", event.FollowerID, event.VloggerID)

	following, err := w.repo.IsFollowing(ctx, event.FollowerID, event.VloggerID)
	if err != nil {
		return err
	}
	if following {
		return nil
	}

	return w.repo.Follow(ctx, &social.Social{
		FollowerID: event.FollowerID,
		VloggerID:  event.VloggerID,
	})
}

func (w *SocialWorker) HandleSocialUnfollowed(ctx context.Context, event mq.SocialEvent) error {
	log.Printf("handle social unfollowed: follower_id=%d vlogger_id=%d", event.FollowerID, event.VloggerID)

	following, err := w.repo.IsFollowing(ctx, event.FollowerID, event.VloggerID)
	if err != nil {
		return err
	}
	if !following {
		return nil
	}

	return w.repo.Unfollow(ctx, event.FollowerID, event.VloggerID)
}

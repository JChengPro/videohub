package notification

import (
	"context"
	"errors"
	"strings"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) List(ctx context.Context, receiverID uint, req ListRequest) (ListResponse, error) {
	if receiverID == 0 {
		return ListResponse{}, errors.New("receiver_id is required")
	}
	req.Type = strings.TrimSpace(req.Type)
	if req.Type != "" && req.Type != "like" && req.Type != "comment" && req.Type != "follow" {
		return ListResponse{}, errors.New("invalid notification type")
	}
	if req.Limit <= 0 {
		req.Limit = 20
	}
	if req.Limit > 50 {
		req.Limit = 50
	}

	items, err := s.repo.List(ctx, receiverID, req.Type, req.Limit+1, req.BeforeID)
	if err != nil {
		return ListResponse{}, err
	}
	hasMore := len(items) > req.Limit
	if hasMore {
		items = items[:req.Limit]
	}
	var nextBeforeID uint
	if len(items) > 0 {
		nextBeforeID = items[len(items)-1].ID
	}
	return ListResponse{Notifications: items, HasMore: hasMore, NextBeforeID: nextBeforeID}, nil
}

func (s *Service) UnreadCount(ctx context.Context, receiverID uint) (int64, error) {
	if receiverID == 0 {
		return 0, errors.New("receiver_id is required")
	}
	return s.repo.UnreadCount(ctx, receiverID)
}

func (s *Service) MarkRead(ctx context.Context, receiverID, id uint) error {
	if receiverID == 0 || id == 0 {
		return errors.New("receiver_id and id are required")
	}
	return s.repo.MarkRead(ctx, receiverID, id)
}

func (s *Service) MarkAllRead(ctx context.Context, receiverID uint) error {
	if receiverID == 0 {
		return errors.New("receiver_id is required")
	}
	return s.repo.MarkAllRead(ctx, receiverID)
}

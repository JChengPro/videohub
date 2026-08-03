package message

import (
	"backend/internal/account"
	"backend/internal/cache"
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

type Service struct {
	repo        *Repository
	accountRepo *account.Repository
	cache       *cache.Client
}

func NewService(
	repo *Repository,
	accountRepo *account.Repository,
	cacheClient *cache.Client,
) *Service {
	return &Service{repo: repo, accountRepo: accountRepo, cache: cacheClient}
}

func (s *Service) Send(
	ctx context.Context,
	senderID uint,
	req SendRequest,
) (SendResponse, error) {
	if senderID == 0 || req.ReceiverID == 0 {
		return SendResponse{}, errors.New("sender and receiver are required")
	}
	if senderID == req.ReceiverID {
		return SendResponse{}, policyError("CANNOT_MESSAGE_SELF", "不能给自己发送私信")
	}
	req.ClientMessageID = strings.TrimSpace(req.ClientMessageID)
	if req.ClientMessageID == "" || len(req.ClientMessageID) > 64 {
		return SendResponse{}, errors.New("client_message_id is required and must not exceed 64 characters")
	}
	req.Content = strings.TrimSpace(req.Content)
	if req.Content == "" {
		return SendResponse{}, errors.New("message content is required")
	}
	if !utf8.ValidString(req.Content) || utf8.RuneCountInString(req.Content) > 1000 {
		return SendResponse{}, errors.New("message content must not exceed 1000 characters")
	}

	exists, err := s.accountRepo.ExistsByID(ctx, req.ReceiverID)
	if err != nil {
		return SendResponse{}, err
	}
	if !exists {
		return SendResponse{}, policyError("RECEIVER_NOT_FOUND", "接收用户不存在")
	}
	if s.cache != nil {
		key := fmt.Sprintf("videohub:message:rate:%d", senderID)
		count, err := s.cache.IncrementWithExpire(ctx, key, time.Minute)
		if err == nil && count > 60 {
			return SendResponse{}, policyError("MESSAGE_RATE_LIMIT", "消息发送过于频繁，请稍后再试")
		}
	}

	result, err := s.repo.Send(ctx, senderID, req.ReceiverID, req.ClientMessageID, req.Content)
	if err != nil {
		return SendResponse{}, err
	}
	status := result.Conversation.Status
	if result.Mutual {
		status = StatusMutual
	}
	return SendResponse{
		Message: MessageView{
			ID:              result.Message.ID,
			ConversationID:  result.Message.ConversationID,
			SenderID:        result.Message.SenderID,
			SenderUsername:  result.SenderUsername,
			ReceiverID:      result.Message.ReceiverID,
			ClientMessageID: result.Message.ClientMessageID,
			MessageType:     result.Message.MessageType,
			Content:         result.Message.Content,
			CreatedAt:       result.Message.CreatedAt,
		},
		ConversationStatus:       status,
		RemainingRequestMessages: result.Remaining,
		Idempotent:               result.Idempotent,
	}, nil
}

func (s *Service) ListConversations(ctx context.Context, accountID uint) (ListConversationsResponse, error) {
	if accountID == 0 {
		return ListConversationsResponse{}, errors.New("account id is required")
	}
	items, err := s.repo.ListConversations(ctx, accountID)
	if err != nil {
		return ListConversationsResponse{}, err
	}
	return ListConversationsResponse{Conversations: items}, nil
}

func (s *Service) ListMessages(
	ctx context.Context,
	accountID uint,
	req ListMessagesRequest,
) (ListMessagesResponse, error) {
	if accountID == 0 || req.ConversationID == 0 {
		return ListMessagesResponse{}, errors.New("conversation id is required")
	}
	if req.Limit <= 0 {
		req.Limit = 30
	}
	if req.Limit > 50 {
		req.Limit = 50
	}
	items, err := s.repo.ListMessages(ctx, accountID, req.ConversationID, req.BeforeID, req.Limit+1)
	if err != nil {
		return ListMessagesResponse{}, err
	}
	hasMore := len(items) > req.Limit
	if hasMore {
		items = items[1:]
	}
	var nextBeforeID uint
	if len(items) > 0 {
		nextBeforeID = items[0].ID
	}
	return ListMessagesResponse{
		Messages:     items,
		HasMore:      hasMore,
		NextBeforeID: nextBeforeID,
	}, nil
}

func (s *Service) MarkRead(
	ctx context.Context,
	accountID uint,
	req MarkReadRequest,
) (MarkReadResult, error) {
	if accountID == 0 || req.ConversationID == 0 {
		return MarkReadResult{}, errors.New("conversation id is required")
	}
	return s.repo.MarkRead(ctx, accountID, req.ConversationID, req.MessageID)
}

func (s *Service) Accept(ctx context.Context, accountID, conversationID uint) (uint, error) {
	if accountID == 0 || conversationID == 0 {
		return 0, errors.New("conversation id is required")
	}
	return s.repo.Accept(ctx, accountID, conversationID)
}

func (s *Service) Reject(ctx context.Context, accountID, conversationID uint) (uint, error) {
	if accountID == 0 || conversationID == 0 {
		return 0, errors.New("conversation id is required")
	}
	return s.repo.Reject(ctx, accountID, conversationID)
}

func (s *Service) Block(ctx context.Context, accountID, userID uint) error {
	if accountID == 0 || userID == 0 {
		return errors.New("user id is required")
	}
	if accountID == userID {
		return errors.New("cannot block yourself")
	}
	exists, err := s.accountRepo.ExistsByID(ctx, userID)
	if err != nil {
		return err
	}
	if !exists {
		return policyError("USER_NOT_FOUND", "用户不存在")
	}
	return s.repo.Block(ctx, accountID, userID)
}

func (s *Service) Unblock(ctx context.Context, accountID, userID uint) error {
	if accountID == 0 || userID == 0 {
		return errors.New("user id is required")
	}
	return s.repo.Unblock(ctx, accountID, userID)
}

func (s *Service) UnreadCount(ctx context.Context, accountID uint) (int64, error) {
	if accountID == 0 {
		return 0, errors.New("account id is required")
	}
	return s.repo.UnreadCount(ctx, accountID)
}

package message

import (
	"backend/internal/social"
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

type sendResult struct {
	Message        Message
	Conversation   Conversation
	Mutual         bool
	Remaining      uint8
	Idempotent     bool
	SenderUsername string
}

type conversationRow struct {
	ID                  uint
	UserLowID           uint
	UserHighID          uint
	Status              string
	RequestSenderID     uint
	RequestSentCount    uint8
	LastMessageID       uint
	LowUnreadCount      int64
	HighUnreadCount     int64
	UpdatedAt           time.Time
	PeerID              uint
	PeerUsername        string
	LastMessageContent  string
	LastMessageSenderID uint
	LastMessageAt       *time.Time
}

func canonicalPair(first, second uint) (uint, uint) {
	if first < second {
		return first, second
	}
	return second, first
}

func (r *Repository) Send(
	ctx context.Context,
	senderID, receiverID uint,
	clientMessageID, content string,
) (sendResult, error) {
	var result sendResult
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		lowID, highID := canonicalPair(senderID, receiverID)
		candidate := Conversation{
			UserLowID:  lowID,
			UserHighID: highID,
			Status:     StatusPending,
		}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&candidate).Error; err != nil {
			return internalError("create conversation", err)
		}

		var conversation Conversation
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("user_low_id = ? AND user_high_id = ?", lowID, highID).
			First(&conversation).Error; err != nil {
			return internalError("lock conversation", err)
		}

		blocked, err := isBlocked(tx, senderID, receiverID)
		if err != nil {
			return internalError("check block", err)
		}
		if blocked {
			return policyError("USER_BLOCKED", "无法向该用户发送消息")
		}

		mutual, err := isMutual(tx, senderID, receiverID)
		if err != nil {
			return internalError("check mutual follow", err)
		}

		var existing Message
		err = tx.Where("sender_id = ? AND client_message_id = ?", senderID, clientMessageID).
			First(&existing).Error
		if err == nil {
			result.Message = existing
			result.Conversation = conversation
			result.Mutual = mutual
			result.Remaining = remainingMessages(conversation)
			result.Idempotent = true
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return internalError("check duplicate message", err)
		}

		now := time.Now()
		if mutual {
			// 互关本身只在当前关系有效时放开权限，不永久修改请求状态。
			// 这样取消互关后仍会恢复到原有的三条请求额度；只有接收者回复或
			// 主动接受，才会把会话永久变为 accepted。
		} else {
			switch conversation.Status {
			case StatusRejected:
				return policyError("CONVERSATION_REJECTED", "对方已拒绝该消息请求")
			case StatusPending:
				if conversation.RequestSenderID == 0 {
					conversation.RequestSenderID = senderID
				}
				if conversation.RequestSenderID == senderID {
					if conversation.RequestSentCount >= RequestLimit {
						return policyError("MESSAGE_REQUEST_LIMIT", "对方接受或回复前最多发送 3 条消息")
					}
					conversation.RequestSentCount++
				} else {
					// 消息请求接收者的第一条回复即代表接受会话。
					conversation.Status = StatusAccepted
					conversation.AcceptedBy = senderID
					conversation.AcceptedAt = &now
				}
			case StatusAccepted:
				// 已接受的会话即使没有互关，也允许双方继续发送文字。
			default:
				return policyError("INVALID_CONVERSATION_STATE", "会话状态异常")
			}
		}

		message := Message{
			ConversationID:  conversation.ID,
			SenderID:        senderID,
			ReceiverID:      receiverID,
			ClientMessageID: clientMessageID,
			MessageType:     MessageTypeText,
			Content:         content,
		}
		if err := tx.Create(&message).Error; err != nil {
			return internalError("create message", err)
		}

		conversation.LastMessageID = message.ID
		if receiverID == conversation.UserLowID {
			conversation.LowUnreadCount++
		} else {
			conversation.HighUnreadCount++
		}
		if err := tx.Model(&Conversation{}).
			Where("id = ?", conversation.ID).
			Updates(map[string]any{
				"status":             conversation.Status,
				"request_sender_id":  conversation.RequestSenderID,
				"request_sent_count": conversation.RequestSentCount,
				"accepted_by":        conversation.AcceptedBy,
				"accepted_at":        conversation.AcceptedAt,
				"last_message_id":    conversation.LastMessageID,
				"low_unread_count":   conversation.LowUnreadCount,
				"high_unread_count":  conversation.HighUnreadCount,
				"updated_at":         now,
			}).Error; err != nil {
			return internalError("update conversation", err)
		}
		conversation.UpdatedAt = now

		result.Message = message
		result.Conversation = conversation
		result.Mutual = mutual
		result.Remaining = remainingMessages(conversation)
		return nil
	})
	if err != nil {
		return sendResult{}, err
	}

	var username string
	if err := r.db.WithContext(ctx).Table("accounts").
		Select("username").
		Where("id = ?", senderID).
		Scan(&username).Error; err == nil {
		result.SenderUsername = username
	}
	return result, nil
}

func (r *Repository) ListConversations(ctx context.Context, accountID uint) ([]ConversationView, error) {
	var rows []conversationRow
	peerExpression := "CASE WHEN conversations.user_low_id = ? THEN conversations.user_high_id ELSE conversations.user_low_id END"
	err := r.db.WithContext(ctx).
		Table(conversationTableName).
		Select(`
			conversations.id,
			conversations.user_low_id,
			conversations.user_high_id,
			conversations.status,
			conversations.request_sender_id,
			conversations.request_sent_count,
			conversations.last_message_id,
			conversations.low_unread_count,
			conversations.high_unread_count,
			conversations.updated_at,
			`+peerExpression+` AS peer_id,
			peer.username AS peer_username,
			last_message.content AS last_message_content,
			last_message.sender_id AS last_message_sender_id,
			last_message.created_at AS last_message_at
		`, accountID).
		Joins("JOIN accounts AS peer ON peer.id = "+peerExpression, accountID).
		Joins("LEFT JOIN "+messageTableName+" AS last_message ON last_message.id = conversations.last_message_id").
		Where("conversations.user_low_id = ? OR conversations.user_high_id = ?", accountID, accountID).
		Order("conversations.updated_at DESC, conversations.id DESC").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	items := make([]ConversationView, 0, len(rows))
	for _, row := range rows {
		mutual, err := r.IsMutual(ctx, accountID, row.PeerID)
		if err != nil {
			return nil, err
		}
		blockedByMe, blockedByPeer, err := r.BlockState(ctx, accountID, row.PeerID)
		if err != nil {
			return nil, err
		}
		blocked := blockedByMe || blockedByPeer
		conversation := Conversation{
			ID:               row.ID,
			UserLowID:        row.UserLowID,
			UserHighID:       row.UserHighID,
			Status:           row.Status,
			RequestSenderID:  row.RequestSenderID,
			RequestSentCount: row.RequestSentCount,
		}
		status, canSend, canReply := effectivePolicy(conversation, accountID, mutual, blocked)
		unread := row.HighUnreadCount
		if accountID == row.UserLowID {
			unread = row.LowUnreadCount
		}
		items = append(items, ConversationView{
			ID:                       row.ID,
			PeerID:                   row.PeerID,
			PeerUsername:             row.PeerUsername,
			Status:                   status,
			RequestSenderID:          row.RequestSenderID,
			RequestSentCount:         row.RequestSentCount,
			RemainingRequestMessages: remainingMessages(conversation),
			CanSend:                  canSend,
			CanReply:                 canReply,
			BlockedByMe:              blockedByMe,
			BlockedByPeer:            blockedByPeer,
			LastMessageID:            row.LastMessageID,
			LastMessageContent:       row.LastMessageContent,
			LastMessageSenderID:      row.LastMessageSenderID,
			LastMessageAt:            row.LastMessageAt,
			UnreadCount:              unread,
			UpdatedAt:                row.UpdatedAt,
		})
	}
	return items, nil
}

func (r *Repository) ListMessages(
	ctx context.Context,
	accountID, conversationID, beforeID uint,
	limit int,
) ([]MessageView, error) {
	conversation, err := r.findConversationForAccount(ctx, accountID, conversationID)
	if err != nil {
		return nil, err
	}
	_ = conversation

	query := r.db.WithContext(ctx).
		Table(messageTableName).
		Select("messages.*, accounts.username AS sender_username").
		Joins("JOIN accounts ON accounts.id = messages.sender_id").
		Where("messages.conversation_id = ?", conversationID).
		Order("messages.id DESC").
		Limit(limit)
	if beforeID > 0 {
		query = query.Where("messages.id < ?", beforeID)
	}
	var messages []MessageView
	if err := query.Scan(&messages).Error; err != nil {
		return nil, err
	}
	for left, right := 0, len(messages)-1; left < right; left, right = left+1, right-1 {
		messages[left], messages[right] = messages[right], messages[left]
	}
	return messages, nil
}

func (r *Repository) MarkRead(
	ctx context.Context,
	accountID, conversationID, requestedMessageID uint,
) (MarkReadResult, error) {
	var result MarkReadResult
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var conversation Conversation
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND (user_low_id = ? OR user_high_id = ?)", conversationID, accountID, accountID).
			First(&conversation).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return policyError("CONVERSATION_NOT_FOUND", "会话不存在")
			}
			return err
		}

		query := tx.Model(&Message{}).
			Where("conversation_id = ? AND receiver_id = ?", conversationID, accountID)
		if requestedMessageID > 0 {
			query = query.Where("id <= ?", requestedMessageID)
		}
		var lastReadID uint
		if err := query.Select("COALESCE(MAX(id), 0)").Scan(&lastReadID).Error; err != nil {
			return err
		}
		currentLastReadID := conversation.HighLastReadMessageID
		if accountID == conversation.UserLowID {
			currentLastReadID = conversation.LowLastReadMessageID
		}
		if currentLastReadID > lastReadID {
			// 迟到或重复的已读请求不能把游标向后移动。
			lastReadID = currentLastReadID
		}

		var unread int64
		if err := tx.Model(&Message{}).
			Where("conversation_id = ? AND receiver_id = ? AND id > ?", conversationID, accountID, lastReadID).
			Count(&unread).Error; err != nil {
			return err
		}

		updates := map[string]any{}
		if accountID == conversation.UserLowID {
			updates["low_last_read_message_id"] = lastReadID
			updates["low_unread_count"] = unread
			result.PeerID = conversation.UserHighID
		} else {
			updates["high_last_read_message_id"] = lastReadID
			updates["high_unread_count"] = unread
			result.PeerID = conversation.UserLowID
		}
		if err := tx.Model(&Conversation{}).Where("id = ?", conversationID).Updates(updates).Error; err != nil {
			return err
		}
		result.ConversationID = conversationID
		result.ReadMessageID = lastReadID
		result.UnreadCount = unread
		return nil
	})
	return result, err
}

func (r *Repository) Accept(ctx context.Context, accountID, conversationID uint) (uint, error) {
	return r.changeRequestStatus(ctx, accountID, conversationID, StatusAccepted)
}

func (r *Repository) Reject(ctx context.Context, accountID, conversationID uint) (uint, error) {
	return r.changeRequestStatus(ctx, accountID, conversationID, StatusRejected)
}

func (r *Repository) changeRequestStatus(
	ctx context.Context,
	accountID, conversationID uint,
	status string,
) (uint, error) {
	var peerID uint
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var conversation Conversation
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND (user_low_id = ? OR user_high_id = ?)", conversationID, accountID, accountID).
			First(&conversation).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return policyError("CONVERSATION_NOT_FOUND", "会话不存在")
			}
			return err
		}
		if accountID == conversation.RequestSenderID {
			return policyError("REQUEST_ACTION_FORBIDDEN", "消息请求发起者不能执行该操作")
		}
		if accountID == conversation.UserLowID {
			peerID = conversation.UserHighID
		} else {
			peerID = conversation.UserLowID
		}

		if status == StatusAccepted {
			if conversation.Status == StatusAccepted {
				return nil
			}
			now := time.Now()
			return tx.Model(&Conversation{}).Where("id = ?", conversationID).
				Updates(map[string]any{
					"status":      StatusAccepted,
					"accepted_by": accountID,
					"accepted_at": &now,
				}).Error
		}
		if conversation.Status == StatusRejected {
			return nil
		}
		if conversation.Status != StatusPending {
			return policyError("REQUEST_ACTION_FORBIDDEN", "当前会话不能拒绝")
		}
		return tx.Model(&Conversation{}).Where("id = ?", conversationID).
			Update("status", StatusRejected).Error
	})
	return peerID, err
}

func (r *Repository) Block(ctx context.Context, blockerID, blockedID uint) error {
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{DoNothing: true}).
		Create(&Block{BlockerID: blockerID, BlockedID: blockedID}).Error
}

func (r *Repository) Unblock(ctx context.Context, blockerID, blockedID uint) error {
	return r.db.WithContext(ctx).
		Where("blocker_id = ? AND blocked_id = ?", blockerID, blockedID).
		Delete(&Block{}).Error
}

func (r *Repository) UnreadCount(ctx context.Context, accountID uint) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&Conversation{}).
		Select(`COALESCE(SUM(CASE
			WHEN user_low_id = ? THEN low_unread_count
			ELSE high_unread_count
		END), 0)`, accountID).
		Where("user_low_id = ? OR user_high_id = ?", accountID, accountID).
		Scan(&count).Error
	return count, err
}

func (r *Repository) IsMutual(ctx context.Context, firstID, secondID uint) (bool, error) {
	return isMutual(r.db.WithContext(ctx), firstID, secondID)
}

func (r *Repository) IsBlocked(ctx context.Context, firstID, secondID uint) (bool, error) {
	return isBlocked(r.db.WithContext(ctx), firstID, secondID)
}

func (r *Repository) BlockState(ctx context.Context, accountID, peerID uint) (bool, bool, error) {
	var blocks []Block
	err := r.db.WithContext(ctx).
		Select("blocker_id", "blocked_id").
		Where(
			"(blocker_id = ? AND blocked_id = ?) OR (blocker_id = ? AND blocked_id = ?)",
			accountID, peerID, peerID, accountID,
		).
		Find(&blocks).Error
	if err != nil {
		return false, false, err
	}
	var blockedByMe, blockedByPeer bool
	for _, block := range blocks {
		if block.BlockerID == accountID {
			blockedByMe = true
		}
		if block.BlockerID == peerID {
			blockedByPeer = true
		}
	}
	return blockedByMe, blockedByPeer, nil
}

func (r *Repository) findConversationForAccount(
	ctx context.Context,
	accountID, conversationID uint,
) (*Conversation, error) {
	var conversation Conversation
	err := r.db.WithContext(ctx).
		Where("id = ? AND (user_low_id = ? OR user_high_id = ?)", conversationID, accountID, accountID).
		First(&conversation).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, policyError("CONVERSATION_NOT_FOUND", "会话不存在")
	}
	return &conversation, err
}

func isMutual(db *gorm.DB, firstID, secondID uint) (bool, error) {
	var count int64
	err := db.Model(&social.Social{}).
		Where(
			"(follower_id = ? AND vlogger_id = ?) OR (follower_id = ? AND vlogger_id = ?)",
			firstID, secondID, secondID, firstID,
		).
		Count(&count).Error
	return count == 2, err
}

func isBlocked(db *gorm.DB, firstID, secondID uint) (bool, error) {
	var count int64
	err := db.Model(&Block{}).
		Where(
			"(blocker_id = ? AND blocked_id = ?) OR (blocker_id = ? AND blocked_id = ?)",
			firstID, secondID, secondID, firstID,
		).
		Count(&count).Error
	return count > 0, err
}

func remainingMessages(conversation Conversation) uint8 {
	if conversation.RequestSentCount >= RequestLimit {
		return 0
	}
	return RequestLimit - conversation.RequestSentCount
}

func effectivePolicy(
	conversation Conversation,
	accountID uint,
	mutual, blocked bool,
) (status string, canSend, canReply bool) {
	if blocked {
		return StatusBlocked, false, false
	}
	if mutual {
		return StatusMutual, true, true
	}
	switch conversation.Status {
	case StatusAccepted:
		return StatusAccepted, true, true
	case StatusRejected:
		return StatusRejected, false, false
	case StatusPending:
		if conversation.RequestSenderID == accountID {
			return StatusPending, conversation.RequestSentCount < RequestLimit, false
		}
		return StatusPending, true, true
	default:
		return conversation.Status, false, false
	}
}

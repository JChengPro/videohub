package message

import "time"

const (
	StatusPending  = "pending"
	StatusAccepted = "accepted"
	StatusRejected = "rejected"
	StatusMutual   = "mutual"
	StatusBlocked  = "blocked"

	MessageTypeText = "text"
	RequestLimit    = uint8(3)

	conversationTableName = "conversations"
	messageTableName      = "messages"
	blockTableName        = "blocks"
)

type Conversation struct {
	ID                    uint       `gorm:"primaryKey" json:"id"`
	UserLowID             uint       `gorm:"uniqueIndex:idx_chat_pair;not null" json:"user_low_id"`
	UserHighID            uint       `gorm:"uniqueIndex:idx_chat_pair;not null" json:"user_high_id"`
	Status                string     `gorm:"type:varchar(20);not null;default:pending;index" json:"status"`
	RequestSenderID       uint       `gorm:"not null;default:0" json:"request_sender_id"`
	RequestSentCount      uint8      `gorm:"not null;default:0" json:"request_sent_count"`
	AcceptedBy            uint       `gorm:"not null;default:0" json:"accepted_by"`
	AcceptedAt            *time.Time `json:"accepted_at"`
	LastMessageID         uint       `gorm:"index;not null;default:0" json:"last_message_id"`
	LowLastReadMessageID  uint       `gorm:"not null;default:0" json:"low_last_read_message_id"`
	HighLastReadMessageID uint       `gorm:"not null;default:0" json:"high_last_read_message_id"`
	LowUnreadCount        int64      `gorm:"not null;default:0" json:"low_unread_count"`
	HighUnreadCount       int64      `gorm:"not null;default:0" json:"high_unread_count"`
	CreatedAt             time.Time  `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt             time.Time  `gorm:"autoUpdateTime;index" json:"updated_at"`
}

type Message struct {
	ID              uint      `gorm:"primaryKey" json:"id"`
	ConversationID  uint      `gorm:"index:idx_chat_message_cursor;not null" json:"conversation_id"`
	SenderID        uint      `gorm:"index;uniqueIndex:idx_chat_client_message;not null" json:"sender_id"`
	ReceiverID      uint      `gorm:"index;not null" json:"receiver_id"`
	ClientMessageID string    `gorm:"type:varchar(64);uniqueIndex:idx_chat_client_message;not null" json:"client_message_id"`
	MessageType     string    `gorm:"type:varchar(20);not null;default:text" json:"message_type"`
	Content         string    `gorm:"type:varchar(4000);not null" json:"content"`
	CreatedAt       time.Time `gorm:"autoCreateTime;index:idx_chat_message_cursor" json:"created_at"`
}

type Block struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	BlockerID uint      `gorm:"uniqueIndex:idx_chat_block;not null" json:"blocker_id"`
	BlockedID uint      `gorm:"uniqueIndex:idx_chat_block;not null" json:"blocked_id"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
}

// 显式固定表名，保证 GORM AutoMigrate/CRUD 与 Repository 中的联表查询一致。
func (Conversation) TableName() string { return conversationTableName }
func (Message) TableName() string      { return messageTableName }
func (Block) TableName() string        { return blockTableName }

type MessageView struct {
	ID              uint      `json:"id"`
	ConversationID  uint      `json:"conversation_id"`
	SenderID        uint      `json:"sender_id"`
	SenderUsername  string    `json:"sender_username"`
	ReceiverID      uint      `json:"receiver_id"`
	ClientMessageID string    `json:"client_message_id"`
	MessageType     string    `json:"message_type"`
	Content         string    `json:"content"`
	CreatedAt       time.Time `json:"created_at"`
}

type ConversationView struct {
	ID                       uint       `json:"id"`
	PeerID                   uint       `json:"peer_id"`
	PeerUsername             string     `json:"peer_username"`
	Status                   string     `json:"status"`
	RequestSenderID          uint       `json:"request_sender_id"`
	RequestSentCount         uint8      `json:"request_sent_count"`
	RemainingRequestMessages uint8      `json:"remaining_request_messages"`
	CanSend                  bool       `json:"can_send"`
	CanReply                 bool       `json:"can_reply"`
	BlockedByMe              bool       `json:"blocked_by_me"`
	BlockedByPeer            bool       `json:"blocked_by_peer"`
	LastMessageID            uint       `json:"last_message_id"`
	LastMessageContent       string     `json:"last_message_content"`
	LastMessageSenderID      uint       `json:"last_message_sender_id"`
	LastMessageAt            *time.Time `json:"last_message_at"`
	UnreadCount              int64      `json:"unread_count"`
	UpdatedAt                time.Time  `json:"updated_at"`
}

type ListConversationsResponse struct {
	Conversations []ConversationView `json:"conversations"`
}

type ListMessagesRequest struct {
	ConversationID uint `json:"conversation_id"`
	BeforeID       uint `json:"before_id"`
	Limit          int  `json:"limit"`
}

type ListMessagesResponse struct {
	Messages     []MessageView `json:"messages"`
	HasMore      bool          `json:"has_more"`
	NextBeforeID uint          `json:"next_before_id"`
}

type SendRequest struct {
	ReceiverID      uint   `json:"receiver_id"`
	ClientMessageID string `json:"client_message_id"`
	Content         string `json:"content"`
}

type SendResponse struct {
	Message                  MessageView `json:"message"`
	ConversationStatus       string      `json:"conversation_status"`
	RemainingRequestMessages uint8       `json:"remaining_request_messages"`
	Idempotent               bool        `json:"idempotent"`
}

type ConversationActionRequest struct {
	ConversationID uint `json:"conversation_id"`
}

type MarkReadRequest struct {
	ConversationID uint `json:"conversation_id"`
	MessageID      uint `json:"message_id"`
}

type MarkReadResult struct {
	ConversationID uint  `json:"conversation_id"`
	ReadMessageID  uint  `json:"read_message_id"`
	PeerID         uint  `json:"peer_id"`
	UnreadCount    int64 `json:"unread_count"`
}

type UserActionRequest struct {
	UserID uint `json:"user_id"`
}

type UnreadCountResponse struct {
	Count int64 `json:"count"`
}

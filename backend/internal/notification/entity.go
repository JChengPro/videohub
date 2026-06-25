package notification

import "time"

type Notification struct {
	ID         uint `gorm:"primaryKey" json:"id"`
	ReceiverID uint `gorm:"index;not null" json:"receiver_id"`
	ActorID    uint `gorm:"index;not null" json:"actor_id"`
	// ActorUsername 由通知列表联表查询返回，不写入 notifications 表。
	ActorUsername string     `gorm:"column:actor_username;->;-:migration" json:"actor_username"`
	Type          string     `gorm:"type:varchar(32);index;not null" json:"type"`
	TargetType    string     `gorm:"type:varchar(32);not null" json:"target_type"`
	TargetID      uint       `gorm:"index;not null" json:"target_id"`
	Content       string     `gorm:"type:varchar(500);not null" json:"content"`
	IsRead        bool       `gorm:"index;not null;default:false" json:"is_read"`
	DedupKey      string     `gorm:"type:varchar(160);not null;uniqueIndex" json:"-"`
	CreateTime    time.Time  `gorm:"autoCreateTime" json:"create_time"`
	ReadTime      *time.Time `json:"read_time"`
}

type ListRequest struct {
	Type     string `json:"type"`
	Limit    int    `json:"limit"`
	BeforeID uint   `json:"before_id"`
}

type ListResponse struct {
	Notifications []Notification `json:"notifications"`
	HasMore       bool           `json:"has_more"`
	NextBeforeID  uint           `json:"next_before_id"`
}

type MarkReadRequest struct {
	ID uint `json:"id"`
}

type UnreadCountResponse struct {
	Count int64 `json:"count"`
}

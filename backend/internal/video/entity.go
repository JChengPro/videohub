package video

import "time"

type Video struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	AuthorID    uint      `gorm:"index;not null" json:"author_id"`
	Username    string    `gorm:"type:varchar(255);not null" json:"username"`
	Title       string    `gorm:"type:varchar(255);not null" json:"title"`
	Description string    `gorm:"type:varchar(255)" json:"description"`
	PlayURL     string    `gorm:"type:varchar(255);not null" json:"play_url"`
	CoverURL    string    `gorm:"type:varchar(255);not null" json:"cover_url"`
	CreateTime  time.Time `gorm:"autoCreateTime" json:"create_time"`
	LikesCount  int64     `gorm:"column:likes_count;not null;default:0" json:"likes_count"`
	Popularity  int64     `gorm:"column:popularity;not null;default:0" json:"popularity"`
}

// outbox表
type OutboxMsg struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	EventType  string    `gorm:"type:varchar(50);not null;index" json:"event_type"`
	QueueName  string    `gorm:"type:varchar(255);index" json:"queue_name"`
	Payload    string    `gorm:"type:text" json:"payload"`
	VideoID    uint      `gorm:"index;not null" json:"video_id"`
	AuthorID   uint      `gorm:"not null" json:"author_id"`
	Title      string    `gorm:"type:varchar(255);not null" json:"title"`
	Status     string    `gorm:"type:varchar(50);not null;default:pending;index" json:"status"`
	RetryCount int       `gorm:"not null;default:0" json:"retry_count"`
	LastError  string    `gorm:"type:varchar(500)" json:"last_error"`
	CreateTime time.Time `gorm:"autoCreateTime" json:"create_time"`
	UpdateTime time.Time `gorm:"autoUpdateTime" json:"update_time"`
}

type PublishRequest struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	PlayURL     string `json:"play_url"`
	CoverURL    string `json:"cover_url"`
}

// 根据ID查找视频结构体
type DetailRequest struct {
	ID uint `json:"id"`
}

// 根据这个结构体的ID去查询相关的信息，要写这个结构体是为了ShouldBindJson传入的是结构体，上面都一样
type ListByAuthorRequest struct {
	AuthorID uint `json:"author_id"`
}

// 删除请求结构体
type DeleteRequest struct {
	ID uint `json:"id"`
}

package video

import "time"

const (
	VideoStatusProcessing = "processing"
	VideoStatusPublished  = "published"
	VideoStatusDeleted    = "deleted"
	VideoStatusFailed     = "failed"
)

type Video struct {
	ID          uint   `gorm:"primaryKey" json:"id"`
	AuthorID    uint   `gorm:"index;not null" json:"author_id"`
	Username    string `gorm:"type:varchar(255);not null" json:"username"`
	Title       string `gorm:"type:varchar(255);not null" json:"title"`
	Description string `gorm:"type:varchar(255)" json:"description"`
	PlayURL     string `gorm:"type:varchar(255);not null" json:"play_url"`
	CoverURL    string `gorm:"type:varchar(255);not null" json:"cover_url"`
	// ObjectKey 是文件在本地存储或 OSS 中的稳定路径。
	// 私有 OSS 的签名 URL 会过期，因此数据库需要保存 ObjectKey。
	PlayObjectKey  string    `gorm:"type:varchar(500);not null;default:''" json:"play_object_key"`
	CoverObjectKey string    `gorm:"type:varchar(500);not null;default:''" json:"cover_object_key"`
	CreateTime     time.Time `gorm:"autoCreateTime" json:"create_time"`
	LikesCount     int64     `gorm:"column:likes_count;not null;default:0" json:"likes_count"`
	CommentsCount  int64     `gorm:"-" json:"comments_count"`
	Popularity     int64     `gorm:"column:popularity;not null;default:0" json:"popularity"`
	Status         string    `gorm:"type:varchar(20);not null;default:published;index" json:"status"`
}

// outbox表
type OutboxMsg struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	EventID    string    `gorm:"type:varchar(64);not null;uniqueIndex" json:"event_id"`
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

// ConsumedEvent 记录 worker 已经成功处理过的 MQ 事件
// 通过 event_id + consumer_name 唯一索引，保证同一消费者不会重复处理同一事情
type ConsumedEvent struct {
	ID      uint   `gorm:"primaryKey" json:"id"`
	EventID string `gorm:"type:varchar(64);not null;uniqueIndex:idx_event_consumer" json:"event_id"`
	// 因为同一个事件未来可能被不同消费者处理，所以需要ConsumerName
	// 两个消费者都可以处理同一个 event_id，但同一个消费者不能重复处理
	ConsumerName string    `gorm:"type:varchar(64);not null;uniqueIndex:idx_event_consumer" json:"consumer_name"`
	CreateTime   time.Time `gorm:"autoCreateTime" json:"create_time"`
}

type PublishRequest struct {
	Title          string `json:"title"`
	Description    string `json:"description"`
	PlayURL        string `json:"play_url"`
	CoverURL       string `json:"cover_url"`
	PlayObjectKey  string `json:"play_object_key"`
	CoverObjectKey string `json:"cover_object_key"`
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

// 给前端查询哪些片已经上传
type ChunkStatusRequest struct {
	FileID string `json:"file_id"`
}

// 返回已上传的片号列表，比如 [0,1,2,5]表示0 1 2 5号片已经上传
type ChunkStatusResponse struct {
	Uploaded []int `json:"uploaded"`
}

// 前端合并分片请求：传 file_id，后端按片号顺序拼成完整视频文件
type MergeChunksRequest struct {
	FileID  string `json:"file_id"`
	FileExt string `json:"file_ext"`
}

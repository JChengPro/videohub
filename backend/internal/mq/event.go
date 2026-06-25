package mq

const VideoPublishedQueueName = "feedsystem.video.published.queue"

type VideoPublishedEvent struct {
	EventID        string `json:"event_id"`
	EventType      string `json:"event_type"`
	VideoID        uint   `json:"video_id"`
	AuthorID       uint   `json:"author_id"`
	Title          string `json:"title"`
	PlayObjectKey  string `json:"play_object_key"`
	CoverObjectKey string `json:"cover_object_key"`
	CreateTime     int64  `json:"create_time"`
}

const LikeQueueName = "feedsystem.like.queue"

type LikeEvent struct {
	EventID   string `json:"event_id"`
	EventType string `json:"event_type"`
	VideoID   uint   `json:"video_id"`
	AccountID uint   `json:"account_id"`
}

const CommentQueueName = "feedsystem.comment.queue"

type CommentEvent struct {
	EventID   string `json:"event_id"`
	EventType string `json:"event_type"`
	CommentID uint   `json:"comment_id"`
	VideoID   uint   `json:"video_id"`
	AuthorID  uint   `json:"author_id"`
	Username  string `json:"username"`
	Content   string `json:"content"`
}

const SocialQueueName = "feedsystem.social.queue"

type SocialEvent struct {
	EventID    string `json:"event_id"`
	EventType  string `json:"event_type"`
	FollowerID uint   `json:"follower_id"`
	VloggerID  uint   `json:"vlogger_id"`
}

const NotificationQueueName = "videohub.notification.queue"

// NotificationEvent 是通知 Worker 需要的最小业务快照。
// 独立通知队列避免通知消费者与热度消费者竞争同一条消息。
type NotificationEvent struct {
	EventID    string `json:"event_id"`
	Type       string `json:"type"`
	ReceiverID uint   `json:"receiver_id"`
	ActorID    uint   `json:"actor_id"`
	TargetType string `json:"target_type"`
	TargetID   uint   `json:"target_id"`
	Content    string `json:"content"`
	DedupKey   string `json:"dedup_key"`
}

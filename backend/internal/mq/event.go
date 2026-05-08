package mq

const VideoPublishedQueueName = "feedsystem.video.published.queue"

type VideoPublishedEvent struct {
	EventType string `json:"event_type"`
	VideoID   uint   `json:"video_id"`
	AuthorID  uint   `json:"author_id"`
	Title     string `json:"title"`
}

const LikeQueueName = "feedsystem.like.queue"

type LikeEvent struct {
	EventType string `json:"event_type"`
	VideoID   uint   `json:"video_id"`
	AccountID uint   `json:"account_id"`
}

const CommentQueueName = "feedsystem.comment.queue"

type CommentEvent struct {
	EventType string `json:"event_type"`
	CommentID uint   `json:"comment_id"`
	VideoID   uint   `json:"video_id"`
	AuthorID  uint   `json:"author_id"`
	Username  string `json:"username"`
	Content   string `json:"content"`
}

const SocialQueueName = "feedsystem.social.queue"

type SocialEvent struct {
	EventType  string `json:"event_type"`
	FollowerID uint   `json:"follower_id"`
	VloggerID  uint   `json:"vlogger_id"`
}

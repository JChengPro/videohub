package worker

import (
	"backend/internal/mq"
	"backend/internal/video"
	"context"
	"fmt"
	"log"
	"time"
)

// poller 后台轮查器 发mq的任务
func StartOutboxPoller(ctx context.Context, repo *video.Repository, rabbit *mq.RabbitMQ, interval time.Duration) {
	if repo == nil || rabbit == nil {
		log.Println("Outbox poller disabled: repository or rabbitmq is nil")
		return
	}
	if interval <= 0 {
		interval = 2 * time.Second
	}
	if err := rabbit.DeclareQueue(mq.VideoPublishedQueueName); err != nil {
		log.Printf("Outbox poller declare queue failed: %v", err)
		return
	}

	go func() {
		//表示定时器，比如每 2 秒触发一次。
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		log.Println("Outbox poller started")
		for {
			select {
			case <-ctx.Done():
				log.Println("Outbox poller stopped")
				return
				//ticker.C 是 time.Ticker 提供的定时 channel，每隔固定时间会触发一次，所以 poller 可以定时扫描 outbox 表。
				//ctx.Done() 是 context 的取消通知 channel，当服务关闭或外部取消 context 时会触发，poller 收到后 return 退出 goroutine，实现优雅停止。
			case <-ticker.C:
				publishPendingOutbox(ctx, repo, rabbit)
			}
		}
	}()
}

func publishPendingOutbox(ctx context.Context, repo *video.Repository, rabbit *mq.RabbitMQ) {
	// 每次 poller 扫 pending 之前，先把卡死的 publishing 消息恢复成 pending。
	if err := repo.ResetStalePublishingOutbox(ctx, time.Minute); err != nil {
		log.Printf("outbox reset stale publishing failed: %v", err)
	}

	messages, err := repo.ListPendingOutbox(ctx, 50)
	if err != nil {
		log.Printf("outbox poller list pending failed: %v", err)
		return
	}
	if len(messages) == 0 {
		return
	}

	for _, msg := range messages {
		locked, err := repo.TryMarkOutboxPublishing(ctx, msg.ID)
		if err != nil {
			log.Printf("outbox mark publishing failed: id=%d err=%v", msg.ID, err)
			continue
		}
		if !locked {
			continue
		}
		queueName := msg.QueueName
		payload := msg.Payload
		if queueName == "" || payload == "" {
			queueName = mq.VideoPublishedQueueName
			payload = fmt.Sprintf(
				`{"event_type":%q,"video_id":%d,"author_id":%d,"title":%q}`,
				msg.EventType,
				msg.VideoID,
				msg.AuthorID,
				msg.Title,
			)
		}
		if err := rabbit.PublishJSONBody(ctx, queueName, payload); err != nil {
			log.Printf("Outbox publish failed: id=%d video_id=%d err=%v", msg.ID, msg.VideoID, err)

			if markErr := repo.RecordOutboxPublishFailure(ctx, msg.ID, err); markErr != nil {
				log.Printf("outbox record publish failure failed: id=%d err=%v", msg.ID, markErr)
			}
			continue
		}
		if err := repo.MarkOutboxPublished(ctx, msg.ID); err != nil {
			log.Printf("Outbox mark published failed; id=%d err=%v", msg.ID, err)
			continue
		}
		log.Printf("Outbox published: id=%d queue=%s event=%s video_id=%d", msg.ID, queueName, msg.EventType, msg.VideoID)
	}
}

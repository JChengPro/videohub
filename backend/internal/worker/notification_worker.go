package worker

import (
	"backend/internal/cache"
	"backend/internal/mq"
	"backend/internal/notification"
	"context"
	"fmt"
	"time"
)

type NotificationWorker struct {
	repo  *notification.Repository
	cache *cache.Client
}

func NewNotificationWorker(repo *notification.Repository, cacheClient *cache.Client) *NotificationWorker {
	return &NotificationWorker{repo: repo, cache: cacheClient}
}

func (w *NotificationWorker) Handle(ctx context.Context, event mq.NotificationEvent) error {
	if event.ReceiverID == 0 || event.ActorID == 0 || event.DedupKey == "" {
		return nil
	}
	if event.ReceiverID == event.ActorID {
		return nil
	}

	allowed, ownsCooldown := w.allowNotification(ctx, event)
	if !allowed {
		return nil
	}

	// Redis 冷却 key 只是一张“30 秒内不要再次通知”的临时纸条。
	// notifications 表中的这条记录才是用户最终能看到的正式通知。
	err := w.repo.CreateOnce(ctx, &notification.Notification{
		ReceiverID: event.ReceiverID,
		ActorID:    event.ActorID,
		Type:       event.Type,
		TargetType: event.TargetType,
		TargetID:   event.TargetID,
		Content:    event.Content,
		DedupKey:   event.DedupKey,
	})

	if err != nil && ownsCooldown {
		// ownsCooldown=true 表示 Redis 中的冷却 key 是当前事件刚创建的，
		// 就像“这张 30 秒内别再通知的纸条是我贴的”。
		//
		// 如果正式通知写入 MySQL 失败，但纸条仍然留在 Redis，
		// RabbitMQ 重试时可能误以为通知已经生成，从而跳过本次通知。
		// 因此当前事件需要删除自己创建的冷却 key，让 MQ 能够重新处理。
		//
		// ownsCooldown=false 时，冷却 key 不是当前事件创建的，
		// 当前事件不能删除它，否则会破坏其他点赞事件的 30 秒冷却限制。
		_ = w.cache.Del(ctx, fmt.Sprintf(
			"notification:cooldown:like:%d:%d:%d",
			event.ReceiverID,
			event.ActorID,
			event.TargetID,
		))
	}
	return err
}

func (w *NotificationWorker) allowNotification(
	ctx context.Context,
	event mq.NotificationEvent,
) (allowed bool, ownsCooldown bool) {
	// allowed：本次事件能不能向 MySQL 写正式通知。
	// ownsCooldown：Redis 冷却 key 是否由本次事件创建。
	// 只有 ownsCooldown=true 时，本次事件才能在 MySQL 写入失败后删除该 key。

	// 评论每一条都要通知，暂时只限制重复点赞。
	if event.Type != "like" || w.cache == nil {
		return true, false
	}

	key := fmt.Sprintf(
		"notification:cooldown:like:%d:%d:%d",
		event.ReceiverID,
		event.ActorID,
		event.TargetID,
	)

	acquired, err := w.cache.SetNX(ctx, key, event.EventID, 30*time.Second)
	if err != nil {
		//redis异常时采用 fail-open，优先保证通知不丢失
		return true, false
	}

	if acquired {
		// 当前事件成功创建了 Redis 冷却 key：
		// 允许继续写通知；如果写 MySQL 失败，也由当前事件负责删除该 key。
		return true, true
	}

	existingEventID, err := w.cache.Get(ctx, key)
	if err != nil {
		return true, false
	}

	// 相同 event_id 说明是同一条 MQ 消息重试，仍允许继续执行，
	// 后续由 MySQL 唯一索引保证不会重复插入。
	if existingEventID == event.EventID {
		return true, false
	}

	// 不同 event_id 说明用户在 30 秒内重新点赞，跳过本次通知。
	return false, false
}

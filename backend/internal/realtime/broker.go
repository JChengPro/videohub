package realtime

import (
	"backend/internal/cache"
	"context"
	"encoding/json"
	"errors"
	"log"
	"time"
)

type Dispatcher struct {
	cache *cache.Client
	hub   *Hub
}

func NewDispatcher(cacheClient *cache.Client, hub *Hub) *Dispatcher {
	return &Dispatcher{cache: cacheClient, hub: hub}
}

func (d *Dispatcher) Publish(ctx context.Context, accountID uint, event Event) {
	if accountID == 0 {
		return
	}
	if d != nil && d.cache != nil {
		if err := Publish(ctx, d.cache, accountID, event); err == nil {
			return
		}
	}
	if d != nil && d.hub != nil {
		d.hub.Publish(accountID, event)
	}
}

func Publish(ctx context.Context, cacheClient *cache.Client, accountID uint, event Event) error {
	if cacheClient == nil {
		return errors.New("redis unavailable")
	}
	payload, err := json.Marshal(RoutedEvent{AccountID: accountID, Event: event})
	if err != nil {
		return err
	}
	return cacheClient.Publish(ctx, RedisChannel, payload)
}

func StartSubscriber(ctx context.Context, cacheClient *cache.Client, hub *Hub) {
	if cacheClient == nil || hub == nil {
		return
	}
	for {
		pubsub := cacheClient.Subscribe(ctx, RedisChannel)
		if _, err := pubsub.Receive(ctx); err != nil {
			_ = pubsub.Close()
			if !waitForRetry(ctx) {
				return
			}
			log.Printf("realtime redis subscription retrying after error: %v", err)
			continue
		}

		channel := pubsub.Channel()
		reconnect := false
		for !reconnect {
			select {
			case <-ctx.Done():
				_ = pubsub.Close()
				return
			case message, ok := <-channel:
				if !ok {
					reconnect = true
					continue
				}
				var routed RoutedEvent
				if err := json.Unmarshal([]byte(message.Payload), &routed); err != nil {
					continue
				}
				hub.Publish(routed.AccountID, routed.Event)
			}
		}
		_ = pubsub.Close()
		if !waitForRetry(ctx) {
			return
		}
	}
}

func waitForRetry(ctx context.Context) bool {
	timer := time.NewTimer(2 * time.Second)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

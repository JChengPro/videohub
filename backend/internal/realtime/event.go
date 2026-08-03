package realtime

import (
	"encoding/json"
	"time"
)

const RedisChannel = "videohub:realtime"

type Event struct {
	Type      string `json:"type"`
	RequestID string `json:"request_id,omitempty"`
	Timestamp int64  `json:"timestamp"`
	Data      any    `json:"data,omitempty"`
}

type IncomingEvent struct {
	Type      string          `json:"type"`
	RequestID string          `json:"request_id,omitempty"`
	Data      json.RawMessage `json:"data,omitempty"`
}

type RoutedEvent struct {
	AccountID uint  `json:"account_id"`
	Event     Event `json:"event"`
}

func NewEvent(eventType string, data any) Event {
	return Event{
		Type:      eventType,
		Timestamp: time.Now().UnixMilli(),
		Data:      data,
	}
}

func ErrorEvent(requestID, code, message string) Event {
	event := NewEvent("error", map[string]any{
		"code":    code,
		"message": message,
	})
	event.RequestID = requestID
	return event
}

package mq

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRabbitMQNewChannelRejectsUnavailableConnection(t *testing.T) {
	tests := []struct {
		name   string
		rabbit *RabbitMQ
	}{
		{name: "nil receiver", rabbit: nil},
		{name: "missing connection", rabbit: &RabbitMQ{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			channel, err := tt.rabbit.NewChannel()
			if channel != nil {
				t.Fatal("expected no channel when the connection is unavailable")
			}
			if !errors.Is(err, errRabbitMQConnectionUnavailable) {
				t.Fatalf("expected connection error, got %v", err)
			}
		})
	}
}

func TestNilCloseIsSafe(t *testing.T) {
	var rabbit *RabbitMQ
	if err := rabbit.Close(); err != nil {
		t.Fatalf("nil RabbitMQ close returned an error: %v", err)
	}

	var channel *AMQPChannel
	if err := channel.Close(); err != nil {
		t.Fatalf("nil AMQPChannel close returned an error: %v", err)
	}
}

func TestAMQPChannelOperationsRejectUnavailableChannel(t *testing.T) {
	channel := &AMQPChannel{}
	ctx := context.Background()

	tests := []struct {
		name string
		run  func() error
	}{
		{name: "declare queue", run: func() error { return channel.DeclareQueue("test") }},
		{name: "publish text", run: func() error { return channel.Publish(ctx, "test", "hello") }},
		{name: "publish json body", run: func() error { return channel.PublishJSONBody(ctx, "test", `{}`) }},
		{name: "publish json", run: func() error { return channel.PublishJSON(ctx, "test", map[string]string{"message": "hello"}) }},
		{name: "consume", run: func() error {
			_, err := channel.Consume("test")
			return err
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.run(); !errors.Is(err, errAMQPChannelUnavailable) {
				t.Fatalf("expected unavailable channel error, got %v", err)
			}
		})
	}
}

func TestPublishHandlerReturnsUnavailableWithoutRabbitMQ(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/mq/publish", strings.NewReader(`{"message":"hello"}`))
	ctx.Request.Header.Set("Content-Type", "application/json")

	NewHandler(nil).Publish(ctx)

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status %d, got %d: %s", http.StatusServiceUnavailable, recorder.Code, recorder.Body.String())
	}
}

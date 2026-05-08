package mq

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

const TestQueueName = "feedsystem.test.queue"

type Handler struct {
	rabbit *RabbitMQ
}

func NewHandler(rabbit *RabbitMQ) *Handler {
	return &Handler{rabbit: rabbit}
}

type PublishRequest struct {
	Message string `json:"message"`
}

func (h *Handler) Publish(c *gin.Context) {
	var req PublishRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Message == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "message is required"})
		return
	}
	if err := h.rabbit.DeclareQueue(TestQueueName); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if err := h.rabbit.Publish(c.Request.Context(), TestQueueName, req.Message); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message": "published",
	})
}

func (h *Handler) PublishVideoEvent(c *gin.Context) {
	evevt := VideoPublishedEvent{
		EventType: "video_published",
		VideoID:   1,
		AuthorID:  1,
		Title:     "测试视频",
	}
	if err := h.rabbit.DeclareQueue(TestQueueName); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if err := h.rabbit.PublishJSON(c.Request.Context(), TestQueueName, evevt); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"message": "video event published",
	})
}

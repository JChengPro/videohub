package video

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func (h *Handler) MultipartInit(c *gin.Context) {
	accountID, ok := currentAccountID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid accountID"})
		return
	}
	var req MultipartInitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	result, err := h.service.InitMultipart(c.Request.Context(), accountID, req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *Handler) MultipartSignParts(c *gin.Context) {
	accountID, ok := currentAccountID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid accountID"})
		return
	}
	var req MultipartSignPartsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	result, err := h.service.SignMultipartParts(c.Request.Context(), accountID, req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"parts": result})
}

func (h *Handler) MultipartPartCompleted(c *gin.Context) {
	accountID, ok := currentAccountID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid accountID"})
		return
	}
	var req MultipartPartCompletedRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.service.ConfirmMultipartPart(c.Request.Context(), accountID, req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "part recorded"})
}

func (h *Handler) MultipartStatus(c *gin.Context) {
	accountID, ok := currentAccountID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid accountID"})
		return
	}
	var req MultipartTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	result, err := h.service.MultipartStatus(c.Request.Context(), accountID, req.TaskID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *Handler) MultipartComplete(c *gin.Context) {
	accountID, ok := currentAccountID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid accountID"})
		return
	}
	usernameValue, ok := c.Get("username")
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "username not found"})
		return
	}
	username, ok := usernameValue.(string)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "username has invalid type"})
		return
	}
	var req MultipartCompleteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	video, err := h.service.CompleteMultipart(c.Request.Context(), accountID, username, req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, video)
}

func (h *Handler) MultipartAbort(c *gin.Context) {
	accountID, ok := currentAccountID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid accountID"})
		return
	}
	var req MultipartTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.service.AbortMultipart(c.Request.Context(), accountID, req.TaskID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "upload aborted"})
}

func (h *Handler) MultipartTasks(c *gin.Context) {
	accountID, ok := currentAccountID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid accountID"})
		return
	}
	tasks, err := h.service.ListMultipartTasks(c.Request.Context(), accountID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"tasks": tasks})
}

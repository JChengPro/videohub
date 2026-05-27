package feed

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func currentAccountID(c *gin.Context) (uint, bool) {
	value, ok := c.Get("accountID")
	if !ok {
		return 0, false
	}
	accountID, ok := value.(uint)
	if !ok {
		return 0, false
	}
	return accountID, true
}

func (h *Handler) ListLatest(c *gin.Context) {
	var req ListLatestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	accountID, _ := currentAccountID(c)

	var latestBefore time.Time
	if req.LatestTime > 0 {
		latestBefore = time.UnixMilli(req.LatestTime)
	}
	resp, err := h.service.ListLatest(c.Request.Context(), req.Limit, latestBefore, accountID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *Handler) ListFollowing(c *gin.Context) {
	var req ListByFollowingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	value, ok := c.Get("accountID")
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "accountID not found"})
		return
	}

	accountID, ok := value.(uint)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "accountID has invalid type"})
		return
	}

	resp, err := h.service.ListFollowing(c.Request.Context(), accountID, req.Limit, req.LatestTime)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *Handler) ListLikesCount(c *gin.Context) {
	var req ListLikesCountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	accountID, _ := currentAccountID(c)

	resp, err := h.service.ListByLikesCount(c.Request.Context(), req.Limit, req.LikesCountBefore, req.IDBefore, accountID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *Handler) ListByPopularity(c *gin.Context) {
	var req ListByPopularityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	accountID, _ := currentAccountID(c)

	resp, err := h.service.ListByPopularity(c.Request.Context(), req.Limit, req.AsOf, req.Offset, accountID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}

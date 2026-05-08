package video

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type LikeHandler struct {
	Service *LikeService
}

func NewLikeHandler(service *LikeService) *LikeHandler {
	return &LikeHandler{Service: service}
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

func (h *LikeHandler) Like(c *gin.Context) {
	var req LikeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	accountID, ok := currentAccountID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid accountID"})
		return
	}
	if err := h.Service.Like(c.Request.Context(), req.VideoID, accountID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "liked"})
}

func (h *LikeHandler) UnLike(c *gin.Context) {
	var req LikeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	accountID, ok := currentAccountID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid accountID"})
		return
	}
	if err := h.Service.Unlike(c.Request.Context(), req.VideoID, accountID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "unliked"})
}

func (h *LikeHandler) IsLiked(c *gin.Context) {
	var req LikeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	accountID, ok := currentAccountID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid accountID"})
		return
	}
	isLiked, err := h.Service.IsLiked(c.Request.Context(), req.VideoID, accountID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, IsLikedResponse{IsLiked: isLiked})
}

func (h *LikeHandler) ListMyLikedVideos(c *gin.Context) {
	accountID, ok := currentAccountID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid accountID"})
		return
	}
	videos, err := h.Service.ListLikedVideos(c.Request.Context(), accountID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, videos)
}

package social

import (
	"errors"
	"io"
	"net/http"

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

func (h *Handler) Follow(c *gin.Context) {
	var req FollowRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	followerID, ok := currentAccountID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid accountID"})
		return
	}

	if err := h.service.Follow(c.Request.Context(), followerID, req.VloggerID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "followed"})
}

func (h *Handler) Unfollow(c *gin.Context) {
	var req FollowRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	followerID, ok := currentAccountID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid accountID"})
		return
	}

	if err := h.service.Unfollow(c.Request.Context(), followerID, req.VloggerID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "unfollowed"})
}

// GetFollowers 查询指定用户的粉丝；未传 vlogger_id 时查询当前用户。
func (h *Handler) GetFollowers(c *gin.Context) {
	currentID, ok := currentAccountID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid accountID"})
		return
	}

	var req GetFollowersRequest
	if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.VloggerID == 0 {
		req.VloggerID = currentID
	}

	followers, err := h.service.ListFollowers(c.Request.Context(), req.VloggerID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, GetAllFollowersResponse{Followers: followers})
}

// GetFollowing 查询指定用户关注的人；未传 follower_id 时查询当前用户。
func (h *Handler) GetFollowing(c *gin.Context) {
	currentID, ok := currentAccountID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid accountID"})
		return
	}

	var req GetFollowingRequest
	if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.FollowerID == 0 {
		req.FollowerID = currentID
	}

	following, err := h.service.ListFollowing(c.Request.Context(), req.FollowerID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, GetAllVloggersResponse{Vloggers: following})
}

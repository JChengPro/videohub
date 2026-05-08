package social

import (
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

//查询“我的粉丝”。

func (h *Handler) GetFollowers(c *gin.Context) {
	accountID, ok := currentAccountID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid accountID"})
		return
	}

	followers, err := h.service.ListFollowers(c.Request.Context(), accountID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, GetAllFollowersResponse{Followers: followers})
}

//查询“我关注的人”。

func (h *Handler) GetFollowing(c *gin.Context) {
	accountID, ok := currentAccountID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid accountID"})
		return
	}

	following, err := h.service.ListFollowing(c.Request.Context(), accountID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, GetAllVloggersResponse{Vloggers: following})
}

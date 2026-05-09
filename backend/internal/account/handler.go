package account

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

func (h *Handler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.service.Register(c.Request.Context(), req.Username, req.Password); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "account created"})
}

func (h *Handler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	token, err := h.service.Login(c.Request.Context(), req.Username, req.Password)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"token": token})
}

func (h *Handler) Me(c *gin.Context) {
	accountID, ok := c.Get("accountID")
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "accountID not found"})
		return
	}
	username, ok := c.Get("username")
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "username not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"account_id": accountID,
		"username":   username,
	})
}

func (h *Handler) Logout(c *gin.Context) {
	value, ok := c.Get("accountID")
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "accountID not found"})
		return
	}

	//因为 c.Get 返回的是 any，你要把它转回 uint。
	//真实值, 是否成功 := 某个接口值.(目标类型)
	accountID, ok := value.(uint)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "accountID has invalid type"})
		return
	}

	if err := h.service.Logout(c.Request.Context(), accountID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "account logged out"})
}

func (h *Handler) ChangePassword(c *gin.Context) {
	var req ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.service.ChangePassword(c.Request.Context(), req.Username, req.OldPassword, req.NewPassword); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "password changed"})
}

func (h *Handler) FindByID(ctx *gin.Context) {
	var req FindByIDRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	account, err := h.service.FindByID(ctx.Request.Context(), req.ID)
	if err != nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, account)
}

func (h *Handler) FindByUsername(c *gin.Context) {
	var req FindByUsernameRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	account, err := h.service.FindByUsername(c.Request.Context(), req.Username)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}
	c.JSON(http.StatusOK, account)
}

func (h *Handler) Rename(c *gin.Context) {
	var req RenameRequest
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
	token, err := h.service.Rename(c.Request.Context(), accountID, req.NewUsername)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"token": token})
}

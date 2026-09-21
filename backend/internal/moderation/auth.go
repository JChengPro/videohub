package moderation

import (
	"backend/internal/account"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

func fingerprint(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func newSecret() (string, error) {
	secret := make([]byte, 32)
	_, err := rand.Read(secret)
	return hex.EncodeToString(secret), err
}

func validRole(role string) bool { return role == "owner" || role == "reviewer" }

func sessionMember(db *gorm.DB, hash string) (Staff, error) {
	var session AdminSession
	var member Staff
	if err := db.Where("token_hash = ? AND expires_at > ?", hash, time.Now()).First(&session).Error; err != nil {
		return member, err
	}
	if err := db.First(&member, session.AccountID).Error; err != nil {
		return member, err
	}
	if member.Status != "active" || !validRole(member.Role) || fingerprint(member.Password) != session.PasswordFingerprint {
		return member, problem(401, "审核权限已失效")
	}
	return member, nil
}

func (h *Handler) Login(c *gin.Context) {
	var input account.LoginRequest
	if c.ShouldBindJSON(&input) != nil {
		c.JSON(400, gin.H{"error": "请输入账号和密码"})
		return
	}
	var user Staff
	err := h.db.WithContext(c.Request.Context()).Where("account_name = ?", strings.ToLower(strings.TrimSpace(input.AccountName))).First(&user).Error
	if err != nil || bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(input.Password)) != nil || user.Status != "active" || !validRole(user.Role) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "账号、密码错误或无审核权限"})
		return
	}
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		c.JSON(500, gin.H{"error": "无法创建会话"})
		return
	}
	token := hex.EncodeToString(secret)
	session := AdminSession{TokenHash: fingerprint(token), AccountID: user.ID, PasswordFingerprint: fingerprint(user.Password), ExpiresAt: time.Now().Add(8 * time.Hour)}
	if err := h.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		if err := lockStaffState(tx); err != nil {
			return err
		}
		var current Staff
		if err := tx.First(&current, user.ID).Error; err != nil {
			return err
		}
		if current.Password != user.Password || current.Status != "active" || !validRole(current.Role) {
			return problem(401, "账号权限已变更，请重新登录")
		}
		user = current
		if err := tx.Model(&user).Update("last_login_at", time.Now()).Error; err != nil {
			return err
		}
		return tx.Create(&session).Error
	}); err != nil {
		c.JSON(500, gin.H{"error": "无法创建会话"})
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(200, gin.H{"token": token, "username": user.Username, "account_id": user.ID, "role": user.Role, "expires_at": session.ExpiresAt})
}

func (h *Handler) Guard() gin.HandlerFunc {
	return func(c *gin.Context) {
		parts := strings.Fields(c.GetHeader("Authorization"))
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			c.AbortWithStatusJSON(401, gin.H{"error": "请登录审核账号"})
			return
		}
		var session AdminSession
		if err := h.db.WithContext(c.Request.Context()).Where("token_hash = ? AND expires_at > ?", fingerprint(parts[1]), time.Now()).First(&session).Error; err != nil {
			c.AbortWithStatusJSON(401, gin.H{"error": "审核登录已过期"})
			return
		}
		user, err := sessionMember(h.db.WithContext(c.Request.Context()), session.TokenHash)
		if err != nil {
			c.AbortWithStatusJSON(401, gin.H{"error": "审核权限已失效"})
			return
		}
		c.Header("Cache-Control", "no-store")
		c.Set("admin", user)
		c.Set("adminSession", session.TokenHash)
		c.Next()
	}
}

func (h *Handler) Me(c *gin.Context) {
	c.JSON(200, c.MustGet("admin").(Staff))
}

func (h *Handler) OwnerGuard() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.MustGet("admin").(Staff).Role != "owner" {
			c.AbortWithStatusJSON(403, gin.H{"error": "仅超级管理员可管理成员"})
			return
		}
		c.Next()
	}
}

func (h *Handler) Logout(c *gin.Context) {
	if err := h.db.WithContext(c.Request.Context()).Where("token_hash = ?", c.GetString("adminSession")).Delete(&AdminSession{}).Error; err != nil {
		c.JSON(500, gin.H{"error": "退出失败，请重试"})
		return
	}
	c.JSON(200, gin.H{"message": "已退出"})
}

func IsAdminSessionValid(db *gorm.DB, tokenHash string) bool {
	_, err := sessionMember(db, tokenHash)
	return err == nil
}

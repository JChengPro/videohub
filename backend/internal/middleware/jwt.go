package middleware

import (
	"backend/internal/account"
	"backend/internal/auth"
	"backend/internal/cache"
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

func parseBearerToken(c *gin.Context) (string, bool) {
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		return "", false
	}

	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return "", false
	}

	tokenString := strings.TrimSpace(parts[1])
	if tokenString == "" {
		return "", false
	}

	return tokenString, true
}

func tokenCacheKey(accountID uint) string {
	return fmt.Sprintf("account:%d", accountID)
}

type authStatus uint8

const (
	authStatusInvalid authStatus = iota
	authStatusValid
	authStatusReplaced
	authStatusRevoked
)

func setAuthContext(c *gin.Context, accountRepo *account.Repository, cacheClient *cache.Client, tokenString string) authStatus {
	claims, err := auth.ParseToken(tokenString)
	if err != nil {
		return authStatusInvalid
	}

	if cacheClient != nil {
		cacheCtx, cancel := context.WithTimeout(c.Request.Context(), 50*time.Millisecond)
		cachedToken, err := cacheClient.Get(cacheCtx, tokenCacheKey(claims.AccountID))
		cancel()

		if err == nil {
			if cachedToken != "" && cachedToken == tokenString {
				c.Set("accountID", claims.AccountID)
				c.Set("accountName", claims.AccountName)
				c.Set("username", claims.Username)
				return authStatusValid
			}
			if cachedToken == "" {
				return authStatusRevoked
			}
			return authStatusReplaced
		}
	}

	accountInfo, err := accountRepo.FindByID(c.Request.Context(), claims.AccountID)
	if err != nil {
		return authStatusInvalid
	}
	if accountInfo.Token == "" {
		return authStatusRevoked
	}
	if accountInfo.Token != tokenString {
		return authStatusReplaced
	}

	if cacheClient != nil {
		cacheCtx, cancel := context.WithTimeout(c.Request.Context(), 50*time.Millisecond)
		_ = cacheClient.Set(cacheCtx, tokenCacheKey(claims.AccountID), tokenString, 24*time.Hour)
		cancel()
	}

	c.Set("accountID", claims.AccountID)
	c.Set("accountName", claims.AccountName)
	c.Set("username", claims.Username)
	return authStatusValid
}

// 强鉴权：没 token / token 非法 都直接拦截
func JWTAuth(accountRepo *account.Repository, cacheClient *cache.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenString, ok := parseBearerToken(c)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing or invalid authorization header"})
			return
		}

		switch setAuthContext(c, accountRepo, cacheClient, tokenString) {
		case authStatusValid:
			c.Next()
		case authStatusReplaced:
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"code":  "SESSION_REPLACED",
				"error": "account signed in on another device",
			})
		case authStatusRevoked:
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"code":  "SESSION_REVOKED",
				"error": "session has been revoked",
			})
		default:
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"code":  "TOKEN_INVALID",
				"error": "invalid or expired token",
			})
		}
		if c.IsAborted() {
			return
		}
	}
}

// 软鉴权：尽量解析 token，失败也不拦截
func SoftJWTAuth(accountRepo *account.Repository, cacheClient *cache.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenString, ok := parseBearerToken(c)
		if !ok {
			c.Next()
			return
		}

		_ = setAuthContext(c, accountRepo, cacheClient, tokenString)
		c.Next()
	}
}

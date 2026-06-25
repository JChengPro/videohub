package ratelimit

import (
	"backend/internal/cache"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type KeyFunc func(*gin.Context) (string, bool)

func Limit(
	cacheClient *cache.Client,
	keyPrefix string,
	maxRequests int64,
	window time.Duration,
	keyFunc KeyFunc,
) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		if cacheClient == nil || keyFunc == nil || maxRequests <= 0 || window <= 0 {
			ctx.Next()
			return
		}
		subject, ok := keyFunc(ctx)
		if !ok {
			ctx.Next()
			return
		}
		key := buildKey(keyPrefix, subject)
		count, err := cacheClient.IncrementWithExpire(ctx, key, window)
		if err != nil {
			ctx.Next()
			return
		}
		if count > maxRequests {
			ctx.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "too many requests",
			})
			return
		}
		ctx.Next()
	}
}

func buildKey(keyPrefix, subject string) string {
	keyPrefix = strings.TrimSpace(keyPrefix)
	if keyPrefix == "" {
		keyPrefix = "default"
	}
	return fmt.Sprintf("feedsystem:ratelimit:%s:%s", keyPrefix, strings.TrimSpace(subject))
}

func KeyByIp(c *gin.Context) (string, bool) {
	ip := strings.TrimSpace(c.ClientIP())
	if ip == "" {
		return "", false
	}
	return ip, true
}

func KeyByAccount(c *gin.Context) (string, bool) {
	value, ok := c.Get("accountID")
	if !ok {
		return "", false
	}
	accountID, ok := value.(uint)
	if !ok || accountID == 0 {
		return "", false
	}
	return strconv.FormatUint(uint64(accountID), 10), true
}

package mediagate

import (
	"backend/internal/account"
	"backend/internal/moderation"
	"backend/internal/storage"
	"backend/internal/video"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"gorm.io/gorm"
)

type Handler struct {
	db      *gorm.DB
	storage storage.Storage
}

func New(db *gorm.DB, store storage.Storage) *Handler { return &Handler{db: db, storage: store} }

type claims struct {
	VideoID   uint   `json:"video_id"`
	AccountID uint   `json:"account_id,omitempty"`
	Session   string `json:"session"`
	Admin     bool   `json:"admin,omitempty"`
	Kind      string `json:"kind"`
	jwt.RegisteredClaims
}

func secret() []byte {
	value := os.Getenv("JWT_SECRET")
	if value == "" {
		value = "videohub-secret"
	}
	return []byte(value)
}
func hash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func (h *Handler) PreviewURLs(c *gin.Context, target *video.Video) error {
	target.PlayURL = ""
	target.CoverURL = ""
	base := claims{VideoID: target.ID, RegisteredClaims: jwt.RegisteredClaims{Audience: jwt.ClaimStrings{"media-preview"}, ExpiresAt: jwt.NewNumericDate(time.Now().Add(5 * time.Minute))}}
	if session := c.GetString("adminSession"); session != "" {
		base.Admin = true
		base.Session = session
	} else {
		base.AccountID = c.GetUint("accountID")
		base.Session = hash(strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer "))
	}
	for _, kind := range []string{"play", "cover"} {
		if kind == "play" && target.PlayObjectKey == "" || kind == "cover" && target.CoverObjectKey == "" {
			continue
		}
		base.Kind = kind
		token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, base).SignedString(secret())
		if err != nil {
			return err
		}
		link := "/api/media/preview?ticket=" + url.QueryEscape(token)
		if kind == "play" {
			target.PlayURL = link
		} else {
			target.CoverURL = link
		}
	}
	return nil
}

func (h *Handler) Preview(c *gin.Context) {
	var claim claims
	token, err := jwt.ParseWithClaims(c.Query("ticket"), &claim, func(t *jwt.Token) (any, error) { return secret(), nil }, jwt.WithValidMethods([]string{"HS256"}), jwt.WithAudience("media-preview"), jwt.WithExpirationRequired())
	if err != nil || !token.Valid {
		c.Status(401)
		return
	}
	var target video.Video
	if h.db.WithContext(c.Request.Context()).First(&target, claim.VideoID).Error != nil || target.Status == video.VideoStatusDeleted {
		c.Status(404)
		return
	}
	if claim.Admin {
		if !moderation.IsAdminSessionValid(h.db.WithContext(c.Request.Context()), claim.Session) {
			c.Status(403)
			return
		}
	} else {
		var user account.Account
		if claim.AccountID != target.AuthorID || h.db.WithContext(c.Request.Context()).First(&user, claim.AccountID).Error != nil || user.Token == "" || hash(user.Token) != claim.Session {
			c.Status(403)
			return
		}
	}
	key := target.PlayObjectKey
	if claim.Kind == "cover" {
		key = target.CoverObjectKey
	} else if claim.Kind != "play" {
		c.Status(400)
		return
	}
	c.Header("Cache-Control", "private, no-store")
	c.Header("Referrer-Policy", "no-referrer")
	h.serve(c, key)
}

// Static preserves published media and avatars, but never exposes upload directories.
func (h *Handler) Static(c *gin.Context) {
	key := strings.TrimPrefix(c.Param("filepath"), "/")
	if key == "" || path.Clean(key) != key || strings.Contains(key, "\\") {
		c.Status(404)
		return
	}
	var count int64
	var err error
	if strings.HasPrefix(key, "avatars/") {
		err = h.db.WithContext(c.Request.Context()).Model(&account.Account{}).Where("avatar_object_key = ?", key).Count(&count).Error
	} else {
		err = h.db.WithContext(c.Request.Context()).Model(&video.Video{}).Where("status = ? AND (play_object_key = ? OR cover_object_key = ?)", video.VideoStatusPublished, key, key).Count(&count).Error
	}
	if err != nil {
		c.Status(503)
		return
	}
	if count == 0 {
		c.Status(404)
		return
	}
	c.Header("Cache-Control", "no-store")
	h.serve(c, key)
}

func (h *Handler) serve(c *gin.Context, key string) {
	if key == "" {
		c.Status(404)
		return
	}
	c.Header("X-Content-Type-Options", "nosniff")
	if _, local := h.storage.(*storage.LocalStorage); local {
		reader, err := h.storage.Open(c.Request.Context(), key)
		if err != nil {
			c.Status(404)
			return
		}
		defer reader.Close()
		seeker, ok := reader.(io.ReadSeeker)
		if !ok {
			c.Status(500)
			return
		}
		http.ServeContent(c.Writer, c.Request, path.Base(key), time.Time{}, seeker)
		return
	}
	// Proxy private OSS media so the origin signature is never returned to the browser.
	link, err := h.storage.URL(c.Request.Context(), key, time.Minute)
	if err != nil {
		c.Status(502)
		return
	}
	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodGet, link, nil)
	if err != nil {
		c.Status(502)
		return
	}
	if value := c.GetHeader("Range"); value != "" {
		req.Header.Set("Range", value)
	}
	resp, err := (&http.Client{Timeout: 2 * time.Minute}).Do(req)
	if err != nil {
		c.Status(502)
		return
	}
	defer resp.Body.Close()
	for _, name := range []string{"Content-Type", "Content-Length", "Content-Range", "Accept-Ranges"} {
		if value := resp.Header.Get(name); value != "" {
			c.Header(name, value)
		}
	}
	c.Status(resp.StatusCode)
	if c.Request.Method != http.MethodHead {
		_, _ = io.Copy(c.Writer, resp.Body)
	}
}

func (h *Handler) Mine(c *gin.Context) {
	var input struct {
		Limit  int `json:"limit"`
		Offset int `json:"offset"`
	}
	if c.ShouldBindJSON(&input) != nil || input.Offset < 0 {
		c.JSON(400, gin.H{"error": "分页参数错误"})
		return
	}
	if input.Limit <= 0 || input.Limit > 50 {
		input.Limit = 20
	}
	items := []video.Video{}
	query := h.db.WithContext(c.Request.Context()).Model(&video.Video{}).Where("author_id = ? AND status <> ?", c.GetUint("accountID"), video.VideoStatusDeleted)
	var total int64
	if query.Count(&total).Error != nil || query.Order("id DESC").Offset(input.Offset).Limit(input.Limit).Find(&items).Error != nil {
		c.JSON(500, gin.H{"error": "无法读取投稿"})
		return
	}
	for i := range items {
		if err := h.PreviewURLs(c, &items[i]); err != nil {
			c.JSON(500, gin.H{"error": "无法加载预览"})
			return
		}
	}
	c.JSON(200, gin.H{"items": items, "total": total})
}

func (h *Handler) Submission(c *gin.Context) {
	var input video.DetailRequest
	if c.ShouldBindJSON(&input) != nil || input.ID == 0 {
		c.JSON(400, gin.H{"error": "视频编号无效"})
		return
	}
	var target video.Video
	if h.db.WithContext(c.Request.Context()).Where("id = ? AND author_id = ? AND status <> ?", input.ID, c.GetUint("accountID"), video.VideoStatusDeleted).First(&target).Error != nil {
		c.JSON(404, gin.H{"error": "投稿不存在"})
		return
	}
	if err := h.PreviewURLs(c, &target); err != nil {
		c.JSON(500, gin.H{"error": "无法加载预览"})
		return
	}
	c.JSON(200, target)
}

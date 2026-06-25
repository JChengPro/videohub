package video

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"backend/internal/storage"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	service     *Service
	fileStorage storage.Storage
}

func NewHandler(service *Service, fileStorage storage.Storage) *Handler {
	return &Handler{
		service:     service,
		fileStorage: fileStorage,
	}
}

// 生成随机文件名，避免用户上传同名文件互相覆盖
func randHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func buildAbsoluteURL(c *gin.Context, p string) string {
	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}
	if xf := c.GetHeader("X-Forwarded-Proto"); xf != "" {
		scheme = xf
	}
	return fmt.Sprintf("%s://%s%s", scheme, c.Request.Host, p)
}

// 下载封面
func (h *Handler) UploadCover(c *gin.Context) {
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
	f, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing file"})
		return
	}
	const maxSize = 10 << 20
	if f.Size <= 0 || f.Size > maxSize {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "cover file is too large"})
		return
	}
	ext := strings.ToLower(filepath.Ext(f.Filename))
	switch ext {
	case ".jpg", ".jpeg", ".png", ".webp":
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "only .jpg/.jpeg/.png/.webp is allowed"})
		return
	}

	date := time.Now().Format("20060102")
	filename := randHex(16) + ext

	// 本地存储和 OSS 使用相同的稳定对象路径。
	objectKey := path.Join(
		"covers",
		strconv.FormatUint(uint64(accountID), 10),
		date,
		filename,
	)

	src, err := f.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer src.Close()

	// Handler 只依赖 Storage 接口，不关心文件最终保存在本地还是 OSS。
	if err := h.fileStorage.Upload(c.Request.Context(), objectKey, src); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 私有 OSS 返回临时签名 URL，本地存储返回静态文件地址。
	coverURL, err := h.fileStorage.URL(c.Request.Context(), objectKey, 15*time.Minute)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"cover_url":  coverURL,
		"object_key": objectKey,
	})
}

func (h *Handler) UploadVideo(c *gin.Context) {
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

	f, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing file"})
		return
	}

	const maxSize = 200 << 20
	if f.Size <= 0 || f.Size > maxSize {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "video file is too large"})
		return
	}

	ext := strings.ToLower(filepath.Ext(f.Filename))
	ext, err = normalizeVideoExtension(ext)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	date := time.Now().Format("20060102")
	filename := randHex(16) + ext
	objectKey := path.Join(
		"videos",
		strconv.FormatUint(uint64(accountID), 10),
		date,
		filename,
	)

	src, err := f.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer src.Close()

	if err := h.fileStorage.Upload(c.Request.Context(), objectKey, src); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	playURL, err := h.fileStorage.URL(c.Request.Context(), objectKey, time.Hour)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"play_url":   playURL,
		"object_key": objectKey,
	})
}

// 在 handler.go 里加 Publish 方法
func (h *Handler) Publish(c *gin.Context) {
	var req PublishRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}

	accountValue, ok := c.Get("accountID")
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "accountID not found"})
		return
	}

	accountID, ok := accountValue.(uint)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "accountID has invalid type"})
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
	video := &Video{
		AuthorID:       accountID,
		Username:       username,
		Title:          req.Title,
		Description:    req.Description,
		PlayURL:        req.PlayURL,
		CoverURL:       req.CoverURL,
		PlayObjectKey:  req.PlayObjectKey,
		CoverObjectKey: req.CoverObjectKey,
		CreateTime:     time.Now(),
	}

	if err := h.service.Publish(c.Request.Context(), video); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, video)
}

func (h *Handler) Detail(ctx *gin.Context) {
	var req DetailRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	video, err := h.service.Detail(ctx.Request.Context(), req.ID)
	if err != nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, video)
}

func (h *Handler) ListByAuthor(c *gin.Context) {
	var req ListByAuthorRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	videos, err := h.service.ListByAuthor(c.Request.Context(), req.AuthorID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, videos)
}

func (h *Handler) Delete(c *gin.Context) {
	var req DeleteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	accountValue, ok := c.Get("accountID")
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "accountID not found"})
		return
	}

	accountID, ok := accountValue.(uint)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "accountID has invalid type"})
		return
	}

	if err := h.service.Delete(c.Request.Context(), req.ID, accountID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "video deleted",
	})
}

// 分片上传-接受一片，存临时磁盘
func (h *Handler) UploadChunk(c *gin.Context) {
	fileID := c.GetHeader("X-File-ID")
	if err := validateUploadFileID(fileID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	chunkIndex, indexErr := strconv.Atoi(c.GetHeader("X-Chunk-Index"))
	totalChunks, totalErr := strconv.Atoi(c.GetHeader("X-Total-Chunks"))
	if indexErr != nil || totalErr != nil || chunkIndex < 0 || totalChunks <= 0 || chunkIndex >= totalChunks || totalChunks > 1000 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid chunk index or total"})
		return
	}

	// 读 Body 原始字节——前端发的是纯二进制 blob，不是 JSON
	const maxChunkSize = 6 << 20
	data, err := io.ReadAll(io.LimitReader(c.Request.Body, maxChunkSize+1))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if len(data) == 0 || len(data) > maxChunkSize {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "invalid chunk size"})
		return
	}

	// 存到 .run/uploads/chunks/{fileID}/{chunkIndex}
	dir := filepath.Join(".run", "uploads", "chunks", fileID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	chunkPath := filepath.Join(dir, fmt.Sprintf("%d", chunkIndex))
	if err := os.WriteFile(chunkPath, data, 0644); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"chunk": chunkIndex})
}

// 拼接视频
func (h *Handler) MergeChunks(c *gin.Context) {
	var req MergeChunksRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	accountID, ok := currentAccountID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid accountID"})
		return
	}

	playURL, objectKey, err := h.service.MergeChunks(c.Request.Context(), req.FileID, req.FileExt, accountID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"play_url":   playURL,
		"object_key": objectKey,
	})
}

// ChunkStatus — 查已传片
func (h *Handler) ChunkStatus(c *gin.Context) {
	var req ChunkStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := validateUploadFileID(req.FileID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	dir := filepath.Join(".run", "uploads", "chunks", req.FileID)
	entries, err := os.ReadDir(dir)
	if err != nil {
		c.JSON(http.StatusOK, ChunkStatusResponse{Uploaded: []int{}})
		return
	}

	uploaded := make([]int, 0, len(entries))
	for _, e := range entries {
		if idx, err := strconv.Atoi(e.Name()); err == nil {
			uploaded = append(uploaded, idx)
		}
	}
	c.JSON(http.StatusOK, ChunkStatusResponse{Uploaded: uploaded})
}

package account

import (
	"fmt"
	"hash/fnv"
	"html"
	"net/http"
	"strconv"
	"strings"
	"time"

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
	if err := h.service.Register(c.Request.Context(), req.AccountName, req.Username, req.Password); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	accountName := req.AccountName
	if strings.TrimSpace(accountName) == "" {
		accountName = req.Username
	}
	c.JSON(http.StatusOK, RegisterResponse{
		Message:     "account created",
		AccountName: NormalizeAccountName(accountName),
		Username:    normalizeUsername(req.Username),
	})
}

func (h *Handler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	identifier := req.AccountName
	if strings.TrimSpace(identifier) == "" {
		identifier = req.Username
	}
	token, err := h.service.Login(c.Request.Context(), identifier, req.Password)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"token": token})
}

func (h *Handler) Me(c *gin.Context) {
	value, ok := c.Get("accountID")
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "accountID not found"})
		return
	}
	accountID, ok := value.(uint)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "accountID has invalid type"})
		return
	}
	account, err := h.service.FindByID(c.Request.Context(), accountID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "account not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"id":           account.ID,
		"account_id":   account.ID,
		"account_name": account.AccountName,
		"username":     account.Username,
		"avatar_url":   fmt.Sprintf("/api/account/avatar/%d", account.ID),
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
	value, ok := c.Get("accountID")
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "accountID not found"})
		return
	}
	accountID, ok := value.(uint)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "accountID has invalid type"})
		return
	}
	if err := h.service.ChangePassword(c.Request.Context(), accountID, req.OldPassword, req.NewPassword); err != nil {
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

func (h *Handler) Search(c *gin.Context) {
	var req SearchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	response, err := h.service.Search(c.Request.Context(), req.Query, req.Limit, req.Offset)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, response)
}

func (h *Handler) CheckAccountName(c *gin.Context) {
	var req CheckAccountNameRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	response, err := h.service.CheckAccountName(c.Request.Context(), req.AccountName)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, response)
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

func (h *Handler) UploadAvatar(c *gin.Context) {
	value, ok := c.Get("accountID")
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "accountID not found"})
		return
	}
	accountID, ok := value.(uint)
	if !ok || accountID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "accountID has invalid type"})
		return
	}
	fileHeader, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请选择头像图片"})
		return
	}
	if fileHeader.Size <= 0 || fileHeader.Size > maxAvatarSize {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "头像文件不能超过 5MB"})
		return
	}
	file, err := fileHeader.Open()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无法读取头像文件"})
		return
	}
	defer file.Close()
	if _, err := h.service.UploadAvatar(c.Request.Context(), accountID, file); err != nil {
		status := http.StatusBadRequest
		if strings.Contains(err.Error(), "storage") {
			status = http.StatusInternalServerError
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, AvatarResponse{
		AvatarURL: fmt.Sprintf("/api/account/avatar/%d?v=%d", accountID, time.Now().UnixNano()),
	})
}

var defaultAvatarPalettes = [][2]string{
	{"#21D4FD", "#4F46E5"},
	{"#FF5C7C", "#FF8A3D"},
	{"#A855F7", "#EC4899"},
	{"#14B8A6", "#22C55E"},
	{"#F59E0B", "#EF4444"},
	{"#6366F1", "#06B6D4"},
	{"#F43F5E", "#8B5CF6"},
	{"#84CC16", "#0EA5E9"},
}

func defaultAvatarSVG(account *Account) string {
	hash := fnv.New32a()
	_, _ = hash.Write([]byte(fmt.Sprintf("%d:%s", account.ID, account.AccountName)))
	palette := defaultAvatarPalettes[int(hash.Sum32())%len(defaultAvatarPalettes)]
	initial := "V"
	if runes := []rune(strings.TrimSpace(account.Username)); len(runes) > 0 {
		initial = strings.ToUpper(string(runes[0]))
	}
	return fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 256 256" role="img" aria-label="%s 的默认头像">
<defs>
<linearGradient id="bg" x1="18%%" y1="8%%" x2="86%%" y2="92%%"><stop stop-color="%s"/><stop offset="1" stop-color="%s"/></linearGradient>
<radialGradient id="glow" cx="25%%" cy="18%%" r="78%%"><stop stop-color="#fff" stop-opacity=".48"/><stop offset=".7" stop-color="#fff" stop-opacity="0"/></radialGradient>
</defs>
<rect width="256" height="256" rx="128" fill="url(#bg)"/>
<circle cx="58" cy="46" r="92" fill="url(#glow)"/>
<path d="M-8 202c48-42 91-52 133-29 45 24 87 16 139-36v127H-8Z" fill="#09090b" opacity=".13"/>
<circle cx="202" cy="62" r="28" fill="#fff" opacity=".12"/>
<text x="128" y="154" text-anchor="middle" font-family="Inter,Arial,sans-serif" font-size="105" font-weight="800" fill="#fff" opacity=".96">%s</text>
</svg>`, html.EscapeString(account.Username), palette[0], palette[1], html.EscapeString(initial))
}

func (h *Handler) Avatar(c *gin.Context) {
	accountID64, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || accountID64 == 0 {
		c.Status(http.StatusNotFound)
		return
	}
	account, avatarURL, err := h.service.AvatarURL(c.Request.Context(), uint(accountID64))
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	c.Header("Cache-Control", "no-cache, must-revalidate")
	if avatarURL != "" {
		c.Redirect(http.StatusTemporaryRedirect, avatarURL)
		return
	}
	c.Data(http.StatusOK, "image/svg+xml; charset=utf-8", []byte(defaultAvatarSVG(account)))
}

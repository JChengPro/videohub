package moderation

import (
	"backend/internal/account"
	"backend/internal/auth"
	"backend/internal/notification"
	"backend/internal/video"
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestModerationIntegration(t *testing.T) {
	dsn := os.Getenv("MODERATION_TEST_DSN")
	if dsn == "" {
		t.Skip("set MODERATION_TEST_DSN to an isolated MySQL test database")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&account.Account{}, &AdminSession{}, &Review{}, &video.Video{}, &video.OutboxMsg{}, &notification.Notification{}); err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&Staff{}, &StaffState{}, &StaffLink{}, &StaffAudit{}); err != nil {
		t.Fatal(err)
	}
	if err := MigrateStaff(db); err != nil {
		t.Fatal(err)
	}
	password, _ := bcrypt.GenerateFromPassword([]byte("ReviewTest123!"), bcrypt.MinCost)
	name := time.Now().Format("150405000000")
	admin := account.Account{AccountName: "a" + name, Username: "reviewer", Password: string(password), Role: "admin", Token: "community-session"}
	user := account.Account{AccountName: "u" + name, Username: "author", Password: string(password)}
	for _, a := range []*account.Account{&admin, &user} {
		if err := db.Create(a).Error; err != nil {
			t.Fatal(err)
		}
	}
	staff := Staff{AccountName: admin.AccountName, Username: admin.Username, Password: admin.Password, Role: "owner", Status: "active"}
	if err := db.Create(&staff).Error; err != nil {
		t.Fatal(err)
	}
	h := NewHandler(db, nil)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/login", h.Login)
	g := r.Group("", h.Guard())
	g.POST("/me", h.Me)
	g.POST("/logout", h.Logout)
	g.POST("/decide", h.Decide)
	call := func(path string, body any, token string) *httptest.ResponseRecorder {
		data, _ := json.Marshal(body)
		req := httptest.NewRequest("POST", path, bytes.NewReader(data))
		req.Header.Set("Content-Type", "application/json")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		out := httptest.NewRecorder()
		r.ServeHTTP(out, req)
		return out
	}
	var token string
	t.Run("independent_admin_login_and_permissions", func(t *testing.T) {
		bad := call("/login", gin.H{"account_name": user.AccountName, "password": "ReviewTest123!"}, "")
		if bad.Code != 401 {
			t.Fatalf("ordinary user: %d", bad.Code)
		}
		ok := call("/login", gin.H{"account_name": admin.AccountName, "password": "ReviewTest123!"}, "")
		if ok.Code != 200 {
			t.Fatal(ok.Body.String())
		}
		var data struct {
			Token string `json:"token"`
		}
		json.Unmarshal(ok.Body.Bytes(), &data)
		token = data.Token
		var saved account.Account
		db.First(&saved, admin.ID)
		if saved.Token != "community-session" {
			t.Fatal("admin login changed community token")
		}
		jwt, _ := auth.GenerateToken(admin.ID, admin.AccountName, admin.Username)
		if call("/me", gin.H{}, jwt).Code != 401 {
			t.Fatal("community JWT accepted by admin API")
		}
		if call("/me", gin.H{}, token).Code != 200 {
			t.Fatal("admin denied")
		}
	})
	newVideo := func() video.Video {
		v := video.Video{AuthorID: user.ID, Username: user.Username, Title: "integration", Status: video.VideoStatusPendingReview, PlayObjectKey: "videos/test.mp4"}
		if err := db.Create(&v).Error; err != nil {
			t.Fatal(err)
		}
		return v
	}
	t.Run("concurrent_review_exactly_once", func(t *testing.T) {
		v := newVideo()
		codes := make(chan int, 2)
		var wg sync.WaitGroup
		for range 2 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				codes <- call("/decide", gin.H{"id": v.ID, "decision": "approve"}, token).Code
			}()
		}
		wg.Wait()
		close(codes)
		counts := map[int]int{}
		for code := range codes {
			counts[code]++
		}
		if counts[200] != 1 || counts[409] != 1 {
			t.Fatalf("codes %v", counts)
		}
		db.First(&v, v.ID)
		if v.Status != video.VideoStatusPublished || v.PublishedAt == nil {
			t.Fatal("not published")
		}
		for _, model := range []any{&Review{}, &video.OutboxMsg{}} {
			var count int64
			query := db.Model(model).Where("video_id = ?", v.ID)
			if _, ok := model.(*video.OutboxMsg); ok {
				query = query.Where("event_type = ?", "video_published")
			}
			query.Count(&count)
			if count != 1 {
				t.Fatalf("%T count %d", model, count)
			}
		}
		var count int64
		db.Model(&video.OutboxMsg{}).Where("video_id = ? AND event_type = ?", v.ID, "notification_review").Count(&count)
		if count != 1 {
			t.Fatal("notification missing")
		}
	})
	t.Run("reject_requires_reason_and_cannot_publish", func(t *testing.T) {
		v := newVideo()
		if call("/decide", gin.H{"id": v.ID, "decision": "reject"}, token).Code != 400 {
			t.Fatal("empty rejection accepted")
		}
		if call("/decide", gin.H{"id": v.ID, "decision": "reject", "reason": "封面与内容不符"}, token).Code != 200 {
			t.Fatal("reject failed")
		}
		db.First(&v, v.ID)
		if v.Status != video.VideoStatusRejected || v.ReviewReason == "" {
			t.Fatal("rejection missing")
		}
		var count int64
		db.Model(&video.OutboxMsg{}).Where("video_id = ? AND event_type = ?", v.ID, "video_published").Count(&count)
		if count != 0 {
			t.Fatal("rejected video published event")
		}
		if call("/decide", gin.H{"id": v.ID, "decision": "approve"}, token).Code != 409 {
			t.Fatal("rejected video approved")
		}
	})
	t.Run("withdrawn_video_cannot_be_approved", func(t *testing.T) {
		v := newVideo()
		db.Model(&v).Update("status", video.VideoStatusDeleted)
		if call("/decide", gin.H{"id": v.ID, "decision": "approve"}, token).Code != 409 {
			t.Fatal("deleted video approved")
		}
	})
	t.Run("role_revocation_and_logout", func(t *testing.T) {
		db.Model(&staff).Update("status", "disabled")
		if call("/me", gin.H{}, token).Code != 401 {
			t.Fatal("revoked role accepted")
		}
		db.Model(&staff).Update("status", "active")
		if call("/logout", gin.H{}, token).Code != 200 || call("/me", gin.H{}, token).Code != 401 {
			t.Fatal("logout failed")
		}
	})
}

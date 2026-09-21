package mediagate

import (
	"backend/internal/account"
	"backend/internal/moderation"
	"backend/internal/storage"
	"backend/internal/video"
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestPrivateMediaIntegration(t *testing.T) {
	dsn := os.Getenv("MODERATION_TEST_DSN")
	if dsn == "" {
		t.Skip("requires isolated MySQL")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&account.Account{}, &video.Video{}, &video.OutboxMsg{}, &moderation.AdminSession{}); err != nil {
		t.Fatal(err)
	}
	user := account.Account{AccountName: "m" + time.Now().Format("150405000000"), Username: "media", Token: "owner-session"}
	db.Create(&user)
	store := storage.NewLocalStorage(t.TempDir(), "")
	key := "videos/private.mp4"
	store.Upload(t.Context(), key, strings.NewReader("0123456789"))
	v := video.Video{AuthorID: user.ID, Title: "private", Status: video.VideoStatusProcessing, PlayObjectKey: key}
	db.Create(&v)
	repo := video.NewRepository(db)
	t.Run("processing_stops_before_publication", func(t *testing.T) {
		ok, err := repo.CompleteProcessingWithOutbox(t.Context(), &v)
		if err != nil || !ok {
			t.Fatal(err)
		}
		db.First(&v, v.ID)
		if v.Status != video.VideoStatusPendingReview {
			t.Fatal(v.Status)
		}
		var n int64
		db.Model(&video.OutboxMsg{}).Where("video_id = ?", v.ID).Count(&n)
		if n != 0 {
			t.Fatal("processing emitted publication")
		}
	})
	h := New(db, store)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/static/*filepath", h.Static)
	r.GET("/media/preview", h.Preview)
	r.POST("/submission", func(c *gin.Context) {
		c.Set("accountID", user.ID)
		c.Request.Header.Set("Authorization", "Bearer owner-session")
		h.Submission(c)
	})
	call := func(method, path, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Range", "bytes=2-5")
		out := httptest.NewRecorder()
		r.ServeHTTP(out, req)
		return out
	}
	var preview string
	t.Run("private_url_and_range_preview", func(t *testing.T) {
		if call("GET", "/static/"+key, "").Code != 404 {
			t.Fatal("pending static media leaked")
		}
		data, _ := json.Marshal(gin.H{"id": v.ID})
		res := call("POST", "/submission", string(data))
		if res.Code != 200 {
			t.Fatal(res.Body.String())
		}
		var result video.Video
		json.Unmarshal(res.Body.Bytes(), &result)
		preview = strings.TrimPrefix(result.PlayURL, "/api")
		resp := call("GET", preview, "")
		if resp.Code != 206 || !bytes.Equal(resp.Body.Bytes(), []byte("2345")) {
			t.Fatalf("range response %d %s", resp.Code, resp.Body.String())
		}
		other := video.Video{AuthorID: user.ID + 1, Title: "other", Status: video.VideoStatusPendingReview}
		db.Create(&other)
		data, _ = json.Marshal(gin.H{"id": other.ID})
		if call("POST", "/submission", string(data)).Code != 404 {
			t.Fatal("other author's draft leaked")
		}
	})
	t.Run("revoked_preview_and_publication", func(t *testing.T) {
		db.Model(&user).Update("token", "")
		if call("GET", preview, "").Code != 403 {
			t.Fatal("logged-out preview accepted")
		}
		db.Model(&v).Update("status", video.VideoStatusPublished)
		if call("GET", "/static/"+key, "").Code != 206 {
			t.Fatal("published media unavailable")
		}
		db.Model(&v).Update("status", video.VideoStatusDeleted)
		if call("GET", "/static/"+key, "").Code != 404 {
			t.Fatal("deleted media leaked")
		}
	})
}

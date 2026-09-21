package moderation

import (
	"backend/internal/account"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestStaffIntegration(t *testing.T) {
	dsn := os.Getenv("STAFF_TEST_DSN")
	if dsn == "" {
		t.Skip("set STAFF_TEST_DSN to an isolated *_staff_test MySQL database")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	var database string
	db.Raw("SELECT DATABASE()").Scan(&database)
	if !strings.HasSuffix(database, "_staff_test") {
		t.Fatal("dedicated staff test database required")
	}
	models := []any{&account.Account{}, &Staff{}, &StaffState{}, &StaffLink{}, &StaffAudit{}, &AdminSession{}, &Review{}}
	if err := db.AutoMigrate(models...); err != nil {
		t.Fatal(err)
	}
	for _, m := range models {
		if err := db.Where("1 = 1").Delete(m).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := MigrateStaff(db); err != nil {
		t.Fatal(err)
	}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewHandler(db, nil)
	r.POST("/setup/status", h.SetupStatus)
	r.POST("/setup", h.Setup)
	r.POST("/login", h.Login)
	r.POST("/link/info", h.LinkInfo)
	r.POST("/link/accept", h.AcceptLink)
	g := r.Group("", h.Guard())
	g.POST("/me", h.Me)
	g.POST("/password", h.ChangePassword)
	o := g.Group("", h.OwnerGuard())
	o.POST("/members", h.Members)
	o.POST("/invite", h.Invite)
	o.POST("/update", h.UpdateMember)
	o.POST("/delete", h.DeleteMember)
	o.POST("/link", h.MemberLink)
	o.POST("/audit", h.AuditLog)
	call := func(path string, body any, token string) (int, map[string]any) {
		data, _ := json.Marshal(body)
		req := httptest.NewRequest("POST", path, bytes.NewReader(data))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		out := httptest.NewRecorder()
		r.ServeHTTP(out, req)
		var result map[string]any
		if err := json.Unmarshal(out.Body.Bytes(), &result); err != nil {
			panic(out.Body.String())
		}
		return out.Code, result
	}
	expect := func(path string, body any, token string, code int) map[string]any {
		t.Helper()
		got, result := call(path, body, token)
		if got != code {
			t.Fatalf("%s: got %d want %d: %v", path, got, code, result)
		}
		return result
	}
	pass := "StaffTest123456!"
	login := func(name, password string) string {
		return expect("/login", gin.H{"account_name": name, "password": password}, "", 200)["token"].(string)
	}
	tokenOf := func(data map[string]any) string {
		return strings.TrimPrefix(data["activation_path"].(string), "/activate#")
	}
	var ownerToken, reviewerToken string
	var owner, reviewer Staff
	t.Run("protected_first_owner_setup", func(t *testing.T) {
		t.Setenv("ADMIN_SETUP_TOKEN", strings.Repeat("a", 32))
		body := gin.H{"setup_token": "wrong", "account_name": "owner", "username": "负责人", "password": pass}
		expect("/setup", body, "", 403)
		body["setup_token"] = strings.Repeat("a", 32)
		expect("/setup", body, "", 200)
		expect("/setup", body, "", 409)
		if expect("/setup/status", gin.H{}, "", 200)["available"] != false {
			t.Fatal("setup remained open")
		}
		ownerToken = login("owner", pass)
		db.Where("account_name = 'owner'").First(&owner)
	})
	t.Run("invite_activation_unique_name_and_role_isolation", func(t *testing.T) {
		body := gin.H{"account_name": "reviewer", "username": "审核员", "role": "reviewer"}
		link := tokenOf(expect("/invite", body, ownerToken, 200))
		expect("/invite", body, ownerToken, 409)
		expect("/login", gin.H{"account_name": "reviewer", "password": pass}, "", 401)
		expect("/link/info", gin.H{"token": link}, "", 200)
		expect("/link/accept", gin.H{"token": link, "password": "short"}, "", 400)
		expect("/link/accept", gin.H{"token": link, "password": pass}, "", 200)
		expect("/link/accept", gin.H{"token": link, "password": pass}, "", 400)
		reviewerToken = login("reviewer", pass)
		db.Where("account_name = 'reviewer'").First(&reviewer)
		for _, path := range []string{"/members", "/invite", "/update", "/link", "/audit", "/delete"} {
			expect(path, gin.H{}, reviewerToken, 403)
		}
		expect("/me", gin.H{}, reviewerToken, 200)
		rows := expect("/members", gin.H{"query": "审核员", "status": "active"}, ownerToken, 200)
		if rows["total"].(float64) != 1 {
			t.Fatal(rows)
		}
		encoded, _ := json.Marshal(rows)
		if strings.Contains(string(encoded), "password") || strings.Contains(string(encoded), "token_hash") {
			t.Fatal("secrets leaked")
		}
		var count int64
		db.Model(&account.Account{}).Count(&count)
		if count != 0 {
			t.Fatal("staff created community identity")
		}
	})
	t.Run("expired_reissued_and_revoked_invitations", func(t *testing.T) {
		link := tokenOf(expect("/invite", gin.H{"account_name": "pending", "username": "待激活", "role": "reviewer"}, ownerToken, 200))
		var pending Staff
		db.Where("account_name = 'pending'").First(&pending)
		db.Model(&StaffLink{}).Where("staff_id = ?", pending.ID).Update("expires_at", time.Now().Add(-time.Hour))
		expect("/link/accept", gin.H{"token": link, "password": pass}, "", 400)
		newLink := tokenOf(expect("/link", gin.H{"id": pending.ID, "action": "invite"}, ownerToken, 200))
		expect("/link/info", gin.H{"token": link}, "", 400)
		expect("/link", gin.H{"id": pending.ID, "action": "revoke"}, ownerToken, 200)
		expect("/link/accept", gin.H{"token": newLink, "password": pass}, "", 400)
	})
	t.Run("last_owner_guard", func(t *testing.T) {
		for _, body := range []gin.H{{"id": owner.ID, "role": "reviewer", "status": "active"}, {"id": owner.ID, "role": "owner", "status": "disabled"}} {
			expect("/update", body, ownerToken, 409)
		}
	})
	t.Run("disable_revokes_sessions_and_preview_authority", func(t *testing.T) {
		expect("/update", gin.H{"id": reviewer.ID, "role": "reviewer", "status": "disabled"}, ownerToken, 200)
		expect("/me", gin.H{}, reviewerToken, 401)
		if IsAdminSessionValid(db, fingerprint(reviewerToken)) {
			t.Fatal("preview session survived disable")
		}
		expect("/login", gin.H{"account_name": "reviewer", "password": pass}, "", 401)
		expect("/update", gin.H{"id": reviewer.ID, "role": "reviewer", "status": "active"}, ownerToken, 200)
		expect("/me", gin.H{}, reviewerToken, 401)
		reviewerToken = login("reviewer", pass)
	})
	t.Run("reset_and_change_password_invalidate_all_sessions", func(t *testing.T) {
		link := tokenOf(expect("/link", gin.H{"id": reviewer.ID, "action": "reset"}, ownerToken, 200))
		expect("/me", gin.H{}, reviewerToken, 401)
		expect("/link/accept", gin.H{"token": link, "password": pass + "new"}, "", 200)
		expect("/login", gin.H{"account_name": "reviewer", "password": pass}, "", 401)
		reviewerToken = login("reviewer", pass+"new")
		expect("/password", gin.H{"old_password": "wrong", "new_password": pass + "final"}, reviewerToken, 400)
		expect("/password", gin.H{"old_password": pass + "new", "new_password": pass + "final"}, reviewerToken, 200)
		expect("/me", gin.H{}, reviewerToken, 401)
		reviewerToken = login("reviewer", pass+"final")
	})
	newMember := func(name, status string) Staff {
		t.Helper()
		hash, _ := bcrypt.GenerateFromPassword([]byte(pass), bcrypt.MinCost)
		member := Staff{AccountName: name, Username: "删除测试", Role: "reviewer", Status: status, Password: string(hash)}
		if status == "pending" {
			member.Password = ""
		}
		if err := db.Create(&member).Error; err != nil {
			t.Fatal(err)
		}
		return member
	}
	t.Run("delete_permissions_confirmation_and_self_protection", func(t *testing.T) {
		member := newMember("delete_permission", "disabled")
		body := gin.H{"id": member.ID, "account_name": member.AccountName}
		expect("/delete", body, "", 401)
		expect("/delete", body, reviewerToken, 403)
		expect("/delete", gin.H{"id": owner.ID, "account_name": owner.AccountName}, ownerToken, 409)
		expect("/delete", gin.H{"id": member.ID, "account_name": "wrong"}, ownerToken, 400)
		expect("/delete", gin.H{"id": member.ID}, ownerToken, 400)
		expect("/delete", body, ownerToken, 200)
		expect("/delete", body, ownerToken, 404)
		if err := protectLastOwner(db, owner, owner.Role, "deleted"); err == nil {
			t.Fatal("last owner not protected")
		}
	})
	t.Run("delete_active_revokes_access_preserves_history_and_identity", func(t *testing.T) {
		member := newMember("delete_active", "active")
		link := tokenOf(expect("/link", gin.H{"id": member.ID, "action": "reset"}, ownerToken, 200))
		token := login(member.AccountName, pass)
		review := Review{VideoID: member.ID, ReviewerID: member.ID, ReviewerName: member.Username, Decision: "approve"}
		if err := db.Create(&review).Error; err != nil {
			t.Fatal(err)
		}
		expect("/delete", gin.H{"id": member.ID, "account_name": member.AccountName}, ownerToken, 200)
		expect("/me", gin.H{}, token, 401)
		if IsAdminSessionValid(db, fingerprint(token)) {
			t.Fatal("deleted preview authority survived")
		}
		expect("/login", gin.H{"account_name": member.AccountName, "password": pass}, "", 401)
		expect("/link/accept", gin.H{"token": link, "password": pass}, "", 400)
		for _, status := range []string{"", "active", "disabled", "deleted"} {
			rows := expect("/members", gin.H{"query": member.AccountName, "status": status}, ownerToken, 200)
			if rows["total"].(float64) != 0 || len(rows["items"].([]any)) != 0 {
				t.Fatal("deleted member listed", rows)
			}
		}
		expect("/update", gin.H{"id": member.ID, "role": "owner", "status": "active"}, ownerToken, 404)
		for _, action := range []string{"invite", "reset", "revoke"} {
			expect("/link", gin.H{"id": member.ID, "action": action}, ownerToken, 404)
		}
		expect("/invite", gin.H{"account_name": member.AccountName, "username": "新人员", "role": "reviewer"}, ownerToken, 409)
		if err := RecoverStaff(db, member.AccountName, false); err == nil {
			t.Fatal("recovery resurrected deleted member")
		}
		if err := MigrateStaff(db); err != nil {
			t.Fatal(err)
		}
		var saved Staff
		if err := db.First(&saved, member.ID).Error; err != nil {
			t.Fatal("identity lost", err)
		}
		if saved.Status != "deleted" || saved.DeletedAt == nil || saved.Password != "" {
			t.Fatal("tombstone incorrect")
		}
		var history Review
		if err := db.First(&history, review.ID).Error; err != nil || history.ReviewerName != member.Username {
			t.Fatal("review history lost")
		}
		var count int64
		db.Model(&StaffAudit{}).Where("target_id = ? AND action = 'delete' AND actor_id = ?", member.ID, owner.ID).Count(&count)
		if count != 1 {
			t.Fatal("delete audit missing")
		}
	})
	t.Run("delete_pending_revokes_invite_and_cannot_be_activated", func(t *testing.T) {
		link := tokenOf(expect("/invite", gin.H{"account_name": "delete_pending", "username": "待激活删除", "role": "reviewer"}, ownerToken, 200))
		var member Staff
		db.Where("account_name = 'delete_pending'").First(&member)
		expect("/delete", gin.H{"id": member.ID, "account_name": member.AccountName}, ownerToken, 200)
		expect("/link/info", gin.H{"token": link}, "", 400)
		expect("/link/accept", gin.H{"token": link, "password": pass}, "", 400)
	})
	t.Run("concurrent_delete_records_once", func(t *testing.T) {
		member := newMember("delete_concurrent", "active")
		var wg sync.WaitGroup
		codes := make(chan int, 2)
		for i := 0; i < 2; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				code, _ := call("/delete", gin.H{"id": member.ID, "account_name": member.AccountName}, ownerToken)
				codes <- code
			}()
		}
		wg.Wait()
		close(codes)
		counts := map[int]int{}
		for code := range codes {
			counts[code]++
		}
		if counts[200] != 1 || counts[404] != 1 {
			t.Fatal(counts)
		}
		var n int64
		db.Model(&StaffAudit{}).Where("target_id = ? AND action = 'delete'", member.ID).Count(&n)
		if n != 1 {
			t.Fatal("duplicate audit", n)
		}
	})
	t.Run("concurrent_last_owner_changes", func(t *testing.T) {
		expect("/update", gin.H{"id": reviewer.ID, "role": "owner", "status": "active"}, ownerToken, 200)
		reviewerToken = login("reviewer", pass+"final")
		var wg sync.WaitGroup
		codes := make(chan int, 2)
		for _, item := range []struct {
			id    uint
			token string
		}{{owner.ID, ownerToken}, {reviewer.ID, reviewerToken}} {
			wg.Add(1)
			go func(id uint, token string) {
				defer wg.Done()
				code, _ := call("/update", gin.H{"id": id, "role": "reviewer", "status": "active"}, token)
				codes <- code
			}(item.id, item.token)
		}
		wg.Wait()
		close(codes)
		counts := map[int]int{}
		for code := range codes {
			counts[code]++
		}
		if counts[200] != 1 || counts[409] != 1 {
			t.Fatal(counts)
		}
		var count int64
		db.Model(&Staff{}).Where("role = 'owner' AND status = 'active'").Count(&count)
		if count != 1 {
			t.Fatal(count)
		}
	})
	t.Run("audit_and_idempotent_legacy_migration", func(t *testing.T) {
		var remaining Staff
		db.Where("role = 'owner'").First(&remaining)
		password := pass
		if remaining.ID == reviewer.ID {
			password += "final"
		}
		logs := expect("/audit", gin.H{}, login(remaining.AccountName, password), 200)
		if logs["total"].(float64) < 10 {
			t.Fatal("missing audit entries", logs)
		}
		for _, m := range models {
			db.Where("1 = 1").Delete(m)
		}
		hash, _ := bcrypt.GenerateFromPassword([]byte(pass), bcrypt.MinCost)
		legacy := account.Account{AccountName: "123456789", Username: "旧管理员", Password: string(hash), Role: "admin"}
		if err := db.Create(&legacy).Error; err != nil {
			t.Fatal(err)
		}
		for i := 0; i < 2; i++ {
			if err := MigrateStaff(db); err != nil {
				t.Fatal(err)
			}
		}
		var migrated Staff
		db.First(&migrated, legacy.ID)
		if migrated.Role != "owner" || migrated.Password != legacy.Password {
			t.Fatal(fmt.Sprint(migrated.ID))
		}
		login(legacy.AccountName, pass)
		var count int64
		db.Model(&Staff{}).Count(&count)
		if count != 1 {
			t.Fatal("duplicate migration")
		}
	})
}

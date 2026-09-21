package moderation

import (
	"crypto/subtle"
	"errors"
	"os"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/go-sql-driver/mysql"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type staffError struct {
	code    int
	message string
}

func (e *staffError) Error() string          { return e.message }
func problem(code int, message string) error { return &staffError{code, message} }
func respondError(c *gin.Context, err error) {
	var p *staffError
	var sql *mysql.MySQLError
	switch {
	case errors.As(err, &p):
		c.JSON(p.code, gin.H{"error": p.message})
	case errors.Is(err, gorm.ErrRecordNotFound):
		c.JSON(404, gin.H{"error": "成员不存在"})
	case errors.As(err, &sql) && sql.Number == 1062:
		c.JSON(409, gin.H{"error": "该登录账号已存在"})
	default:
		c.JSON(500, gin.H{"error": "操作失败，请稍后重试"})
	}
}

func audit(tx *gorm.DB, actor, target Staff, action, before, after string) error {
	return tx.Create(&StaffAudit{ActorID: actor.ID, ActorName: actor.Username, TargetID: target.ID, TargetName: target.AccountName, Action: action, Before: before, After: after}).Error
}

var loginName = regexp.MustCompile(`^[a-z0-9][a-z0-9._@+-]{2,63}$`)

func normalizeMember(member *Staff) error {
	member.AccountName = strings.ToLower(strings.TrimSpace(member.AccountName))
	member.Username = strings.TrimSpace(member.Username)
	if !loginName.MatchString(member.AccountName) {
		return problem(400, "登录账号须为 3 至 64 位字母、数字或 . _ @ + -")
	}
	if utf8.RuneCountInString(member.Username) < 1 || utf8.RuneCountInString(member.Username) > 24 {
		return problem(400, "姓名须为 1 至 24 字")
	}
	if !validRole(member.Role) {
		return problem(400, "请选择有效角色")
	}
	return nil
}

func hashPassword(password string) (string, error) {
	if utf8.RuneCountInString(password) < 12 || len(password) > 72 || strings.TrimSpace(password) != password {
		return "", problem(400, "密码至少 12 位、最多 72 字节，首尾不能有空格")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(hash), err
}

// Recheck authorization after the lock so revoked in-flight writes cannot succeed.
func (h *Handler) staffTransaction(c *gin.Context, owner bool, fn func(*gorm.DB, Staff) error) error {
	return h.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		if err := lockStaffState(tx); err != nil {
			return err
		}
		actor, err := sessionMember(tx, c.GetString("adminSession"))
		if err != nil {
			return problem(401, "登录已失效，请重新登录")
		}
		if owner && actor.Role != "owner" {
			return problem(403, "仅超级管理员可管理成员")
		}
		return fn(tx, actor)
	})
}

func invalidate(tx *gorm.DB, id uint) error {
	return tx.Where("account_id = ?", id).Delete(&AdminSession{}).Error
}
func removeLink(tx *gorm.DB, id uint) error {
	return tx.Where("staff_id = ?", id).Delete(&StaffLink{}).Error
}
func issueLink(tx *gorm.DB, id uint, kind string) (string, time.Time, error) {
	token, err := newSecret()
	if err != nil {
		return "", time.Time{}, err
	}
	if err := removeLink(tx, id); err != nil {
		return "", time.Time{}, err
	}
	expires := time.Now().Add(24 * time.Hour)
	err = tx.Create(&StaffLink{StaffID: id, TokenHash: fingerprint(token), Kind: kind, ExpiresAt: expires}).Error
	return token, expires, err
}

func linkResponse(c *gin.Context, token string, expires time.Time) {
	c.Header("Cache-Control", "no-store")
	// Fragments keep bearer secrets out of proxy access logs and referrers.
	c.JSON(200, gin.H{"activation_path": "/activate#" + token, "expires_at": expires})
}

func (h *Handler) Members(c *gin.Context) {
	var input struct {
		Query  string `json:"query"`
		Status string `json:"status"`
		Offset int    `json:"offset"`
	}
	if c.ShouldBindJSON(&input) != nil || input.Offset < 0 || utf8.RuneCountInString(input.Query) > 100 {
		c.JSON(400, gin.H{"error": "查询参数错误"})
		return
	}
	q := h.db.WithContext(c.Request.Context()).Model(&Staff{}).Where("staffs.status <> ?", "deleted")
	if input.Status != "" {
		q = q.Where("staffs.status = ?", input.Status)
	}
	if input.Query != "" {
		q = q.Where("LOCATE(?, staffs.username) > 0 OR LOCATE(?, staffs.account_name) > 0", input.Query, input.Query)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		respondError(c, err)
		return
	}
	type memberRow struct {
		Staff
		Activated     bool       `json:"activated"`
		LinkKind      *string    `json:"link_kind"`
		LinkExpiresAt *time.Time `json:"link_expires_at"`
		CreatorName   string     `json:"creator_name"`
	}
	items := []memberRow{}
	err := q.Select("staffs.*, staffs.password <> '' AS activated, staff_links.kind AS link_kind, staff_links.expires_at AS link_expires_at, COALESCE(creator.username, '') AS creator_name").
		Joins("LEFT JOIN staff_links ON staff_links.staff_id = staffs.id").Joins("LEFT JOIN staffs AS creator ON creator.id = staffs.created_by").Order("staffs.id DESC").Offset(input.Offset).Limit(20).Scan(&items).Error
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(200, gin.H{"items": items, "total": total})
}

func (h *Handler) Invite(c *gin.Context) {
	var input struct {
		AccountName string `json:"account_name"`
		Username    string `json:"username"`
		Role        string `json:"role"`
	}
	if c.ShouldBindJSON(&input) != nil {
		c.JSON(400, gin.H{"error": "成员资料无效"})
		return
	}
	member := Staff{AccountName: input.AccountName, Username: input.Username, Role: input.Role, Status: "pending"}
	if err := normalizeMember(&member); err != nil {
		respondError(c, err)
		return
	}
	var token string
	var expires time.Time
	err := h.staffTransaction(c, true, func(tx *gorm.DB, actor Staff) error {
		member.CreatedBy = actor.ID
		if err := tx.Create(&member).Error; err != nil {
			return err
		}
		var err error
		token, expires, err = issueLink(tx, member.ID, "invite")
		if err != nil {
			return err
		}
		return audit(tx, actor, member, "invite", "", member.Role+"/pending")
	})
	if err != nil {
		respondError(c, err)
		return
	}
	linkResponse(c, token, expires)
}

func protectLastOwner(tx *gorm.DB, member Staff, role, status string) error {
	if member.Role != "owner" || member.Status != "active" || (role == "owner" && status == "active") {
		return nil
	}
	var count int64
	if err := tx.Model(&Staff{}).Where("role = 'owner' AND status = 'active'").Count(&count).Error; err != nil {
		return err
	}
	if count <= 1 {
		return problem(409, "必须保留至少一位正常的超级管理员")
	}
	return nil
}

// RecoverStaff is an operator-only fallback; daily membership changes use the website.
func RecoverStaff(db *gorm.DB, name string, revoke bool) error {
	return db.Transaction(func(tx *gorm.DB) error {
		if err := lockStaffState(tx); err != nil {
			return err
		}
		var member Staff
		if err := tx.Where("account_name = ? AND status <> ?", strings.ToLower(strings.TrimSpace(name)), "deleted").First(&member).Error; err != nil {
			return err
		}
		role, status := "owner", "active"
		if revoke {
			role, status = member.Role, "disabled"
		}
		if member.Password == "" {
			return problem(400, "成员尚未激活")
		}
		if err := protectLastOwner(tx, member, role, status); err != nil {
			return err
		}
		before := member.Role + "/" + member.Status
		if err := tx.Model(&member).Updates(map[string]any{"role": role, "status": status}).Error; err != nil {
			return err
		}
		if err := invalidate(tx, member.ID); err != nil {
			return err
		}
		if err := removeLink(tx, member.ID); err != nil {
			return err
		}
		return audit(tx, Staff{Username: "运维恢复"}, member, "recovery", before, role+"/"+status)
	})
}

func (h *Handler) UpdateMember(c *gin.Context) {
	var input struct {
		ID     uint   `json:"id"`
		Role   string `json:"role"`
		Status string `json:"status"`
	}
	if c.ShouldBindJSON(&input) != nil || input.ID == 0 || !validRole(input.Role) || (input.Status != "active" && input.Status != "disabled" && input.Status != "pending") {
		c.JSON(400, gin.H{"error": "成员参数无效"})
		return
	}
	err := h.staffTransaction(c, true, func(tx *gorm.DB, actor Staff) error {
		var member Staff
		if err := tx.Where("status <> ?", "deleted").First(&member, input.ID).Error; err != nil {
			return err
		}
		if (input.Status == "active" && member.Password == "") || (input.Status == "pending" && member.Password != "") {
			return problem(400, "尚未激活的成员需接受邀请后才能启用")
		}
		if err := protectLastOwner(tx, member, input.Role, input.Status); err != nil {
			return err
		}
		before := member.Role + "/" + member.Status
		if err := tx.Model(&member).Updates(map[string]any{"role": input.Role, "status": input.Status}).Error; err != nil {
			return err
		}
		if err := invalidate(tx, member.ID); err != nil {
			return err
		}
		if err := removeLink(tx, member.ID); err != nil {
			return err
		}
		return audit(tx, actor, member, "update", before, input.Role+"/"+input.Status)
	})
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(200, gin.H{"message": "成员已更新"})
}

func (h *Handler) MemberLink(c *gin.Context) {
	var input struct {
		ID     uint   `json:"id"`
		Action string `json:"action"`
	}
	if c.ShouldBindJSON(&input) != nil || input.ID == 0 || (input.Action != "invite" && input.Action != "reset" && input.Action != "revoke") {
		c.JSON(400, gin.H{"error": "操作参数无效"})
		return
	}
	var token string
	var expires time.Time
	err := h.staffTransaction(c, true, func(tx *gorm.DB, actor Staff) error {
		var member Staff
		if err := tx.Where("status <> ?", "deleted").First(&member, input.ID).Error; err != nil {
			return err
		}
		if input.Action == "revoke" {
			if err := removeLink(tx, member.ID); err != nil {
				return err
			}
		} else {
			if member.Status == "disabled" || (input.Action == "invite" && member.Status != "pending") || (input.Action == "reset" && member.Status != "active") {
				return problem(409, "成员当前状态不支持此操作")
			}
			var err error
			token, expires, err = issueLink(tx, member.ID, input.Action)
			if err != nil {
				return err
			}
			if input.Action == "reset" {
				if err := invalidate(tx, member.ID); err != nil {
					return err
				}
			}
		}
		return audit(tx, actor, member, input.Action+"_link", "", "")
	})
	if err != nil {
		respondError(c, err)
		return
	}
	if token == "" {
		c.JSON(200, gin.H{"message": "链接已撤销"})
		return
	}
	linkResponse(c, token, expires)
}

func findLink(tx *gorm.DB, token string) (StaffLink, Staff, error) {
	var link StaffLink
	var member Staff
	bad := problem(400, "链接已过期、已使用或已撤销，请联系超级管理员重新发送")
	if len(token) != 64 {
		return link, member, bad
	}
	if err := tx.Where("token_hash = ? AND expires_at > ?", fingerprint(token), time.Now()).First(&link).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return link, member, bad
		}
		return link, member, err
	}
	if err := tx.First(&member, link.StaffID).Error; err != nil {
		return link, member, err
	}
	if (link.Kind == "invite" && member.Status != "pending") || (link.Kind == "reset" && member.Status != "active") {
		return link, member, bad
	}
	return link, member, nil
}

func (h *Handler) LinkInfo(c *gin.Context) {
	var input struct {
		Token string `json:"token"`
	}
	if c.ShouldBindJSON(&input) != nil {
		c.JSON(400, gin.H{"error": "链接无效"})
		return
	}
	link, member, err := findLink(h.db.WithContext(c.Request.Context()), input.Token)
	if err != nil {
		respondError(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(200, gin.H{"username": member.Username, "account_name": member.AccountName, "role": member.Role, "kind": link.Kind, "expires_at": link.ExpiresAt})
}

func (h *Handler) AcceptLink(c *gin.Context) {
	var input struct {
		Token    string `json:"token"`
		Password string `json:"password"`
	}
	if c.ShouldBindJSON(&input) != nil {
		c.JSON(400, gin.H{"error": "参数无效"})
		return
	}
	hash, err := hashPassword(input.Password)
	if err != nil {
		respondError(c, err)
		return
	}
	err = h.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		if err := lockStaffState(tx); err != nil {
			return err
		}
		link, member, err := findLink(tx, input.Token)
		if err != nil {
			return err
		}
		if err := tx.Model(&member).Updates(map[string]any{"password": hash, "status": "active"}).Error; err != nil {
			return err
		}
		if err := removeLink(tx, member.ID); err != nil {
			return err
		}
		if err := invalidate(tx, member.ID); err != nil {
			return err
		}
		return audit(tx, member, member, "accept_"+link.Kind, "", "active")
	})
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(200, gin.H{"message": "密码已设置，请登录审核中心"})
}

func (h *Handler) ChangePassword(c *gin.Context) {
	var input struct {
		Old string `json:"old_password"`
		New string `json:"new_password"`
	}
	if c.ShouldBindJSON(&input) != nil {
		c.JSON(400, gin.H{"error": "参数无效"})
		return
	}
	hash, err := hashPassword(input.New)
	if err != nil {
		respondError(c, err)
		return
	}
	err = h.staffTransaction(c, false, func(tx *gorm.DB, actor Staff) error {
		if bcrypt.CompareHashAndPassword([]byte(actor.Password), []byte(input.Old)) != nil {
			return problem(400, "原密码错误")
		}
		if input.Old == input.New {
			return problem(400, "新密码不能与原密码相同")
		}
		if err := tx.Model(&actor).Update("password", hash).Error; err != nil {
			return err
		}
		if err := invalidate(tx, actor.ID); err != nil {
			return err
		}
		if err := removeLink(tx, actor.ID); err != nil {
			return err
		}
		return audit(tx, actor, actor, "change_password", "", "")
	})
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(200, gin.H{"message": "密码已修改，请重新登录"})
}

func (h *Handler) AuditLog(c *gin.Context) {
	var input struct {
		Offset int `json:"offset"`
	}
	if c.ShouldBindJSON(&input) != nil || input.Offset < 0 {
		c.JSON(400, gin.H{"error": "查询参数错误"})
		return
	}
	q := h.db.WithContext(c.Request.Context()).Model(&StaffAudit{})
	var total int64
	if err := q.Count(&total).Error; err != nil {
		respondError(c, err)
		return
	}
	items := []StaffAudit{}
	if err := q.Order("id DESC").Offset(input.Offset).Limit(20).Find(&items).Error; err != nil {
		respondError(c, err)
		return
	}
	c.JSON(200, gin.H{"items": items, "total": total})
}

func (h *Handler) SetupStatus(c *gin.Context) {
	var count int64
	if err := h.db.Model(&Staff{}).Count(&count).Error; err != nil {
		respondError(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.JSON(200, gin.H{"available": count == 0 && len(os.Getenv("ADMIN_SETUP_TOKEN")) >= 32})
}

func (h *Handler) Setup(c *gin.Context) {
	var input struct {
		Token       string `json:"setup_token"`
		AccountName string `json:"account_name"`
		Username    string `json:"username"`
		Password    string `json:"password"`
	}
	if c.ShouldBindJSON(&input) != nil {
		c.JSON(400, gin.H{"error": "参数无效"})
		return
	}
	secret := os.Getenv("ADMIN_SETUP_TOKEN")
	if len(secret) < 32 || subtle.ConstantTimeCompare([]byte(fingerprint(secret)), []byte(fingerprint(input.Token))) != 1 {
		c.JSON(403, gin.H{"error": "安装凭证无效或初始化未开放"})
		return
	}
	member := Staff{AccountName: input.AccountName, Username: input.Username, Role: "owner", Status: "active"}
	if err := normalizeMember(&member); err != nil {
		respondError(c, err)
		return
	}
	hash, err := hashPassword(input.Password)
	if err != nil {
		respondError(c, err)
		return
	}
	member.Password = hash
	err = h.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		if err := lockStaffState(tx); err != nil {
			return err
		}
		var count int64
		if err := tx.Model(&Staff{}).Count(&count).Error; err != nil {
			return err
		}
		if count != 0 {
			return problem(409, "初始化已完成")
		}
		if err := tx.Create(&member).Error; err != nil {
			return err
		}
		return audit(tx, member, member, "setup", "", "owner/active")
	})
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(200, gin.H{"message": "超级管理员已创建，请登录"})
}

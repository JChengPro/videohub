package moderation

import "time"

type Staff struct {
	ID          uint       `gorm:"primaryKey" json:"id"`
	AccountName string     `gorm:"size:64;not null;uniqueIndex" json:"account_name"`
	Username    string     `gorm:"size:24;not null" json:"username"`
	Password    string     `gorm:"size:100;not null" json:"-"`
	Role        string     `gorm:"size:16;not null" json:"role"`
	Status      string     `gorm:"size:16;not null" json:"status"`
	CreatedBy   uint       `json:"created_by"`
	CreatedAt   time.Time  `json:"created_at"`
	LastLoginAt *time.Time `json:"last_login_at"`
	DeletedAt   *time.Time `gorm:"index" json:"deleted_at,omitempty"`
}

// This singleton serializes membership changes, including concurrent owner transfers.
type StaffState struct {
	ID       uint `gorm:"primaryKey"`
	Migrated bool
}

type StaffLink struct {
	ID        uint      `gorm:"primaryKey" json:"-"`
	StaffID   uint      `gorm:"uniqueIndex;not null" json:"-"`
	TokenHash string    `gorm:"size:64;uniqueIndex;not null" json:"-"`
	Kind      string    `gorm:"size:16;not null" json:"kind"`
	ExpiresAt time.Time `json:"expires_at"`
}

type StaffAudit struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	ActorID    uint      `gorm:"index" json:"actor_id"`
	ActorName  string    `gorm:"size:24" json:"actor_name"`
	TargetID   uint      `gorm:"index" json:"target_id"`
	TargetName string    `gorm:"size:64" json:"target_name"`
	Action     string    `gorm:"size:32" json:"action"`
	Before     string    `gorm:"size:100" json:"before"`
	After      string    `gorm:"size:100" json:"after"`
	CreatedAt  time.Time `gorm:"index" json:"created_at"`
}

// Admin sessions are independent of community JWT sessions and checked on every request.
type AdminSession struct {
	TokenHash           string    `gorm:"primaryKey;size:64" json:"-"`
	AccountID           uint      `gorm:"index;not null" json:"-"`
	PasswordFingerprint string    `gorm:"size:64;not null" json:"-"`
	ExpiresAt           time.Time `gorm:"index;not null" json:"-"`
}

type Review struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	VideoID      uint      `gorm:"uniqueIndex;not null" json:"video_id"`
	ReviewerID   uint      `gorm:"index;not null" json:"reviewer_id"`
	ReviewerName string    `gorm:"size:24;not null" json:"reviewer_name"`
	Decision     string    `gorm:"size:16;not null" json:"decision"`
	Reason       string    `gorm:"size:500;not null" json:"reason"`
	CreateTime   time.Time `gorm:"autoCreateTime" json:"create_time"`
}

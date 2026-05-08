package social

import (
	"backend/internal/account"
	"time"
)

type Social struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	FollowerID uint      `gorm:"uniqueIndex:idx_follow;not null" json:"follower_id"`
	VloggerID  uint      `gorm:"uniqueIndex:idx_follow;not null" json:"vlogger_id"`
	CreatedAt  time.Time `gorm:"autoCreateTime" json:"created_at"`
}

type FollowRequest struct {
	VloggerID uint `json:"vlogger_id"`
}

type GetAllFollowersResponse struct {
	Followers []account.Account `json:"followers"`
}

type GetAllVloggersResponse struct {
	Vloggers []account.Account `json:"vloggers"`
}

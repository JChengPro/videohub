package video

import "time"

type Like struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	VideoID   uint      `gorm:"uniqueIndex:idx_video_account;not null" json:"video_id"`
	AccountID uint      `gorm:"uniqueIndex:idx_video_account;not null" json:"account_id"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
}

type LikeRequest struct {
	VideoID uint `json:"video_id"`
}

type IsLikedResponse struct {
	IsLiked bool `json:"is_liked"`
}

package db

import (
	"backend/internal/account"
	"backend/internal/config"
	"backend/internal/social"
	"backend/internal/video"
	"fmt"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func New(cfg config.DatabaseConfig) (*gorm.DB, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		cfg.User,
		cfg.Password,
		cfg.Host,
		cfg.Port,
		cfg.DBName,
	)
	return gorm.Open(mysql.Open(dsn), &gorm.Config{})
}

func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(&account.Account{}, &video.Video{}, &video.Like{}, &video.Comment{}, &social.Social{})
}

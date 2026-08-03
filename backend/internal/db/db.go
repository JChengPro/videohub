package db

import (
	"backend/internal/account"
	"backend/internal/config"
	"backend/internal/message"
	"backend/internal/notification"
	"backend/internal/social"
	"backend/internal/video"
	"fmt"
	"strings"

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
	if err := migrateAccounts(db); err != nil {
		return err
	}
	return db.AutoMigrate(
		&account.Account{},
		&video.Video{},
		&video.Like{},
		&video.Comment{},
		&social.Social{},
		&video.OutboxMsg{},
		&video.ConsumedEvent{},
		&notification.Notification{},
		&message.Conversation{},
		&message.Message{},
		&message.Block{},
	)
}

func migrateAccounts(database *gorm.DB) error {
	if !database.Migrator().HasTable(&account.Account{}) {
		return nil
	}

	accountNameAdded := false
	if !database.Migrator().HasColumn(&account.Account{}, "account_name") {
		if err := database.Exec("ALTER TABLE accounts ADD COLUMN account_name VARCHAR(24) NULL").Error; err != nil {
			return fmt.Errorf("add accounts.account_name: %w", err)
		}
		accountNameAdded = true
	}

	legacyLoginAdded := false
	if !database.Migrator().HasColumn(&account.Account{}, "legacy_login") {
		if err := database.Exec("ALTER TABLE accounts ADD COLUMN legacy_login BOOLEAN NOT NULL DEFAULT FALSE").Error; err != nil {
			return fmt.Errorf("add accounts.legacy_login: %w", err)
		}
		legacyLoginAdded = true
	}

	type existingAccount struct {
		ID          uint
		Username    string
		AccountName string
	}
	var accounts []existingAccount
	if err := database.Table("accounts").Select("id", "username", "account_name").Order("id").Scan(&accounts).Error; err != nil {
		return fmt.Errorf("load accounts for account-name migration: %w", err)
	}
	used := make(map[string]struct{}, len(accounts))
	for _, item := range accounts {
		if value := account.NormalizeAccountName(item.AccountName); value != "" {
			used[value] = struct{}{}
		}
	}
	for _, item := range accounts {
		if strings.TrimSpace(item.AccountName) != "" {
			continue
		}
		candidate := account.NormalizeAccountName(item.Username)
		if account.ValidateAccountName(candidate) != nil {
			candidate = ""
		}
		if _, exists := used[candidate]; candidate == "" || exists {
			candidate = fmt.Sprintf("vh_%08d", item.ID)
		}
		for suffix := 1; ; suffix++ {
			if _, exists := used[candidate]; !exists {
				break
			}
			candidate = fmt.Sprintf("vh_%08d_%d", item.ID, suffix)
		}
		if err := database.Table("accounts").Where("id = ?", item.ID).Updates(map[string]any{
			"account_name": candidate,
			"legacy_login": true,
		}).Error; err != nil {
			return fmt.Errorf("backfill account name for account %d: %w", item.ID, err)
		}
		used[candidate] = struct{}{}
	}

	if accountNameAdded || legacyLoginAdded {
		if err := database.Table("accounts").Where("id > 0").Update("legacy_login", true).Error; err != nil {
			return fmt.Errorf("enable legacy login for migrated accounts: %w", err)
		}
	}

	var uniqueUsernameIndexes []string
	if err := database.Raw(`
		SELECT DISTINCT INDEX_NAME
		FROM INFORMATION_SCHEMA.STATISTICS
		WHERE TABLE_SCHEMA = DATABASE()
		  AND TABLE_NAME = 'accounts'
		  AND COLUMN_NAME = 'username'
		  AND NON_UNIQUE = 0
		  AND INDEX_NAME <> 'PRIMARY'
	`).Scan(&uniqueUsernameIndexes).Error; err != nil {
		return fmt.Errorf("inspect username indexes: %w", err)
	}
	for _, indexName := range uniqueUsernameIndexes {
		safeName := strings.ReplaceAll(indexName, "`", "``")
		if err := database.Exec("DROP INDEX `" + safeName + "` ON accounts").Error; err != nil {
			return fmt.Errorf("drop legacy username unique index %s: %w", indexName, err)
		}
	}
	return nil
}

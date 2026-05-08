package account

import (
	"context"

	"gorm.io/gorm"
)

// Repository 持有数据库连接。
type Repository struct {
	db *gorm.DB
}

// 构造函数，把外面传进来的 db 保存起来。
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(ctx context.Context, account *Account) error {
	return r.db.WithContext(ctx).Create(account).Error
}

func (r *Repository) FindByUsername(ctx context.Context, username string) (*Account, error) {
	var account Account
	if err := r.db.WithContext(ctx).Where("username = ?", username).First(&account).Error; err != nil {
		return nil, err
	}
	return &account, nil
}

func (r *Repository) SaveToken(ctx context.Context, accountID uint, token string) error {
	return r.db.WithContext(ctx).Model(&Account{}).Where("id = ?", accountID).Update("token", token).Error
}

func (r *Repository) FindByID(ctx context.Context, accountID uint) (*Account, error) {
	var account Account
	if err := r.db.WithContext(ctx).First(&account, accountID).Error; err != nil {
		return nil, err
	}
	return &account, nil
}

func (r *Repository) ClearToken(ctx context.Context, accountID uint) error {
	return r.db.WithContext(ctx).
		Model(&Account{}).
		Where("id = ?", accountID).
		Update("token", "").
		Error
}

func (r *Repository) UpdatePassword(ctx context.Context, username string, newPasswordHash string) error {
	return r.db.WithContext(ctx).Model(&Account{}).Where("username = ?", username).Update("password", newPasswordHash).Error
}

func (r *Repository) UpdateUsername(ctx context.Context, accountID uint, newUsername string) error {
	return r.db.WithContext(ctx).Model(&Account{}).Where("id = ?", accountID).Update("username", newUsername).Error
}

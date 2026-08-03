package account

import (
	"context"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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

func (r *Repository) FindLegacyByUsername(ctx context.Context, username string) (*Account, error) {
	var account Account
	if err := r.db.WithContext(ctx).
		Where("username = ? AND legacy_login = ?", username, true).
		First(&account).Error; err != nil {
		return nil, err
	}
	return &account, nil
}

func (r *Repository) FindByAccountName(ctx context.Context, accountName string) (*Account, error) {
	var account Account
	if err := r.db.WithContext(ctx).Where("account_name = ?", NormalizeAccountName(accountName)).First(&account).Error; err != nil {
		return nil, err
	}
	return &account, nil
}

func (r *Repository) AccountNameExists(ctx context.Context, accountName string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&Account{}).
		Where("account_name = ?", NormalizeAccountName(accountName)).
		Count(&count).
		Error
	return count > 0, err
}

func (r *Repository) Search(ctx context.Context, query string, limit int, offset int) ([]Account, error) {
	var accounts []Account
	err := searchAccountsQuery(r.db.WithContext(ctx), query, limit, offset).
		Find(&accounts).
		Error
	if err != nil {
		return nil, err
	}
	return accounts, nil
}

func searchAccountsQuery(db *gorm.DB, query string, limit int, offset int) *gorm.DB {
	return db.
		Where("LOCATE(LOWER(?), LOWER(username)) > 0 OR LOCATE(LOWER(?), LOWER(account_name)) > 0", query, query).
		Clauses(clause.OrderBy{Expression: clause.Expr{
			SQL:                "CASE WHEN LOWER(account_name) = LOWER(?) THEN 0 WHEN LOWER(username) = LOWER(?) THEN 1 WHEN LOWER(account_name) LIKE CONCAT(LOWER(?), '%') THEN 2 WHEN LOWER(username) LIKE CONCAT(LOWER(?), '%') THEN 3 ELSE 4 END ASC, id ASC",
			Vars:               []interface{}{query, query, query, query},
			WithoutParentheses: true,
		}}).
		Limit(limit).
		Offset(offset)
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

func (r *Repository) ExistsByID(ctx context.Context, accountID uint) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).
		Model(&Account{}).
		Where("id = ?", accountID).
		Count(&count).
		Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *Repository) ClearToken(ctx context.Context, accountID uint) error {
	return r.db.WithContext(ctx).
		Model(&Account{}).
		Where("id = ?", accountID).
		Update("token", "").
		Error
}

func (r *Repository) UpdatePassword(ctx context.Context, accountID uint, newPasswordHash string) error {
	return r.db.WithContext(ctx).Model(&Account{}).Where("id = ?", accountID).Update("password", newPasswordHash).Error
}

func (r *Repository) UpdateUsername(ctx context.Context, accountID uint, newUsername string) error {
	return r.db.WithContext(ctx).Model(&Account{}).Where("id = ?", accountID).Update("username", newUsername).Error
}

func (r *Repository) UpdateAvatarObjectKey(ctx context.Context, accountID uint, objectKey string) error {
	return r.db.WithContext(ctx).
		Model(&Account{}).
		Where("id = ?", accountID).
		Update("avatar_object_key", objectKey).
		Error
}

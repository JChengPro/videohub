package account

import (
	"backend/internal/auth"
	"backend/internal/cache"
	"backend/internal/storage"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-sql-driver/mysql"
	"golang.org/x/crypto/bcrypt"
)

type Service struct {
	repo        *Repository
	cache       *cache.Client
	fileStorage storage.Storage
}

func NewService(repo *Repository, cacheClient *cache.Client, stores ...storage.Storage) *Service {
	service := &Service{
		repo:  repo,
		cache: cacheClient,
	}
	if len(stores) > 0 {
		service.fileStorage = stores[0]
	}
	return service
}

func tokenCacheKey(accountID uint) string {
	return fmt.Sprintf("account:%d", accountID)
}

func (s *Service) Register(ctx context.Context, accountName string, username string, password string) error {
	accountName = NormalizeAccountName(accountName)
	username = normalizeUsername(username)
	// 兼容旧版客户端和端到端脚本：未传 account_name 时，仅允许把符合
	// 新规则的旧 username 直接升级为登录账号。
	if accountName == "" && ValidateAccountName(username) == nil {
		accountName = NormalizeAccountName(username)
	}
	if err := ValidateAccountName(accountName); err != nil {
		return err
	}
	if err := ValidateUsername(username); err != nil {
		return err
	}
	if err := ValidatePassword(password); err != nil {
		return err
	}

	exists, err := s.repo.AccountNameExists(ctx, accountName)
	if err != nil {
		return err
	}
	if exists {
		return errors.New("账号名已被使用，请更换一个")
	}

	//它不是加密，而是生成密码哈希。数据库里存的是哈希，不是明文密码。
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	account := &Account{
		AccountName: accountName,
		Username:    username,
		Password:    string(passwordHash),
	}
	if err := s.repo.Create(ctx, account); err != nil {
		var mysqlErr *mysql.MySQLError
		if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
			return errors.New("账号名已被使用，请更换一个")
		}
		return err
	}
	return nil
}

func (s *Service) Login(ctx context.Context, accountName string, password string) (string, error) {
	accountName = strings.TrimSpace(accountName)
	password = strings.TrimSpace(password)
	if accountName == "" {
		return "", errors.New("account_name is required")
	}
	if password == "" {
		return "", errors.New("password is required")
	}
	account, err := s.repo.FindByAccountName(ctx, accountName)
	if err != nil {
		// 迁移前创建的账号仍可暂时使用旧昵称登录；新注册账号不会进入该分支。
		legacyAccount, legacyErr := s.repo.FindLegacyByUsername(ctx, accountName)
		if legacyErr != nil {
			return "", errors.New("account name or password is wrong")
		}
		account = legacyAccount
	}
	if err := bcrypt.CompareHashAndPassword([]byte(account.Password), []byte(password)); err != nil {
		return "", errors.New("account name or password is wrong")
	}
	token, err := auth.GenerateToken(account.ID, account.AccountName, account.Username)
	if err != nil {
		return "", err
	}
	if err := s.repo.SaveToken(ctx, account.ID, token); err != nil {
		return "", err
	}
	if s.cache != nil {
		cacheCtx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
		defer cancel()

		if err := s.cache.Set(cacheCtx, tokenCacheKey(account.ID), token, 24*time.Hour); err != nil {
			log.Printf("failed to set token cache: %v", err)
		}
	}
	return token, nil
}
func (s *Service) FindByID(ctx context.Context, accountID uint) (*Account, error) {
	return s.repo.FindByID(ctx, accountID)
}

func (s *Service) Logout(ctx context.Context, accountID uint) error {
	if err := s.repo.ClearToken(ctx, accountID); err != nil {
		return err
	}
	if s.cache != nil {
		cacheCtx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
		defer cancel()

		if err := s.cache.Del(cacheCtx, tokenCacheKey(accountID)); err != nil {
			log.Printf("failed to delete token cache: %v", err)
		}
	}
	return nil
}

func (s *Service) ChangePassword(ctx context.Context, accountID uint, oldPassword, newPassword string) error {
	if accountID == 0 {
		return errors.New("account id is required")
	}
	if oldPassword == "" {
		return errors.New("请输入原密码")
	}
	if err := ValidatePassword(newPassword); err != nil {
		return err
	}
	if oldPassword == newPassword {
		return errors.New("新密码不能与原密码相同")
	}
	//查用户
	account, err := s.repo.FindByID(ctx, accountID)
	if err != nil {
		return errors.New("user not found")
	}
	//验证旧密码
	if err := bcrypt.CompareHashAndPassword([]byte(account.Password), []byte(oldPassword)); err != nil {
		return errors.New("old password is wrong")
	}
	//哈希新密码
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	// 更新密码后撤销当前 token，避免其他已登录设备继续使用旧凭证。
	if err := s.repo.UpdatePassword(ctx, accountID, string(hash)); err != nil {
		return err
	}
	if err := s.repo.ClearToken(ctx, accountID); err != nil {
		return err
	}
	if s.cache != nil {
		cacheCtx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()
		if err := s.cache.Del(cacheCtx, tokenCacheKey(accountID)); err != nil {
			log.Printf("failed to clear token cache after password change: %v", err)
		}
	}
	return nil
}

func (s *Service) FindByUsername(ctx context.Context, username string) (*Account, error) {
	return s.repo.FindByUsername(ctx, username)
}

func (s *Service) Search(ctx context.Context, query string, limit int, offset int) (SearchResponse, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return SearchResponse{}, errors.New("请输入要搜索的用户名")
	}
	if utf8.RuneCountInString(query) > 64 {
		return SearchResponse{}, errors.New("搜索关键词不能超过 64 个字符")
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > 50 {
		limit = 50
	}
	if offset < 0 {
		return SearchResponse{}, errors.New("offset must not be negative")
	}

	users, err := s.repo.Search(ctx, query, limit+1, offset)
	if err != nil {
		return SearchResponse{}, err
	}
	hasMore := len(users) > limit
	if hasMore {
		users = users[:limit]
	}
	return SearchResponse{
		Users:      users,
		HasMore:    hasMore,
		NextOffset: offset + len(users),
	}, nil
}

func (s *Service) CheckAccountName(ctx context.Context, accountName string) (CheckAccountNameResponse, error) {
	accountName = NormalizeAccountName(accountName)
	if err := ValidateAccountName(accountName); err != nil {
		return CheckAccountNameResponse{}, err
	}
	exists, err := s.repo.AccountNameExists(ctx, accountName)
	if err != nil {
		return CheckAccountNameResponse{}, err
	}
	return CheckAccountNameResponse{AccountName: accountName, Available: !exists}, nil
}

func (s *Service) Rename(ctx context.Context, accountID uint, newUsername string) (string, error) {
	newUsername = normalizeUsername(newUsername)
	if err := ValidateUsername(newUsername); err != nil {
		return "", err
	}
	account, err := s.repo.FindByID(ctx, accountID)
	if err != nil {
		return "", err
	}
	// 更新用户名
	if err := s.repo.UpdateUsername(ctx, accountID, newUsername); err != nil {
		return "", err
	}
	// 重新生成 token（因为 JWT 的 claims 里存了 username）
	token, err := auth.GenerateToken(accountID, account.AccountName, newUsername)
	if err != nil {
		return "", err
	}
	// 保存新 token
	if err := s.repo.SaveToken(ctx, accountID, token); err != nil {
		return "", err
	}
	// 更新 Redis 缓存为新 token，否则旧缓存会导致新 token 验证失败
	if s.cache != nil {
		cacheCtx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()
		if err := s.cache.Set(cacheCtx, tokenCacheKey(accountID), token, 24*time.Hour); err != nil {
			log.Printf("failed to update token cache after rename: %v", err)
		}
	}
	return token, nil
}

const maxAvatarSize = 5 << 20

func randomAvatarName() string {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 10)
	}
	return hex.EncodeToString(value)
}

func avatarExtension(contentType string) (string, bool) {
	switch contentType {
	case "image/jpeg":
		return ".jpg", true
	case "image/png":
		return ".png", true
	case "image/webp":
		return ".webp", true
	default:
		return "", false
	}
}

func (s *Service) UploadAvatar(ctx context.Context, accountID uint, reader io.Reader) (string, error) {
	if accountID == 0 {
		return "", errors.New("account id is required")
	}
	if s.fileStorage == nil {
		return "", errors.New("file storage is unavailable")
	}
	current, err := s.repo.FindByID(ctx, accountID)
	if err != nil {
		return "", err
	}
	data, err := io.ReadAll(io.LimitReader(reader, maxAvatarSize+1))
	if err != nil {
		return "", err
	}
	if len(data) == 0 {
		return "", errors.New("头像文件不能为空")
	}
	if len(data) > maxAvatarSize {
		return "", errors.New("头像文件不能超过 5MB")
	}
	contentType := http.DetectContentType(data)
	ext, ok := avatarExtension(contentType)
	if !ok {
		return "", errors.New("头像仅支持 JPG、PNG 或 WebP 图片")
	}
	objectKey := path.Join("avatars", strconv.FormatUint(uint64(accountID), 10), randomAvatarName()+ext)
	if err := s.fileStorage.Upload(ctx, objectKey, bytes.NewReader(data)); err != nil {
		return "", err
	}
	if err := s.repo.UpdateAvatarObjectKey(ctx, accountID, objectKey); err != nil {
		_ = s.fileStorage.Delete(context.Background(), objectKey)
		return "", err
	}
	if current.AvatarObjectKey != "" && current.AvatarObjectKey != objectKey {
		deleteCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := s.fileStorage.Delete(deleteCtx, current.AvatarObjectKey); err != nil {
			log.Printf("delete previous avatar failed: %v", err)
		}
	}
	return objectKey, nil
}

func (s *Service) AvatarURL(ctx context.Context, accountID uint) (*Account, string, error) {
	if accountID == 0 {
		return nil, "", errors.New("account id is required")
	}
	account, err := s.repo.FindByID(ctx, accountID)
	if err != nil {
		return nil, "", err
	}
	if account.AvatarObjectKey == "" {
		return account, "", nil
	}
	if s.fileStorage == nil {
		return nil, "", errors.New("file storage is unavailable")
	}
	url, err := s.fileStorage.URL(ctx, account.AvatarObjectKey, 15*time.Minute)
	if err != nil {
		return nil, "", err
	}
	return account, url, nil
}

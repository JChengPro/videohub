package account

import (
	"backend/internal/auth"
	"backend/internal/cache"
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type Service struct {
	repo  *Repository
	cache *cache.Client
}

func NewService(repo *Repository, cacheClient *cache.Client) *Service {
	return &Service{
		repo:  repo,
		cache: cacheClient,
	}
}

func tokenCacheKey(accountID uint) string {
	return fmt.Sprintf("account:%d", accountID)
}

func (s *Service) Register(ctx context.Context, username string, password string) error {
	username = strings.TrimSpace(username)
	password = strings.TrimSpace(password)

	if username == "" {
		return errors.New("username is required")
	}
	if password == "" {
		return errors.New("password is required")
	}

	//它不是加密，而是生成密码哈希。数据库里存的是哈希，不是明文密码。
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	account := &Account{
		Username: username,
		Password: string(passwordHash),
	}
	return s.repo.Create(ctx, account)
}

func (s *Service) Login(ctx context.Context, username string, password string) (string, error) {
	username = strings.TrimSpace(username)
	password = strings.TrimSpace(password)
	if username == "" {
		return "", errors.New("username is required")
	}
	if password == "" {
		return "", errors.New("password is required")
	}
	account, err := s.repo.FindByUsername(ctx, username)
	if err != nil {
		return "", errors.New("username or password is wrong")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(account.Password), []byte(password)); err != nil {
		return "", errors.New("username or password is wrong")
	}
	token, err := auth.GenerateToken(account.ID, account.Username)
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

func (s *Service) ChangePassword(ctx context.Context, username, oldPassword, newPassword string) error {
	//查用户
	account, err := s.repo.FindByUsername(ctx, username)
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
	//更新数据库
	return s.repo.UpdatePassword(ctx, username, string(hash))
}

func (s *Service) FindByUsername(ctx context.Context, username string) (*Account, error) {
	return s.repo.FindByUsername(ctx, username)
}

func (s *Service) Rename(ctx context.Context, accountID uint, newUsername string) (string, error) {
	newUsername = strings.TrimSpace(newUsername)
	if newUsername == "" {
		return "", errors.New("new username is required")
	}
	// 检查新用户名是否已被占用
	if _, err := s.repo.FindByUsername(ctx, newUsername); err == nil {
		return "", errors.New("username already taken")
	}
	// 更新用户名
	if err := s.repo.UpdateUsername(ctx, accountID, newUsername); err != nil {
		return "", err
	}
	// 重新生成 token（因为 JWT 的 claims 里存了 username）
	token, err := auth.GenerateToken(accountID, newUsername)
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

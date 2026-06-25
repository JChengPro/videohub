package storage

import (
	"errors"
	"strings"

	"backend/internal/config"
)

// New 根据配置创建对应的存储实现。
func New(cfg config.StorageConfig) (Storage, error) {
	switch strings.ToLower(strings.TrimSpace(cfg.Type)) {
	case "", "local":
		// 默认使用本地存储，保证未配置 OSS 时项目仍可运行。
		return NewLocalStorage(".run/uploads", "http://localhost:8080"), nil

	case "oss":
		return NewOSSStorage(
			cfg.OSS.Endpoint,
			cfg.OSS.BucketName,
			cfg.OSS.AccessKeyID,
			cfg.OSS.AccessKeySecret,
		)

	default:
		return nil, errors.New("unsupported storage type")
	}
}

package storage

import (
	"context"
	"errors"
	"io"
	"path"
	"strings"
	"time"
)

// Storage 定义统一的文件存储能力。
// 业务层只依赖该接口，不需要关心文件实际保存在本地磁盘还是阿里云 OSS。
type Storage interface {
	// Upload 将 reader 中的数据上传到 objectKey 指定的位置。
	//
	// objectKey 是文件在存储系统中的相对路径，例如：
	// videos/1/20260609/abc.mp4
	//
	// 使用 io.Reader 后，调用方可以直接上传 HTTP 文件流，
	// 不需要先将文件完整保存到本地。
	Upload(ctx context.Context, objectKey string, reader io.Reader) error

	// Delete 删除 objectKey 对应的文件。
	// 删除不存在的文件时，具体实现应尽量按幂等方式处理。
	Delete(ctx context.Context, objectKey string) error

	// URL 根据 objectKey 生成文件访问地址。
	//
	// 本地存储返回静态文件地址。
	// 私有 OSS 返回具有有效期的签名 URL。
	URL(ctx context.Context, objectKey string, expires time.Duration) (string, error)
}

// validateObjectKey 保证对象路径是规范的相对路径，避免本地存储目录穿越。
func validateObjectKey(objectKey string) error {
	objectKey = strings.TrimSpace(objectKey)
	if objectKey == "" ||
		strings.HasPrefix(objectKey, "/") ||
		strings.Contains(objectKey, "\\") ||
		path.Clean(objectKey) != objectKey ||
		strings.HasPrefix(objectKey, "../") {
		return errors.New("invalid object key")
	}
	return nil
}

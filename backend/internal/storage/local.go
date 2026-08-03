package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// LocalStorage 使用本地磁盘保存文件。
// 它实现了 Storage 接口，主要用于本地开发以及未配置 OSS 的运行环境。
type LocalStorage struct {
	// rootDir 是文件实际保存的根目录，例如 .run/uploads。
	rootDir string

	// baseURL 是浏览器访问 API 服务时使用的基础地址，例如 http://localhost:8080。
	baseURL string
}

// NewLocalStorage 创建本地文件存储实现。
func NewLocalStorage(rootDir, baseURL string) *LocalStorage {
	return &LocalStorage{
		// 去除末尾的斜杠，避免拼接路径时出现重复斜杠。
		rootDir: strings.TrimRight(rootDir, "/"),
		baseURL: strings.TrimRight(baseURL, "/"),
	}
}

// Upload 将 reader 中的数据保存到本地磁盘。
func (s *LocalStorage) Upload(ctx context.Context, objectKey string, reader io.Reader) error {
	// 如果请求已经取消或超时，则不再继续写入文件。
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := validateObjectKey(objectKey); err != nil {
		return err
	}

	// 将 objectKey 转换为当前操作系统可使用的文件路径。
	// 例如 videos/1/a.mp4 会转换为 .run/uploads/videos/1/a.mp4。
	absPath := filepath.Join(s.rootDir, filepath.FromSlash(objectKey))

	// 确保目标文件所在目录已经创建。
	if err := os.MkdirAll(filepath.Dir(absPath), 0o755); err != nil {
		return err
	}

	file, err := os.Create(absPath)
	if err != nil {
		return err
	}
	defer file.Close()

	// 将上传数据流写入目标文件。
	_, err = io.Copy(file, reader)
	return err
}

// Delete 删除本地磁盘中的文件。
func (s *LocalStorage) Delete(ctx context.Context, objectKey string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := validateObjectKey(objectKey); err != nil {
		return err
	}

	absPath := filepath.Join(s.rootDir, filepath.FromSlash(objectKey))

	err := os.Remove(absPath)

	// 文件已经不存在时，也认为删除成功，使删除操作具有幂等性。
	if os.IsNotExist(err) {
		return nil
	}

	return err
}

// URL 返回本地文件的静态访问地址。
func (s *LocalStorage) URL(ctx context.Context, objectKey string, expires time.Duration) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if err := validateObjectKey(objectKey); err != nil {
		return "", err
	}

	// 本地静态文件地址不会过期，因此这里暂时不使用 expires。
	return fmt.Sprintf("/static/%s", objectKey), nil
}

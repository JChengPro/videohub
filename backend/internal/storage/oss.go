package storage

import (
	"context"
	"errors"
	"io"
	"strings"
	"time"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
)

// OSSStorage 使用阿里云 OSS 保存文件。
type OSSStorage struct {
	bucket *oss.Bucket
}

// NewOSSStorage 创建 OSS 存储实现并连接指定 Bucket。
func NewOSSStorage(endpoint, bucketName, accessKeyID, accessKeySecret string) (*OSSStorage, error) {
	if strings.TrimSpace(endpoint) == "" ||
		strings.TrimSpace(bucketName) == "" ||
		strings.TrimSpace(accessKeyID) == "" ||
		strings.TrimSpace(accessKeySecret) == "" {
		return nil, errors.New("oss configuration is incomplete")
	}

	client, err := oss.New(endpoint, accessKeyID, accessKeySecret)
	if err != nil {
		return nil, err
	}

	bucket, err := client.Bucket(bucketName)
	if err != nil {
		return nil, err
	}

	return &OSSStorage{bucket: bucket}, nil
}

// Upload 将数据流上传到 OSS objectKey。
func (s *OSSStorage) Upload(ctx context.Context, objectKey string, reader io.Reader) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := validateObjectKey(objectKey); err != nil {
		return err
	}

	return s.bucket.PutObject(objectKey, reader)
}

// Delete 删除 OSS 中的对象。
// OSS 删除不存在的对象也会返回成功，因此该操作天然幂等。
func (s *OSSStorage) Delete(ctx context.Context, objectKey string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := validateObjectKey(objectKey); err != nil {
		return err
	}

	return s.bucket.DeleteObject(objectKey)
}

// URL 为私有 Bucket 生成临时签名访问地址。
func (s *OSSStorage) URL(ctx context.Context, objectKey string, expires time.Duration) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if err := validateObjectKey(objectKey); err != nil {
		return "", err
	}

	if expires <= 0 {
		expires = 15 * time.Minute
	}

	return s.bucket.SignURL(objectKey, oss.HTTPGet, int64(expires.Seconds()))
}

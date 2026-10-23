package storage

import (
	"context"
	"errors"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
)

// OSSStorage 使用阿里云 OSS 保存文件。
type OSSStorage struct {
	internalBucket *oss.Bucket
	publicBucket   *oss.Bucket
}

// NewOSSStorage 创建 OSS 存储实现并连接指定 Bucket。
func NewOSSStorage(internalEndpoint, publicEndpoint, bucketName, accessKeyID, accessKeySecret string) (*OSSStorage, error) {
	if strings.TrimSpace(internalEndpoint) == "" || strings.TrimSpace(publicEndpoint) == "" ||
		strings.TrimSpace(bucketName) == "" ||
		strings.TrimSpace(accessKeyID) == "" ||
		strings.TrimSpace(accessKeySecret) == "" {
		return nil, errors.New("oss configuration is incomplete")
	}

	internalClient, err := oss.New(internalEndpoint, accessKeyID, accessKeySecret)
	if err != nil {
		return nil, err
	}

	internalBucket, err := internalClient.Bucket(bucketName)
	if err != nil {
		return nil, err
	}

	publicClient, err := oss.New(publicEndpoint, accessKeyID, accessKeySecret)
	if err != nil {
		return nil, err
	}
	publicBucket, err := publicClient.Bucket(bucketName)
	if err != nil {
		return nil, err
	}

	return &OSSStorage{internalBucket: internalBucket, publicBucket: publicBucket}, nil
}

// Upload 将数据流上传到 OSS objectKey。
func (s *OSSStorage) Upload(ctx context.Context, objectKey string, reader io.Reader) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := validateObjectKey(objectKey); err != nil {
		return err
	}

	return s.internalBucket.PutObject(objectKey, reader, oss.ObjectACL(oss.ACLPrivate))
}

func (s *OSSStorage) Open(ctx context.Context, objectKey string) (io.ReadCloser, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := validateObjectKey(objectKey); err != nil {
		return nil, err
	}
	return s.internalBucket.GetObject(objectKey)
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

	return s.internalBucket.DeleteObject(objectKey)
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

	return s.publicBucket.SignURL(objectKey, oss.HTTPGet, int64(expires.Seconds()))
}

func (s *OSSStorage) InitiateMultipart(ctx context.Context, objectKey string) (string, error) {
	if err := validateMultipartRequest(ctx, objectKey); err != nil {
		return "", err
	}
	result, err := s.internalBucket.InitiateMultipartUpload(objectKey, oss.ObjectACL(oss.ACLPrivate))
	if err != nil {
		return "", err
	}
	return result.UploadID, nil
}

func (s *OSSStorage) SignMultipartPart(ctx context.Context, objectKey, uploadID string, partNumber int, expires time.Duration) (string, error) {
	if err := validateMultipartRequest(ctx, objectKey); err != nil {
		return "", err
	}
	if strings.TrimSpace(uploadID) == "" || partNumber < 1 || partNumber > 10000 {
		return "", errors.New("invalid multipart upload request")
	}
	if expires <= 0 {
		expires = 15 * time.Minute
	}
	return s.publicBucket.SignURL(objectKey, oss.HTTPPut, int64(expires.Seconds()),
		oss.AddParam("uploadId", uploadID),
		oss.AddParam("partNumber", strconv.Itoa(partNumber)),
		oss.ContentType("application/octet-stream"),
	)
}

func (s *OSSStorage) ListMultipartParts(ctx context.Context, objectKey, uploadID string) ([]MultipartPart, error) {
	if err := validateMultipartRequest(ctx, objectKey); err != nil {
		return nil, err
	}
	result, err := s.internalBucket.ListUploadedParts(oss.InitiateMultipartUploadResult{Key: objectKey, UploadID: uploadID})
	if err != nil {
		return nil, err
	}
	parts := make([]MultipartPart, 0, len(result.UploadedParts))
	for _, part := range result.UploadedParts {
		parts = append(parts, MultipartPart{PartNumber: part.PartNumber, ETag: part.ETag, Size: int64(part.Size)})
	}
	return parts, nil
}

func (s *OSSStorage) CompleteMultipart(ctx context.Context, objectKey, uploadID string, parts []MultipartPart) error {
	if err := validateMultipartRequest(ctx, objectKey); err != nil {
		return err
	}
	ossParts := make([]oss.UploadPart, 0, len(parts))
	for _, part := range parts {
		ossParts = append(ossParts, oss.UploadPart{PartNumber: part.PartNumber, ETag: part.ETag})
	}
	_, err := s.internalBucket.CompleteMultipartUpload(
		oss.InitiateMultipartUploadResult{Key: objectKey, UploadID: uploadID}, ossParts,
	)
	return err
}

func (s *OSSStorage) AbortMultipart(ctx context.Context, objectKey, uploadID string) error {
	if err := validateMultipartRequest(ctx, objectKey); err != nil {
		return err
	}
	return s.internalBucket.AbortMultipartUpload(oss.InitiateMultipartUploadResult{Key: objectKey, UploadID: uploadID})
}

func validateMultipartRequest(ctx context.Context, objectKey string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return validateObjectKey(objectKey)
}

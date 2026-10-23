package video

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type closingTestStorage struct{}

func (closingTestStorage) Upload(_ context.Context, _ string, reader io.Reader) error {
	if closer, ok := reader.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}

func (closingTestStorage) Open(context.Context, string) (io.ReadCloser, error) {
	return nil, os.ErrNotExist
}

func (closingTestStorage) Delete(context.Context, string) error { return nil }

func (closingTestStorage) URL(_ context.Context, objectKey string, _ time.Duration) (string, error) {
	return "https://example.test/" + objectKey, nil
}

func TestNormalizeVideoExtension(t *testing.T) {
	t.Parallel()

	for _, input := range []string{
		".mp4",
		"clip.MOV",
		".m4v",
		".webm",
		".3gp",
		".3gpp",
		"clip.MKV",
		".avi",
		".flv",
		".wmv",
		".mpeg",
		".mpg",
	} {
		if _, err := normalizeVideoExtension(input); err != nil {
			t.Fatalf("expected %q to be supported: %v", input, err)
		}
	}
	if _, err := normalizeVideoExtension("clip.exe"); err == nil {
		t.Fatal("expected unsupported extension to fail")
	}
}

func TestValidateUploadFileID(t *testing.T) {
	t.Parallel()

	if err := validateUploadFileID("0123456789abcdef0123456789abcdef"); err != nil {
		t.Fatalf("expected generated file id to be valid: %v", err)
	}
	for _, input := range []string{"../outside", "short", "folder/file", "has space"} {
		if err := validateUploadFileID(input); err == nil {
			t.Fatalf("expected %q to be invalid", input)
		}
	}
}

func TestUploadChunkDirSeparatesAccounts(t *testing.T) {
	t.Parallel()

	fileID := "0123456789abcdef0123456789abcdef"
	first := uploadChunkDir(1, fileID)
	second := uploadChunkDir(2, fileID)
	if first == second {
		t.Fatalf("different accounts must not share a chunk directory: %q", first)
	}
}

func TestMergeChunksAllowsStorageToCloseReader(t *testing.T) {
	fileID := "0123456789abcdef0123456789abcdef"
	chunkDir := uploadChunkDir(7, fileID)
	t.Cleanup(func() { _ = os.RemoveAll(chunkDir) })

	if err := os.MkdirAll(chunkDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(chunkDir, "0"), []byte("video-data"), 0o644); err != nil {
		t.Fatal(err)
	}

	service := &Service{fileStorage: closingTestStorage{}}
	_, objectKey, err := service.MergeChunks(context.Background(), fileID, ".mov", 7, 1, int64(len("video-data")))
	if err != nil {
		t.Fatalf("merge should not fail when storage closes its reader: %v", err)
	}
	if !strings.HasSuffix(objectKey, ".mov") {
		t.Fatalf("expected object key to preserve extension, got %q", objectKey)
	}
}

func TestMergeChunksRejectsMissingChunk(t *testing.T) {
	fileID := "abcdef0123456789abcdef0123456789"
	chunkDir := uploadChunkDir(8, fileID)
	t.Cleanup(func() { _ = os.RemoveAll(chunkDir) })

	if err := os.MkdirAll(chunkDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(chunkDir, "0"), []byte("first"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(chunkDir, "2"), []byte("third"), 0o644); err != nil {
		t.Fatal(err)
	}

	service := &Service{fileStorage: closingTestStorage{}}
	if _, _, err := service.MergeChunks(context.Background(), fileID, ".mp4", 8, 3, 10); err == nil || !strings.Contains(err.Error(), "chunk 1 missing") {
		t.Fatalf("expected missing chunk error, got %v", err)
	}
}

func TestMergeChunksRejectsWrongFileSize(t *testing.T) {
	fileID := "fedcba9876543210fedcba9876543210"
	chunkDir := uploadChunkDir(9, fileID)
	t.Cleanup(func() { _ = os.RemoveAll(chunkDir) })

	if err := os.MkdirAll(chunkDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(chunkDir, "0"), []byte("video-data"), 0o644); err != nil {
		t.Fatal(err)
	}

	service := &Service{fileStorage: closingTestStorage{}}
	if _, _, err := service.MergeChunks(context.Background(), fileID, ".mp4", 9, 1, 999); err == nil || !strings.Contains(err.Error(), "size mismatch") {
		t.Fatalf("expected file size mismatch, got %v", err)
	}
}

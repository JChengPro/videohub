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

func (closingTestStorage) Delete(context.Context, string) error { return nil }

func (closingTestStorage) URL(_ context.Context, objectKey string, _ time.Duration) (string, error) {
	return "https://example.test/" + objectKey, nil
}

func TestNormalizeVideoExtension(t *testing.T) {
	t.Parallel()

	for _, input := range []string{".mp4", "clip.MOV", ".m4v", ".webm", ".3gp", ".3gpp"} {
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

func TestMergeChunksAllowsStorageToCloseReader(t *testing.T) {
	fileID := "0123456789abcdef0123456789abcdef"
	chunkDir := filepath.Join(".run", "uploads", "chunks", fileID)
	t.Cleanup(func() { _ = os.RemoveAll(chunkDir) })

	if err := os.MkdirAll(chunkDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(chunkDir, "0"), []byte("video-data"), 0o644); err != nil {
		t.Fatal(err)
	}

	service := &Service{fileStorage: closingTestStorage{}}
	_, objectKey, err := service.MergeChunks(context.Background(), fileID, ".mov", 7)
	if err != nil {
		t.Fatalf("merge should not fail when storage closes its reader: %v", err)
	}
	if !strings.HasSuffix(objectKey, ".mov") {
		t.Fatalf("expected object key to preserve extension, got %q", objectKey)
	}
}

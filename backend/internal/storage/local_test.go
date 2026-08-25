package storage

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"
)

func TestLocalStorageOpenReadsUploadedObject(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	store := NewLocalStorage(root, "")
	ctx := context.Background()
	if err := store.Upload(ctx, "videos/7/input.mp4", strings.NewReader("video-data")); err != nil {
		t.Fatal(err)
	}
	reader, err := store.Open(ctx, "videos/7/input.mp4")
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "video-data" {
		t.Fatalf("Open() returned %q", string(data))
	}
}

func TestLocalStorageURLUsesCurrentWebOrigin(t *testing.T) {
	store := NewLocalStorage(".run/uploads", "http://localhost:8080")
	url, err := store.URL(context.Background(), "avatars/7/avatar.png", time.Minute)
	if err != nil {
		t.Fatalf("URL returned error: %v", err)
	}
	if url != "/static/avatars/7/avatar.png" {
		t.Fatalf("URL = %q, want a reverse-proxy-safe relative URL", url)
	}
}

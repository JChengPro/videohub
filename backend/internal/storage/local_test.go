package storage

import (
	"context"
	"testing"
	"time"
)

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

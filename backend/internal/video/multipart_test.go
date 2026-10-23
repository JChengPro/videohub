package video

import (
	"context"
	"testing"

	"backend/internal/storage"
)

func TestInitMultipartFallsBackToLocalStorage(t *testing.T) {
	service := &Service{fileStorage: storage.NewLocalStorage(t.TempDir(), "")}
	result, err := service.InitMultipart(context.Background(), 1, MultipartInitRequest{})
	if err != nil {
		t.Fatalf("InitMultipart() error = %v", err)
	}
	if result.Mode != "local" || result.PartSize != MultipartPartSize {
		t.Fatalf("InitMultipart() = %#v, want local mode with configured part size", result)
	}
}

func TestValidateMultipartInitBoundaries(t *testing.T) {
	base := MultipartInitRequest{FileID: "12345678", FileName: "video.mp4"}
	tests := []struct {
		name    string
		size    int64
		parts   int
		wantErr bool
	}{
		{name: "empty", size: 0, parts: 0, wantErr: true},
		{name: "one byte", size: 1, parts: 1},
		{name: "499 MiB", size: 499 << 20, parts: 50},
		{name: "500 MiB", size: 500 << 20, parts: 50},
		{name: "over limit", size: (500 << 20) + 1, parts: 51, wantErr: true},
		{name: "wrong part count", size: 20 << 20, parts: 1, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := base
			req.FileSize = tt.size
			req.TotalParts = tt.parts
			if err := validateMultipartInit(req); (err != nil) != tt.wantErr {
				t.Fatalf("validateMultipartInit() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateCompletedParts(t *testing.T) {
	task := &MultipartUpload{FileSize: (2 * MultipartPartSize) + 7, TotalParts: 3}
	valid := []storage.MultipartPart{
		{PartNumber: 3, ETag: "third", Size: 7},
		{PartNumber: 1, ETag: "first", Size: MultipartPartSize},
		{PartNumber: 2, ETag: "second", Size: MultipartPartSize},
	}
	if err := validateCompletedParts(task, valid); err != nil {
		t.Fatalf("valid parts rejected: %v", err)
	}

	missing := valid[:2]
	if err := validateCompletedParts(task, missing); err == nil {
		t.Fatal("missing part was accepted")
	}

	wrongSize := append([]storage.MultipartPart(nil), valid...)
	wrongSize[0].Size = 8
	if err := validateCompletedParts(task, wrongSize); err == nil {
		t.Fatal("wrong final part size was accepted")
	}
}

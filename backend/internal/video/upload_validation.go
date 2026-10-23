package video

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"unicode"
)

const (
	maxUploadChunks   = 1000
	maxUploadFileSize = 200 << 20
)

var allowedVideoExtensions = map[string]struct{}{
	".3gp":  {},
	".3gpp": {},
	".avi":  {},
	".flv":  {},
	".m4v":  {},
	".mkv":  {},
	".mov":  {},
	".mp4":  {},
	".mpeg": {},
	".mpg":  {},
	".webm": {},
	".wmv":  {},
}

func normalizeVideoExtension(value string) (string, error) {
	ext := strings.ToLower(strings.TrimSpace(value))
	if !strings.HasPrefix(ext, ".") {
		ext = filepath.Ext(ext)
	}
	if _, ok := allowedVideoExtensions[ext]; !ok {
		return "", errors.New("unsupported video format; allowed: mp4, mov, m4v, webm, mkv, avi, 3gp, 3gpp, flv, wmv, mpeg, mpg")
	}
	return ext, nil
}

func validateUploadFileID(value string) error {
	if len(value) < 8 || len(value) > 128 {
		return errors.New("invalid file_id")
	}
	for _, char := range value {
		if !unicode.IsLetter(char) && !unicode.IsDigit(char) && char != '-' && char != '_' {
			return errors.New("invalid file_id")
		}
	}
	return nil
}

func uploadChunkDir(accountID uint, fileID string) string {
	return filepath.Join(".run", "uploads", "chunks", fmt.Sprintf("%d", accountID), fileID)
}

func validateMergeRequest(totalChunks int, fileSize int64) error {
	if totalChunks <= 0 || totalChunks > maxUploadChunks {
		return errors.New("invalid total_chunks")
	}
	if fileSize <= 0 || fileSize > maxUploadFileSize {
		return errors.New("invalid file_size")
	}
	return nil
}

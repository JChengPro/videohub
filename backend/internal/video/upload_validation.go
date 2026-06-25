package video

import (
	"errors"
	"path/filepath"
	"strings"
	"unicode"
)

var allowedVideoExtensions = map[string]struct{}{
	".3gp":  {},
	".3gpp": {},
	".m4v":  {},
	".mov":  {},
	".mp4":  {},
	".webm": {},
}

func normalizeVideoExtension(value string) (string, error) {
	ext := strings.ToLower(strings.TrimSpace(value))
	if !strings.HasPrefix(ext, ".") {
		ext = filepath.Ext(ext)
	}
	if _, ok := allowedVideoExtensions[ext]; !ok {
		return "", errors.New("unsupported video format; allowed: mp4, mov, m4v, webm, 3gp")
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

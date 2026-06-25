package account

import (
	"errors"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	MinUsernameLength = 3
	MaxUsernameLength = 24
	MinPasswordLength = 8
	MaxPasswordLength = 64
)

var usernamePattern = regexp.MustCompile(`^[\p{Han}A-Za-z0-9_]+$`)

// ValidateUsername keeps account names predictable for display, search, and URLs.
func ValidateUsername(username string) error {
	length := utf8.RuneCountInString(username)
	if length < MinUsernameLength || length > MaxUsernameLength {
		return errors.New("用户名长度必须为 3-24 个字符")
	}
	if !usernamePattern.MatchString(username) {
		return errors.New("用户名只能包含中文、字母、数字和下划线")
	}
	return nil
}

// ValidatePassword enforces the same password policy for registration and password changes.
func ValidatePassword(password string) error {
	length := utf8.RuneCountInString(password)
	if length < MinPasswordLength || length > MaxPasswordLength {
		return errors.New("密码长度必须为 8-64 个字符")
	}
	if len([]byte(password)) > 72 {
		return errors.New("密码内容过长，请减少中文或特殊字符")
	}

	var hasLetter, hasNumber bool
	for _, r := range password {
		if unicode.IsSpace(r) {
			return errors.New("密码不能包含空白字符")
		}
		hasLetter = hasLetter || unicode.IsLetter(r)
		hasNumber = hasNumber || unicode.IsNumber(r)
	}
	if !hasLetter || !hasNumber {
		return errors.New("密码必须同时包含字母和数字")
	}
	return nil
}

func normalizeUsername(username string) string {
	return strings.TrimSpace(username)
}

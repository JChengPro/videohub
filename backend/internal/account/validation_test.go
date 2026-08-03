package account

import "testing"

func TestValidateAccountName(t *testing.T) {
	tests := []struct {
		name        string
		accountName string
		valid       bool
	}{
		{name: "letters numbers underscore", accountName: "jcheng_2026", valid: true},
		{name: "mixed case", accountName: "VideoHubUser", valid: true},
		{name: "chinese", accountName: "视频用户01", valid: false},
		{name: "starts with number", accountName: "2026user", valid: false},
		{name: "too short", accountName: "abc", valid: false},
		{name: "symbol", accountName: "user-name", valid: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateAccountName(tt.accountName)
			if (err == nil) != tt.valid {
				t.Fatalf("ValidateAccountName(%q) error = %v, valid = %v", tt.accountName, err, tt.valid)
			}
		})
	}
}

func TestValidateUsername(t *testing.T) {
	tests := []struct {
		name     string
		username string
		valid    bool
	}{
		{name: "letters numbers underscore", username: "user_123", valid: true},
		{name: "chinese", username: "视频用户01", valid: true},
		{name: "too short", username: "ab", valid: false},
		{name: "space", username: "user name", valid: false},
		{name: "symbol", username: "user-name", valid: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateUsername(tt.username)
			if (err == nil) != tt.valid {
				t.Fatalf("ValidateUsername(%q) error = %v, valid = %v", tt.username, err, tt.valid)
			}
		})
	}
}

func TestValidatePassword(t *testing.T) {
	tests := []struct {
		name     string
		password string
		valid    bool
	}{
		{name: "letters numbers symbols", password: "VideoHub123!", valid: true},
		{name: "too short", password: "abc123", valid: false},
		{name: "missing number", password: "VideoHubOnly", valid: false},
		{name: "missing letter", password: "1234567890", valid: false},
		{name: "contains space", password: "Video Hub123", valid: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePassword(tt.password)
			if (err == nil) != tt.valid {
				t.Fatalf("ValidatePassword(%q) error = %v, valid = %v", tt.password, err, tt.valid)
			}
		})
	}
}

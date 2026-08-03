package account

import (
	"reflect"
	"strings"
	"testing"
)

func TestAccountNameModelIsUniqueAndNicknameIsNot(t *testing.T) {
	model := reflect.TypeOf(Account{})
	accountName, ok := model.FieldByName("AccountName")
	if !ok || !strings.Contains(accountName.Tag.Get("gorm"), "uniqueIndex:idx_accounts_account_name") {
		t.Fatal("account_name must have a database unique index")
	}
	username, ok := model.FieldByName("Username")
	if !ok {
		t.Fatal("username field is missing")
	}
	if strings.Contains(username.Tag.Get("gorm"), "unique") {
		t.Fatal("public nickname must not be used as the unique login identity")
	}
}

func TestDefaultAvatarSVGIsEscapedAndUsesLivelyPalette(t *testing.T) {
	account := &Account{ID: 7, AccountName: "video_user", Username: `<小王&朋友>`}
	svg := defaultAvatarSVG(account)
	if strings.Contains(svg, `<小王&朋友>`) {
		t.Fatal("avatar SVG contains unescaped user content")
	}
	if !strings.Contains(svg, "&lt;") || !strings.Contains(svg, "linearGradient") {
		t.Fatalf("unexpected avatar SVG: %s", svg)
	}
	if strings.Contains(svg, "#526b78") || strings.Contains(svg, "#354852") {
		t.Fatal("legacy gray avatar palette is still in use")
	}
}

func TestAvatarExtensionUsesDetectedContentType(t *testing.T) {
	tests := map[string]string{
		"image/jpeg": ".jpg",
		"image/png":  ".png",
		"image/webp": ".webp",
	}
	for contentType, expected := range tests {
		extension, ok := avatarExtension(contentType)
		if !ok || extension != expected {
			t.Fatalf("avatarExtension(%q) = %q, %v", contentType, extension, ok)
		}
	}
	if _, ok := avatarExtension("image/svg+xml"); ok {
		t.Fatal("SVG uploads must be rejected to avoid active-content risks")
	}
}

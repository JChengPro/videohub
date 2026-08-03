package message

import (
	"net/http/httptest"
	"testing"

	"golang.org/x/net/websocket"
)

func TestValidateOrigin(t *testing.T) {
	tests := []struct {
		name          string
		origin        string
		host          string
		forwardedHost string
		wantError     bool
	}{
		{
			name:      "same hostname with different ports",
			origin:    "http://localhost:5173",
			host:      "localhost:8080",
			wantError: false,
		},
		{
			name:          "reverse proxy host",
			origin:        "https://video.example.com",
			host:          "backend:8080",
			forwardedHost: "video.example.com",
			wantError:     false,
		},
		{
			name:      "cross site origin",
			origin:    "https://evil.example",
			host:      "video.example.com",
			wantError: true,
		},
		{
			name:      "missing origin",
			host:      "video.example.com",
			wantError: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest("GET", "http://"+test.host+"/ws", nil)
			request.Host = test.host
			if test.origin != "" {
				request.Header.Set("Origin", test.origin)
			}
			if test.forwardedHost != "" {
				request.Header.Set("X-Forwarded-Host", test.forwardedHost)
			}
			err := validateOrigin(&websocket.Config{}, request)
			if (err != nil) != test.wantError {
				t.Fatalf("validateOrigin() error = %v, wantError = %v", err, test.wantError)
			}
		})
	}
}

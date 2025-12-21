package grpc

import (
	"testing"
	"time"

	"right-sizer/config"

	"github.com/golang-jwt/jwt/v5"
)

func TestIsValidToken(t *testing.T) {
	secret := "test-secret" // pragma: allowlist secret
	cfg := &config.Config{
		JWTSecret: secret,
	}
	server := &Server{
		config: cfg,
	}

	// Helper to create token
	createToken := func(s string, expiry time.Time) string {
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"exp": expiry.Unix(),
		})
		tokenString, _ := token.SignedString([]byte(s))
		return tokenString
	}

	tests := []struct {
		name     string
		token    string
		expected bool
	}{
		{
			name:     "Valid token",
			token:    createToken(secret, time.Now().Add(time.Hour)),
			expected: true,
		},
		{
			name:     "Expired token",
			token:    createToken(secret, time.Now().Add(-time.Hour)),
			expected: false,
		},
		{
			name:     "Invalid secret",
			token:    createToken("wrong-secret", time.Now().Add(time.Hour)),
			expected: false,
		},
		{
			name:     "Malformed token",
			token:    "not-a-token",
			expected: false,
		},
		{
			name:     "Empty token",
			token:    "",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := server.isValidToken(tt.token)
			if result != tt.expected {
				t.Errorf("isValidToken() for %s = %v, want %v", tt.name, result, tt.expected)
			}
		})
	}
}

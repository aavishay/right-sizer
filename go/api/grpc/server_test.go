package grpc

import (
	"testing"
	"time"

	"right-sizer/config"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
)

func TestIsValidToken(t *testing.T) {
	secret := "test-secret" // pragma: allowlist secret
	cfg := &config.Config{
		JWTSecret: secret,
	}
	s := &Server{
		config: cfg,
	}

	tests := []struct {
		name          string
		tokenCreate   func() (string, error)
		tokenString   string
		expectedValid bool
	}{
		{
			name: "Valid Token",
			tokenCreate: func() (string, error) {
				token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
					"sub": "user123",
					"exp": time.Now().Add(time.Hour).Unix(),
				})
				return token.SignedString([]byte(secret))
			},
			expectedValid: true,
		},
		{
			name: "Expired Token",
			tokenCreate: func() (string, error) {
				token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
					"sub": "user123",
					"exp": time.Now().Add(-time.Hour).Unix(),
				})
				return token.SignedString([]byte(secret))
			},
			expectedValid: false,
		},
		{
			name: "Wrong Secret",
			tokenCreate: func() (string, error) {
				token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
					"sub": "user123",
					"exp": time.Now().Add(time.Hour).Unix(),
				})
				return token.SignedString([]byte("wrong-secret"))
			},
			expectedValid: false,
		},
		{
			name: "Invalid Signature Algorithm (None)",
			tokenCreate: func() (string, error) {
				token := jwt.NewWithClaims(jwt.SigningMethodNone, jwt.MapClaims{
					"sub": "user123",
					"exp": time.Now().Add(time.Hour).Unix(),
				})
				return token.SignedString(jwt.UnsafeAllowNoneSignatureType)
			},
			expectedValid: false,
		},
		{
			name:          "Garbage Token",
			tokenString:   "invalid-token-string",
			expectedValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var token string
			var err error
			if tt.tokenCreate != nil {
				token, err = tt.tokenCreate()
				assert.NoError(t, err)
			} else {
				token = tt.tokenString
			}

			valid := s.isValidToken(token)
			assert.Equal(t, tt.expectedValid, valid)
		})
	}
}

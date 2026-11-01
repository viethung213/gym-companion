package middleware

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/viethung213/gym-companion/internal/shared/internaltoken"
)

func TestTokenTranslator(t *testing.T) {
	t.Parallel()

	secret := "test-secret-key-1234567890123456"
	tt := NewTokenTranslator(secret, time.Minute)

	tokenStr, err := tt.Generate("usr-123", "Trainer", "req-abc")
	require.NoError(t, err)
	assert.NotEmpty(t, tokenStr)

	// Verify token using internaltoken.Validate
	claims, err := internaltoken.Validate(tokenStr, secret)
	require.NoError(t, err)
	require.NotNil(t, claims)

	assert.Equal(t, "usr-123", claims.UserID)
	assert.Equal(t, "Trainer", claims.Role)
	assert.Equal(t, "req-abc", claims.RequestID)
}

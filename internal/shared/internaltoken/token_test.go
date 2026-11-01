package internaltoken

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInternalToken_GenerateAndValidate(t *testing.T) {
	t.Parallel()

	secret := "test-internal-secret-key-12345"
	userID := "user-uuid-123"
	role := "member"
	reqID := "req-trace-abc"
	ttl := 2 * time.Second

	t.Run("valid token verification", func(t *testing.T) {
		tokenStr, err := Generate(userID, role, reqID, ttl, secret)
		require.NoError(t, err)
		assert.NotEmpty(t, tokenStr)

		claims, err := Validate(tokenStr, secret)
		require.NoError(t, err)
		assert.Equal(t, userID, claims.UserID)
		assert.Equal(t, role, claims.Role)
		assert.Equal(t, reqID, claims.RequestID)
	})

	t.Run("expired token rejection", func(t *testing.T) {
		tokenStr, err := Generate(userID, role, reqID, 10*time.Millisecond, secret)
		require.NoError(t, err)

		time.Sleep(20 * time.Millisecond)

		claims, err := Validate(tokenStr, secret)
		assert.ErrorIs(t, err, ErrExpiredToken)
		assert.Nil(t, claims)
	})

	t.Run("invalid secret signature rejection", func(t *testing.T) {
		tokenStr, err := Generate(userID, role, reqID, ttl, secret)
		require.NoError(t, err)

		claims, err := Validate(tokenStr, "wrong-secret-key")
		assert.ErrorIs(t, err, ErrInvalidToken)
		assert.Nil(t, claims)
	})

	t.Run("empty token string", func(t *testing.T) {
		claims, err := Validate("", secret)
		assert.ErrorIs(t, err, ErrMissingToken)
		assert.Nil(t, claims)
	})

	t.Run("empty secret error", func(t *testing.T) {
		_, err := Generate(userID, role, reqID, ttl, "")
		assert.Error(t, err)

		_, err = Validate("dummy-token", "")
		assert.Error(t, err)
	})
}

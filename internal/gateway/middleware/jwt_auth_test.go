package middleware

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type staticKeyProvider struct {
	pemStr string
}

func (s *staticKeyProvider) GetPublicKeyPEM(_ context.Context, _ string) (string, error) {
	return s.pemStr, nil
}

func TestJWTValidator(t *testing.T) {
	t.Parallel()

	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	pubDER, err := x509.MarshalPKIXPublicKey(&privKey.PublicKey)
	require.NoError(t, err)
	pubPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubDER,
	})

	kp := &staticKeyProvider{pemStr: string(pubPEM)}
	val := NewJWTValidator(kp)

	ctx := context.Background()

	t.Run("missing header returns error", func(t *testing.T) {
		_, _, err := val.Verify(ctx, "")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "missing authorization header")
	})

	t.Run("invalid format returns error", func(t *testing.T) {
		_, _, err := val.Verify(ctx, "Basic 123456")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid authorization format")
	})

	t.Run("valid token extracts sub and role", func(t *testing.T) {
		token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
			"sub":  "usr-test-123",
			"role": "Trainer",
			"exp":  time.Now().Add(time.Hour).Unix(),
		})
		token.Header["kid"] = "kid-static-1"
		tokenStr, err := token.SignedString(privKey)
		require.NoError(t, err)

		sub, role, err := val.Verify(ctx, "Bearer "+tokenStr)
		require.NoError(t, err)
		assert.Equal(t, "usr-test-123", sub)
		assert.Equal(t, "Trainer", role)
	})
}

func TestIsAuthRequired(t *testing.T) {
	t.Parallel()

	// Public bypass defined in Proto OpenAPI contracts: security: {}
	publicRoute := "/contracts.supporting.exercise.v1.service.ExerciseService/SearchExercises"
	assert.False(t, IsAuthRequired(publicRoute), "SearchExercises should not require auth (public bypass)")

	// Protected method requiring BearerAuth
	protectedRoute := "/contracts.core.workout.v1.service.WorkoutService/CreateWorkoutPlan"
	assert.True(t, IsAuthRequired(protectedRoute), "CreateWorkoutPlan should require auth")

	// Unknown or malformed method defaults to secure (true)
	assert.True(t, IsAuthRequired("invalid-route"))
	assert.True(t, IsAuthRequired("/nonexistent.Service/NonExistentMethod"))
}

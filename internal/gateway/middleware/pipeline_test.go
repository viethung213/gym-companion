package middleware

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockTestKeyProvider struct {
	pubKey *rsa.PublicKey
}

func (m *mockTestKeyProvider) GetPublicKeyPEM(_ context.Context, _ string) (string, error) {
	return "", nil
}

func TestPipeline_E2E(t *testing.T) {
	t.Parallel()

	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	kp := &mockTestKeyProvider{pubKey: &privKey.PublicKey}

	hf := NewHostFilter()
	hf.SetAllowedHosts([]string{"localhost"})

	pipeline := NewPipeline(PipelineOptions{
		HostFilter:     hf,
		KeyProvider:    kp,
		InternalSecret: "secret",
		TokenTTL:       time.Minute,
		EnforceTLS:     false,
	})
	// Pre-fill keyCache on internal validator for test
	pipeline.jwtVal.keyCache["kid-1"] = &privKey.PublicKey

	var receivedToken, receivedReqID string
	backendHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedToken = r.Header.Get(HeaderInternalToken)
		receivedReqID = r.Header.Get(HeaderRequestID)
		w.WriteHeader(http.StatusOK)
	})

	handler := pipeline.Wrap(backendHandler)

	// Create valid user JWT
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"sub":  "usr-999",
		"role": "Member",
		"exp":  time.Now().Add(time.Hour).Unix(),
	})
	token.Header["kid"] = "kid-1"
	tokenStr, err := token.SignedString(privKey)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/contracts.profile.v1.ProfileService/GetProfile", nil)
	req.Host = "localhost:8000"
	req.Header.Set("Authorization", "Bearer "+tokenStr)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.NotEmpty(t, receivedToken)
	assert.NotEmpty(t, receivedReqID)
}

func TestPipeline_TLSEnforcement(t *testing.T) {
	t.Parallel()

	pipeline := NewPipeline(PipelineOptions{
		InternalSecret: "secret",
		EnforceTLS:     true,
	})

	dummyHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := pipeline.Wrap(dummyHandler)

	publicPath := "/contracts.supporting.exercise.v1.service.ExerciseService/SearchExercises"

	// Plain HTTP without X-Forwarded-Proto -> 426 Upgrade Required
	req := httptest.NewRequest(http.MethodGet, "http://example.com"+publicPath, nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUpgradeRequired, rec.Code)

	// HTTPS via X-Forwarded-Proto -> 200 OK
	reqTLS := httptest.NewRequest(http.MethodGet, "http://example.com"+publicPath, nil)
	reqTLS.Header.Set("X-Forwarded-Proto", "https")
	recTLS := httptest.NewRecorder()
	handler.ServeHTTP(recTLS, reqTLS)

	assert.Equal(t, http.StatusOK, recTLS.Code)
	assert.Contains(t, recTLS.Header().Get("Strict-Transport-Security"), "max-age=")
}

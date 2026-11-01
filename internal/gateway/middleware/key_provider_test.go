package middleware

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/viethung213/gym-companion/internal/gen/go/contracts/generic/auth/v1/service/authv1serviceconnect"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

func TestRemoteJWKSKeyProvider_StrictH2C(t *testing.T) {
	t.Parallel()

	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	nBase64 := base64.RawURLEncoding.EncodeToString(privKey.N.Bytes())
	eBase64 := base64.RawURLEncoding.EncodeToString(big.NewInt(int64(privKey.E)).Bytes())

	var receivedProtoMajor int
	var receivedPath string

	// Create strict H2C mock Auth Service
	mockAuthServer := httptest.NewServer(h2c.NewHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedProtoMajor = r.ProtoMajor
		receivedPath = r.URL.Path

		resp := jwksResponse{
			Keys: []struct {
				Kty string `json:"kty"`
				Kid string `json:"kid"`
				Use string `json:"use"`
				Alg string `json:"alg"`
				N   string `json:"n"`
				E   string `json:"e"`
			}{
				{
					Kty: "RSA",
					Kid: "kid-h2c-1",
					Use: "sig",
					Alg: "RS256",
					N:   nBase64,
					E:   eBase64,
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}), &http2.Server{}))
	defer mockAuthServer.Close()

	kp := NewRemoteJWKSKeyProvider(func() string {
		return mockAuthServer.URL
	}, "test-secret", 5*time.Minute)

	pubKey, err := kp.GetPublicKey(context.Background(), "kid-h2c-1")
	require.NoError(t, err)
	require.NotNil(t, pubKey)
	assert.Equal(t, privKey.N, pubKey.N)
	assert.Equal(t, privKey.E, pubKey.E)

	// Verify that the request arrived over strict HTTP/2 (ProtoMajor == 2)
	assert.Equal(t, 2, receivedProtoMajor, "RemoteJWKSKeyProvider must query Auth Service over HTTP/2 h2c")
	assert.Equal(t, authv1serviceconnect.AuthServiceGetJWKSProcedure, receivedPath)
}

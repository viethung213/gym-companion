package gateway

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

func TestGateway_HostAndAuthorityFiltering(t *testing.T) {
	t.Parallel()

	backend := httptest.NewServer(h2c.NewHandler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"backend reached"}`))
	}), &http2.Server{}))
	defer backend.Close()

	tmpDir := t.TempDir()
	cfgFile := filepath.Join(tmpDir, "gateway_routes.json")
	cfgJSON := []byte(`{
		"allowed_hosts": [
			"api.gymcompanion.com",
			"staging.gymcompanion.com"
		],
		"services": [
			{
				"id": "mock-backend",
				"prefixes": ["/"],
				"upstreams": ["` + backend.URL + `"]
			}
		]
	}`)
	require.NoError(t, os.WriteFile(cfgFile, cfgJSON, 0o600))

	gw, err := NewGateway(Options{
		ConfigPath:     cfgFile,
		KeyProvider:    &mockKeyProvider{},
		InternalSecret: "test-secret",
		TokenTTL:       time.Minute,
		WatchInterval:  50 * time.Millisecond,
	})
	require.NoError(t, err)
	defer gw.Close()

	gatewayServer := httptest.NewServer(gw.Handler())
	defer gatewayServer.Close()

	client := gatewayServer.Client()

	t.Run("whitelisted domain passes through", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodGet, gatewayServer.URL+"/contracts.supporting.exercise.v1.service.ExerciseService/SearchExercises", nil)
		require.NoError(t, err)
		req.Host = "api.gymcompanion.com" // Matches whitelist

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("whitelisted domain with port passes through", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodGet, gatewayServer.URL+"/contracts.supporting.exercise.v1.service.ExerciseService/SearchExercises", nil)
		require.NoError(t, err)
		req.Host = "staging.gymcompanion.com:8000" // Splits port and matches whitelist

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("direct IP access is rejected with 403 Forbidden", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodGet, gatewayServer.URL+"/contracts.supporting.exercise.v1.service.ExerciseService/SearchExercises", nil)
		require.NoError(t, err)
		req.Host = "123.45.67.89:8000" // Attacker scanning IP directly

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	})

	t.Run("unauthorized domain is rejected with 403 Forbidden", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodGet, gatewayServer.URL+"/contracts.supporting.exercise.v1.service.ExerciseService/SearchExercises", nil)
		require.NoError(t, err)
		req.Host = "evil-hacker.com" // Unauthorized host header

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	})
}

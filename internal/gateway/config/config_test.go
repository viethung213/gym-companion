package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStartupConfig_DefaultsAndCustomValues(t *testing.T) {
	t.Setenv("INTERNAL_SECRET", "test-secret")
	// 1. Test Default Values
	cfg, err := LoadStartupConfig()
	require.NoError(t, err)
	assert.NotEmpty(t, cfg.Port)
	assert.NotEmpty(t, cfg.ConfigPath)
	assert.Equal(t, "test-secret", cfg.InternalSecret)
	assert.Equal(t, time.Minute, cfg.TokenTTL)
	assert.Equal(t, 2*time.Second, cfg.WatchInterval)
	assert.Equal(t, "8001", cfg.AdminPort)
	assert.Equal(t, "8002", cfg.InternalPort)
	assert.NotEmpty(t, cfg.AdminKey)
	assert.True(t, cfg.AdminEnabled)
	assert.Equal(t, 2000, cfg.TraceBufferSize)
}

func TestStartupConfig_LoadsFromEnvFile(t *testing.T) {
	tmpDir := t.TempDir()
	customEnv := filepath.Join(tmpDir, "custom.env")
	envContent := []byte(`
GATEWAY_PORT=9999
GATEWAY_INTERNAL_PORT=8005
INTERNAL_SECRET=unit-test-secret-from-env-file
TOKEN_TTL=45s
WATCH_INTERVAL=5s
ENFORCE_TLS=true
`)
	require.NoError(t, os.WriteFile(customEnv, envContent, 0o600))

	t.Setenv("GATEWAY_ENV_FILE", customEnv)
	t.Setenv("GATEWAY_PORT", "")
	t.Setenv("GATEWAY_INTERNAL_PORT", "")
	t.Setenv("INTERNAL_SECRET", "")
	t.Setenv("TOKEN_TTL", "")
	t.Setenv("WATCH_INTERVAL", "")
	t.Setenv("ENFORCE_TLS", "")

	cfg, err := LoadStartupConfig()
	require.NoError(t, err)
	assert.Equal(t, "9999", cfg.Port)
	assert.Equal(t, "8005", cfg.InternalPort)
	assert.Equal(t, "unit-test-secret-from-env-file", cfg.InternalSecret)
	assert.Equal(t, 45*time.Second, cfg.TokenTTL)
	assert.Equal(t, 5*time.Second, cfg.WatchInterval)
	assert.True(t, cfg.EnforceTLS)
}

func TestStartupConfig_Validation(t *testing.T) {
	t.Run("fails when INTERNAL_SECRET is missing", func(t *testing.T) {
		cfg := &StartupConfig{AppEnv: "development", InternalSecret: ""}
		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "INTERNAL_SECRET")
	})

	t.Run("fails in production when using hardcoded default secret", func(t *testing.T) {
		cfg := &StartupConfig{
			AppEnv:         "production",
			InternalSecret: "gym-companion-internal-secret-2026-key",
		}
		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "forbidden in production")
	})

	t.Run("succeeds when INTERNAL_SECRET is provided", func(t *testing.T) {
		cfg := &StartupConfig{
			AppEnv:         "production",
			InternalSecret: "a-strong-production-internal-secret-key-32bytes",
		}
		require.NoError(t, cfg.Validate())
	})

	t.Run("LoadStartupConfig fails when INTERNAL_SECRET is missing", func(t *testing.T) {
		t.Setenv("GATEWAY_ENV_FILE", "/non-existent-path")
		t.Setenv("INTERNAL_SECRET", "")
		cfg, err := LoadStartupConfig()
		require.Error(t, err)
		assert.Nil(t, cfg)
		assert.Contains(t, err.Error(), "INTERNAL_SECRET")
	})
}

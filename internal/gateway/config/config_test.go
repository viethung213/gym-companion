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
	// 1. Test Default Values
	cfg := LoadStartupConfig()
	assert.NotEmpty(t, cfg.Port)
	assert.NotEmpty(t, cfg.ConfigPath)
	assert.NotEmpty(t, cfg.InternalSecret)
	assert.Equal(t, time.Minute, cfg.TokenTTL)
	assert.Equal(t, 2*time.Second, cfg.WatchInterval)
	assert.Equal(t, "8001", cfg.AdminPort)
	assert.NotEmpty(t, cfg.AdminKey)
	assert.True(t, cfg.AdminEnabled)
	assert.Equal(t, 2000, cfg.TraceBufferSize)
}

func TestStartupConfig_LoadsFromEnvFile(t *testing.T) {
	tmpDir := t.TempDir()
	customEnv := filepath.Join(tmpDir, "custom.env")
	envContent := []byte(`
GATEWAY_PORT=9999
INTERNAL_SECRET=unit-test-secret-from-env-file
TOKEN_TTL=45s
WATCH_INTERVAL=5s
ENFORCE_TLS=true
`)
	require.NoError(t, os.WriteFile(customEnv, envContent, 0o600))

	t.Setenv("GATEWAY_ENV_FILE", customEnv)
	t.Setenv("GATEWAY_PORT", "")
	t.Setenv("INTERNAL_SECRET", "")
	t.Setenv("TOKEN_TTL", "")
	t.Setenv("WATCH_INTERVAL", "")
	t.Setenv("ENFORCE_TLS", "")

	cfg := LoadStartupConfig()
	assert.Equal(t, "9999", cfg.Port)
	assert.Equal(t, "unit-test-secret-from-env-file", cfg.InternalSecret)
	assert.Equal(t, 45*time.Second, cfg.TokenTTL)
	assert.Equal(t, 5*time.Second, cfg.WatchInterval)
	assert.True(t, cfg.EnforceTLS)
}

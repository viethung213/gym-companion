package config_test

import (
	"os"
	"testing"

	"github.com/viethung213/gym-companion/internal/notification/infrastructure/config"
)

func TestLoadConfig(t *testing.T) {
	os.Setenv("FCM_PROJECT_ID", "test-project-id")
	os.Setenv("FCM_CLIENT_EMAIL", "test@example.com")
	os.Setenv("FCM_PRIVATE_KEY", "test-private-key")
	os.Setenv("FCM_PRIVATE_KEY_ID", "key-id-123")
	os.Setenv("FCM_CLIENT_ID", "client-id-456")
	os.Setenv("FCM_SERVER_KEY", "test-server-key")
	os.Setenv("KAFKA_BROKERS", "localhost:9092")
	defer func() {
		os.Unsetenv("FCM_PROJECT_ID")
		os.Unsetenv("FCM_CLIENT_EMAIL")
		os.Unsetenv("FCM_PRIVATE_KEY")
		os.Unsetenv("FCM_PRIVATE_KEY_ID")
		os.Unsetenv("FCM_CLIENT_ID")
		os.Unsetenv("FCM_SERVER_KEY")
		os.Unsetenv("KAFKA_BROKERS")
	}()

	cfg := config.LoadConfig()

	if got, want := cfg.FCMProjectID, "test-project-id"; got != want {
		t.Errorf("got FCMProjectID %s, want %s", got, want)
	}
	if got, want := cfg.FCMClientEmail, "test@example.com"; got != want {
		t.Errorf("got FCMClientEmail %s, want %s", got, want)
	}
	if got, want := cfg.FCMPrivateKey, "test-private-key"; got != want {
		t.Errorf("got FCMPrivateKey %s, want %s", got, want)
	}
	if got, want := cfg.FCMPrivateKeyID, "key-id-123"; got != want {
		t.Errorf("got FCMPrivateKeyID %s, want %s", got, want)
	}
	if got, want := cfg.FCMClientID, "client-id-456"; got != want {
		t.Errorf("got FCMClientID %s, want %s", got, want)
	}
	if got, want := cfg.FCMServerKey, "test-server-key"; got != want {
		t.Errorf("got FCMServerKey %s, want %s", got, want)
	}
	if got, want := cfg.KafkaBrokers, "localhost:9092"; got != want {
		t.Errorf("got KafkaBrokers %s, want %s", got, want)
	}
}

package config_test

import (
	"os"
	"testing"

	"github.com/viethung213/gym-companion/internal/notification/infrastructure/config"
)

func TestLoadConfig(t *testing.T) {
	os.Setenv("FCM_SERVER_KEY", "test-server-key")
	os.Setenv("KAFKA_BROKERS", "localhost:9092")
	defer func() {
		os.Unsetenv("FCM_SERVER_KEY")
		os.Unsetenv("KAFKA_BROKERS")
	}()

	cfg := config.LoadConfig()

	if got, want := cfg.FCMServerKey, "test-server-key"; got != want {
		t.Errorf("got FCMServerKey %s, want %s", got, want)
	}
	if got, want := cfg.KafkaBrokers, "localhost:9092"; got != want {
		t.Errorf("got KafkaBrokers %s, want %s", got, want)
	}
}

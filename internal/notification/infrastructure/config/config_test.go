package config_test

import (
	"errors"
	"os"
	"testing"

	"github.com/viethung213/gym-companion/internal/notification/infrastructure/config"
)

func TestLoadConfig_Success(t *testing.T) {
	os.Setenv("FCM_PROJECT_ID", "test-project-id")
	os.Setenv("FCM_CLIENT_EMAIL", "test@example.com")
	os.Setenv("FCM_PRIVATE_KEY", "test-private-key")
	os.Setenv("FCM_PRIVATE_KEY_ID", "key-id-123")
	os.Setenv("FCM_CLIENT_ID", "client-id-456")
	os.Setenv("FCM_SERVER_KEY", "test-server-key")
	os.Setenv("KAFKA_BROKERS", "localhost:9092")
	os.Setenv("EMAIL_PROVIDER", "smtp")
	os.Setenv("SMTP_HOST", "smtp.gmail.com")
	os.Setenv("SMTP_PORT", "587")
	os.Setenv("SMTP_USERNAME", "testuser@gmail.com")
	os.Setenv("SMTP_PASSWORD", "apppassword123")
	os.Setenv("SMTP_FROM_EMAIL", "testuser@gmail.com")
	os.Setenv("SMTP_FROM_NAME", "Gym Companion")
	os.Setenv("SMS_PROVIDER", "twilio")
	os.Setenv("TWILIO_ACCOUNT_SID", "AC123456")
	os.Setenv("TWILIO_AUTH_TOKEN", "auth-token-xyz")
	os.Setenv("TWILIO_FROM_NUMBER", "+1234567890")
	os.Setenv("INFOBIP_BASE_URL", "https://xyz.api.infobip.com")
	os.Setenv("INFOBIP_API_KEY", "api-key-abc")
	os.Setenv("INFOBIP_FROM", "GymComp")
	defer func() {
		os.Unsetenv("FCM_PROJECT_ID")
		os.Unsetenv("FCM_CLIENT_EMAIL")
		os.Unsetenv("FCM_PRIVATE_KEY")
		os.Unsetenv("FCM_PRIVATE_KEY_ID")
		os.Unsetenv("FCM_CLIENT_ID")
		os.Unsetenv("FCM_SERVER_KEY")
		os.Unsetenv("KAFKA_BROKERS")
		os.Unsetenv("EMAIL_PROVIDER")
		os.Unsetenv("SMTP_HOST")
		os.Unsetenv("SMTP_PORT")
		os.Unsetenv("SMTP_USERNAME")
		os.Unsetenv("SMTP_PASSWORD")
		os.Unsetenv("SMTP_FROM_EMAIL")
		os.Unsetenv("SMTP_FROM_NAME")
		os.Unsetenv("SMS_PROVIDER")
		os.Unsetenv("TWILIO_ACCOUNT_SID")
		os.Unsetenv("TWILIO_AUTH_TOKEN")
		os.Unsetenv("TWILIO_FROM_NUMBER")
		os.Unsetenv("INFOBIP_BASE_URL")
		os.Unsetenv("INFOBIP_API_KEY")
		os.Unsetenv("INFOBIP_FROM")
	}()

	cfg, err := config.LoadConfig()
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}

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

	// Email config should be loaded directly without defaults
	if got, want := cfg.EmailProvider, "smtp"; got != want {
		t.Errorf("got EmailProvider %s, want %s", got, want)
	}
	if got, want := cfg.SMTPHost, "smtp.gmail.com"; got != want {
		t.Errorf("got SMTPHost %s, want %s", got, want)
	}
	if got, want := cfg.SMTPPort, "587"; got != want {
		t.Errorf("got SMTPPort %s, want %s", got, want)
	}
	if got, want := cfg.SMTPUsername, "testuser@gmail.com"; got != want {
		t.Errorf("got SMTPUsername %s, want %s", got, want)
	}
	if got, want := cfg.SMTPPassword, "apppassword123"; got != want {
		t.Errorf("got SMTPPassword %s, want %s", got, want)
	}
	if got, want := cfg.SMTPFromEmail, "testuser@gmail.com"; got != want {
		t.Errorf("got SMTPFromEmail %s, want %s", got, want)
	}
	if got, want := cfg.SMTPFromName, "Gym Companion"; got != want {
		t.Errorf("got SMTPFromName %s, want %s", got, want)
	}

	// SMS config: Twilio should be loaded, Infobip should NOT be loaded
	if got, want := cfg.SMSProvider, "twilio"; got != want {
		t.Errorf("got SMSProvider %s, want %s", got, want)
	}
	if got, want := cfg.TwilioAccountSID, "AC123456"; got != want {
		t.Errorf("got TwilioAccountSID %s, want %s", got, want)
	}
	if got := cfg.InfobipBaseURL; got != "" {
		t.Errorf("got InfobipBaseURL %s, want empty because SMSProvider is twilio", got)
	}
}

func TestLoadConfig_InfobipSelected(t *testing.T) {
	os.Setenv("EMAIL_PROVIDER", "mock")
	os.Setenv("SMS_PROVIDER", "infobip")
	os.Setenv("TWILIO_ACCOUNT_SID", "AC123456")
	os.Setenv("INFOBIP_BASE_URL", "https://xyz.api.infobip.com")
	os.Setenv("INFOBIP_API_KEY", "api-key-abc")
	os.Setenv("INFOBIP_FROM", "GymComp")
	defer func() {
		os.Unsetenv("EMAIL_PROVIDER")
		os.Unsetenv("SMS_PROVIDER")
		os.Unsetenv("TWILIO_ACCOUNT_SID")
		os.Unsetenv("INFOBIP_BASE_URL")
		os.Unsetenv("INFOBIP_API_KEY")
		os.Unsetenv("INFOBIP_FROM")
	}()

	cfg, err := config.LoadConfig()
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}

	// Infobip should be loaded
	if got, want := cfg.InfobipBaseURL, "https://xyz.api.infobip.com"; got != want {
		t.Errorf("got InfobipBaseURL %s, want %s", got, want)
	}
	if got, want := cfg.InfobipAPIKey, "api-key-abc"; got != want {
		t.Errorf("got InfobipAPIKey %s, want %s", got, want)
	}
	// Twilio should NOT be loaded
	if got := cfg.TwilioAccountSID; got != "" {
		t.Errorf("got TwilioAccountSID %s, want empty when SMSProvider is infobip", got)
	}
	// SMTP should NOT be loaded because EmailProvider is mock
	if got := cfg.SMTPUsername; got != "" {
		t.Errorf("got SMTPUsername %s, want empty when EmailProvider is mock", got)
	}
}

func TestLoadConfig_MissingEmailProvider(t *testing.T) {
	os.Unsetenv("EMAIL_PROVIDER")

	_, err := config.LoadConfig()
	if err == nil {
		t.Fatal("expected error when EMAIL_PROVIDER is empty, got nil")
	}
	if !errors.Is(err, config.ErrMissingEmailProvider) {
		t.Errorf("got error %v, want %v", err, config.ErrMissingEmailProvider)
	}
}

func TestLoadConfig_MissingSMSProvider(t *testing.T) {
	os.Setenv("EMAIL_PROVIDER", "mock")
	os.Unsetenv("SMS_PROVIDER")
	defer os.Unsetenv("EMAIL_PROVIDER")

	_, err := config.LoadConfig()
	if err == nil {
		t.Fatal("expected error when SMS_PROVIDER is empty, got nil")
	}
	if !errors.Is(err, config.ErrMissingSMSProvider) {
		t.Errorf("got error %v, want %v", err, config.ErrMissingSMSProvider)
	}
}

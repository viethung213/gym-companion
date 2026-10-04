package config

import (
	"errors"
	"os"
	"strings"
)

var ErrMissingEmailProvider = errors.New("EMAIL_PROVIDER environment variable is required")
var ErrMissingSMSProvider = errors.New("SMS_PROVIDER environment variable is required")

type Config struct {
	FCMProjectID    string
	FCMClientEmail  string
	FCMPrivateKey   string
	FCMPrivateKeyID string
	FCMClientID     string
	FCMServerKey    string
	KafkaBrokers    string

	// Email Provider selection ("smtp" or "mock")
	EmailProvider string

	// Gmail / SMTP configuration (chỉ đọc khi EmailProvider là "smtp" hoặc "gmail")
	SMTPHost      string
	SMTPPort      string
	SMTPUsername  string
	SMTPPassword  string
	SMTPFromEmail string
	SMTPFromName  string

	// SMS Provider selection ("twilio", "infobip", or "mock")
	SMSProvider string

	// Twilio configuration (chỉ đọc khi SMSProvider là "twilio")
	TwilioAccountSID string
	TwilioAuthToken  string
	TwilioFromNumber string

	// Infobip configuration (chỉ đọc khi SMSProvider là "infobip")
	InfobipBaseURL string
	InfobipAPIKey  string
	InfobipFrom    string
}

func LoadConfig() (Config, error) {
	emailProvider := strings.ToLower(strings.TrimSpace(os.Getenv("EMAIL_PROVIDER")))
	if emailProvider == "" {
		return Config{}, ErrMissingEmailProvider
	}

	smsProvider := strings.ToLower(strings.TrimSpace(os.Getenv("SMS_PROVIDER")))
	if smsProvider == "" {
		return Config{}, ErrMissingSMSProvider
	}

	cfg := Config{
		FCMProjectID:    os.Getenv("FCM_PROJECT_ID"),
		FCMClientEmail:  os.Getenv("FCM_CLIENT_EMAIL"),
		FCMPrivateKey:   os.Getenv("FCM_PRIVATE_KEY"),
		FCMPrivateKeyID: os.Getenv("FCM_PRIVATE_KEY_ID"),
		FCMClientID:     os.Getenv("FCM_CLIENT_ID"),
		FCMServerKey:    os.Getenv("FCM_SERVER_KEY"),
		KafkaBrokers:    os.Getenv("KAFKA_BROKERS"),
		EmailProvider:   emailProvider,
		SMSProvider:     smsProvider,
	}

	// 1. Chỉ đọc biến môi trường Email khi provider tương ứng được chọn
	switch emailProvider {
	case "smtp", "gmail":
		cfg.SMTPHost = os.Getenv("SMTP_HOST")
		cfg.SMTPPort = os.Getenv("SMTP_PORT")
		cfg.SMTPUsername = os.Getenv("SMTP_USERNAME")
		cfg.SMTPPassword = os.Getenv("SMTP_PASSWORD")
		cfg.SMTPFromEmail = os.Getenv("SMTP_FROM_EMAIL")
		cfg.SMTPFromName = os.Getenv("SMTP_FROM_NAME")
	case "mock":
	}

	// 2. Chỉ đọc biến môi trường SMS khi provider tương ứng được chọn
	switch smsProvider {
	case "twilio":
		cfg.TwilioAccountSID = os.Getenv("TWILIO_ACCOUNT_SID")
		cfg.TwilioAuthToken = os.Getenv("TWILIO_AUTH_TOKEN")
		cfg.TwilioFromNumber = os.Getenv("TWILIO_FROM_NUMBER")
	case "infobip":
		cfg.InfobipBaseURL = os.Getenv("INFOBIP_BASE_URL")
		cfg.InfobipAPIKey = os.Getenv("INFOBIP_API_KEY")
		cfg.InfobipFrom = os.Getenv("INFOBIP_FROM")
	case "mock":
		// Môi trường test/dev: không cần đọc cấu hình Twilio/Infobip
	}

	return cfg, nil
}

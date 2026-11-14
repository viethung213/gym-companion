package sms

import (
	"log/slog"
	"strings"

	"github.com/viethung213/gym-companion/internal/notification/application/port"
	"github.com/viethung213/gym-companion/internal/notification/infrastructure/config"
)

// NewSMSProvider instantiates the appropriate SMS provider based on the configured SMS_PROVIDER environment variable.
// Supported providers: "twilio", "infobip", or "mock".
func NewSMSProvider(cfg *config.Config) port.SMSProvider {
	if cfg == nil {
		return NewMockSMSProvider()
	}

	providerType := strings.ToLower(strings.TrimSpace(cfg.SMSProvider))
	switch providerType {
	case "twilio":
		slog.Info("Initializing Twilio SMS Provider", slog.String("from", cfg.TwilioFromNumber))
		return NewTwilioProvider(cfg)
	case "infobip":
		slog.Info("Initializing Infobip SMS Provider", slog.String("base_url", cfg.InfobipBaseURL), slog.String("from", cfg.InfobipFrom))
		return NewInfobipProvider(cfg)
	case "mock":
		slog.Info("Initializing Mock SMS Provider (local/dev)")
		return NewMockSMSProvider()
	default:
		slog.Warn("Unknown SMS_PROVIDER, defaulting to Mock SMS Provider", slog.String("provider", providerType))
		return NewMockSMSProvider()
	}
}

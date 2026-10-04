package sms

import (
	"log"
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
		log.Printf("📱 Initializing Twilio SMS Provider (From: %s)", cfg.TwilioFromNumber)
		return NewTwilioProvider(cfg)
	case "infobip":
		log.Printf("📱 Initializing Infobip SMS Provider (BaseURL: %s, From: %s)", cfg.InfobipBaseURL, cfg.InfobipFrom)
		return NewInfobipProvider(cfg)
	case "mock":
		log.Println("📱 Initializing Mock SMS Provider (local/dev)")
		return NewMockSMSProvider()
	default:
		log.Printf("⚠️ Unknown SMS_PROVIDER '%s', defaulting to Mock SMS Provider", providerType)
		return NewMockSMSProvider()
	}
}

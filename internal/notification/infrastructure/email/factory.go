package email

import (
	"log"
	"strings"

	"github.com/viethung213/gym-companion/internal/notification/application/port"
	"github.com/viethung213/gym-companion/internal/notification/infrastructure/config"
)

// NewEmailProvider instantiates the appropriate Email provider based on the configured EMAIL_PROVIDER environment variable.
// Supported providers:
// - "smtp" (or "gmail"): Sends email via SMTP (Gmail App Password or SMTP server).
// - "mock": In-memory mock for dev / automated test environments.
func NewEmailProvider(cfg *config.Config) port.EmailProvider {
	if cfg == nil {
		return NewMockEmailProvider()
	}

	providerType := strings.ToLower(strings.TrimSpace(cfg.EmailProvider))
	switch providerType {
	case "smtp":
		log.Printf("📧 Initializing SMTP Email Provider (Host: %s:%s, From: %s)", cfg.SMTPHost, cfg.SMTPPort, cfg.SMTPFromEmail)
		return NewGmailProvider(cfg)
	case "mock":
		log.Println("📧 Initializing Mock Email Provider (local/dev)")
		return NewMockEmailProvider()
	default:
		log.Printf("⚠️ Unknown EMAIL_PROVIDER '%s', defaulting to Mock Email Provider", providerType)
		return NewMockEmailProvider()
	}
}

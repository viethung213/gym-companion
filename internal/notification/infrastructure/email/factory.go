package email

import (
	"log/slog"
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
		slog.Info("Initializing SMTP Email Provider", slog.String("host", cfg.SMTPHost), slog.String("port", cfg.SMTPPort), slog.String("from", cfg.SMTPFromEmail))
		return NewGmailProvider(cfg)
	case "mock":
		slog.Info("Initializing Mock Email Provider (local/dev)")
		return NewMockEmailProvider()
	default:
		slog.Warn("Unknown EMAIL_PROVIDER, defaulting to Mock Email Provider", slog.String("provider", providerType))
		return NewMockEmailProvider()
	}
}

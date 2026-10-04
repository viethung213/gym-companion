package port

import "context"

// EmailProvider defines the outbound port for sending emails via third-party providers.
type EmailProvider interface {
	SendEmail(ctx context.Context, to, subject, body string) error
}

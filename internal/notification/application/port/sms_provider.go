package port

import "context"

// SMSProvider defines the outbound port for sending SMS messages via third-party providers.
type SMSProvider interface {
	SendSMS(ctx context.Context, toPhoneNumber, message string) error
}

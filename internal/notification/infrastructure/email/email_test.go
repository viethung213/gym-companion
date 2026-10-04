package email_test

import (
	"context"
	"errors"
	"testing"

	"github.com/viethung213/gym-companion/internal/notification/infrastructure/config"
	"github.com/viethung213/gym-companion/internal/notification/infrastructure/email"
)

func TestFactory_Selection(t *testing.T) {
	tests := []struct {
		provider string
	}{
		{provider: "smtp"},
		{provider: "gmail"},
		{provider: "mock"},
		{provider: "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.provider, func(t *testing.T) {
			cfg := &config.Config{EmailProvider: tt.provider}
			p := email.NewEmailProvider(cfg)
			if p == nil {
				t.Fatal("expected non-nil provider from factory")
			}
		})
	}
}

func TestGmailProvider_DisabledFallback(t *testing.T) {
	cfg := &config.Config{
		SMTPHost:     "smtp.gmail.com",
		SMTPPort:     "587",
		SMTPUsername: "", // Empty credentials simulates unconfigured/dev mode
		SMTPPassword: "",
	}

	provider := email.NewGmailProvider(cfg)
	ctx := context.Background()

	err := provider.SendEmail(ctx, "recipient@example.com", "Test Subject", "Test Body")
	if err != nil {
		t.Fatalf("expected nil error on disabled fallback, got: %v", err)
	}
}

func TestGmailProvider_EmptyRecipient(t *testing.T) {
	provider := email.NewGmailProvider(&config.Config{})
	err := provider.SendEmail(context.Background(), "", "Subject", "Body")
	if err == nil {
		t.Fatal("expected error for empty recipient, got nil")
	}
}

func TestMockEmailProvider(t *testing.T) {
	mock := email.NewMockEmailProvider()
	ctx := context.Background()

	err := mock.SendEmail(ctx, "user@example.com", "Hello", "World")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sent := mock.GetSentEmails()
	if len(sent) != 1 {
		t.Fatalf("expected 1 sent email, got %d", len(sent))
	}
	if sent[0].To != "user@example.com" || sent[0].Subject != "Hello" || sent[0].Body != "World" {
		t.Errorf("sent email content mismatch: %+v", sent[0])
	}

	// Test error simulation
	testErr := errors.New("smtp connection failed")
	mock.SetError(testErr)
	if err := mock.SendEmail(ctx, "user2@example.com", "Hi", "There"); !errors.Is(err, testErr) {
		t.Fatalf("expected error %v, got %v", testErr, err)
	}

	mock.Clear()
	if len(mock.GetSentEmails()) != 0 {
		t.Errorf("expected 0 emails after Clear")
	}
}

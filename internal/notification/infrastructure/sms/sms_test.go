package sms_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/viethung213/gym-companion/internal/notification/infrastructure/config"
	"github.com/viethung213/gym-companion/internal/notification/infrastructure/sms"
)

func TestFactory_Selection(t *testing.T) {
	tests := []struct {
		provider string
		wantType string
	}{
		{provider: "twilio", wantType: "*sms.TwilioProvider"},
		{provider: "infobip", wantType: "*sms.InfobipProvider"},
		{provider: "mock", wantType: "*sms.MockSMSProvider"},
		{provider: "unknown", wantType: "*sms.MockSMSProvider"},
	}

	for _, tt := range tests {
		t.Run(tt.provider, func(t *testing.T) {
			cfg := &config.Config{SMSProvider: tt.provider}
			p := sms.NewSMSProvider(cfg)
			if p == nil {
				t.Fatal("expected non-nil provider")
			}
		})
	}
}

func TestMockSMSProvider(t *testing.T) {
	mock := sms.NewMockSMSProvider()
	ctx := context.Background()

	err := mock.SendSMS(ctx, "+84912345678", "Your OTP is 123456")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sent := mock.GetSentMessages()
	if len(sent) != 1 {
		t.Fatalf("expected 1 sent message, got %d", len(sent))
	}
	if sent[0].To != "+84912345678" || sent[0].Message != "Your OTP is 123456" {
		t.Errorf("sent message mismatch: %+v", sent[0])
	}

	testErr := errors.New("network error")
	mock.SetError(testErr)
	if err := mock.SendSMS(ctx, "+84912345678", "test"); !errors.Is(err, testErr) {
		t.Fatalf("expected error %v, got %v", testErr, err)
	}

	mock.Clear()
	if len(mock.GetSentMessages()) != 0 {
		t.Errorf("expected 0 messages after Clear")
	}
}

func TestTwilioProvider_DisabledFallback(t *testing.T) {
	cfg := &config.Config{
		TwilioAccountSID: "",
		TwilioAuthToken:  "",
	}
	provider := sms.NewTwilioProvider(cfg)
	err := provider.SendSMS(context.Background(), "+84912345678", "Hello")
	if err != nil {
		t.Fatalf("expected nil error on disabled fallback, got: %v", err)
	}
}

func TestTwilioProvider_EmptyRecipient(t *testing.T) {
	provider := sms.NewTwilioProvider(&config.Config{})
	err := provider.SendSMS(context.Background(), "", "Hello")
	if err == nil {
		t.Fatal("expected error on empty recipient, got nil")
	}
}

func TestInfobipProvider_DisabledFallback(t *testing.T) {
	cfg := &config.Config{
		InfobipBaseURL: "",
		InfobipAPIKey:  "",
	}
	provider := sms.NewInfobipProvider(cfg)
	err := provider.SendSMS(context.Background(), "+84912345678", "Hello")
	if err != nil {
		t.Fatalf("expected nil error on disabled fallback, got: %v", err)
	}
}

func TestInfobipProvider_SuccessHTTP(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "App test-api-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"messages":[{"status":{"groupName":"PENDING"}}]}`))
	}))
	defer server.Close()

	cfg := &config.Config{
		InfobipBaseURL: server.URL,
		InfobipAPIKey:  "test-api-key",
		InfobipFrom:    "GymComp",
	}

	provider := sms.NewInfobipProvider(cfg)
	err := provider.SendSMS(context.Background(), "+84912345678", "Test Infobip message")
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
}

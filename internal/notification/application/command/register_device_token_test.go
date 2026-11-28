package command_test

import (
	"context"
	"errors"
	"testing"

	"github.com/viethung213/gym-companion/internal/notification/application/command"
)

func TestRegisterDeviceTokenHandler(t *testing.T) {
	t.Parallel()

	deviceRepo := &mockDeviceRepo{}
	pushProvider := &mockPushProvider{}
	handler := command.NewRegisterDeviceTokenHandler(deviceRepo, pushProvider)

	t.Run("successful registration", func(t *testing.T) {
		t.Parallel()

		cmd := command.RegisterDeviceTokenCommand{
			UserID:      "usr-100",
			DeviceToken: "fcm-token-100",
			DeviceType:  "ANDROID",
		}

		err := handler.Handle(context.Background(), cmd)
		if err != nil {
			t.Fatalf("got unexpected error: %v", err)
		}

		if len(pushProvider.subscribedTokens) != 1 || pushProvider.subscribedTokens[0] != "fcm-token-100" {
			t.Errorf("expected subscribed token 'fcm-token-100', got %v", pushProvider.subscribedTokens)
		}
		if pushProvider.subscribedTopic != "all_users" {
			t.Errorf("expected subscribed topic 'all_users', got %s", pushProvider.subscribedTopic)
		}
	})

	t.Run("invalid device type error", func(t *testing.T) {
		t.Parallel()

		cmd := command.RegisterDeviceTokenCommand{
			UserID:      "usr-100",
			DeviceToken: "fcm-token-100",
			DeviceType:  "INVALID_TYPE",
		}

		err := handler.Handle(context.Background(), cmd)
		if err == nil {
			t.Fatal("got nil error, want error for invalid device type")
		}
	})

	t.Run("fcm topic subscription failure fails registration fast without saving", func(t *testing.T) {
		t.Parallel()

		failingRepo := &mockDeviceRepo{}
		failingProvider := &mockPushProvider{subErr: errors.New("fcm network timeout")}
		h := command.NewRegisterDeviceTokenHandler(failingRepo, failingProvider)

		cmd := command.RegisterDeviceTokenCommand{
			UserID:      "usr-fail",
			DeviceToken: "fcm-token-fail",
			DeviceType:  "ANDROID",
		}

		err := h.Handle(context.Background(), cmd)
		if err == nil {
			t.Fatal("expected error when FCM subscribe fails, got nil")
		}

		// Verify DB save was NOT called
		if len(failingRepo.devices) != 0 {
			t.Fatalf("expected 0 devices saved in DB when FCM subscription fails, got %d", len(failingRepo.devices))
		}
	})
}

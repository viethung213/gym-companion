package command_test

import (
	"context"
	"testing"

	"github.com/viethung213/gym-companion/internal/notification/application/command"
)

func TestRegisterDeviceTokenHandler(t *testing.T) {
	t.Parallel()

	deviceRepo := &mockDeviceRepo{}
	handler := command.NewRegisterDeviceTokenHandler(deviceRepo)

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
}

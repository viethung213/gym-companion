package aggregate_test

import (
	"testing"

	"github.com/viethung213/gym-companion/internal/notification/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/notification/domain/derror"
	"github.com/viethung213/gym-companion/internal/notification/domain/vo"
)

func TestNewDevice(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		giveID     string
		giveUserID string
		giveToken  string
		giveType   vo.DeviceType
		wantErrIs  error
		wantActive bool
	}{
		{
			name:       "valid device creation",
			giveID:     "dev-123",
			giveUserID: "usr-123",
			giveToken:  "fcm-token-abc",
			giveType:   vo.DeviceTypeAndroid,
			wantErrIs:  nil,
			wantActive: true,
		},
		{
			name:       "empty user ID",
			giveID:     "dev-123",
			giveUserID: "",
			giveToken:  "fcm-token-abc",
			giveType:   vo.DeviceTypeIOS,
			wantErrIs:  derror.ErrEmptyUserID,
			wantActive: false,
		},
		{
			name:       "empty device token",
			giveID:     "dev-123",
			giveUserID: "usr-123",
			giveToken:  "",
			giveType:   vo.DeviceTypeWeb,
			wantErrIs:  derror.ErrEmptyDeviceToken,
			wantActive: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dev, err := aggregate.NewDevice(tt.giveID, tt.giveUserID, tt.giveToken, tt.giveType)
			if tt.wantErrIs != nil {
				if err == nil {
					t.Fatalf("got nil error, want %v", tt.wantErrIs)
				}
				return
			}

			if err != nil {
				t.Fatalf("got unexpected error: %v", err)
			}

			if got := dev.ID(); got != tt.giveID {
				t.Errorf("got ID %s, want %s", got, tt.giveID)
			}

			if got := dev.IsActive(); got != tt.wantActive {
				t.Errorf("got isActive = %v, want %v", got, tt.wantActive)
			}

			dev.Deactivate()
			if got := dev.IsActive(); got != false {
				t.Errorf("after Deactivate(), got isActive = %v, want false", got)
			}
		})
	}
}

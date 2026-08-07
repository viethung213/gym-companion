package vo_test

import (
	"errors"
	"testing"

	"github.com/viethung213/gym-companion/internal/notification/domain/vo"
)

func TestNewDeviceType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		give      string
		want      vo.DeviceType
		wantErrIs error
	}{
		{
			name:      "valid IOS lowercase",
			give:      "ios",
			want:      vo.DeviceTypeIOS,
			wantErrIs: nil,
		},
		{
			name:      "valid ANDROID uppercase",
			give:      "ANDROID",
			want:      vo.DeviceTypeAndroid,
			wantErrIs: nil,
		},
		{
			name:      "valid WEB with whitespace",
			give:      " web ",
			want:      vo.DeviceTypeWeb,
			wantErrIs: nil,
		},
		{
			name:      "invalid device type",
			give:      "WINDOWS_PHONE",
			want:      "",
			wantErrIs: vo.ErrInvalidDeviceType,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := vo.NewDeviceType(tt.give)
			if tt.wantErrIs != nil {
				if !errors.Is(err, tt.wantErrIs) {
					t.Fatalf("got error %v, want %v", err, tt.wantErrIs)
				}
				return
			}

			if err != nil {
				t.Fatalf("got unexpected error: %v", err)
			}

			if got != tt.want {
				t.Errorf("got device type %s, want %s", got, tt.want)
			}

			if got.String() != string(tt.want) {
				t.Errorf("got String() %s, want %s", got.String(), tt.want)
			}
		})
	}
}

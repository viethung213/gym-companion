package vo_test

import (
	"errors"
	"testing"

	"github.com/viethung213/gym-companion/internal/notification/domain/vo"
)

func TestNewChannelType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		give      string
		want      vo.ChannelType
		wantErrIs error
	}{
		{
			name:      "valid PUSH lowercase",
			give:      "push",
			want:      vo.ChannelTypePush,
			wantErrIs: nil,
		},
		{
			name:      "valid EMAIL uppercase",
			give:      "EMAIL",
			want:      vo.ChannelTypeEmail,
			wantErrIs: nil,
		},
		{
			name:      "valid SMS with whitespace",
			give:      " sms ",
			want:      vo.ChannelTypeSMS,
			wantErrIs: nil,
		},
		{
			name:      "invalid channel type",
			give:      "PIGEON",
			want:      "",
			wantErrIs: vo.ErrInvalidChannel,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := vo.NewChannelType(tt.give)
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
				t.Errorf("got channel type %s, want %s", got, tt.want)
			}

			if got.String() != string(tt.want) {
				t.Errorf("got String() %s, want %s", got.String(), tt.want)
			}
		})
	}
}

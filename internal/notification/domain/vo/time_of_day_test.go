package vo_test

import (
	"errors"
	"testing"

	"github.com/viethung213/gym-companion/internal/notification/domain/vo"
)

func TestNewTimeOfDay(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		give      string
		want      string
		wantErrIs error
	}{
		{
			name:      "empty string is valid",
			give:      "",
			want:      "",
			wantErrIs: nil,
		},
		{
			name:      "whitespace string is valid empty",
			give:      "   ",
			want:      "",
			wantErrIs: nil,
		},
		{
			name:      "valid 22:00",
			give:      "22:00",
			want:      "22:00",
			wantErrIs: nil,
		},
		{
			name:      "valid 07:30",
			give:      "07:30",
			want:      "07:30",
			wantErrIs: nil,
		},
		{
			name:      "invalid 25:00",
			give:      "25:00",
			want:      "",
			wantErrIs: vo.ErrInvalidTimeFormat,
		},
		{
			name:      "invalid format 9:00",
			give:      "9:00",
			want:      "",
			wantErrIs: vo.ErrInvalidTimeFormat,
		},
		{
			name:      "invalid random string",
			give:      "abc",
			want:      "",
			wantErrIs: vo.ErrInvalidTimeFormat,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := vo.NewTimeOfDay(tt.give)
			if tt.wantErrIs != nil {
				if !errors.Is(err, tt.wantErrIs) {
					t.Fatalf("got error %v, want %v", err, tt.wantErrIs)
				}
				return
			}

			if err != nil {
				t.Fatalf("got unexpected error: %v", err)
			}

			if got.String() != tt.want {
				t.Errorf("got String() %s, want %s", got.String(), tt.want)
			}
		})
	}
}

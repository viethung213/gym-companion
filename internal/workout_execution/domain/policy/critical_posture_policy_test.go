package policy_test

import (
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/workout_execution/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/policy"
)

func TestIsCritical(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		give aggregate.SessionError
		want bool
	}{
		{
			name: "severity CRITICAL returns true",
			give: aggregate.SessionError{Severity: "CRITICAL", ErrorCode: "ERR_UNKNOWN"},
			want: true,
		},
		{
			name: "ERR_BAR_TRAPPED returns true even without CRITICAL severity",
			give: aggregate.SessionError{Severity: "WARNING", ErrorCode: "ERR_BAR_TRAPPED"},
			want: true,
		},
		{
			name: "ERR_FALL_DETECTED returns true even without CRITICAL severity",
			give: aggregate.SessionError{Severity: "WARNING", ErrorCode: "ERR_FALL_DETECTED"},
			want: true,
		},
		{
			name: "non-critical severity and unknown code returns false",
			give: aggregate.SessionError{Severity: "WARNING", ErrorCode: "ERR_ELBOW_FLARE"},
			want: false,
		},
		{
			name: "empty severity and unknown code returns false",
			give: aggregate.SessionError{},
			want: false,
		},
		{
			name: "both CRITICAL severity and registered code returns true",
			give: aggregate.SessionError{
				Severity:  "CRITICAL",
				ErrorCode: "ERR_BAR_TRAPPED",
				Timestamp: time.Now(),
			},
			want: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := policy.IsCritical(&tt.give)
			if got != tt.want {
				t.Errorf("IsCritical() = %v, want %v (errorCode=%q, severity=%q)",
					got, tt.want, tt.give.ErrorCode, tt.give.Severity)
			}
		})
	}
}

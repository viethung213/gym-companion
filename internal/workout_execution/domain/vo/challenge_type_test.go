package vo_test

import (
	"testing"

	"github.com/viethung213/gym-companion/internal/workout_execution/domain/vo"
)

func TestParseChallengeType(t *testing.T) {
	tests := []struct {
		name string
		give string
		want vo.ChallengeType
	}{
		{
			name: "uppercase reps",
			give: "REPS",
			want: vo.ChallengeTypeReps,
		},
		{
			name: "lowercase reps",
			give: "reps",
			want: vo.ChallengeTypeReps,
		},
		{
			name: "proto reps enum string",
			give: "SOLO_CHALLENGE_TYPE_REPS",
			want: vo.ChallengeTypeReps,
		},
		{
			name: "uppercase duration",
			give: "DURATION",
			want: vo.ChallengeTypeDuration,
		},
		{
			name: "lowercase duration with spaces",
			give: "  duration  ",
			want: vo.ChallengeTypeDuration,
		},
		{
			name: "proto duration enum string",
			give: "SOLO_CHALLENGE_TYPE_DURATION",
			want: vo.ChallengeTypeDuration,
		},
		{
			name: "unspecified / empty",
			give: "",
			want: vo.ChallengeTypeUnspecified,
		},
		{
			name: "unknown value",
			give: "UNKNOWN_TYPE",
			want: vo.ChallengeTypeUnspecified,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := vo.ParseChallengeType(tt.give)
			if got != tt.want {
				t.Fatalf("ParseChallengeType(%q) = %v, want %v", tt.give, got, tt.want)
			}
		})
	}
}

func TestChallengeType_IsValid(t *testing.T) {
	tests := []struct {
		name string
		give vo.ChallengeType
		want bool
	}{
		{
			name: "valid reps",
			give: vo.ChallengeTypeReps,
			want: true,
		},
		{
			name: "valid duration",
			give: vo.ChallengeTypeDuration,
			want: true,
		},
		{
			name: "invalid unspecified",
			give: vo.ChallengeTypeUnspecified,
			want: false,
		},
		{
			name: "invalid arbitrary",
			give: vo.ChallengeType("INVALID"),
			want: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := tt.give.IsValid()
			if got != tt.want {
				t.Fatalf("ChallengeType(%q).IsValid() = %v, want %v", tt.give, got, tt.want)
			}
		})
	}
}

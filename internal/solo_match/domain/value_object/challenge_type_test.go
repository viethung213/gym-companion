package valueobject_test

import (
	"testing"

	valueobject "github.com/viethung213/gym-companion/internal/solo_match/domain/value_object"
)

func TestChallengeType(t *testing.T) {
	tests := []struct {
		give    string
		want    valueobject.ChallengeType
		wantErr bool
	}{
		{give: "REPS", want: valueobject.ChallengeTypeReps, wantErr: false},
		{give: "DURATION", want: valueobject.ChallengeTypeDuration, wantErr: false},
		{give: "UNKNOWN_TYPE", want: valueobject.ChallengeTypeUnknown, wantErr: true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.give, func(t *testing.T) {
			got, err := valueobject.NewChallengeType(tt.give)
			if (err != nil) != tt.wantErr {
				t.Fatalf("NewChallengeType(%q) err got %v, wantErr %v", tt.give, err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("NewChallengeType(%q) got %v, want %v", tt.give, got, tt.want)
			}
		})
	}
}

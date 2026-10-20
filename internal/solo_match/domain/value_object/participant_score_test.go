package valueobject_test

import (
	"testing"

	valueobject "github.com/viethung213/gym-companion/internal/solo_match/domain/value_object"
)

func TestParticipantScore(t *testing.T) {
	tests := []struct {
		name    string
		give    float64
		want    float64
		wantErr bool
	}{
		{name: "valid score 25", give: 25.0, want: 25.0, wantErr: false},
		{name: "zero score", give: 0.0, want: 0.0, wantErr: false},
		{name: "negative score", give: -1.0, want: 0.0, wantErr: true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			got, err := valueobject.NewParticipantScore(tt.give)
			if (err != nil) != tt.wantErr {
				t.Fatalf("NewParticipantScore(%v) err got %v, wantErr %v", tt.give, err, tt.wantErr)
			}
			if !tt.wantErr && got.Value() != tt.want {
				t.Errorf("ParticipantScore.Value() got %v, want %v", got.Value(), tt.want)
			}
		})
	}
}

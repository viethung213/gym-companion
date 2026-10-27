package valueobject_test

import (
	"testing"

	valueobject "github.com/viethung213/gym-companion/internal/solo_match/domain/value_object"
)

func TestMatchResult(t *testing.T) {
	tests := []struct {
		give    string
		want    valueobject.MatchResult
		wantErr bool
	}{
		{give: "PENDING", want: valueobject.MatchResultPending, wantErr: false},
		{give: "WIN", want: valueobject.MatchResultWin, wantErr: false},
		{give: "LOSE", want: valueobject.MatchResultLose, wantErr: false},
		{give: "DRAW", want: valueobject.MatchResultDraw, wantErr: false},
		{give: "FORFEITED", want: valueobject.MatchResultForfeited, wantErr: false},
		{give: "BAD", want: valueobject.MatchResultUnknown, wantErr: true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.give, func(t *testing.T) {
			got, err := valueobject.NewMatchResult(tt.give)
			if (err != nil) != tt.wantErr {
				t.Fatalf("NewMatchResult(%q) err got %v, wantErr %v", tt.give, err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("NewMatchResult(%q) got %v, want %v", tt.give, got, tt.want)
			}
		})
	}
}

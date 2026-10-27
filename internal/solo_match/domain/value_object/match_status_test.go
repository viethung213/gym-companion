package valueobject_test

import (
	"testing"

	valueobject "github.com/viethung213/gym-companion/internal/solo_match/domain/value_object"
)

func TestMatchStatus(t *testing.T) {
	tests := []struct {
		give    string
		want    valueobject.MatchStatus
		wantErr bool
	}{
		{give: "PENDING_READY", want: valueobject.MatchStatusPendingReady, wantErr: false},
		{give: "IN_PROGRESS", want: valueobject.MatchStatusInProgress, wantErr: false},
		{give: "COMPLETED", want: valueobject.MatchStatusCompleted, wantErr: false},
		{give: "FORFEITED", want: valueobject.MatchStatusForfeited, wantErr: false},
		{give: "CANCELLED", want: valueobject.MatchStatusCancelled, wantErr: false},
		{give: "INVALID", want: valueobject.MatchStatusUnknown, wantErr: true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.give, func(t *testing.T) {
			got, err := valueobject.NewMatchStatus(tt.give)
			if (err != nil) != tt.wantErr {
				t.Fatalf("NewMatchStatus(%q) err got %v, wantErr %v", tt.give, err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("NewMatchStatus(%q) got %v, want %v", tt.give, got, tt.want)
			}
		})
	}

	t.Run("IsFinal checks final states", func(t *testing.T) {
		if !valueobject.MatchStatusCompleted.IsFinal() {
			t.Errorf("MatchStatusCompleted.IsFinal() got false, want true")
		}
		if !valueobject.MatchStatusForfeited.IsFinal() {
			t.Errorf("MatchStatusForfeited.IsFinal() got false, want true")
		}
		if !valueobject.MatchStatusCancelled.IsFinal() {
			t.Errorf("MatchStatusCancelled.IsFinal() got false, want true")
		}
		if valueobject.MatchStatusInProgress.IsFinal() {
			t.Errorf("MatchStatusInProgress.IsFinal() got true, want false")
		}
		if valueobject.MatchStatusPendingReady.IsFinal() {
			t.Errorf("MatchStatusPendingReady.IsFinal() got true, want false")
		}
	})
}

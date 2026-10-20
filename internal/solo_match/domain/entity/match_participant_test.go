package entity_test

import (
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/solo_match/domain/entity"
	valueobject "github.com/viethung213/gym-companion/internal/solo_match/domain/value_object"
)

func TestMatchParticipant(t *testing.T) {
	now := time.Now().UTC()

	t.Run("fails when required IDs are empty", func(t *testing.T) {
		if _, err := entity.NewMatchParticipant("", "m_1", "u_1"); err == nil {
			t.Errorf("expected error for empty participant ID, got nil")
		}
		if _, err := entity.NewMatchParticipant("p_1", "", "u_1"); err == nil {
			t.Errorf("expected error for empty match ID, got nil")
		}
		if _, err := entity.NewMatchParticipant("p_1", "m_1", ""); err == nil {
			t.Errorf("expected error for empty user ID, got nil")
		}
	})

	t.Run("successfully creates participant with initial pending state", func(t *testing.T) {
		p, err := entity.NewMatchParticipant("p_1", "m_1", "u_1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if p.ID() != "p_1" || p.MatchID() != "m_1" || p.UserID() != "u_1" {
			t.Errorf("unexpected participant identifiers: %+v", p)
		}
		if p.IsCompleted() {
			t.Errorf("IsCompleted got true, want false")
		}
		if p.CompletedAt() != nil {
			t.Errorf("CompletedAt got %v, want nil", p.CompletedAt())
		}
		if p.Result() != valueobject.MatchResultPending {
			t.Errorf("Result got %v, want PENDING", p.Result())
		}
	})

	t.Run("fails to record negative score", func(t *testing.T) {
		p, _ := entity.NewMatchParticipant("p_1", "m_1", "u_1")
		err := p.RecordResult(-5, now)
		if err == nil {
			t.Errorf("expected error for negative score, got nil")
		}
	})

	t.Run("successfully records valid score", func(t *testing.T) {
		p, _ := entity.NewMatchParticipant("p_1", "m_1", "u_1")
		err := p.RecordResult(20, now)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !p.IsCompleted() {
			t.Errorf("IsCompleted got false, want true")
		}
		if p.Score().Value() != 20 {
			t.Errorf("Score got %v, want 20", p.Score().Value())
		}
		if p.CompletedAt() == nil || !p.CompletedAt().Equal(now) {
			t.Errorf("CompletedAt got %v, want %v", p.CompletedAt(), now)
		}
	})

	t.Run("fails to record result if already finished", func(t *testing.T) {
		p, _ := entity.NewMatchParticipant("p_1", "m_1", "u_1")
		_ = p.RecordResult(20, now)

		err := p.RecordResult(25, now)
		if err == nil {
			t.Errorf("expected error when recording result again, got nil")
		}
	})

	t.Run("successfully sets forfeit state", func(t *testing.T) {
		p, _ := entity.NewMatchParticipant("p_2", "m_1", "u_2")
		p.Forfeit()

		if !p.IsCompleted() {
			t.Errorf("Forfeited participant IsCompleted got false, want true")
		}
		if p.Result() != valueobject.MatchResultForfeited {
			t.Errorf("Forfeited participant Result got %v, want FORFEITED", p.Result())
		}
	})

	t.Run("reconstitutes participant accurately from persistence", func(t *testing.T) {
		score, _ := valueobject.NewParticipantScore(15)
		p := entity.ReconstituteParticipant("p_3", "m_1", "u_3", score, true, &now, valueobject.MatchResultWin)

		if p.ID() != "p_3" || p.Score().Value() != 15 || p.Result() != valueobject.MatchResultWin {
			t.Errorf("Reconstituted participant mismatch: %+v", p)
		}
	})
}

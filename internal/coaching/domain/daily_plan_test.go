package domain_test

import (
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/coaching/domain"
)

func TestNewDailyWorkoutPlan(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC()
	mainEx := []domain.PrescribedExercise{
		domain.NewPrescribedExercise("ex_bench", "Bench Press", 3, 10, 60.0, 0, "Focus on chest stretch", 90, 120, 7.5),
	}
	prescription := domain.NewWorkoutPrescription(nil, mainEx, nil)

	t.Run("successful creation", func(t *testing.T) {
		t.Parallel()
		p, err := domain.NewDailyWorkoutPlan("dwp_1", "usr_100", "rdp_1", "ws_1", now, domain.DailyPlanStatusActive, prescription, "Reasoning", "")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if p.ID() != "dwp_1" {
			t.Errorf("expected ID dwp_1, got %s", p.ID())
		}
		if len(p.Prescription().MainExercises()) != 1 {
			t.Errorf("expected 1 main exercise, got %d", len(p.Prescription().MainExercises()))
		}
	})

	t.Run("empty main exercises returns error", func(t *testing.T) {
		t.Parallel()
		emptyPrescription := domain.NewWorkoutPrescription(nil, nil, nil)
		_, err := domain.NewDailyWorkoutPlan("dwp_1", "usr_100", "rdp_1", "ws_1", now, domain.DailyPlanStatusActive, emptyPrescription, "", "")
		if err != domain.ErrEmptyPrescription {
			t.Errorf("expected ErrEmptyPrescription, got %v", err)
		}
	})

	t.Run("activate and complete plan", func(t *testing.T) {
		t.Parallel()
		p, _ := domain.NewDailyWorkoutPlan("dwp_1", "usr_100", "rdp_1", "ws_1", now, domain.DailyPlanStatusDraft, prescription, "", "")
		p.Activate()
		if p.Status() != domain.DailyPlanStatusActive {
			t.Errorf("expected status DailyPlanStatusActive, got %v", p.Status())
		}
		p.Complete()
		if p.Status() != domain.DailyPlanStatusCompleted {
			t.Errorf("expected status DailyPlanStatusCompleted, got %v", p.Status())
		}
	})
}

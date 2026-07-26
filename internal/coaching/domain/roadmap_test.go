package domain_test

import (
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/coaching/domain"
)

func TestNewWorkoutRoadmap(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC()
	startDate := now
	endDate := now.AddDate(0, 0, 28)

	t.Run("successful creation", func(t *testing.T) {
		t.Parallel()
		r, err := domain.NewWorkoutRoadmap("rdp_123", "usr_100", startDate, endDate)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if r.ID() != "rdp_123" {
			t.Errorf("expected ID rdp_123, got %s", r.ID())
		}
		if r.UserID() != "usr_100" {
			t.Errorf("expected UserID usr_100, got %s", r.UserID())
		}
		if !r.IsActive() {
			t.Errorf("expected active roadmap")
		}
	})

	t.Run("empty user id returns error", func(t *testing.T) {
		t.Parallel()
		_, err := domain.NewWorkoutRoadmap("rdp_123", "", startDate, endDate)
		if err != domain.ErrInvalidUser {
			t.Errorf("expected ErrInvalidUser, got %v", err)
		}
	})

	t.Run("invalid date range returns error", func(t *testing.T) {
		t.Parallel()
		_, err := domain.NewWorkoutRoadmap("rdp_123", "usr_100", endDate, startDate)
		if err != domain.ErrInvalidDates {
			t.Errorf("expected ErrInvalidDates, got %v", err)
		}
	})

	t.Run("complete active roadmap", func(t *testing.T) {
		t.Parallel()
		r, _ := domain.NewWorkoutRoadmap("rdp_123", "usr_100", startDate, endDate)
		err := r.Complete()
		if err != nil {
			t.Fatalf("expected no error on complete, got %v", err)
		}
		if r.IsActive() {
			t.Errorf("expected inactive roadmap after complete")
		}
		if r.Status() != domain.RoadmapStatusCompleted {
			t.Errorf("expected status RoadmapStatusCompleted, got %v", r.Status())
		}
	})
}

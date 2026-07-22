package domain

import (
	"testing"
	"time"
)

func TestWorkoutRoadmap(t *testing.T) {
	now := time.Now()
	
	// Test NewWorkoutRoadmap
	r, err := NewWorkoutRoadmap("rm-1", "user-1", PlanningTierBeginner, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.ID() != "rm-1" {
		t.Errorf("got %v, want rm-1", r.ID())
	}
	if r.UserID() != "user-1" {
		t.Errorf("got %v, want user-1", r.UserID())
	}
	if r.Status() != PlanStatusDraft {
		t.Errorf("got %v, want DRAFT", r.Status())
	}
	if r.PlanningTier() != PlanningTierBeginner {
		t.Errorf("got %v, want BEGINNER", r.PlanningTier())
	}
	
	// Test Activate
	if err := r.Activate(now); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.Status() != PlanStatusActive {
		t.Errorf("got %v, want ACTIVE", r.Status())
	}
	
	// Test Activate again (should fail)
	if err := r.Activate(now); err == nil {
		t.Errorf("expected error activating already active roadmap")
	}
	
	// Test Complete
	if err := r.Complete(now); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.Status() != PlanStatusCompleted {
		t.Errorf("got %v, want COMPLETED", r.Status())
	}
	
	// Invalid Creation
	_, err = NewWorkoutRoadmap("", "user-1", PlanningTierBeginner, now)
	if err == nil {
		t.Errorf("expected error on empty ID")
	}
}

package domain

import (
	"testing"
	"time"
)

func TestDailyWorkoutPlan(t *testing.T) {
	now := time.Now()
	
	dp, err := NewDailyWorkoutPlan("dp-1", "ws-1", "user-1", now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	
	if dp.Status() != PlanStatusDraft {
		t.Errorf("expected DRAFT")
	}
	
	presc, err := NewWorkoutPrescription(3, 10, 50, 7, 90)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	
	if err := dp.AddPrescription(presc); err != nil {
		t.Fatalf("unexpected error adding prescription: %v", err)
	}
	
	if err := dp.Activate(now); err != nil {
		t.Fatalf("unexpected error activating plan: %v", err)
	}
	
	if err := dp.Complete(now); err != nil {
		t.Fatalf("unexpected error completing plan: %v", err)
	}
	
	if err := dp.AddPrescription(presc); err == nil {
		t.Errorf("expected error adding prescription to completed plan")
	}
	
	// Cancel
	dp2, _ := NewDailyWorkoutPlan("dp-2", "ws-1", "user-1", now)
	if err := dp2.Cancel(now); err != nil {
		t.Errorf("unexpected error cancelling plan")
	}
}

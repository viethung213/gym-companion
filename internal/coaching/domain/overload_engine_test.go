package domain

import (
	"testing"
)

func TestOverloadEngine(t *testing.T) {
	engine := NewOverloadEngine()
	
	// 1RM
	bench1rm := engine.Calculate1RMEstimate(100, "Bench Press")
	if bench1rm != 50 {
		t.Errorf("got %v, want 50", bench1rm)
	}
	
	// Log evaluation - FastTrack
	prog, err := engine.EvaluateLog(10, 10, 6, 80)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if prog != ProgressionFastTrack {
		t.Errorf("got %v, want FAST_TRACK", prog)
	}
	
	// Log evaluation - DownTrack
	prog, err = engine.EvaluateLog(6, 10, 9, 50)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if prog != ProgressionDownTrack {
		t.Errorf("got %v, want DOWN_TRACK", prog)
	}
	
	// Hard cap
	_, err = engine.EvaluateLog(25, 10, 6, 80)
	if err == nil {
		t.Errorf("expected error for exceeding hard cap")
	}
	
	// Next weight fast track (experienced vs beginner)
	nw := engine.NextWeight(100, ProgressionFastTrack, false)
	if nw != 105 {
		t.Errorf("got %v, want 105", nw)
	}

	nwBeginner := engine.NextWeight(100, ProgressionFastTrack, true)
	if nwBeginner != 105 {
		t.Errorf("got %v, want 105", nwBeginner)
	}

	// Warmup
	wu := engine.CalculateWarmupSet(100, true)
	if wu == nil || wu.Weight() != 50 {
		t.Errorf("invalid warmup")
	}

	// Periodization Multipliers
	if m := engine.GetPeriodizationMultiplier(1); m != 0.65 {
		t.Errorf("week 1 periodization got %v, want 0.65", m)
	}
	if m := engine.GetPeriodizationMultiplier(3); m != 0.85 {
		t.Errorf("week 3 periodization got %v, want 0.85", m)
	}

	// Rest period
	rest := engine.RestPeriod("Compound", true)
	if rest != 150 {
		t.Errorf("got %v, want 150", rest)
	}

	// GeneratePrescription
	wp, err := engine.GeneratePrescription(FitnessGoalMuscleGain, "BEGINNER", 1, 100, "Compound")
	if err != nil {
		t.Fatalf("GeneratePrescription failed: %v", err)
	}
	if wp.Reps() != 10 || wp.Sets() != 3 {
		t.Errorf("got reps %v sets %v, want reps 10 sets 3", wp.Reps(), wp.Sets())
	}

	// Estimate duration
	pe, err := NewPlannedExercise("ex-1", wp, "note")
	if err != nil {
		t.Fatalf("NewPlannedExercise failed: %v", err)
	}
	duration := engine.EstimateSessionDurationMinutes([]PlannedExercise{pe}, true)
	if duration <= 10 {
		t.Errorf("got duration %v, want > 10", duration)
	}
}




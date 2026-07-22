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
	
	// Next weight fast track
	nw := engine.NextWeight(100, ProgressionFastTrack)
	if nw != 105 {
		t.Errorf("got %v, want 105", nw)
	}
	
	// Warmup
	wu := engine.CalculateWarmupSet(100, true)
	if wu == nil || wu.Weight != 50 {
		t.Errorf("invalid warmup")
	}
	
	// Rest period
	rest := engine.RestPeriod("Compound", true)
	if rest != 150 {
		t.Errorf("got %v, want 150", rest)
	}
}

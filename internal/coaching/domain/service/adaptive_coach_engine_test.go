package service

import (
	"testing"
	"time"
)

func TestEvaluateTriggerA_ProgressiveOverload(t *testing.T) {
	e := NewAdaptiveCoachEngine()

	weeks := []WeeklyMetrics{
		{WeekNumber: 1, SCR: 85, AvgDeltaRPE: 0.5, MaxActualRPE: 8, HighRPESessions: 0, TotalSessions: 5},
	}

	d := e.EvaluateTriggerA(weeks)

	if d.Kind != AdaptationIncreaseLoad {
		t.Fatalf("kind=%v, want IncreaseLoad", d.Kind)
	}

	if d.Params.WeightPctChange <= 0 {
		t.Errorf("expected positive weight change, got %v", d.Params.WeightPctChange)
	}
}

func TestEvaluateTriggerA_DeloadOnFatigue(t *testing.T) {
	e := NewAdaptiveCoachEngine()

	weeks := []WeeklyMetrics{
		{WeekNumber: 1, SCR: 90, AvgDeltaRPE: 2.5, TotalSessions: 5},
	}

	d := e.EvaluateTriggerA(weeks)

	if d.Kind != AdaptationTriggerDeload {
		t.Fatalf("kind=%v, want TriggerDeload", d.Kind)
	}

	if d.Params.VolumePctChange != -0.30 || d.Params.WeightPctChange != -0.10 {
		t.Errorf("deload params wrong: %+v", d.Params)
	}
}

func TestEvaluateTriggerA_DeloadOnHighRPESessions(t *testing.T) {
	e := NewAdaptiveCoachEngine()

	weeks := []WeeklyMetrics{
		{WeekNumber: 1, SCR: 100, AvgDeltaRPE: 1.5, HighRPESessions: 3, TotalSessions: 5},
	}

	d := e.EvaluateTriggerA(weeks)

	if d.Kind != AdaptationTriggerDeload {
		t.Fatalf("kind=%v, want TriggerDeload (3+ RPE≥9)", d.Kind)
	}
}

func TestEvaluateTriggerA_ReduceOnLowSCR2Weeks(t *testing.T) {
	e := NewAdaptiveCoachEngine()

	weeks := []WeeklyMetrics{
		{WeekNumber: 1, SCR: 40, TotalSessions: 5},
		{WeekNumber: 2, SCR: 45, TotalSessions: 5},
	}

	d := e.EvaluateTriggerA(weeks)

	if d.Kind != AdaptationProposeExpressFormat {
		t.Fatalf("kind=%v, want ProposeExpressFormat", d.Kind)
	}
}

func TestEvaluateTriggerA_NoOpWhenBorderline(t *testing.T) {
	e := NewAdaptiveCoachEngine()

	weeks := []WeeklyMetrics{
		{WeekNumber: 1, SCR: 70, AvgDeltaRPE: -1.5, TotalSessions: 5},
	}

	d := e.EvaluateTriggerA(weeks)

	if d.Kind != AdaptationNoOp {
		t.Errorf("expected NoOp, got %v", d.Kind)
	}
}

func TestEvaluateTriggerA_EmptyReturnsNoOp(t *testing.T) {
	e := NewAdaptiveCoachEngine()

	d := e.EvaluateTriggerA(nil)

	if d.Kind != AdaptationNoOp {
		t.Errorf("expected NoOp, got %v", d.Kind)
	}
}

func TestDetectSignalB1_ThreeConsecutiveSkips(t *testing.T) {
	e := NewAdaptiveCoachEngine()

	base := time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC)

	history := []SessionOutcome{
		{ScheduledDate: base, WasCompleted: true, Weekday: time.Monday},
		{ScheduledDate: base.AddDate(0, 0, 2), WasCompleted: true, Weekday: time.Wednesday},
		{ScheduledDate: base.AddDate(0, 0, 7), Skipped: true, Weekday: time.Monday},
		{ScheduledDate: base.AddDate(0, 0, 9), Skipped: true, Weekday: time.Wednesday},
		{ScheduledDate: base.AddDate(0, 0, 11), Skipped: true, Weekday: time.Friday},
	}

	now := base.AddDate(0, 0, 12)

	d := e.DetectSignalB1(history, now)

	if d.Kind != AdaptationSignalB1AcuteDropout {
		t.Errorf("expected B1, got %v", d.Kind)
	}
}

func TestDetectSignalB1_ThreeConsecutiveAborted(t *testing.T) {
	e := NewAdaptiveCoachEngine()

	base := time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC)

	history := []SessionOutcome{
		{ScheduledDate: base, WasCompleted: true, Weekday: time.Monday},
		{ScheduledDate: base.AddDate(0, 0, 2), WasCompleted: true, Weekday: time.Wednesday},
		{ScheduledDate: base.AddDate(0, 0, 7), Aborted: true, Weekday: time.Monday},
		{ScheduledDate: base.AddDate(0, 0, 9), Aborted: true, Weekday: time.Wednesday},
		{ScheduledDate: base.AddDate(0, 0, 11), Aborted: true, Weekday: time.Friday},
	}

	now := base.AddDate(0, 0, 12)

	d := e.DetectSignalB1(history, now)

	if d.Kind != AdaptationSignalB1AcuteDropout {
		t.Errorf("expected B1 for aborted sessions, got %v", d.Kind)
	}
}

func TestDetectSignalB1_SevenDaysNoCompleted(t *testing.T) {
	e := NewAdaptiveCoachEngine()

	base := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)

	history := []SessionOutcome{
		{ScheduledDate: base, WasCompleted: true, Weekday: time.Monday},
	}

	// 10 days later, no new completions

	now := base.AddDate(0, 0, 10)

	d := e.DetectSignalB1(history, now)

	if d.Kind != AdaptationSignalB1AcuteDropout {
		t.Errorf("expected B1 (inactivity), got %v", d.Kind)
	}
}

func TestDetectSignalB1_NoOpWhenRecent(t *testing.T) {
	e := NewAdaptiveCoachEngine()

	now := time.Date(2026, 7, 28, 0, 0, 0, 0, time.UTC)

	history := []SessionOutcome{
		{ScheduledDate: now.AddDate(0, 0, -2), WasCompleted: true, Weekday: time.Sunday},
	}

	d := e.DetectSignalB1(history, now)

	if d.Kind != AdaptationNoOp {
		t.Errorf("expected NoOp, got %v", d.Kind)
	}
}

func TestDetectSignalB2_SameWeekdaySkipped(t *testing.T) {
	e := NewAdaptiveCoachEngine()

	base := time.Date(2026, 7, 6, 0, 0, 0, 0, time.UTC) // Monday
	// 3 weeks with Monday always skipped, other days sometimes completed.

	history := []SessionOutcome{
		{ScheduledDate: base, Skipped: true, Weekday: time.Monday},
		{ScheduledDate: base.AddDate(0, 0, 2), WasCompleted: true, Weekday: time.Wednesday},
		{ScheduledDate: base.AddDate(0, 0, 7), Skipped: true, Weekday: time.Monday},
		{ScheduledDate: base.AddDate(0, 0, 9), WasCompleted: true, Weekday: time.Wednesday},
		{ScheduledDate: base.AddDate(0, 0, 14), Skipped: true, Weekday: time.Monday},
	}

	d := e.DetectSignalB2(history)

	if d.Kind != AdaptationSignalB2ScheduleMismatch {
		t.Fatalf("expected B2, got %v", d.Kind)
	}

	if d.Params.StaleWeekday != time.Monday {
		t.Errorf("expected Monday, got %v", d.Params.StaleWeekday)
	}
}

func TestDetectSignalB4_Plateau(t *testing.T) {
	e := NewAdaptiveCoachEngine()

	weeks := []PlateauInput{
		{WeekNumber: 1, SCR: 85, Best1RM: 100},
		{WeekNumber: 2, SCR: 90, Best1RM: 100.2}, // stagnant (<1%)
	}

	d := e.DetectSignalB4(weeks)

	if d.Kind != AdaptationSignalB4Plateau {
		t.Errorf("expected B4 plateau, got %v", d.Kind)
	}
}

func TestDetectSignalB4_IgnoresLowSCRWeeks(t *testing.T) {
	e := NewAdaptiveCoachEngine()

	weeks := []PlateauInput{
		{WeekNumber: 1, SCR: 50, Best1RM: 100},
		{WeekNumber: 2, SCR: 60, Best1RM: 100},
	}

	d := e.DetectSignalB4(weeks)

	if d.Kind != AdaptationNoOp {
		t.Errorf("expected NoOp (low SCR), got %v", d.Kind)
	}
}

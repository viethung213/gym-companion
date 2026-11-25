package service

import "testing"

// Without reported effort the ΔRPE rules stay silent; completion alone still
// triggers the shorter-sessions rule.
func TestEvaluateTriggerA_UnreportedEffort(t *testing.T) {
	e := NewAdaptiveCoachEngine()

	full := WeeklyMetrics{WeekNumber: 1, SCR: 100, TotalSessions: 4, EffortUnreported: true}
	if got := e.EvaluateTriggerA([]WeeklyMetrics{full}); got.Kind != AdaptationNoOp {
		t.Errorf("full week, no RPE: %v, want NoOp", got.Kind)
	}

	low := WeeklyMetrics{SCR: 30, TotalSessions: 4, EffortUnreported: true}
	if got := e.EvaluateTriggerA([]WeeklyMetrics{low, low}); got.Kind != AdaptationProposeExpressFormat {
		t.Errorf("two low-completion weeks, no RPE: %v, want ProposeExpressFormat", got.Kind)
	}
}

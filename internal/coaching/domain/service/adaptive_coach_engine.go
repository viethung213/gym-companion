package service

import "time"

// AdaptationKind classifies a decision emitted by AdaptiveCoachEngine.
type AdaptationKind int

const (
	// AdaptationNoOp indicates no action is needed.
	AdaptationNoOp AdaptationKind = iota
	// AdaptationIncreaseLoad progresses the remaining sessions (BR-AC-04 case
	// 1). The backend sets the numbers by double progression
	// (roadmap.ProgressUpcoming), so the decision carries no percentage.
	AdaptationIncreaseLoad
	// AdaptationTriggerDeload converts next week to Deload phase (BR-AC-04 case 2).
	AdaptationTriggerDeload
	// AdaptationProposeExpressFormat suggests 30-min sessions or fewer per week (BR-AC-04 case 3).
	AdaptationProposeExpressFormat
	// AdaptationSignalB1AcuteDropout indicates 3 consecutive skips or 7d inactivity.
	AdaptationSignalB1AcuteDropout
	// AdaptationSignalB2ScheduleMismatch indicates same-weekday skip 3+ weeks.
	AdaptationSignalB2ScheduleMismatch
	// AdaptationSignalB4Plateau indicates 1RM stagnation ≥ 2 weeks (SCR ≥ 80%).
	AdaptationSignalB4Plateau
	// AdaptationPostInjuryProtect indicates 3-session protective window (BR-AC-09).
	AdaptationPostInjuryProtect
)

// AdaptationDecision is a value object describing the recommended action.
// Callers translate this into concrete SessionPlan mutations.
type AdaptationDecision struct {
	Kind   AdaptationKind
	Reason string
	Params AdaptationParams
}

// AdaptationParams carries per-kind numeric knobs. Only the fields relevant to
// Kind are meaningful; the rest are zero.
type AdaptationParams struct {
	WeightPctChange float64      // e.g. -0.10 for Deload
	VolumePctChange float64      // e.g. -0.30 for Deload
	SessionsToDrop  int          // for Express/reduce recommendation
	StaleWeekday    time.Weekday // for B2 mismatch
	SessionIDs      []string     // affected session IDs
	MuscleGroup     string       // for post-injury
}

// WeeklyMetrics is the per-week aggregate used by Trigger A (BR-AC-04).
type WeeklyMetrics struct {
	WeekNumber      int32
	SCR             float64 // Session Completion Rate (0..100)
	AvgDeltaRPE     float64 // ΔRPE averaged across sessions
	MaxActualRPE    float64 // Highest RPE observed
	HighRPESessions int     // sessions felt ≥ 1 RPE above their highest target (or at 10)
	TotalSessions   int
	// EffortUnreported is set when no completed session reported an RPE: the
	// ΔRPE figures are then empty, not "on target", and the rules that read
	// them stay silent rather than act on a missing signal.
	EffortUnreported bool
}

// AdaptiveCoachEngine is a stateless domain service. All inputs are passed
// explicitly; no repository lookup happens here.
type AdaptiveCoachEngine struct{}

// NewAdaptiveCoachEngine constructs the domain service.
func NewAdaptiveCoachEngine() *AdaptiveCoachEngine { return &AdaptiveCoachEngine{} }

// EvaluateTriggerA applies BR-AC-04 rules against the recent weekly metrics.
// weeks: chronological order, weeks[len-1] is the most recent completed week.
func (e *AdaptiveCoachEngine) EvaluateTriggerA(weeks []WeeklyMetrics) AdaptationDecision {
	if len(weeks) == 0 {
		return AdaptationDecision{Kind: AdaptationNoOp, Reason: "no data"}
	}
	last := weeks[len(weeks)-1]
	// Case 3: SCR < 50% for 2 consecutive weeks → propose reduce/Express.
	if len(weeks) >= 2 {
		prev := weeks[len(weeks)-2]
		if last.SCR < 50.0 && prev.SCR < 50.0 {
			return AdaptationDecision{
				Kind:   AdaptationProposeExpressFormat,
				Reason: "SCR < 50% in 2 consecutive weeks",
				Params: AdaptationParams{SessionsToDrop: 1},
			}
		}
	}
	// Cases 1 and 2 read the athlete's reported effort; without it there is
	// nothing to judge fatigue or progress by, so the plan is held.
	if last.EffortUnreported {
		return AdaptationDecision{Kind: AdaptationNoOp, Reason: "no effort (RPE) reported"}
	}
	// Case 2: Excessive fatigue → Deload.
	// Trigger if 3+ sessions felt ≥ 1 RPE above plan OR avgΔRPE ≥ +2.0.
	if last.HighRPESessions >= 3 || last.AvgDeltaRPE >= 2.0 {
		return AdaptationDecision{
			Kind:   AdaptationTriggerDeload,
			Reason: "excessive fatigue (3 sessions ≥1 RPE above plan or ΔRPE≥+2)",
			Params: AdaptationParams{
				VolumePctChange: -0.30,
				WeightPctChange: -0.10,
			},
		}
	}
	// Case 1: Progressive overload.
	// SCR ≥ 80% AND -1 ≤ ΔRPE ≤ +1.
	if last.SCR >= 80.0 && last.AvgDeltaRPE >= -1.0 && last.AvgDeltaRPE <= 1.0 {
		return AdaptationDecision{
			Kind:   AdaptationIncreaseLoad,
			Reason: "SCR ≥ 80% and ΔRPE in [-1, +1]",
		}
	}

	return AdaptationDecision{Kind: AdaptationNoOp, Reason: "no rule matched"}
}

// SessionOutcome is a compact history entry used by signal detectors.
type SessionOutcome struct {
	SessionPlanID string
	ScheduledDate time.Time
	Skipped       bool
	Aborted       bool
	WasCompleted  bool
	Weekday       time.Weekday
}

// DetectSignalB1 checks for 3 consecutive skipped/aborted sessions or 7 days without
// completed sessions relative to now. Returns AdaptationSignalB1AcuteDropout
// or AdaptationNoOp.
func (e *AdaptiveCoachEngine) DetectSignalB1(history []SessionOutcome, now time.Time) AdaptationDecision {
	// 3 consecutive skips/aborts (walk history newest→oldest).
	consecutive := 0
	for i := len(history) - 1; i >= 0; i-- {
		if history[i].Skipped || history[i].Aborted {
			consecutive++
			if consecutive >= 3 {
				return AdaptationDecision{
					Kind:   AdaptationSignalB1AcuteDropout,
					Reason: "3 consecutive skipped/aborted sessions",
				}
			}
		} else if history[i].WasCompleted {
			break
		}
	}
	// 7 days without a completed session.
	sevenDaysAgo := now.AddDate(0, 0, -7)
	lastCompletedTooOld := true
	for i := len(history) - 1; i >= 0; i-- {
		if history[i].WasCompleted && history[i].ScheduledDate.After(sevenDaysAgo) {
			lastCompletedTooOld = false
			break
		}
	}
	if lastCompletedTooOld && len(history) > 0 {
		return AdaptationDecision{
			Kind:   AdaptationSignalB1AcuteDropout,
			Reason: "no completed session in 7 days",
		}
	}
	return AdaptationDecision{Kind: AdaptationNoOp}
}

// DetectSignalB2 checks for the same weekday skipped/aborted in 3+ consecutive weeks.
func (e *AdaptiveCoachEngine) DetectSignalB2(history []SessionOutcome) AdaptationDecision {
	// Group by weekday: for each weekday, count consecutive weeks where it was skipped/aborted.
	// Assume history is chronological.
	skippedByWeek := map[time.Weekday]int{}
	completedByWeek := map[time.Weekday]int{}
	for _, s := range history {
		if s.Skipped || s.Aborted {
			skippedByWeek[s.Weekday]++
		} else if s.WasCompleted {
			completedByWeek[s.Weekday]++
		}
	}
	for wd, skips := range skippedByWeek {
		if skips >= 3 && completedByWeek[wd] == 0 {
			return AdaptationDecision{
				Kind:   AdaptationSignalB2ScheduleMismatch,
				Reason: "same weekday skipped 3+ times",
				Params: AdaptationParams{StaleWeekday: wd},
			}
		}
	}
	return AdaptationDecision{Kind: AdaptationNoOp}
}

// PlateauInput carries per-week 1RM data for one exercise.
type PlateauInput struct {
	WeekNumber int32
	SCR        float64
	Best1RM    float64
}

// DetectSignalB4 checks whether 1RM has been stagnant for 2+ consecutive
// weeks (only counting weeks with SCR ≥ 80%).
func (e *AdaptiveCoachEngine) DetectSignalB4(weeks []PlateauInput) AdaptationDecision {
	// Filter to eligible weeks.
	elig := make([]PlateauInput, 0, len(weeks))
	for _, w := range weeks {
		if w.SCR >= 80.0 {
			elig = append(elig, w)
		}
	}
	if len(elig) < 2 {
		return AdaptationDecision{Kind: AdaptationNoOp}
	}
	// Check the last two eligible weeks for stagnation (delta ~0).
	a := elig[len(elig)-2]
	b := elig[len(elig)-1]
	if b.Best1RM > 0 && (b.Best1RM-a.Best1RM)/b.Best1RM < 0.01 {
		return AdaptationDecision{
			Kind:   AdaptationSignalB4Plateau,
			Reason: "1RM plateau ≥ 2 eligible weeks",
		}
	}
	return AdaptationDecision{Kind: AdaptationNoOp}
}

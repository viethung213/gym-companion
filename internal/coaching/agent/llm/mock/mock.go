// Package mock is the deterministic rules-based CoachAgent used in phase-1
// (no LLM). It also serves as the fallback when the real LLM adapter exceeds
// its cost/time budget (D5).
package mock

import (
	"context"
	"fmt"
	"hash/fnv"
	"time"

	"github.com/viethung213/gym-companion/internal/coaching/agent"
	"github.com/viethung213/gym-companion/internal/coaching/application/port"
	"github.com/viethung213/gym-companion/internal/coaching/domain/roadmap"
)

// MockCoachAgent implements agent.CoachAgent using deterministic templates.
type MockCoachAgent struct {
	ids   port.IDGenerator
	clock port.Clock
}

// NewMockCoachAgent constructs the deterministic generator.
func NewMockCoachAgent(ids port.IDGenerator, clock port.Clock) *MockCoachAgent {
	return &MockCoachAgent{ids: ids, clock: clock}
}

// phaseSpec defines RPE + relative volume/intensity per phase.
type phaseSpec struct {
	phase     roadmap.Phase
	targetRPE float32
	volume    float64 // multiplier on baseline sets/reps (1.0 = 100%)
	intensity float64 // multiplier on baseline weight (1.0 = 100%)
}

var defaultPhaseSpecs = []phaseSpec{
	{roadmap.PhaseAccumulation, 6.5, 1.0, 0.85},
	{roadmap.PhaseOverload, 7.5, 1.05, 0.92},
	{roadmap.PhasePeak, 8.5, 0.95, 1.0},
	{roadmap.PhaseDeload, 5.5, 0.70, 0.90},
}

// GenerateRoadmap creates a full 4-week roadmap. It is deterministic given
// (userID, startDate, profile hash).
func (m *MockCoachAgent) GenerateRoadmap(ctx context.Context, cc *agent.CoachContext, _ *agent.Feedback) (*roadmap.Roadmap, error) {
	if cc == nil {
		return nil, fmt.Errorf("nil CoachContext")
	}
	now := m.clock.Now()
	roadmapID := m.ids.NewID()
	startDate := nextMonday(now)
	endDate := startDate.AddDate(0, 0, roadmap.WeeksPerRoadmap*roadmap.DaysPerWeek)

	weeks := make([]*roadmap.WeekPlan, 0, roadmap.WeeksPerRoadmap)
	seed := hashSeed(cc.UserID)
	for wi, spec := range defaultPhaseSpecs {
		weekStart := startDate.AddDate(0, 0, wi*roadmap.DaysPerWeek)
		weekPlanID := m.ids.NewID()
		wp, err := roadmap.NewWeekPlan(&roadmap.WeekPlanInfo{
			WeekPlanID:      weekPlanID,
			RoadmapID:       roadmapID,
			UserID:          cc.UserID,
			WeekNumber:      int32(wi + 1),
			Phase:           spec.phase,
			TargetRPE:       spec.targetRPE,
			StartDate:       weekStart,
			EndDate:         weekStart.AddDate(0, 0, 6),
			MuscleSplitType: chooseSplit(cc.Profile.PreferredMuscleGroups),
		})
		if err != nil {
			return nil, fmt.Errorf("new week plan: %w", err)
		}

		trainingDays := selectTrainingDays(cc.Profile.AvailableSlots, seed)
		for di := 0; di < roadmap.DaysPerWeek; di++ {
			if !trainingDays[di] {
				continue
			}
			date := weekStart.AddDate(0, 0, di)
			dayID := m.ids.NewID()
			dp, err := roadmap.NewDayPlan(&roadmap.DayPlanInfo{
				DayPlanID:     dayID,
				WeekPlanID:    weekPlanID,
				RoadmapID:     roadmapID,
				UserID:        cc.UserID,
				ScheduledDate: date,
			})
			if err != nil {
				return nil, err
			}
			sp, err := m.buildSessionPlan(
				roadmapID, weekPlanID, dayID, cc.UserID,
				date, spec, cc.InjuryStatus, now,
			)
			if err != nil {
				return nil, err
			}
			if err := dp.AddSession(sp); err != nil {
				return nil, err
			}
			if err := wp.AddDay(dp); err != nil {
				return nil, err
			}
		}
		weeks = append(weeks, wp)
	}

	r, err := roadmap.NewRoadmap(&roadmap.RoadmapInfo{
		RoadmapID: roadmapID,
		UserID:    cc.UserID,
		StartDate: startDate,
		EndDate:   endDate,
	}, weeks, now)
	if err != nil {
		return nil, err
	}
	return r, nil
}

// RegeneratePending returns updated prescriptions for the given session IDs.
// It uses the current roadmap snapshot to know phase & muscle groups.
func (m *MockCoachAgent) RegeneratePending(ctx context.Context, cc *agent.CoachContext, sessionIDs []string, _ *agent.Feedback) ([]*roadmap.SessionPlanInfo, error) {
	out := make([]*roadmap.SessionPlanInfo, 0, len(sessionIDs))
	if cc == nil || cc.CurrentRoadmap == nil {
		return out, nil
	}
	spec := findPhaseSpec(cc.CurrentRoadmap.Phase)
	now := m.clock.Now()
	for _, id := range sessionIDs {
		out = append(out, &roadmap.SessionPlanInfo{
			SessionPlanID:      id,
			UserID:             cc.UserID,
			RoadmapID:          cc.CurrentRoadmap.RoadmapID,
			Status:             roadmap.SessionPlanStatusPending,
			TargetMuscleGroups: cc.Profile.PreferredMuscleGroups,
			Prescription:       buildPrescription(spec, cc.InjuryStatus),
			Reasoning:          "regenerated (mock) after context change",
			GeneratedAt:        now,
		})
	}
	return out, nil
}

// Adapt is used by Trigger A / signal handlers. Phase-1 mock returns the same
// mapping as RegeneratePending but tags reasoning with the decisionReason.
func (m *MockCoachAgent) Adapt(ctx context.Context, cc *agent.CoachContext, decisionReason string, fb *Feedback) ([]*roadmap.SessionPlanInfo, error) {
	if cc == nil || cc.CurrentRoadmap == nil {
		return nil, nil
	}
	ids := make([]string, 0, len(cc.CurrentRoadmap.PendingSessions))
	for _, s := range cc.CurrentRoadmap.PendingSessions {
		ids = append(ids, s.SessionPlanID)
	}
	out, err := m.RegeneratePending(ctx, cc, ids, fb)
	if err != nil {
		return nil, err
	}
	for i := range out {
		out[i].Reasoning = "adapt (mock): " + decisionReason
	}
	return out, nil
}

// Feedback is a type alias to shorten the signature above (avoid re-import).
type Feedback = agent.Feedback

// SuggestAdHocSession returns a single-session suggestion. Read-only: no
// persistence, no side effects. Deterministic given the hint + profile.
func (m *MockCoachAgent) SuggestAdHocSession(ctx context.Context, cc *agent.CoachContext, hint agent.AdHocHint) (agent.SuggestedSession, error) {
	if cc == nil {
		return agent.SuggestedSession{}, fmt.Errorf("nil CoachContext")
	}
	// Pick the phase to size volume/intensity from — prefer current roadmap phase,
	// otherwise default to Accumulation (moderate).
	phase := roadmap.PhaseAccumulation
	if cc.CurrentRoadmap != nil && cc.CurrentRoadmap.Phase.Valid() {
		phase = cc.CurrentRoadmap.Phase
	}
	spec := findPhaseSpec(phase)

	// If user indicated intensity, override phase spec accordingly.
	switch hint.IntensityHint {
	case "light":
		spec.intensity *= 0.85
		spec.targetRPE = 5.5
	case "hard":
		spec.intensity *= 1.10
		spec.targetRPE = 8.0
	}

	// Merge muscle groups: hint overrides profile preferences.
	muscles := hint.MuscleGroups
	if len(muscles) == 0 {
		muscles = cc.Profile.PreferredMuscleGroups
	}
	if len(muscles) == 0 {
		muscles = []string{"chest", "back"} // safe default
	}

	presc := buildPrescription(spec, cc.InjuryStatus)

	// If a duration cap is set, drop extra sets from the last main exercise until
	// the estimated total time fits. Rough heuristic: 3 min per set + 5 min warmup
	// + 5 min cooldown.
	if hint.DurationMinutes > 0 {
		presc = shrinkToDuration(presc, hint.DurationMinutes)
	}

	return agent.SuggestedSession{
		MuscleGroups: muscles,
		Prescription: presc,
		Reasoning:    "ad-hoc suggestion (mock): phase=" + string(spec.phase) + " intensity=" + hint.IntensityHint,
		EstimatedRPE: spec.targetRPE,
	}, nil
}

// shrinkToDuration reduces set counts on main exercises to fit budgetMinutes.
func shrinkToDuration(p roadmap.WorkoutPrescription, budgetMinutes int) roadmap.WorkoutPrescription {
	// Assume warmup 5' + cooldown 5' = 10' fixed overhead.
	remaining := budgetMinutes - 10
	if remaining <= 0 {
		p.MainExercises = nil
		return p
	}
	// Rough 3 minutes per set (including rest).
	setsBudget := remaining / 3
	if setsBudget <= 0 {
		p.MainExercises = nil
		return p
	}
	out := make([]roadmap.PrescribedExercise, 0, len(p.MainExercises))
	used := 0
	for _, ex := range p.MainExercises {
		if used >= setsBudget {
			break
		}
		if used+int(ex.TargetSets) > setsBudget {
			ex.TargetSets = int32(setsBudget - used)
			if ex.TargetSets <= 0 {
				break
			}
		}
		used += int(ex.TargetSets)
		out = append(out, ex)
	}
	p.MainExercises = out
	return p
}

func (m *MockCoachAgent) buildSessionPlan(
	roadmapID, weekPlanID, dayID, userID string,
	date time.Time,
	spec phaseSpec,
	injuries []port.InjuryStatus,
	now time.Time,
) (*roadmap.SessionPlan, error) {
	muscles := []string{"chest", "back"}
	presc := buildPrescription(spec, injuries)
	sp, err := roadmap.NewSessionPlan(&roadmap.SessionPlanInfo{
		SessionPlanID:      m.ids.NewID(),
		DayPlanID:          dayID,
		WeekPlanID:         weekPlanID,
		RoadmapID:          roadmapID,
		UserID:             userID,
		ScheduledDate:      date,
		SlotTime:           "06:00-07:00",
		TargetMuscleGroups: muscles,
		Prescription:       presc,
		Reasoning:          fmt.Sprintf("phase=%s targetRPE=%.1f", spec.phase, spec.targetRPE),
	}, now)
	if err != nil {
		return nil, err
	}
	return sp, nil
}

func buildPrescription(spec phaseSpec, injuries []port.InjuryStatus) roadmap.WorkoutPrescription {
	baseWeight := 40.0 * spec.intensity
	baseSets := int32(4.0 * spec.volume)
	if baseSets < 2 {
		baseSets = 2
	}
	main := []roadmap.PrescribedExercise{
		{
			ExerciseID:   "ex-bench-press",
			ExerciseName: "Bench Press",
			TargetSets:   baseSets,
			TargetReps:   int32(10.0 * spec.volume),
			TargetWeight: float32(baseWeight),
			TargetRPE:    spec.targetRPE,
			RestSetSec:   90,
		},
		{
			ExerciseID:   "ex-row",
			ExerciseName: "Barbell Row",
			TargetSets:   baseSets,
			TargetReps:   int32(10.0 * spec.volume),
			TargetWeight: float32(baseWeight * 0.9),
			TargetRPE:    spec.targetRPE,
			RestSetSec:   90,
		},
	}
	// BR-AC-09 stub: drop exercises hitting injured muscle group.
	filtered := make([]roadmap.PrescribedExercise, 0, len(main))
	for _, ex := range main {
		if !intersects(injuries, ex.ExerciseName) {
			filtered = append(filtered, ex)
		}
	}
	return roadmap.WorkoutPrescription{
		WarmUps: []roadmap.PrescribedExercise{
			{ExerciseID: "wu-general", ExerciseName: "General warmup", DurationSeconds: 300, TargetRPE: 4},
		},
		MainExercises: filtered,
		CoolDowns: []roadmap.PrescribedExercise{
			{ExerciseID: "cd-stretch", ExerciseName: "Static stretch", DurationSeconds: 300, TargetRPE: 3},
		},
	}
}

func intersects(injuries []port.InjuryStatus, name string) bool {
	// Simplified for phase-1: no fuzzy matching.
	_ = injuries
	_ = name
	return false
}

func chooseSplit(prefs []string) string {
	if len(prefs) == 0 {
		return "push_pull_legs"
	}
	return "custom"
}

func selectTrainingDays(slots []port.Slot, seed uint32) [7]bool {
	// If user provided slots, mark those days true (up to cap 6).
	var days [7]bool
	if len(slots) > 0 {
		count := 0
		for _, s := range slots {
			if count >= roadmap.MaxSessionsPerWeek {
				break
			}
			days[s.DayOfWeek] = true
			count++
		}
		return days
	}
	// Otherwise pick 4 days deterministically from seed.
	idx := int(seed % 7)
	count := 0
	for count < 4 {
		if !days[idx] {
			days[idx] = true
			count++
		}
		idx = (idx + 2) % 7
	}
	return days
}

func findPhaseSpec(p roadmap.Phase) phaseSpec {
	for _, s := range defaultPhaseSpecs {
		if s.phase == p {
			return s
		}
	}
	return defaultPhaseSpecs[0]
}

func nextMonday(now time.Time) time.Time {
	// Truncate to date at UTC.
	d := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	// Weekday: Sunday=0, Monday=1, ..., Saturday=6.
	offset := (int(time.Monday) - int(d.Weekday()) + 7) % 7
	if offset == 0 {
		offset = 7
	}
	return d.AddDate(0, 0, offset)
}

func hashSeed(s string) uint32 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(s))
	return h.Sum32()
}

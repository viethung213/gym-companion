// Package contextbuilder assembles the CoachContext payload that CoachAgent
// consumes. It gathers snapshots from UserProfileReader + WorkoutSessionReader
// + the current Roadmap and applies the flow-specific instructions template.
package contextbuilder

import (
	"context"
	"time"

	"github.com/viethung213/gym-companion/internal/coaching/application/port"
	"github.com/viethung213/gym-companion/internal/coaching/domain/roadmap"
)

// Builder is a plain struct — inject its dependencies via NewBuilder.
type Builder struct {
	profile   port.UserProfileReader
	workouts  port.WorkoutSessionReader
	lookback  time.Duration
	prompts   PromptRegistry
}

// NewBuilder wires a ContextBuilder.
func NewBuilder(profile port.UserProfileReader, workouts port.WorkoutSessionReader, prompts PromptRegistry) *Builder {
	return &Builder{
		profile:  profile,
		workouts: workouts,
		lookback: 28 * 24 * time.Hour, // 4 weeks of history is enough for BR-AC-04..08
		prompts:  prompts,
	}
}

// Build produces a CoachContext for the given flow. currentRoadmap may be nil
// (e.g. UC-02.1 InitiateRoadmap where no prior roadmap exists).
func (b *Builder) Build(ctx context.Context, flow port.FlowType, userID string, currentRoadmap *roadmap.Roadmap, now time.Time) (port.CoachContext, error) {
	profile, err := b.profile.GetProfile(ctx, userID)
	if err != nil {
		return port.CoachContext{}, err
	}

	sessions, err := b.workouts.GetRecentSessions(ctx, userID, now.Add(-b.lookback))
	if err != nil {
		return port.CoachContext{}, err
	}

	cc := port.CoachContext{
		Flow:           flow,
		UserID:         userID,
		Profile:        profile,
		RecentSessions: sessions,
		InjuryStatus:   profile.ActiveInjuries,
	}

	if currentRoadmap != nil {
		cc.CurrentRoadmap = buildSnapshot(currentRoadmap, now)
	}

	instr, schema, err := b.prompts.Render(flow, cc)
	if err != nil {
		return port.CoachContext{}, err
	}
	cc.Instructions = instr
	cc.OutputSchemaHint = schema
	return cc, nil
}

func buildSnapshot(r *roadmap.Roadmap, now time.Time) *port.RoadmapSnapshot {
	snap := &port.RoadmapSnapshot{
		RoadmapID:      r.ID(),
		Phase:          currentPhase(r, now),
		CurrentWeekNum: currentWeekNum(r, now),
	}
	for _, s := range r.PendingSessionsFrom(now) {
		info := s.Info()
		snap.PendingSessions = append(snap.PendingSessions, port.SessionSnapshot{
			SessionPlanID: info.SessionPlanID,
			ScheduledDate: info.ScheduledDate.Format("2006-01-02"),
			MuscleGroups:  info.TargetMuscleGroups,
		})
	}
	return snap
}

func currentPhase(r *roadmap.Roadmap, now time.Time) roadmap.Phase {
	for _, w := range r.Weeks() {
		info := w.Info()
		if !now.Before(info.StartDate) && !now.After(info.EndDate) {
			return info.Phase
		}
	}
	return ""
}

func currentWeekNum(r *roadmap.Roadmap, now time.Time) int32 {
	for _, w := range r.Weeks() {
		info := w.Info()
		if !now.Before(info.StartDate) && !now.After(info.EndDate) {
			return info.WeekNumber
		}
	}
	return 0
}

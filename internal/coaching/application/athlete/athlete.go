// Package athlete holds what coaching assumes and derives about an athlete from
// the profile and workout contexts: defaults for fields a profile leaves
// empty, and the load baselines taken from personal records.
package athlete

import (
	"context"
	"log/slog"
	"time"

	"github.com/viethung213/gym-companion/internal/coaching/application/port"
	"github.com/viethung213/gym-companion/internal/coaching/domain/guardrail"
	"github.com/viethung213/gym-companion/internal/coaching/domain/service"
)

// Coaching plans for every athlete, including one whose profile leaves a field
// empty. These are the assumptions it makes then, in one place: the profile
// adapter reports only what the profile holds.
const (
	// DefaultBodyWeightKg stands in for an unknown bodyweight. It bounds the
	// load of an exercise the athlete has no record of (1.5x bodyweight).
	DefaultBodyWeightKg = 75.0

	// DefaultPrimaryGoal is the goal planned for when none is set.
	DefaultPrimaryGoal = "hypertrophy"
)

// DefaultTrainingSlots are the weekly slots planned for when the athlete
// selected none: three mornings with a rest day between.
func DefaultTrainingSlots() []port.Slot {
	return []port.Slot{
		{DayOfWeek: time.Monday, StartTime: "06:00", EndTime: "07:30"},
		{DayOfWeek: time.Wednesday, StartTime: "06:00", EndTime: "07:30"},
		{DayOfWeek: time.Friday, StartTime: "06:00", EndTime: "07:30"},
	}
}

// WithProfileDefaults wraps a profile reader so every coaching flow sees the
// same assumptions for empty fields (see the constants above). An empty
// equipment list means a regular gym (docs/05 §8.3).
func WithProfileDefaults(next port.UserProfileReader) port.UserProfileReader {
	if next == nil {
		return nil
	}
	return profileDefaults{next: next}
}

type profileDefaults struct{ next port.UserProfileReader }

func (r profileDefaults) GetProfile(ctx context.Context, userID string) (port.Profile, error) {
	p, err := r.next.GetProfile(ctx, userID)
	if err != nil {
		return p, err
	}
	applyDefaults(&p)
	return p, nil
}

func applyDefaults(p *port.Profile) {
	if p.WeightKg <= 0 {
		p.WeightKg = DefaultBodyWeightKg
	}
	if p.PrimaryGoal == "" {
		p.PrimaryGoal = DefaultPrimaryGoal
	}
	if len(p.AvailableEquipment) == 0 {
		p.AvailableEquipment = service.DefaultGymEquipment()
	}
	if len(p.AvailableSlots) == 0 {
		p.AvailableSlots = DefaultTrainingSlots()
	}
}

// WeightBaselines maps each exercise to the heaviest weight of its personal
// records: the baseline BR-AC-02 caps a prescribed load against (+30%).
// Records without a load (bodyweight, timed) are left out.
func WeightBaselines(prs []port.PersonalRecordSummary) map[string]float64 {
	out := make(map[string]float64, len(prs))
	for _, pr := range prs {
		if pr.Weight > out[pr.ExerciseID] {
			out[pr.ExerciseID] = pr.Weight
		}
	}
	return out
}

// PRBaselines feeds the guardrail's BR-AC-02 check with the athlete's personal
// records. A failed read skips that ceiling rather than blocking the plan: the
// hard load limits (BR-AC-13) and the generation-time bodyweight cap still
// apply without it.
func PRBaselines(reader port.WorkoutSessionReader) guardrail.PRLookup {
	if reader == nil {
		return nil
	}
	return func(ctx context.Context, userID string) map[string]float64 {
		prs, err := reader.GetPersonalRecords(ctx, userID)
		if err != nil {
			slog.WarnContext(ctx, "Guardrail weight band skipped: personal records unavailable",
				slog.String("user_id", userID), slog.Any("error", err))
			return nil
		}
		return WeightBaselines(prs)
	}
}

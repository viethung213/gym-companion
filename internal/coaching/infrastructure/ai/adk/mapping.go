package adk

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/viethung213/gym-companion/internal/coaching/domain/roadmap"
)

// mapToDomainRoadmap builds the Roadmap aggregate, minting every identifier.
// One DayPlan per date; populate it fully before AddDay checks the weekly cap.
func (c *CoachingContextAgent) mapToDomainRoadmap(
	ctx context.Context,
	plan *GeneratedPlan,
	names map[string]string,
	userID string,
	now time.Time,
) (*roadmap.Roadmap, error) {
	if plan == nil || len(plan.Weeks) == 0 {
		return nil, fmt.Errorf("%w: plan has no weeks", ErrPlanGenerationFailed)
	}

	// Anchor on the plan's earliest session, not now, or sessions fall outside
	// their week and every date-based query downstream breaks.
	startDate, err := earliestScheduledDate(plan)
	if err != nil {
		return nil, err
	}

	roadmapID := c.idgen.NewID()
	roadmapInfo := &roadmap.Info{
		RoadmapID: roadmapID,
		UserID:    userID,
		Status:    roadmap.StatusActive,
		StartDate: startDate,
		EndDate:   startDate.AddDate(0, 0, roadmap.WeeksPerRoadmap*7),
		CreatedAt: now,
		UpdatedAt: now,
	}

	weeks := make([]*roadmap.WeekPlan, 0, len(plan.Weeks))
	for _, wp := range plan.Weeks {
		weekPlanID := c.idgen.NewID()
		weekStart := startDate.AddDate(0, 0, (wp.WeekNumber-1)*7)

		w, err := roadmap.NewWeekPlan(&roadmap.WeekPlanInfo{
			WeekPlanID:      weekPlanID,
			RoadmapID:       roadmapID,
			UserID:          userID,
			WeekNumber:      int32(wp.WeekNumber),
			Phase:           roadmap.Phase(wp.Phase),
			TargetRPE:       float32((wp.TargetRPEMin + wp.TargetRPEMax) / 2.0),
			StartDate:       weekStart,
			EndDate:         weekStart.AddDate(0, 0, 7),
			MuscleSplitType: defaultMuscleSplitType,
		})
		if err != nil {
			return nil, fmt.Errorf("new week plan %d: %w", wp.WeekNumber, err)
		}

		buckets, order, err := groupSessionsByDate(wp.Sessions)
		if err != nil {
			return nil, fmt.Errorf("week %d: %w", wp.WeekNumber, err)
		}

		for _, key := range order {
			bucket := buckets[key]
			dayPlanID := c.idgen.NewID()

			d, err := roadmap.NewDayPlan(&roadmap.DayPlanInfo{
				DayPlanID:     dayPlanID,
				WeekPlanID:    weekPlanID,
				RoadmapID:     roadmapID,
				UserID:        userID,
				ScheduledDate: bucket.date,
			})
			if err != nil {
				return nil, fmt.Errorf("new day plan %s: %w", key, err)
			}

			for i := range bucket.sessions {
				sp := &bucket.sessions[i]
				dur := sp.EstimatedDurationMinutes
				if dur <= 0 {
					dur = 45
				}
				slotTime := sp.SlotTime
				if slotTime == "" {
					slotTime = "07:00"
				}

				s, err := roadmap.NewSessionPlan(&roadmap.SessionPlanInfo{
					SessionPlanID:            c.idgen.NewID(),
					DayPlanID:                dayPlanID,
					WeekPlanID:               weekPlanID,
					RoadmapID:                roadmapID,
					UserID:                   userID,
					ScheduledDate:            bucket.date,
					SlotTime:                 slotTime,
					EstimatedDurationMinutes: dur,
					Status:                   roadmap.SessionPlanStatusPending,
					Source:                   roadmap.SessionPlanSourceScheduled,
					TargetMuscleGroups:       sp.TargetMuscleGroups,
					Prescription:             c.mapPrescriptionToDomain(ctx, sp.Prescription, names),
					Reasoning:                sp.Reasoning,
					GeneratedAt:              now,
				}, now)
				if err != nil {
					return nil, fmt.Errorf("new session plan on %s: %w", key, err)
				}
				if err := d.AddSession(s); err != nil {
					return nil, fmt.Errorf("add session on %s: %w", key, err)
				}
			}

			// Only after the day is fully populated — see the doc comment.
			if err := w.AddDay(d); err != nil {
				return nil, fmt.Errorf("add day %s to week %d: %w", key, wp.WeekNumber, err)
			}
		}

		weeks = append(weeks, w)
	}

	return roadmap.NewRoadmap(roadmapInfo, weeks, now)
}

// mapToRegeneratedSessions yields prescriptions for sessions that already exist.
// Identity stays empty: the caller owns them; a minted ID would orphan the row.
func (c *CoachingContextAgent) mapToRegeneratedSessions(
	ctx context.Context,
	plan *GeneratedPlan,
	names map[string]string,
	userID string,
	now time.Time,
) []*roadmap.SessionPlanInfo {
	if plan == nil {
		return nil
	}

	var infos []*roadmap.SessionPlanInfo
	for _, wp := range plan.Weeks {
		for i := range wp.Sessions {
			sp := &wp.Sessions[i]
			scheduledTime, err := time.Parse(scheduledDateISO, sp.ScheduledDate)
			if err != nil {
				// Validation already reported this; skip rather than emit a
				// session the domain would reject for a zero date.
				log.Printf("coaching: skipping regenerated session with unparseable date %q for user %s",
					sp.ScheduledDate, userID)
				continue
			}

			dur := sp.EstimatedDurationMinutes
			if dur <= 0 {
				dur = 45
			}
			slotTime := sp.SlotTime
			if slotTime == "" {
				slotTime = "07:00"
			}

			infos = append(infos, &roadmap.SessionPlanInfo{
				UserID:                   userID,
				ScheduledDate:            scheduledTime.UTC(),
				SlotTime:                 slotTime,
				EstimatedDurationMinutes: dur,
				Status:                   roadmap.SessionPlanStatusPending,
				TargetMuscleGroups:       sp.TargetMuscleGroups,
				Prescription:             c.mapPrescriptionToDomain(ctx, sp.Prescription, names),
				Reasoning:                sp.Reasoning,
				GeneratedAt:              now,
			})
		}
	}
	return infos
}

func (c *CoachingContextAgent) mapPrescriptionToDomain(
	ctx context.Context, p WorkoutPrescription, names map[string]string,
) roadmap.WorkoutPrescription {
	return roadmap.WorkoutPrescription{
		WarmUps:       c.mapExercisesToDomain(ctx, p.WarmUps, names),
		MainExercises: c.mapExercisesToDomain(ctx, p.MainExercises, names),
		CoolDowns:     c.mapExercisesToDomain(ctx, p.CoolDowns, names),
	}
}

func (c *CoachingContextAgent) mapExercisesToDomain(
	ctx context.Context, exs []PrescribedExercise, names map[string]string,
) []roadmap.PrescribedExercise {
	if len(exs) == 0 {
		return nil
	}

	out := make([]roadmap.PrescribedExercise, len(exs))
	for i, e := range exs {
		out[i] = roadmap.PrescribedExercise{
			ExerciseID:      e.ExerciseID,
			ExerciseName:    c.resolveExerciseName(ctx, e.ExerciseID, names),
			TargetSets:      int32(e.TargetSets),
			TargetReps:      int32(e.TargetReps),
			TargetWeight:    float32(e.TargetWeightKg),
			TargetRPE:       float32(e.TargetRPE),
			RestSetSec:      int32(e.RestSetSec),
			RestExerciseSec: defaultRestExerciseSec,
		}
	}
	return out
}

// resolveExerciseName prefers the validated name; a catalog miss is logged, not swallowed.
func (c *CoachingContextAgent) resolveExerciseName(
	ctx context.Context, exerciseID string, names map[string]string,
) string {
	if name, ok := names[exerciseID]; ok {
		return name
	}

	ex, err := c.catalog.GetByID(ctx, exerciseID)
	if err != nil {
		log.Printf("coaching: no catalog name for exercise %s: %v", exerciseID, err)
		return ""
	}
	return ex.Name
}

// prescriptionToDTO converts a stored prescription back into the agent's shape.
func prescriptionToDTO(p roadmap.WorkoutPrescription) WorkoutPrescription {
	return WorkoutPrescription{
		WarmUps:       exercisesToDTO(p.WarmUps),
		MainExercises: exercisesToDTO(p.MainExercises),
		CoolDowns:     exercisesToDTO(p.CoolDowns),
	}
}

func exercisesToDTO(exs []roadmap.PrescribedExercise) []PrescribedExercise {
	if len(exs) == 0 {
		return nil
	}

	out := make([]PrescribedExercise, len(exs))
	for i, e := range exs {
		out[i] = PrescribedExercise{
			ExerciseID:     e.ExerciseID,
			TargetSets:     int(e.TargetSets),
			TargetReps:     int(e.TargetReps),
			TargetWeightKg: float64(e.TargetWeight),
			TargetRPE:      float64(e.TargetRPE),
			RestSetSec:     int(e.RestSetSec),
		}
	}
	return out
}

// currentPhase reports the phase of the week holding s.
func currentPhase(rm *roadmap.Roadmap, s *roadmap.SessionPlan) string {
	want := s.Info().WeekPlanID
	for _, w := range rm.Weeks() {
		if w.ID() == want {
			return string(w.Phase())
		}
	}
	return ""
}

// dayBucket collects the sessions the model scheduled on one date.
type dayBucket struct {
	date     time.Time
	sessions []SessionPlan
}

// groupSessionsByDate buckets sessions by scheduled_date, returning the buckets
// and the dates in first-seen order. The explicit ordering matters: ranging a
// map would emit DayPlans in a different order on every run.
func groupSessionsByDate(sessions []SessionPlan) (buckets map[string]*dayBucket, order []string, err error) {
	buckets = make(map[string]*dayBucket)

	for i := range sessions {
		sp := &sessions[i]
		// Midnight UTC: ad-hoc session matching truncates to a day and assumes it.
		d, err := time.Parse(scheduledDateISO, sp.ScheduledDate)
		if err != nil {
			return nil, nil, fmt.Errorf("scheduled_date %q is not a valid date: %w", sp.ScheduledDate, err)
		}
		d = d.UTC()

		key := d.Format(scheduledDateISO)
		if buckets[key] == nil {
			buckets[key] = &dayBucket{date: d}
			order = append(order, key)
		}
		buckets[key].sessions = append(buckets[key].sessions, sessions[i])
	}

	return buckets, order, nil
}

// earliestScheduledDate returns the first date any session is scheduled on.
func earliestScheduledDate(plan *GeneratedPlan) (time.Time, error) {
	var earliest time.Time
	for _, wp := range plan.Weeks {
		for i := range wp.Sessions {
			sp := &wp.Sessions[i]
			d, err := time.Parse(scheduledDateISO, sp.ScheduledDate)
			if err != nil {
				return time.Time{}, fmt.Errorf("scheduled_date %q is not a valid date: %w",
					sp.ScheduledDate, err)
			}
			if earliest.IsZero() || d.Before(earliest) {
				earliest = d
			}
		}
	}

	if earliest.IsZero() {
		return time.Time{}, fmt.Errorf("%w: plan contains no sessions", ErrPlanGenerationFailed)
	}
	return earliest.UTC(), nil
}

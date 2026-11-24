package adk

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/viethung213/gym-companion/internal/coaching/domain/roadmap"
)

// mapToDomainRoadmap builds the Roadmap aggregate, minting every identifier.
// One DayPlan per date; populate it fully before AddDay checks the weekly cap.
func (c *CoachingContextAgent) mapToDomainRoadmap(
	ctx context.Context,
	plan *GeneratedPlan,
	names map[string]string,
	primaries map[string]string,
	userID string,
	now time.Time,
	weekStart time.Time,
) (*roadmap.Roadmap, error) {
	if plan == nil || len(plan.Weeks) == 0 {
		return nil, fmt.Errorf("%w: plan has no weeks", ErrPlanGenerationFailed)
	}

	// Week N spans [startDate + 7(N-1), +7). The slot schedule counted weeks from
	// weekStart, and the validator kept every session inside its week, so the
	// roadmap uses the same anchor; without a schedule it falls back to the
	// earliest session.
	startDate := weekStart
	if startDate.IsZero() {
		var err error
		if startDate, err = earliestScheduledDate(plan); err != nil {
			return nil, err
		}
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
				targets, err := sessionTargetMuscles(sp.Prescription, primaries)
				if err != nil {
					return nil, fmt.Errorf("session on %s: %w", key, err)
				}
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
					Kind:                     sessionKind(sp.SessionKind),
					TargetMuscleGroups:       targets,
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
	primaries map[string]string,
	userID string,
	now time.Time,
) ([]*roadmap.SessionPlanInfo, error) {
	if plan == nil {
		return nil, nil
	}

	var infos []*roadmap.SessionPlanInfo
	for _, wp := range plan.Weeks {
		for i := range wp.Sessions {
			sp := &wp.Sessions[i]
			scheduledTime, err := time.Parse(scheduledDateISO, sp.ScheduledDate)
			if err != nil {
				// Validation already reported this; skip rather than emit a
				// session the domain would reject for a zero date.
				slog.Warn("Skipping regenerated session with unparseable date",
					slog.String("date", sp.ScheduledDate),
					slog.String("user_id", userID))
				continue
			}

			targets, err := sessionTargetMuscles(sp.Prescription, primaries)
			if err != nil {
				return nil, fmt.Errorf("session on %s: %w", sp.ScheduledDate, err)
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
				Kind:                     sessionKind(sp.SessionKind),
				TargetMuscleGroups:       targets,
				Prescription:             c.mapPrescriptionToDomain(ctx, sp.Prescription, names),
				Reasoning:                sp.Reasoning,
				GeneratedAt:              now,
			})
		}
	}
	return infos, nil
}

func (c *CoachingContextAgent) mapPrescriptionToDomain(
	ctx context.Context, p WorkoutPrescription, names map[string]string,
) roadmap.WorkoutPrescription {
	return roadmap.WorkoutPrescription{
		WarmUps:       c.mapExercisesToDomain(ctx, p.WarmUps, names, true),
		MainExercises: c.mapExercisesToDomain(ctx, p.MainExercises, names, false),
		CoolDowns:     c.mapExercisesToDomain(ctx, p.CoolDowns, names, true),
	}
}

func (c *CoachingContextAgent) mapExercisesToDomain(
	ctx context.Context, exs []PrescribedExercise, names map[string]string, isWarmup ...bool,
) []roadmap.PrescribedExercise {
	if len(exs) == 0 {
		return nil
	}

	isWarmupCoolDown := false
	if len(isWarmup) > 0 {
		isWarmupCoolDown = isWarmup[0]
	}

	out := make([]roadmap.PrescribedExercise, len(exs))
	for i, e := range exs {
		name, measType := c.resolveExerciseDetails(ctx, e.ExerciseID, names)
		targetReps := int32(e.TargetReps)
		durationSec := int32(e.DurationSeconds)
		targetWeight := float32(e.TargetWeightKg)

		if measType == "DURATION" {
			// Bài tập tính theo thời gian (Cardio, Isometric, Planks...)
			if durationSec <= 0 {
				switch {
				case e.TargetReps > 0:
					// Quy đổi nếu AI lỡ điền reps (ví dụ 15 reps * 3 -> ~45s)
					scaled := int32(e.TargetReps * 3)
					if scaled < 30 {
						scaled = 30
					} else if scaled > 60 {
						scaled = 60
					}
					durationSec = scaled
				case isWarmupCoolDown:
					durationSec = 30 // Fallback mặc định cho khởi động/hạ nhiệt
				default:
					durationSec = 45 // Fallback mặc định cho bài chính
				}
			}
			targetReps = 0   // Triệt tiêu reps thừa
			targetWeight = 0 // Bodyweight cardio không mang tạ
		} else {
			// Bài tập tính theo số lần (REPS)
			if targetReps <= 0 {
				targetReps = 10 // Fallback mặc định
			}
			durationSec = 0 // Triệt tiêu duration thừa
		}

		out[i] = roadmap.PrescribedExercise{
			ExerciseID:      e.ExerciseID,
			ExerciseName:    name,
			TargetSets:      int32(e.TargetSets),
			TargetReps:      targetReps,
			TargetWeight:    targetWeight,
			DurationSeconds: durationSec,
			TargetRPE:       float32(e.TargetRPE),
			RestSetSec:      int32(e.RestSetSec),
			RestExerciseSec: defaultRestExerciseSec,
		}
	}
	return out
}

// sessionTargetMuscles derives a session's TargetMuscleGroups as the primary
// MuscleGroup codes of its main exercises (docs/05 §8.4), from the catalog data
// resolved during validation. A main exercise without that data was never
// validated, which is a defect, not an empty muscle list.
func sessionTargetMuscles(p WorkoutPrescription, primaries map[string]string) ([]string, error) {
	var out []string
	seen := make(map[string]bool)
	for _, e := range p.MainExercises {
		muscle, ok := primaries[e.ExerciseID]
		if !ok {
			return nil, fmt.Errorf("%w: exercise %s has no validated primary muscle", ErrPlanGenerationFailed, e.ExerciseID)
		}
		if !seen[muscle] {
			seen[muscle] = true
			out = append(out, muscle)
		}
	}
	return out, nil
}

// resolveExerciseDetails returns the catalog name and measurement type ("REPS" | "DURATION").
func (c *CoachingContextAgent) resolveExerciseDetails(
	ctx context.Context, exerciseID string, names map[string]string,
) (name, measurementType string) {
	name = names[exerciseID]
	if c.catalog == nil {
		return name, "REPS"
	}
	ex, err := c.catalog.GetByID(ctx, exerciseID)
	if err != nil {
		if name == "" {
			slog.WarnContext(ctx, "No catalog entry for exercise", slog.String("exercise_id", exerciseID), slog.Any("error", err))
		}
		return name, "REPS"
	}
	if name == "" {
		name = ex.Name
	}
	measType := ex.MeasurementType
	if measType == "" {
		measType = "REPS"
	}
	return name, measType
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
			ExerciseID:      e.ExerciseID,
			TargetSets:      int(e.TargetSets),
			TargetReps:      int(e.TargetReps),
			DurationSeconds: int(e.DurationSeconds),
			TargetWeightKg:  float64(e.TargetWeight),
			TargetRPE:       float64(e.TargetRPE),
			RestSetSec:      int(e.RestSetSec),
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

// sessionKind maps the pinned kind to the domain; anything but RECOVERY trains.
func sessionKind(k string) roadmap.SessionKind {
	if k == kindRecovery {
		return roadmap.SessionKindRecovery
	}
	return roadmap.SessionKindTraining
}

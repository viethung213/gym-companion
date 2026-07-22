package domain

import (
	"fmt"
	"time"
)

type DayStatus string

const (
	DayStatusTraining    DayStatus = "TRAINING"
	DayStatusRest        DayStatus = "REST"
	DayStatusSkipped     DayStatus = "SKIPPED"
	DayStatusRescheduled DayStatus = "RESCHEDULED"
)

type ScheduleDay struct {
	ScheduledDate          time.Time
	DayOfWeek              int
	Status                 DayStatus
	TargetMuscleGroups     []string
	DailyWorkoutPlanID     string
	TimeWindow             string
	PlannedDurationMinutes int
}

type WeeklySchedule struct {
	id              string
	roadmapID       string
	userID          string
	weekNumber      int
	startDate       time.Time
	endDate         time.Time
	muscleSplitType string
	scheduleDays    []ScheduleDay
}

func NewWeeklySchedule(id, roadmapID, userID string, weekNumber int, startDate time.Time) (*WeeklySchedule, error) {
	if id == "" || roadmapID == "" || userID == "" {
		return nil, fmt.Errorf("%w: id, roadmapID, and userID cannot be empty", ErrInvalidSchedule)
	}
	return &WeeklySchedule{
		id:           id,
		roadmapID:    roadmapID,
		userID:       userID,
		weekNumber:   weekNumber,
		startDate:    startDate,
		endDate:      startDate.AddDate(0, 0, 6),
		scheduleDays: make([]ScheduleDay, 0),
	}, nil
}

func (w *WeeklySchedule) AddDay(day ScheduleDay) {
	w.scheduleDays = append(w.scheduleDays, day)
}

func (w *WeeklySchedule) SkipDay(date, now time.Time) error {
	for i, d := range w.scheduleDays {
		if d.ScheduledDate.Equal(date) {
			if d.ScheduledDate.Before(now.Truncate(24 * time.Hour)) {
				return fmt.Errorf("%w: cannot skip past day", ErrInvalidSchedule)
			}
			w.scheduleDays[i].Status = DayStatusSkipped
			return nil
		}
	}
	return fmt.Errorf("%w: day not found", ErrInvalidSchedule)
}

func (w *WeeklySchedule) RescheduleDay(from, to, now time.Time) error {
	for i, d := range w.scheduleDays {
		if d.ScheduledDate.Equal(from) {
			if d.ScheduledDate.Before(now.Truncate(24 * time.Hour)) {
				return fmt.Errorf("%w: cannot reschedule past day", ErrInvalidSchedule)
			}
			w.scheduleDays[i].ScheduledDate = to
			w.scheduleDays[i].Status = DayStatusRescheduled
			return nil
		}
	}
	return fmt.Errorf("%w: day not found", ErrInvalidSchedule)
}

func (w *WeeklySchedule) ValidateMuscleRecovery(minHoursBetweenMajorMuscles int) error {
	majorMuscles := map[string]bool{"Chest": true, "Back": true, "Legs": true, "Shoulders": true}
	
	for i := 0; i < len(w.scheduleDays); i++ {
		for j := i + 1; j < len(w.scheduleDays); j++ {
			day1 := w.scheduleDays[i]
			day2 := w.scheduleDays[j]
			
			if day1.Status != DayStatusTraining || day2.Status != DayStatusTraining {
				continue
			}
			
			diff := day2.ScheduledDate.Sub(day1.ScheduledDate).Hours()
			if diff < float64(minHoursBetweenMajorMuscles) && diff > -float64(minHoursBetweenMajorMuscles) {
				for _, m1 := range day1.TargetMuscleGroups {
					if majorMuscles[m1] {
						for _, m2 := range day2.TargetMuscleGroups {
							if m1 == m2 {
								return fmt.Errorf("%w: muscle %s trained again within %d hours", ErrMuscleRecoveryViolation, m1, minHoursBetweenMajorMuscles)
							}
						}
					}
				}
			}
		}
	}
	return nil
}

func (w *WeeklySchedule) ID() string { return w.id }
func (w *WeeklySchedule) Days() []ScheduleDay {
	days := make([]ScheduleDay, len(w.scheduleDays))
	copy(days, w.scheduleDays)
	return days
}

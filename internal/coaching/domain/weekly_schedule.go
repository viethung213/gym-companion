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
	scheduledDate          time.Time
	dayOfWeek              int
	status                 DayStatus
	targetMuscleGroups     []string
	dailyWorkoutPlanID     string
	timeWindow             string
	plannedDurationMinutes int
}

func NewScheduleDay(
	scheduledDate time.Time,
	dayOfWeek int,
	status DayStatus,
	targetMuscleGroups []string,
	dailyWorkoutPlanID string,
	timeWindow string,
	plannedDurationMinutes int,
) ScheduleDay {
	musclesCopy := make([]string, len(targetMuscleGroups))
	copy(musclesCopy, targetMuscleGroups)

	return ScheduleDay{
		scheduledDate:          scheduledDate,
		dayOfWeek:              dayOfWeek,
		status:                 status,
		targetMuscleGroups:     musclesCopy,
		dailyWorkoutPlanID:     dailyWorkoutPlanID,
		timeWindow:             timeWindow,
		plannedDurationMinutes: plannedDurationMinutes,
	}
}

func (s ScheduleDay) ScheduledDate() time.Time          { return s.scheduledDate }
func (s ScheduleDay) DayOfWeek() int                  { return s.dayOfWeek }
func (s ScheduleDay) Status() DayStatus               { return s.status }
func (s ScheduleDay) DailyWorkoutPlanID() string     { return s.dailyWorkoutPlanID }
func (s ScheduleDay) TimeWindow() string             { return s.timeWindow }
func (s ScheduleDay) PlannedDurationMinutes() int    { return s.plannedDurationMinutes }

func (s ScheduleDay) TargetMuscleGroups() []string {
	muscles := make([]string, len(s.targetMuscleGroups))
	copy(muscles, s.targetMuscleGroups)
	return muscles
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

func NewWeeklySchedule(id, roadmapID, userID string, weekNumber int, startDate time.Time, days []ScheduleDay) (*WeeklySchedule, error) {
	if id == "" || roadmapID == "" || userID == "" {
		return nil, fmt.Errorf("%w: id, roadmapID, and userID cannot be empty", ErrInvalidSchedule)
	}
	
	daysCopy := make([]ScheduleDay, len(days))
	for i, d := range days {
		daysCopy[i] = NewScheduleDay(
			d.scheduledDate,
			d.dayOfWeek,
			d.status,
			d.targetMuscleGroups,
			d.dailyWorkoutPlanID,
			d.timeWindow,
			d.plannedDurationMinutes,
		)
	}

	return &WeeklySchedule{
		id:           id,
		roadmapID:    roadmapID,
		userID:       userID,
		weekNumber:   weekNumber,
		startDate:    startDate,
		endDate:      startDate.AddDate(0, 0, 6),
		scheduleDays: daysCopy,
	}, nil
}


func (w *WeeklySchedule) AddDay(day ScheduleDay) {
	// Deep copy day and its slice
	copiedDay := NewScheduleDay(
		day.scheduledDate,
		day.dayOfWeek,
		day.status,
		day.targetMuscleGroups,
		day.dailyWorkoutPlanID,
		day.timeWindow,
		day.plannedDurationMinutes,
	)
	w.scheduleDays = append(w.scheduleDays, copiedDay)
}

func (w *WeeklySchedule) SkipDay(date, now time.Time) error {
	for i, d := range w.scheduleDays {
		if d.scheduledDate.Equal(date) {
			if d.scheduledDate.Before(now.Truncate(24 * time.Hour)) {
				return fmt.Errorf("%w: cannot skip past day", ErrInvalidSchedule)
			}
			w.scheduleDays[i].status = DayStatusSkipped
			return nil
		}
	}
	return fmt.Errorf("%w: day not found", ErrInvalidSchedule)
}

func (w *WeeklySchedule) RescheduleDay(from, to, now time.Time) error {
	for i, d := range w.scheduleDays {
		if d.scheduledDate.Equal(from) {
			if d.scheduledDate.Before(now.Truncate(24 * time.Hour)) {
				return fmt.Errorf("%w: cannot reschedule past day", ErrInvalidSchedule)
			}
			w.scheduleDays[i].scheduledDate = to
			w.scheduleDays[i].status = DayStatusRescheduled
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

			if day1.status != DayStatusTraining || day2.status != DayStatusTraining {
				continue
			}

			diff := day2.scheduledDate.Sub(day1.scheduledDate).Hours()
			if diff < float64(minHoursBetweenMajorMuscles) && diff > -float64(minHoursBetweenMajorMuscles) {
				for _, m1 := range day1.targetMuscleGroups {
					if majorMuscles[m1] {
						for _, m2 := range day2.targetMuscleGroups {
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
	for i, d := range w.scheduleDays {
		days[i] = NewScheduleDay(
			d.scheduledDate,
			d.dayOfWeek,
			d.status,
			d.targetMuscleGroups,
			d.dailyWorkoutPlanID,
			d.timeWindow,
			d.plannedDurationMinutes,
		)
	}
	return days
}


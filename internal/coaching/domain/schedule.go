package domain

import (
	"errors"
	"fmt"
	"time"
)

var (
	ErrInvalidWeekNumber = errors.New("week_number must be between 1 and 4")
	ErrViolationBRAC01   = errors.New("BR-AC-01 violation: weekly schedule must have at least 1 rest day and at most 6 training days")
	ErrInvalidSchedule   = errors.New("schedule_days must contain exactly 7 days")
)

type WorkoutDayStatus int32

const (
	WorkoutDayStatusUnspecified  WorkoutDayStatus = 0
	WorkoutDayStatusTraining     WorkoutDayStatus = 1
	WorkoutDayStatusRest         WorkoutDayStatus = 2
	WorkoutDayStatusSkipped      WorkoutDayStatus = 3
	WorkoutDayStatusRescheduled  WorkoutDayStatus = 4
)

type ScheduleDay struct {
	scheduledDate      time.Time
	dayOfWeek          string
	status             WorkoutDayStatus
	targetMuscleGroups []string
	dailyPlanID        string
}

func NewScheduleDay(scheduledDate time.Time, dayOfWeek string, status WorkoutDayStatus, targetMuscleGroups []string, dailyPlanID string) ScheduleDay {
	return ScheduleDay{
		scheduledDate:      scheduledDate,
		dayOfWeek:          dayOfWeek,
		status:             status,
		targetMuscleGroups: targetMuscleGroups,
		dailyPlanID:        dailyPlanID,
	}
}

func (d ScheduleDay) ScheduledDate() time.Time      { return d.scheduledDate }
func (d ScheduleDay) DayOfWeek() string              { return d.dayOfWeek }
func (d ScheduleDay) Status() WorkoutDayStatus       { return d.status }
func (d ScheduleDay) TargetMuscleGroups() []string  { return d.targetMuscleGroups }
func (d ScheduleDay) DailyPlanID() string            { return d.dailyPlanID }

func (d ScheduleDay) IsRestDay() bool {
	return d.status == WorkoutDayStatusRest
}

func (d ScheduleDay) IsTrainingDay() bool {
	return d.status == WorkoutDayStatusTraining
}

// WeeklySchedule represents a 7-day training schedule Aggregate Root.
type WeeklySchedule struct {
	id               string
	roadmapID        string
	userID           string
	weekNumber       int32
	startDate        time.Time
	endDate          time.Time
	muscleSplitType  string
	scheduleDays     []ScheduleDay
	createdAt        time.Time
	updatedAt        time.Time
}

func NewWeeklySchedule(id string, roadmapID string, userID string, weekNumber int32, startDate time.Time, endDate time.Time, muscleSplitType string, days []ScheduleDay) (*WeeklySchedule, error) {
	if userID == "" || roadmapID == "" {
		return nil, ErrInvalidUser
	}
	if weekNumber < 1 || weekNumber > 4 {
		return nil, ErrInvalidWeekNumber
	}
	if len(days) != 7 {
		return nil, ErrInvalidSchedule
	}

	// Validate BR-AC-01: At least 1 rest day, at most 6 training days
	trainingCount := 0
	restCount := 0
	for _, day := range days {
		if day.IsTrainingDay() {
			trainingCount++
		} else if day.IsRestDay() {
			restCount++
		}
	}
	if restCount < 1 || trainingCount > 6 {
		return nil, ErrViolationBRAC01
	}

	if id == "" {
		id = fmt.Sprintf("ws_%d", time.Now().UnixNano())
	}
	now := time.Now().UTC()

	return &WeeklySchedule{
		id:              id,
		roadmapID:       roadmapID,
		userID:          userID,
		weekNumber:      weekNumber,
		startDate:       startDate,
		endDate:         endDate,
		muscleSplitType: muscleSplitType,
		scheduleDays:    days,
		createdAt:       now,
		updatedAt:       now,
	}, nil
}

func ReconstituteWeeklySchedule(id, roadmapID, userID string, weekNumber int32, startDate, endDate time.Time, muscleSplitType string, days []ScheduleDay, createdAt, updatedAt time.Time) *WeeklySchedule {
	return &WeeklySchedule{
		id:              id,
		roadmapID:       roadmapID,
		userID:          userID,
		weekNumber:      weekNumber,
		startDate:       startDate,
		endDate:         endDate,
		muscleSplitType: muscleSplitType,
		scheduleDays:    days,
		createdAt:       createdAt,
		updatedAt:       updatedAt,
	}
}

func (w *WeeklySchedule) ID() string                { return w.id }
func (w *WeeklySchedule) RoadmapID() string         { return w.roadmapID }
func (w *WeeklySchedule) UserID() string            { return w.userID }
func (w *WeeklySchedule) WeekNumber() int32         { return w.weekNumber }
func (w *WeeklySchedule) StartDate() time.Time      { return w.startDate }
func (w *WeeklySchedule) EndDate() time.Time        { return w.endDate }
func (w *WeeklySchedule) MuscleSplitType() string   { return w.muscleSplitType }
func (w *WeeklySchedule) ScheduleDays() []ScheduleDay { return w.scheduleDays }
func (w *WeeklySchedule) CreatedAt() time.Time      { return w.createdAt }
func (w *WeeklySchedule) UpdatedAt() time.Time      { return w.updatedAt }

// MarkDaySkipped automatically marks a missed day as Skipped (BR-AC-03 Decision 1.1)
func (w *WeeklySchedule) MarkDaySkipped(date time.Time) bool {
	for i, day := range w.scheduleDays {
		if day.scheduledDate.Year() == date.Year() && day.scheduledDate.YearDay() == date.YearDay() {
			if day.status == WorkoutDayStatusTraining {
				w.scheduleDays[i].status = WorkoutDayStatusSkipped
				w.updatedAt = time.Now().UTC()
				return true
			}
		}
	}
	return false
}

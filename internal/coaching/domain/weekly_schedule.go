package domain

import (
	"errors"
	"time"
)

var (
	ErrScheduleMustContainSevenDays = errors.New("weekly schedule must contain seven days")
	ErrScheduleMustContainRestDay   = errors.New("weekly schedule must contain at least one rest day")
)

type ScheduleDayStatus string

const (
	ScheduleDayStatusTraining    ScheduleDayStatus = "training"
	ScheduleDayStatusRest        ScheduleDayStatus = "rest"
	ScheduleDayStatusSkipped     ScheduleDayStatus = "skipped"
	ScheduleDayStatusRescheduled ScheduleDayStatus = "rescheduled"
)

type ScheduleDay struct {
	Date         time.Time         `json:"date"`
	Status       ScheduleDayStatus `json:"status"`
	MuscleGroups []string          `json:"muscle_groups"`
	DailyPlanID  string            `json:"daily_plan_id"`
}

type WeeklySchedule struct {
	ID         string
	RoadmapID  string
	UserID     string
	WeekNumber int
	StartDate  time.Time
	EndDate    time.Time
	Days       []ScheduleDay
}

func NewWeeklySchedule(
	id, roadmapID, userID string,
	weekNumber int,
	days []ScheduleDay,
) (*WeeklySchedule, error) {
	if len(days) != 7 {
		return nil, ErrScheduleMustContainSevenDays
	}

	var restDays int
	for _, day := range days {
		if day.Status == ScheduleDayStatusRest {
			restDays++
		}
	}
	if restDays == 0 {
		return nil, ErrScheduleMustContainRestDay
	}

	copiedDays := copyScheduleDays(days)
	return &WeeklySchedule{
		ID:         id,
		RoadmapID:  roadmapID,
		UserID:     userID,
		WeekNumber: weekNumber,
		StartDate:  dateOnly(copiedDays[0].Date),
		EndDate:    dateOnly(copiedDays[6].Date),
		Days:       copiedDays,
	}, nil
}

func (s *WeeklySchedule) AttachDailyPlan(scheduledDate time.Time, dailyPlanID string) error {
	for index := range s.Days {
		if !dateOnly(s.Days[index].Date).Equal(dateOnly(scheduledDate)) {
			continue
		}
		s.Days[index].DailyPlanID = dailyPlanID
		return nil
	}

	return errors.New("scheduled date does not belong to weekly schedule")
}

func copyScheduleDays(days []ScheduleDay) []ScheduleDay {
	copiedDays := make([]ScheduleDay, len(days))
	for index, day := range days {
		day.MuscleGroups = append([]string(nil), day.MuscleGroups...)
		copiedDays[index] = day
	}
	return copiedDays
}

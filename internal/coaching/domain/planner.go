package domain

import (
	"errors"
	"time"
)

var ErrPreferredWeekdaysInsufficient = errors.New(
	"preferred weekdays are fewer than training days",
)

type SchedulePlanner struct{}

func (SchedulePlanner) PlanWeek(input *PlanningInput, weekNumber int) ([]ScheduleDay, error) {
	if err := input.Validate(); err != nil {
		return nil, err
	}

	trainingDays := selectedWeekdays(input.PreferredWeekdays, input.TrainingDaysPerWeek)
	if len(trainingDays) != input.TrainingDaysPerWeek {
		return nil, ErrPreferredWeekdaysInsufficient
	}

	startDate := dateOnly(input.StartDate).AddDate(0, 0, (weekNumber-1)*7)
	days := make([]ScheduleDay, 0, 7)
	for offset := 0; offset < 7; offset++ {
		date := startDate.AddDate(0, 0, offset)
		day := ScheduleDay{Date: date, Status: ScheduleDayStatusRest}
		if trainingDays[date.Weekday()] {
			day.Status = ScheduleDayStatusTraining
			day.MuscleGroups = muscleGroupsForTrainingDay(offset, input.TrainingDaysPerWeek)
		}
		days = append(days, day)
	}

	return days, nil
}

func selectedWeekdays(preferred []time.Weekday, required int) map[time.Weekday]bool {
	selected := make(map[time.Weekday]bool, required)
	for _, weekday := range preferred {
		if len(selected) == required {
			break
		}
		selected[weekday] = true
	}
	return selected
}

func muscleGroupsForTrainingDay(dayOffset, trainingDays int) []string {
	splits := [][]string{
		{"chest", "triceps"},
		{"back", "biceps"},
		{"legs"},
		{"shoulders", "core"},
		{"chest", "back"},
		{"legs", "core"},
	}
	return append([]string(nil), splits[dayOffset%trainingDays]...)
}

type PrescriptionPlanner struct{}

func (PrescriptionPlanner) Plan(
	options []ExerciseOption,
	experienceLevel ExperienceLevel,
) []PrescribedExercise {
	sets, reps := prescriptionTargets(experienceLevel)
	exercises := make([]PrescribedExercise, 0, len(options))
	for _, option := range options {
		restSeconds := option.DefaultRestSeconds
		if restSeconds <= 0 {
			restSeconds = 60
		}
		exercises = append(exercises, PrescribedExercise{
			ExerciseID:   option.ID,
			ExerciseName: option.Name,
			Sets:         sets,
			Reps:         reps,
			RestSeconds:  restSeconds,
		})
	}
	return exercises
}

func (PrescriptionPlanner) PlanSessionActivities(
	maxSessionMinutes int,
) (warmUpItems, coolDownItems []PlannedActivity) {
	warmUpDuration := 5 * 60
	coolDownDuration := 5 * 60
	if maxSessionMinutes >= 60 {
		warmUpDuration = 8 * 60
		coolDownDuration = 7 * 60
	}
	return []PlannedActivity{{
			Name:            "Dynamic warm-up",
			DurationSeconds: warmUpDuration,
			Notes:           "Prepare the joints and movement patterns used in the session.",
		}}, []PlannedActivity{{
			Name:            "Cool-down mobility",
			DurationSeconds: coolDownDuration,
			Notes:           "Reduce intensity gradually and restore comfortable range of motion.",
		}}
}

func prescriptionTargets(experienceLevel ExperienceLevel) (sets, reps int) {
	switch experienceLevel {
	case ExperienceLevelAdvanced:
		return 4, 8
	case ExperienceLevelIntermediate:
		return 3, 10
	default:
		return 2, 12
	}
}

type PlannedVolumeValidator struct{}

func (PlannedVolumeValidator) Validate(previousVolume, nextVolume int) bool {
	if previousVolume <= 0 {
		return true
	}
	return nextVolume <= previousVolume*110/100
}

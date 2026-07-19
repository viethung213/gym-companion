package domain

import (
	"errors"
	"fmt"
	"time"
)

// WorkoutDayStatus represents the status of a single day in the weekly schedule.
type WorkoutDayStatus string

const (
	WorkoutDayTraining    WorkoutDayStatus = "TRAINING"
	WorkoutDayRest        WorkoutDayStatus = "REST"
	WorkoutDaySkipped     WorkoutDayStatus = "SKIPPED"
	WorkoutDayRescheduled WorkoutDayStatus = "RESCHEDULED"
)

const (
	maxTrainingDaysPerWeek = 6
	minRestDaysPerWeek     = 1
	daysPerWeek            = 7
)

var (
	ErrInvalidSchedule        = errors.New("invalid weekly schedule")
	ErrScheduleInvariant      = errors.New("schedule invariant violated")
	ErrInvalidExperienceLevel = errors.New("invalid experience level")
)

// ScheduleDay is a Value Object representing one day within a weekly schedule.
type ScheduleDay struct {
	Date               time.Time
	DayOfWeek          string
	Status             WorkoutDayStatus
	TargetMuscleGroups []string
	DailyPlanID        string
}

// WeeklySchedule is the Aggregate Root for distributing training load
// and muscle groups across a specific week.
type WeeklySchedule struct {
	id              string
	roadmapID       string
	userID          string
	weekNumber      int
	startDate       time.Time
	endDate         time.Time
	muscleSplitType string
	scheduleDays    []ScheduleDay
	createdAt       time.Time
	updatedAt       time.Time
}

// GenerateWeeklySchedule creates a new WeeklySchedule based on the user's experience level.
// It allocates training vs rest days according to:
//   - Beginner: 3 days/week
//   - Intermediate: 4 days/week
//   - Advanced: 5 days/week
func GenerateWeeklySchedule(
	id, roadmapID, userID string,
	weekNumber int,
	startDate time.Time,
	experienceLevel string,
	now time.Time,
) (*WeeklySchedule, error) {
	if id == "" {
		return nil, fmt.Errorf("%w: id is required", ErrInvalidSchedule)
	}
	if roadmapID == "" {
		return nil, fmt.Errorf("%w: roadmap_id is required", ErrInvalidSchedule)
	}
	if userID == "" {
		return nil, fmt.Errorf("%w: user_id is required", ErrInvalidSchedule)
	}
	if weekNumber < 1 {
		return nil, fmt.Errorf("%w: week_number must be >= 1", ErrInvalidSchedule)
	}

	trainingDays, splitType, err := resolveTrainingPlan(experienceLevel)
	if err != nil {
		return nil, err
	}

	endDate := startDate.AddDate(0, 0, daysPerWeek-1)
	days := buildScheduleDays(startDate, trainingDays, splitType)

	return &WeeklySchedule{
		id:              id,
		roadmapID:       roadmapID,
		userID:          userID,
		weekNumber:      weekNumber,
		startDate:       startDate,
		endDate:         endDate,
		muscleSplitType: splitType,
		scheduleDays:    days,
		createdAt:       now,
		updatedAt:       now,
	}, nil
}

// RehydrateWeeklySchedule reconstructs a WeeklySchedule from persistence.
func RehydrateWeeklySchedule(
	id, roadmapID, userID string,
	weekNumber int,
	startDate, endDate time.Time,
	muscleSplitType string,
	scheduleDays []ScheduleDay,
	createdAt, updatedAt time.Time,
) *WeeklySchedule {
	return &WeeklySchedule{
		id:              id,
		roadmapID:       roadmapID,
		userID:          userID,
		weekNumber:      weekNumber,
		startDate:       startDate,
		endDate:         endDate,
		muscleSplitType: muscleSplitType,
		scheduleDays:    copyScheduleDays(scheduleDays),
		createdAt:       createdAt,
		updatedAt:       updatedAt,
	}
}

func (s *WeeklySchedule) ID() string              { return s.id }
func (s *WeeklySchedule) RoadmapID() string        { return s.roadmapID }
func (s *WeeklySchedule) UserID() string           { return s.userID }
func (s *WeeklySchedule) WeekNumber() int          { return s.weekNumber }
func (s *WeeklySchedule) StartDate() time.Time     { return s.startDate }
func (s *WeeklySchedule) EndDate() time.Time       { return s.endDate }
func (s *WeeklySchedule) MuscleSplitType() string  { return s.muscleSplitType }
func (s *WeeklySchedule) CreatedAt() time.Time     { return s.createdAt }
func (s *WeeklySchedule) UpdatedAt() time.Time     { return s.updatedAt }

// ScheduleDays returns a defensive copy of the schedule days.
func (s *WeeklySchedule) ScheduleDays() []ScheduleDay {
	return copyScheduleDays(s.scheduleDays)
}

// TrainingDays returns only the days with TRAINING status.
func (s *WeeklySchedule) TrainingDays() []ScheduleDay {
	var result []ScheduleDay
	for _, d := range s.scheduleDays {
		if d.Status == WorkoutDayTraining {
			result = append(result, d)
		}
	}
	return result
}

// SetDailyPlanID links a DailyWorkoutPlan to a specific training day by date.
func (s *WeeklySchedule) SetDailyPlanID(date time.Time, planID string) error {
	for i := range s.scheduleDays {
		if s.scheduleDays[i].Date.Equal(date) {
			if s.scheduleDays[i].Status != WorkoutDayTraining {
				return fmt.Errorf("%w: cannot assign plan to non-training day %s",
					ErrScheduleInvariant, date.Format("2006-01-02"))
			}
			s.scheduleDays[i].DailyPlanID = planID
			return nil
		}
	}
	return fmt.Errorf("%w: date %s not found in schedule",
		ErrInvalidSchedule, date.Format("2006-01-02"))
}

// resolveTrainingPlan determines training days count and muscle split type.
func resolveTrainingPlan(experienceLevel string) (int, string, error) {
	switch experienceLevel {
	case "beginner":
		return 3, "FullBody", nil
	case "intermediate":
		return 4, "Upper/Lower", nil
	case "advanced":
		return 5, "Push/Pull/Legs", nil
	default:
		return 0, "", fmt.Errorf("%w: %s", ErrInvalidExperienceLevel, experienceLevel)
	}
}

// buildScheduleDays creates the 7-day schedule with training/rest allocation.
// Training days are spread across the week with rest days in between.
func buildScheduleDays(startDate time.Time, trainingDays int, splitType string) []ScheduleDay {
	muscleGroups := resolveMuscleGroups(splitType, trainingDays)
	trainingSlots := spreadTrainingSlots(trainingDays)

	days := make([]ScheduleDay, daysPerWeek)
	trainingIdx := 0
	for i := range daysPerWeek {
		date := startDate.AddDate(0, 0, i)
		day := ScheduleDay{
			Date:      date,
			DayOfWeek: date.Weekday().String(),
		}
		if trainingSlots[i] {
			day.Status = WorkoutDayTraining
			day.TargetMuscleGroups = muscleGroups[trainingIdx]
			trainingIdx++
		} else {
			day.Status = WorkoutDayRest
		}
		days[i] = day
	}
	return days
}

// spreadTrainingSlots distributes training days across the week with maximum spacing.
func spreadTrainingSlots(count int) [daysPerWeek]bool {
	var slots [daysPerWeek]bool
	if count <= 0 || count > maxTrainingDaysPerWeek {
		return slots
	}
	// Evenly distribute training days: Mon(0), Wed(2), Fri(4) for 3 days, etc.
	gap := daysPerWeek / count
	for i := range count {
		idx := i * gap
		if idx >= daysPerWeek {
			idx = daysPerWeek - 1
		}
		slots[idx] = true
	}
	return slots
}

// resolveMuscleGroups returns muscle group assignments per training day.
func resolveMuscleGroups(splitType string, trainingDays int) [][]string {
	switch splitType {
	case "FullBody":
		groups := make([][]string, trainingDays)
		for i := range trainingDays {
			groups[i] = []string{"FullBody"}
		}
		return groups
	case "Upper/Lower":
		return [][]string{
			{"Chest", "Back", "Shoulders"},
			{"Quads", "Hamstrings", "Glutes"},
			{"Chest", "Back", "Arms"},
			{"Quads", "Hamstrings", "Calves"},
		}[:trainingDays]
	case "Push/Pull/Legs":
		return [][]string{
			{"Chest", "Shoulders", "Triceps"},
			{"Back", "Biceps"},
			{"Quads", "Hamstrings", "Glutes", "Calves"},
			{"Chest", "Shoulders", "Triceps"},
			{"Back", "Biceps"},
		}[:trainingDays]
	default:
		groups := make([][]string, trainingDays)
		for i := range trainingDays {
			groups[i] = []string{"FullBody"}
		}
		return groups
	}
}

func copyScheduleDays(days []ScheduleDay) []ScheduleDay {
	if days == nil {
		return nil
	}
	copied := make([]ScheduleDay, len(days))
	for i, d := range days {
		copied[i] = d
		copied[i].TargetMuscleGroups = copyStrings(d.TargetMuscleGroups)
	}
	return copied
}

func copyStrings(values []string) []string {
	if values == nil {
		return nil
	}
	copied := make([]string, len(values))
	copy(copied, values)
	return copied
}

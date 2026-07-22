package domain

import "time"

// Unmarshal functions for Persistence reconstruction

func UnmarshalWorkoutRoadmap(
	id, userID, status string,
	startDate time.Time,
	endDate *time.Time,
	planningTier string,
	createdAt, updatedAt time.Time,
) *WorkoutRoadmap {
	return &WorkoutRoadmap{
		id:           id,
		userID:       userID,
		status:       PlanStatus(status),
		startDate:    startDate,
		endDate:      endDate,
		planningTier: PlanningTier(planningTier),
		createdAt:    createdAt,
		updatedAt:    updatedAt,
	}
}

func (w *WorkoutRoadmap) StartDate() time.Time  { return w.startDate }
func (w *WorkoutRoadmap) EndDate() *time.Time   { return w.endDate }
func (w *WorkoutRoadmap) CreatedAt() time.Time  { return w.createdAt }
func (w *WorkoutRoadmap) UpdatedAt() time.Time  { return w.updatedAt }

func UnmarshalScheduleDay(
	id string,
	scheduledDate time.Time,
	dayOfWeek int,
	status string,
	targetMuscleGroups []string,
	dailyWorkoutPlanID string,
	timeWindow string,
	plannedDurationMinutes int,
) ScheduleDay {
	return ScheduleDay{
		id:                     id,
		scheduledDate:          scheduledDate,
		dayOfWeek:              dayOfWeek,
		status:                 DayStatus(status),
		targetMuscleGroups:     targetMuscleGroups,
		dailyWorkoutPlanID:     dailyWorkoutPlanID,
		timeWindow:             timeWindow,
		plannedDurationMinutes: plannedDurationMinutes,
	}
}

func UnmarshalWeeklySchedule(
	id, roadmapID, userID string,
	weekNumber int,
	startDate, endDate time.Time,
	muscleSplitType string,
	scheduleDays []ScheduleDay,
) *WeeklySchedule {
	return &WeeklySchedule{
		id:              id,
		roadmapID:       roadmapID,
		userID:          userID,
		weekNumber:      weekNumber,
		startDate:       startDate,
		endDate:         endDate,
		muscleSplitType: muscleSplitType,
		scheduleDays:    scheduleDays,
	}
}

func (w *WeeklySchedule) RoadmapID() string       { return w.roadmapID }
func (w *WeeklySchedule) UserID() string          { return w.userID }
func (w *WeeklySchedule) WeekNumber() int         { return w.weekNumber }
func (w *WeeklySchedule) StartDate() time.Time    { return w.startDate }
func (w *WeeklySchedule) EndDate() time.Time      { return w.endDate }
func (w *WeeklySchedule) MuscleSplitType() string { return w.muscleSplitType }

func UnmarshalPlannedExercise(
	id string,
	exerciseID string,
	sets, reps int,
	weight float64,
	rpe float64,
	restSeconds int,
	notes string,
) PlannedExercise {
	return PlannedExercise{
		id:         id,
		exerciseID: exerciseID,
		prescription: WorkoutPrescription{
			sets:        sets,
			reps:        reps,
			weight:      weight,
			rpe:         rpe,
			restSeconds: restSeconds,
		},
		notes: notes,
	}
}


func UnmarshalDailyWorkoutPlan(
	id, scheduleID, userID, status string,
	exercises []PlannedExercise,
	createdAt, updatedAt time.Time,
) *DailyWorkoutPlan {
	return &DailyWorkoutPlan{
		id:         id,
		scheduleID: scheduleID,
		userID:     userID,
		status:     PlanStatus(status),
		exercises:  exercises,
		createdAt:  createdAt,
		updatedAt:  updatedAt,
	}
}

func (d *DailyWorkoutPlan) CreatedAt() time.Time { return d.createdAt }
func (d *DailyWorkoutPlan) UpdatedAt() time.Time { return d.updatedAt }

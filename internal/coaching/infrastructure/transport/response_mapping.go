package transport

import (
	"strings"
	"time"

	"github.com/viethung213/gym-companion/internal/coaching/domain"
	coachingmsg "github.com/viethung213/gym-companion/internal/gen/go/contracts/core/coaching/v1/message"
	datepb "google.golang.org/genproto/googleapis/type/date"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func toProtoRoadmap(roadmap *domain.WorkoutRoadmap) *coachingmsg.WorkoutRoadmap {
	return &coachingmsg.WorkoutRoadmap{
		RoadmapId:         roadmap.ID,
		UserId:            roadmap.UserID,
		Status:            toProtoRoadmapStatus(roadmap.Status),
		StartDate:         toProtoDate(roadmap.StartDate),
		EndDate:           toProtoDate(roadmap.EndDate),
		Phases:            roadmapPhases(),
		CompletionRate:    &coachingmsg.CompletionRate{Value: 0, Band: "not_started"},
		ProfileSnapshotId: roadmap.Input.ProfileSnapshotID,
		PlanningSnapshot:  toProtoPlanningSnapshot(roadmap.Input),
		PlannerVersion:    roadmap.PlannerVersion,
	}
}

func toProtoPlanningSnapshot(input domain.PlanningInput) *coachingmsg.PlanningProfileSnapshot {
	weekdays := make([]string, 0, len(input.PreferredWeekdays))
	for _, weekday := range input.PreferredWeekdays {
		weekdays = append(weekdays, strings.ToLower(weekday.String()))
	}
	return &coachingmsg.PlanningProfileSnapshot{
		Goal:                toProtoGoal(input.Goal),
		ExperienceLevel:     toProtoExperience(input.ExperienceLevel),
		TrainingDaysPerWeek: int32(input.TrainingDaysPerWeek),
		PreferredWeekdays:   weekdays,
		MaxSessionMinutes:   int32(input.MaxSessionMinutes),
		EquipmentIds:        append([]string(nil), input.EquipmentIDs...),
		ActiveInjuryAreas:   append([]string(nil), input.ActiveInjuryAreas...),
		Timezone:            input.Timezone,
		StartDate:           toProtoDate(input.StartDate),
	}
}

func roadmapPhases() []*coachingmsg.RoadmapPhase {
	return []*coachingmsg.RoadmapPhase{
		{WeekNumber: 1, Name: "Foundation", Focus: "Establish the planned schedule and baseline volume."},
		{WeekNumber: 2, Name: "Build", Focus: "Progress planned volume within the ten-percent limit.", TargetOverloadPercent: 10},
		{WeekNumber: 3, Name: "Consolidate", Focus: "Repeat the schedule with controlled planned progression.", TargetOverloadPercent: 10},
		{WeekNumber: 4, Name: "Complete", Focus: "Complete the fixed cycle before a future review."},
	}
}

func toProtoSchedule(schedule *domain.WeeklySchedule) *coachingmsg.WeeklySchedule {
	response := &coachingmsg.WeeklySchedule{
		WeeklyScheduleId: schedule.ID,
		RoadmapId:        schedule.RoadmapID,
		UserId:           schedule.UserID,
		WeekNumber:       int32(schedule.WeekNumber),
		StartDate:        toProtoDate(schedule.StartDate),
		EndDate:          toProtoDate(schedule.EndDate),
		MuscleSplit:      &coachingmsg.MuscleSplit{SplitType: "deterministic_weekly_split"},
	}
	seenGroups := make(map[string]struct{})
	for _, day := range schedule.Days {
		response.ScheduleDays = append(response.ScheduleDays, &coachingmsg.ScheduleDay{
			ScheduledDate:      toProtoDate(day.Date),
			DayOfWeek:          strings.ToLower(day.Date.Weekday().String()),
			Status:             toProtoScheduleDayStatus(day.Status),
			MuscleGroups:       append([]string(nil), day.MuscleGroups...),
			DailyWorkoutPlanId: day.DailyPlanID,
		})
		if day.DailyPlanID != "" {
			response.DailyWorkoutPlanIds = append(response.DailyWorkoutPlanIds, day.DailyPlanID)
		}
		for _, group := range day.MuscleGroups {
			if _, exists := seenGroups[group]; exists {
				continue
			}
			seenGroups[group] = struct{}{}
			response.MuscleSplit.PrimaryMuscleGroups = append(response.MuscleSplit.PrimaryMuscleGroups, group)
		}
	}
	return response
}

func toProtoDailyPlan(plan *domain.DailyWorkoutPlan) *coachingmsg.DailyWorkoutPlan {
	prescription := &coachingmsg.WorkoutPrescription{}
	for _, exercise := range plan.Exercises {
		prescription.Exercises = append(prescription.Exercises, &coachingmsg.PrescribedExercise{
			ExerciseId:   exercise.ExerciseID,
			ExerciseName: exercise.ExerciseName,
			TargetSets:   int32(exercise.Sets),
			TargetReps:   int32(exercise.Reps),
			RestSeconds:  int32(exercise.RestSeconds),
		})
	}
	for _, item := range plan.WarmUpItems {
		prescription.WarmUpItems = append(prescription.WarmUpItems, toProtoActivity(item))
	}
	for _, item := range plan.CoolDownItems {
		prescription.CoolDownItems = append(prescription.CoolDownItems, toProtoActivity(item))
	}
	return &coachingmsg.DailyWorkoutPlan{
		DailyWorkoutPlanId:  plan.ID,
		UserId:              plan.UserID,
		RoadmapId:           plan.RoadmapID,
		WeeklyScheduleId:    plan.WeeklyScheduleID,
		ScheduledDate:       toProtoDate(plan.ScheduledDate),
		Status:              toProtoDailyPlanStatus(plan.Status),
		WorkoutPrescription: prescription,
		GeneratedAt:         timestamppb.New(plan.GeneratedAt),
	}
}

func toProtoActivity(item domain.PlannedActivity) *coachingmsg.AccessoryExercise {
	return &coachingmsg.AccessoryExercise{
		ExerciseName:    item.Name,
		DurationSeconds: int32(item.DurationSeconds),
		Notes:           item.Notes,
	}
}

func toProtoDate(value time.Time) *datepb.Date {
	return &datepb.Date{Year: int32(value.Year()), Month: int32(value.Month()), Day: int32(value.Day())}
}

func toDomainGoal(value coachingmsg.PlanningGoal) domain.PlanningGoal {
	switch value {
	case coachingmsg.PlanningGoal_PLANNING_GOAL_MUSCLE_GAIN:
		return domain.PlanningGoalMuscleGain
	case coachingmsg.PlanningGoal_PLANNING_GOAL_FAT_LOSS:
		return domain.PlanningGoalFatLoss
	case coachingmsg.PlanningGoal_PLANNING_GOAL_GENERAL_FITNESS:
		return domain.PlanningGoalGeneralFitness
	default:
		return ""
	}
}

func toDomainExperience(value coachingmsg.ExperienceLevel) domain.ExperienceLevel {
	switch value {
	case coachingmsg.ExperienceLevel_EXPERIENCE_LEVEL_BEGINNER:
		return domain.ExperienceLevelBeginner
	case coachingmsg.ExperienceLevel_EXPERIENCE_LEVEL_INTERMEDIATE:
		return domain.ExperienceLevelIntermediate
	case coachingmsg.ExperienceLevel_EXPERIENCE_LEVEL_ADVANCED:
		return domain.ExperienceLevelAdvanced
	default:
		return ""
	}
}

func toProtoGoal(value domain.PlanningGoal) coachingmsg.PlanningGoal {
	switch value {
	case domain.PlanningGoalMuscleGain:
		return coachingmsg.PlanningGoal_PLANNING_GOAL_MUSCLE_GAIN
	case domain.PlanningGoalFatLoss:
		return coachingmsg.PlanningGoal_PLANNING_GOAL_FAT_LOSS
	case domain.PlanningGoalGeneralFitness:
		return coachingmsg.PlanningGoal_PLANNING_GOAL_GENERAL_FITNESS
	default:
		return coachingmsg.PlanningGoal_PLANNING_GOAL_UNSPECIFIED
	}
}

func toProtoExperience(value domain.ExperienceLevel) coachingmsg.ExperienceLevel {
	switch value {
	case domain.ExperienceLevelBeginner:
		return coachingmsg.ExperienceLevel_EXPERIENCE_LEVEL_BEGINNER
	case domain.ExperienceLevelIntermediate:
		return coachingmsg.ExperienceLevel_EXPERIENCE_LEVEL_INTERMEDIATE
	case domain.ExperienceLevelAdvanced:
		return coachingmsg.ExperienceLevel_EXPERIENCE_LEVEL_ADVANCED
	default:
		return coachingmsg.ExperienceLevel_EXPERIENCE_LEVEL_UNSPECIFIED
	}
}

func toProtoRoadmapStatus(value domain.RoadmapStatus) coachingmsg.RoadmapStatus {
	switch value {
	case domain.RoadmapStatusActive:
		return coachingmsg.RoadmapStatus_ROADMAP_STATUS_ACTIVE
	case domain.RoadmapStatusPaused:
		return coachingmsg.RoadmapStatus_ROADMAP_STATUS_PAUSED
	case domain.RoadmapStatusCompleted:
		return coachingmsg.RoadmapStatus_ROADMAP_STATUS_COMPLETED
	case domain.RoadmapStatusCancelled:
		return coachingmsg.RoadmapStatus_ROADMAP_STATUS_CANCELLED
	default:
		return coachingmsg.RoadmapStatus_ROADMAP_STATUS_UNSPECIFIED
	}
}

func toProtoScheduleDayStatus(value domain.ScheduleDayStatus) coachingmsg.ScheduleDayStatus {
	switch value {
	case domain.ScheduleDayStatusTraining:
		return coachingmsg.ScheduleDayStatus_SCHEDULE_DAY_STATUS_TRAINING
	case domain.ScheduleDayStatusRest:
		return coachingmsg.ScheduleDayStatus_SCHEDULE_DAY_STATUS_REST
	case domain.ScheduleDayStatusSkipped:
		return coachingmsg.ScheduleDayStatus_SCHEDULE_DAY_STATUS_SKIPPED
	case domain.ScheduleDayStatusRescheduled:
		return coachingmsg.ScheduleDayStatus_SCHEDULE_DAY_STATUS_RESCHEDULED
	default:
		return coachingmsg.ScheduleDayStatus_SCHEDULE_DAY_STATUS_UNSPECIFIED
	}
}

func toProtoDailyPlanStatus(value domain.DailyPlanStatus) coachingmsg.DailyPlanStatus {
	switch value {
	case domain.DailyPlanStatusGenerated:
		return coachingmsg.DailyPlanStatus_DAILY_PLAN_STATUS_GENERATED
	case domain.DailyPlanStatusUsed:
		return coachingmsg.DailyPlanStatus_DAILY_PLAN_STATUS_USED
	case domain.DailyPlanStatusReplaced:
		return coachingmsg.DailyPlanStatus_DAILY_PLAN_STATUS_REPLACED
	default:
		return coachingmsg.DailyPlanStatus_DAILY_PLAN_STATUS_UNSPECIFIED
	}
}

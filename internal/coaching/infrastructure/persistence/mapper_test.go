package persistence_test

import (
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/coaching/domain"
	"github.com/viethung213/gym-companion/internal/coaching/infrastructure/persistence"
)

func TestMappers(t *testing.T) {
	now := time.Now().UTC()
	startDate := now
	endDate := now.AddDate(0, 0, 28)

	t.Run("WorkoutRoadmap Mapper", func(t *testing.T) {
		roadmap, _ := domain.NewWorkoutRoadmap("rdp_1", "usr_100", startDate, endDate)
		model := persistence.RoadmapToPersistence(roadmap)
		if model.ID != "rdp_1" || model.UserID != "usr_100" {
			t.Errorf("unexpected model data: %+v", model)
		}

		domainRoadmap := persistence.RoadmapToDomain(model)
		if domainRoadmap.ID() != "rdp_1" || domainRoadmap.UserID() != "usr_100" {
			t.Errorf("unexpected domain data: %+v", domainRoadmap)
		}
	})

	t.Run("WeeklySchedule Mapper", func(t *testing.T) {
		days := make([]domain.ScheduleDay, 7)
		for i := 0; i < 7; i++ {
			st := domain.WorkoutDayStatusTraining
			if i == 6 {
				st = domain.WorkoutDayStatusRest
			}
			days[i] = domain.NewScheduleDay(now.AddDate(0, 0, i), "Day", st, []string{"Chest"}, "")
		}
		schedule, _ := domain.NewWeeklySchedule("ws_1", "rdp_1", "usr_100", 1, startDate, endDate, "PPL", days)

		model, err := persistence.WeeklyScheduleToPersistence(schedule)
		if err != nil {
			t.Fatalf("failed to convert schedule to persistence: %v", err)
		}

		domainSchedule, err := persistence.WeeklyScheduleToDomain(model)
		if err != nil {
			t.Fatalf("failed to convert model to domain: %v", err)
		}

		if domainSchedule.ID() != "ws_1" || domainSchedule.WeekNumber() != 1 {
			t.Errorf("unexpected domain schedule: %+v", domainSchedule)
		}
	})

	t.Run("DailyWorkoutPlan Mapper", func(t *testing.T) {
		exs := []domain.PrescribedExercise{
			domain.NewPrescribedExercise("ex_1", "Bench", 3, 10, 60, 0, "", 90, 120, 7.5),
		}
		prescription := domain.NewWorkoutPrescription(nil, exs, nil)
		plan, _ := domain.NewDailyWorkoutPlan("dwp_1", "usr_100", "rdp_1", "ws_1", now, domain.DailyPlanStatusActive, prescription, "Reasoning", "")

		model, err := persistence.DailyWorkoutPlanToPersistence(plan)
		if err != nil {
			t.Fatalf("failed to convert daily plan to persistence: %v", err)
		}

		domainPlan, err := persistence.DailyWorkoutPlanToDomain(model)
		if err != nil {
			t.Fatalf("failed to convert model to domain plan: %v", err)
		}

		if domainPlan.ID() != "dwp_1" || len(domainPlan.Prescription().MainExercises()) != 1 {
			t.Errorf("unexpected domain plan: %+v", domainPlan)
		}
	})
}

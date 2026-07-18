//go:build integration

package persistence

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/coaching/domain"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestPostgresRepositoryPersistsPlanningFlowAndOutbox(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is required")
	}
	db, err := gorm.Open(postgres.Open(databaseURL), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	ctx := context.Background()
	if err := prepareCoachingSchema(ctx, db); err != nil {
		t.Fatalf("prepare coaching schema: %v", err)
	}
	repository := NewPostgresRepository(db)
	startDate := time.Date(2026, time.July, 20, 0, 0, 0, 0, time.UTC)
	input := domain.PlanningInput{
		ProfileSnapshotID:   "profile-snapshot-1",
		Goal:                domain.PlanningGoalGeneralFitness,
		ExperienceLevel:     domain.ExperienceLevelBeginner,
		TrainingDaysPerWeek: 1,
		PreferredWeekdays:   []time.Weekday{time.Monday},
		MaxSessionMinutes:   45,
		EquipmentIDs:        []string{"bodyweight"},
		Timezone:            "Asia/Ho_Chi_Minh",
		StartDate:           startDate,
	}
	roadmap, err := domain.NewWorkoutRoadmap("roadmap-1", "user-1", &input, "rules-v1")
	if err != nil {
		t.Fatalf("new roadmap: %v", err)
	}
	days, err := (domain.SchedulePlanner{}).PlanWeek(&input, 1)
	if err != nil {
		t.Fatalf("plan week: %v", err)
	}
	schedule, err := domain.NewWeeklySchedule("schedule-1", roadmap.ID, roadmap.UserID, 1, days)
	if err != nil {
		t.Fatalf("new schedule: %v", err)
	}
	if err := repository.CreateRoadmapWithSchedule(ctx, roadmap, schedule, []domain.Event{
		testEvent("roadmap-1", "roadmapInitiated", startDate),
		testEvent("schedule-1", "weeklyScheduleGenerated", startDate),
	}); err != nil {
		t.Fatalf("save roadmap and schedule: %v", err)
	}

	plan := domain.NewDailyWorkoutPlan(
		"plan-1",
		roadmap.UserID,
		roadmap.ID,
		schedule.ID,
		startDate,
		[]domain.PrescribedExercise{{ExerciseID: "push-up", Sets: 2, Reps: 12, RestSeconds: 60}},
		[]domain.PlannedActivity{{Name: "Warm-up", DurationSeconds: 300}},
		[]domain.PlannedActivity{{Name: "Cool-down", DurationSeconds: 300}},
		startDate.Add(time.Hour),
	)
	if err := schedule.AttachDailyPlan(startDate, plan.ID); err != nil {
		t.Fatalf("attach plan: %v", err)
	}
	if err := repository.SaveDailyPlan(
		ctx,
		schedule,
		plan,
		eventPointer(testEvent("plan-1", "dailyWorkoutPlanGenerated", startDate)),
	); err != nil {
		t.Fatalf("save daily plan: %v", err)
	}

	loaded, err := repository.FindDailyPlan(ctx, roadmap.UserID, plan.ID)
	if err != nil {
		t.Fatalf("find daily plan: %v", err)
	}
	if len(loaded.WarmUpItems) != 1 || loaded.Exercises[0].ExerciseID != "push-up" {
		t.Fatalf("unexpected loaded daily plan: %#v", loaded)
	}
	loadedSchedule, err := repository.FindSchedule(ctx, roadmap.UserID, schedule.ID)
	if err != nil {
		t.Fatalf("find schedule: %v", err)
	}
	if loadedSchedule.Days[0].DailyPlanID != plan.ID {
		t.Fatal("daily plan reference was not saved atomically with the plan")
	}
	records, err := repository.FetchUnpublished(ctx, 10)
	if err != nil {
		t.Fatalf("fetch outbox: %v", err)
	}
	if len(records) != 3 {
		t.Fatalf("len(outbox) = %d, want 3", len(records))
	}
}

func eventPointer(event domain.Event) *domain.Event {
	return &event
}

func testEvent(id string, eventType string, eventTime time.Time) domain.Event {
	return domain.Event{
		ID:           id,
		Type:         "contracts.coaching." + eventType,
		Source:       "/coaching",
		Subject:      id,
		PartitionKey: "user-1",
		Time:         eventTime,
		Data:         map[string]any{"userId": "user-1"},
	}
}

func prepareCoachingSchema(ctx context.Context, db *gorm.DB) error {
	statements := []string{
		`CREATE SCHEMA IF NOT EXISTS coaching`,
		`DROP TABLE IF EXISTS coaching.daily_workout_plans`,
		`DROP TABLE IF EXISTS coaching.weekly_schedules`,
		`DROP TABLE IF EXISTS coaching.workout_roadmaps`,
		`DROP TABLE IF EXISTS coaching.outbox_events`,
		`CREATE TABLE coaching.workout_roadmaps (
			id VARCHAR(255) PRIMARY KEY, user_id VARCHAR(255) NOT NULL,
			status VARCHAR(32) NOT NULL, start_date DATE NOT NULL, end_date DATE NOT NULL,
			planning_input JSONB NOT NULL, planner_version VARCHAR(64) NOT NULL
		)`,
		`CREATE TABLE coaching.weekly_schedules (
			id VARCHAR(255) PRIMARY KEY, roadmap_id VARCHAR(255) NOT NULL REFERENCES coaching.workout_roadmaps(id),
			user_id VARCHAR(255) NOT NULL, week_number INTEGER NOT NULL,
			start_date DATE NOT NULL, end_date DATE NOT NULL, schedule_days JSONB NOT NULL,
			UNIQUE (roadmap_id, week_number)
		)`,
		`CREATE TABLE coaching.daily_workout_plans (
			id VARCHAR(255) PRIMARY KEY, user_id VARCHAR(255) NOT NULL,
			roadmap_id VARCHAR(255) NOT NULL REFERENCES coaching.workout_roadmaps(id),
			weekly_schedule_id VARCHAR(255) NOT NULL REFERENCES coaching.weekly_schedules(id),
			scheduled_date DATE NOT NULL, status VARCHAR(32) NOT NULL, exercises JSONB NOT NULL,
			warm_up_items JSONB NOT NULL, cool_down_items JSONB NOT NULL, generated_at TIMESTAMPTZ NOT NULL,
			UNIQUE (weekly_schedule_id, scheduled_date)
		)`,
		`CREATE TABLE coaching.outbox_events (
			id VARCHAR(255) PRIMARY KEY, event_type VARCHAR(255) NOT NULL, source VARCHAR(255) NOT NULL,
			subject VARCHAR(255) NOT NULL, partition_key VARCHAR(255) NOT NULL,
			event_time TIMESTAMPTZ NOT NULL, data JSONB NOT NULL,
			published BOOLEAN NOT NULL DEFAULT FALSE, published_at TIMESTAMPTZ
		)`,
	}
	for _, statement := range statements {
		if err := db.WithContext(ctx).Exec(statement).Error; err != nil {
			return err
		}
	}
	return nil
}

package athlete_test

import (
	"context"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/coaching/application/athlete"
	"github.com/viethung213/gym-companion/internal/coaching/application/port"
	"github.com/viethung213/gym-companion/internal/coaching/domain/service"
	"github.com/viethung213/gym-companion/internal/coaching/domain/taxonomy"
)

type stubProfiles struct{ p port.Profile }

func (s stubProfiles) GetProfile(context.Context, string) (port.Profile, error) { return s.p, nil }

// Empty fields get coaching's documented assumptions.
func TestWithProfileDefaults_FillsEmptyFields(t *testing.T) {
	got, err := athlete.WithProfileDefaults(stubProfiles{}).GetProfile(context.Background(), "u")
	if err != nil {
		t.Fatal(err)
	}
	if got.WeightKg != athlete.DefaultBodyWeightKg || got.PrimaryGoal != athlete.DefaultPrimaryGoal {
		t.Errorf("weight %v goal %q, want the defaults", got.WeightKg, got.PrimaryGoal)
	}
	if len(got.AvailableEquipment) != len(service.DefaultGymEquipment()) {
		t.Errorf("equipment %v, want a regular gym", got.AvailableEquipment)
	}
	if len(got.AvailableSlots) != len(athlete.DefaultTrainingSlots()) {
		t.Errorf("slots %v, want the default week", got.AvailableSlots)
	}
}

// What the athlete set is never replaced.
func TestWithProfileDefaults_KeepsWhatIsSet(t *testing.T) {
	set := port.Profile{
		WeightKg:           55,
		PrimaryGoal:        "fat_loss",
		AvailableEquipment: []taxonomy.Equipment{taxonomy.EquipmentDumbbell},
		AvailableSlots:     []port.Slot{{DayOfWeek: time.Sunday, StartTime: "09:00", EndTime: "10:00"}},
	}
	got, err := athlete.WithProfileDefaults(stubProfiles{p: set}).GetProfile(context.Background(), "u")
	if err != nil {
		t.Fatal(err)
	}
	if got.WeightKg != 55 || got.PrimaryGoal != "fat_loss" || len(got.AvailableEquipment) != 1 || len(got.AvailableSlots) != 1 {
		t.Errorf("got %+v, want the athlete's own values", got)
	}
}

// The baseline is the heaviest load ever recorded for the exercise.
func TestWeightBaselines(t *testing.T) {
	got := athlete.WeightBaselines([]port.PersonalRecordSummary{
		{ExerciseID: "squat", Weight: 100}, {ExerciseID: "squat", Weight: 120}, {ExerciseID: "plank"},
	})
	if got["squat"] != 120 {
		t.Errorf("squat = %v, want 120", got["squat"])
	}
	if _, ok := got["plank"]; ok {
		t.Error("an unloaded record became a baseline")
	}
}

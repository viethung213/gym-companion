package command_test

import (
	"context"
	"errors"
	"testing"
	"time"
	_ "time/tzdata"

	"github.com/viethung213/gym-companion/internal/nutrition/application/command"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/repository"
)

type stubProfileClient struct {
	metrics *repository.UserBiologicalMetrics
	err     error
}

func (s stubProfileClient) GetBiologicalMetrics(context.Context, string) (*repository.UserBiologicalMetrics, error) {
	return s.metrics, s.err
}

func TestResolveAthleteProfile_UsesProfile(t *testing.T) {
	got := command.ResolveAthleteProfile(context.Background(), stubProfileClient{metrics: &repository.UserBiologicalMetrics{
		WeightKg: 58, HeightCm: 162, Age: 31, Gender: "female", ActivityLevel: "VERY_ACTIVE", Timezone: "Asia/Ho_Chi_Minh",
	}}, "u-1")

	m := got.Metrics
	if m.WeightKg != 58 || m.HeightCm != 162 || m.Age != 31 || m.Gender != "FEMALE" || m.ActivityLevel != "VERY_ACTIVE" {
		t.Fatalf("metrics = %+v, want the profile's", m)
	}
	if got.Location.String() != "Asia/Ho_Chi_Minh" {
		t.Fatalf("location = %s, want Asia/Ho_Chi_Minh", got.Location)
	}
}

func TestResolveAthleteProfile_FallsBack(t *testing.T) {
	def := command.DefaultBiologicalMetrics()
	cases := map[string]repository.ProfileClient{
		"no client":   nil,
		"error":       stubProfileClient{err: errors.New("gateway down")},
		"not found":   stubProfileClient{},
		"empty field": stubProfileClient{metrics: &repository.UserBiologicalMetrics{Timezone: "Mars/Base"}},
	}
	for name, client := range cases {
		got := command.ResolveAthleteProfile(context.Background(), client, "u-1")
		if got.Metrics != def {
			t.Errorf("%s: metrics = %+v, want defaults", name, got.Metrics)
		}
		if got.Location != time.UTC {
			t.Errorf("%s: location = %s, want UTC", name, got.Location)
		}
	}
}

func TestAthleteProfile_TodayIsTheAthletesCalendarDay(t *testing.T) {
	ict, _ := time.LoadLocation("Asia/Ho_Chi_Minh")
	athlete := command.AthleteProfile{Location: ict}

	// 2026-11-24 20:00 UTC is already 2026-11-25 03:00 in Vietnam.
	got := athlete.Today(time.Date(2026, 11, 24, 20, 0, 0, 0, time.UTC))
	if want := time.Date(2026, 11, 25, 0, 0, 0, 0, time.UTC); !got.Equal(want) {
		t.Fatalf("Today = %s, want %s", got, want)
	}
}

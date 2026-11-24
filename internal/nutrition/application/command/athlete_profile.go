package command

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/viethung213/gym-companion/internal/nutrition/domain/repository"
	"github.com/viethung213/gym-companion/internal/nutrition/domain/service"
)

// AthleteProfile is what daily-menu generation needs about the athlete.
type AthleteProfile struct {
	Metrics service.BiologicalMetrics
	// Location is the athlete's zone: plan_date and meal times are wall clock in it.
	Location *time.Location
}

// Today is the athlete's calendar day at now, encoded the way plan_date (a
// DATE column) is stored: midnight UTC of that day. Passing a zoned instant
// instead lets the database cast it to the UTC day, a day off east of UTC.
func (a AthleteProfile) Today(now time.Time) time.Time {
	loc := a.Location
	if loc == nil {
		loc = time.UTC
	}
	y, m, d := now.In(loc).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// DefaultBiologicalMetrics fills in when the profile is missing or incomplete.
func DefaultBiologicalMetrics() service.BiologicalMetrics {
	return service.BiologicalMetrics{
		WeightKg:      70.0,
		HeightCm:      170.0,
		Age:           25,
		Gender:        "MALE",
		ActivityLevel: "MODERATELY_ACTIVE",
	}
}

// ResolveAthleteProfile reads the athlete's metrics and zone from the Profile
// context. It never fails: an unreachable profile or a missing field falls back
// to DefaultBiologicalMetrics and UTC, so a menu can always be generated.
func ResolveAthleteProfile(ctx context.Context, client repository.ProfileClient, userID string) AthleteProfile {
	out := AthleteProfile{Metrics: DefaultBiologicalMetrics(), Location: time.UTC}
	if client == nil {
		slog.WarnContext(ctx, "profileClient not configured, using fallback metrics", slog.String("user_id", userID))
		return out
	}

	pm, err := client.GetBiologicalMetrics(ctx, userID)
	if err != nil {
		slog.WarnContext(ctx, "Could not fetch profile metrics, using fallback",
			slog.String("user_id", userID), slog.String("error", err.Error()))
		return out
	}
	if pm == nil {
		slog.WarnContext(ctx, "User profile not found, using fallback", slog.String("user_id", userID))
		return out
	}

	if pm.WeightKg > 0 {
		out.Metrics.WeightKg = pm.WeightKg
	}
	if pm.HeightCm > 0 {
		out.Metrics.HeightCm = pm.HeightCm
	}
	if pm.Age > 0 {
		out.Metrics.Age = pm.Age
	}
	if g := strings.ToUpper(strings.TrimSpace(pm.Gender)); g != "" {
		out.Metrics.Gender = g
	}
	if pm.ActivityLevel != "" {
		out.Metrics.ActivityLevel = pm.ActivityLevel
	}
	out.Location = LoadLocation(pm.Timezone)
	return out
}

// LoadLocation resolves an IANA zone name, falling back to UTC.
func LoadLocation(name string) *time.Location {
	if name = strings.TrimSpace(name); name != "" {
		if loc, err := time.LoadLocation(name); err == nil {
			return loc
		}
	}
	return time.UTC
}

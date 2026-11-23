package adk

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/coaching/application/port"
)

func TestBuildContextNode_EncodesRecentSessionsAndTrimsMusclePRs(t *testing.T) {
	now := time.Now().UTC()

	// Seed 5 sessions: 4 within 5 days, 1 at 7 days ago (must be excluded by 5-day window)
	sessions := []port.WorkoutSession{
		{SessionID: "s1", CompletedAt: now.Add(-24 * time.Hour), TotalSets: 16, AverageRPE: 8.5, Aborted: false},
		{SessionID: "s2", CompletedAt: now.Add(-48 * time.Hour), TotalSets: 12, AverageRPE: 9.5, Aborted: true},
		{SessionID: "s3", CompletedAt: now.Add(-72 * time.Hour), TotalSets: 18, AverageRPE: 7.0, Aborted: false},
		{SessionID: "s4", CompletedAt: now.Add(-96 * time.Hour), TotalSets: 10, AverageRPE: 5.5, Aborted: false},
		{SessionID: "s5-old", CompletedAt: now.Add(-7 * 24 * time.Hour), TotalSets: 20, AverageRPE: 8.0, Aborted: false},
	}

	// Seed 11 exercises (6 CHEST, 5 BACK) and 11 PRs
	var exercises []port.Exercise
	var prs []port.PersonalRecordSummary
	for i := 1; i <= 6; i++ {
		id := fmt.Sprintf("chest-%d", i)
		exercises = append(exercises, port.Exercise{
			ExerciseID:  id,
			Name:        fmt.Sprintf("Chest Exercise %d", i),
			MuscleGroup: "CHEST",
			Equipment:   "BARBELL",
			Difficulty:  "INTERMEDIATE",
		})
		prs = append(prs, port.PersonalRecordSummary{
			ExerciseID: id,
			Weight:     float64(50 + i*5),
			Reps:       8,
			OneRepMax:  float64(60 + i*6),
			AchievedAt: now.Add(-time.Duration(i) * 24 * time.Hour),
		})
	}
	for i := 1; i <= 5; i++ {
		id := fmt.Sprintf("back-%d", i)
		exercises = append(exercises, port.Exercise{
			ExerciseID:  id,
			Name:        fmt.Sprintf("Back Exercise %d", i),
			MuscleGroup: "BACK",
			Equipment:   "CABLE",
			Difficulty:  "BEGINNER",
		})
		prs = append(prs, port.PersonalRecordSummary{
			ExerciseID: id,
			Weight:     float64(40 + i*5),
			Reps:       10,
			OneRepMax:  float64(50 + i*6),
			AchievedAt: now.Add(-time.Duration(i) * 24 * time.Hour),
		})
	}

	since := now.AddDate(0, 0, -5)
	encoded := make([]string, 0, len(sessions))
	for i := range sessions {
		if sessions[i].CompletedAt.Before(since) {
			continue
		}
		encoded = append(encoded, formatRecentSession(&sessions[i], now))
	}
	if len(encoded) != 4 {
		t.Fatalf("expected 4 sessions within 5 days (excluding 7-day-old session), got %d", len(encoded))
	}

	// Verify buildMusclePRs trims 6 CHEST -> 3 and 5 BACK -> 3
	musclePRs := buildMusclePRs(exercises, prs, 3)
	if len(musclePRs["CHEST"]) != 3 {
		t.Fatalf("expected 3 CHEST PRs, got %d", len(musclePRs["CHEST"]))
	}
	if len(musclePRs["BACK"]) != 3 {
		t.Fatalf("expected 3 BACK PRs, got %d", len(musclePRs["BACK"]))
	}

	sample := CoachInput{
		Flow:           "GENERATE_ROADMAP",
		RecentSessions: encoded,
		MusclePRs:      musclePRs,
	}
	b, _ := json.MarshalIndent(sample, "", "  ")
	t.Logf("Sample CoachInput:\n%s", string(b))
}

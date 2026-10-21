package event_test

import (
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/solo_match/domain/event"
)

func TestDomainEvents(t *testing.T) {
	now := time.Now().UTC()

	eStarted := &event.SoloMatchStarted{
		MatchID:       "m_1",
		ExerciseID:    "ex_1",
		ExerciseName:  "Push Up",
		ChallengeType: "REPS",
		Player1ID:     "u_1",
		Player2ID:     "u_2",
		BotUserID:     "",
		StartedAt:     now,
	}
	if eStarted.EventName() != "contracts.core.solo_match.v1.soloMatchStarted" {
		t.Errorf("EventName got %v", eStarted.EventName())
	}
	if !eStarted.OccurredAt().Equal(now) {
		t.Errorf("OccurredAt got %v", eStarted.OccurredAt())
	}

	eCompleted := &event.SoloMatchCompleted{
		MatchID:       "m_1",
		ExerciseID:    "ex_1",
		ChallengeType: "REPS",
		WinnerID:      "u_1",
		IsDraw:        false,
		Player1ID:     "u_1",
		Player1Score:  30,
		Player2ID:     "u_2",
		Player2Score:  20,
		BotUserID:     "",
		CompletedAt:   now,
	}
	if eCompleted.EventName() != "contracts.core.solo_match.v1.soloMatchCompleted" {
		t.Errorf("EventName got %v", eCompleted.EventName())
	}
	if !eCompleted.OccurredAt().Equal(now) {
		t.Errorf("OccurredAt got %v", eCompleted.OccurredAt())
	}

	eForfeited := &event.SoloMatchForfeited{
		MatchID:       "m_1",
		ForfeitedByID: "u_2",
		WinnerID:      "u_1",
		Reason:        "timeout",
		ForfeitedAt:   now,
	}
	if eForfeited.EventName() != "contracts.core.solo_match.v1.soloMatchForfeited" {
		t.Errorf("EventName got %v", eForfeited.EventName())
	}
	if !eForfeited.OccurredAt().Equal(now) {
		t.Errorf("OccurredAt got %v", eForfeited.OccurredAt())
	}
}

package event_test

import (
	"encoding/json"
	"testing"
	"time"

	domainEvent "github.com/viethung213/gym-companion/internal/solo_match/domain/event"
	infraEvent "github.com/viethung213/gym-companion/internal/solo_match/infrastructure/event"
)

func TestPackCloudEvent_SoloMatchStarted(t *testing.T) {
	now := time.Now().UTC()
	evt := &domainEvent.SoloMatchStarted{
		MatchID:       "match_1",
		ExerciseID:    "ex_pushup",
		ExerciseName:  "Push Up",
		ChallengeType: "REPS",
		Player1ID:     "user_1",
		Player2ID:     "user_2",
		BotUserID:     "",
		StartedAt:     now,
	}

	eventID, eventType, payload, partitionKey, err := infraEvent.PackCloudEvent(evt)
	if err != nil {
		t.Fatalf("PackCloudEvent failed: %v", err)
	}

	if eventID == "" {
		t.Errorf("expected non-empty eventID")
	}
	if eventType != "contracts.core.solo_match.v1.soloMatchStarted" {
		t.Errorf("got eventType %s, want contracts.core.solo_match.v1.soloMatchStarted", eventType)
	}
	if partitionKey != "user_1" {
		t.Errorf("got partitionKey %s, want user_1", partitionKey)
	}

	var envelope infraEvent.CloudEventEnvelope
	if err := json.Unmarshal(payload, &envelope); err != nil {
		t.Fatalf("unmarshal payload failed: %v", err)
	}

	if envelope.SpecVersion != "1.0" {
		t.Errorf("got specversion %s, want 1.0", envelope.SpecVersion)
	}
	if envelope.Source != "services/solo-match-service" {
		t.Errorf("got source %s, want services/solo-match-service", envelope.Source)
	}
	if envelope.DataContentType != "application/json" {
		t.Errorf("got datacontenttype %s, want application/json", envelope.DataContentType)
	}
}

func TestPackCloudEvent_SoloMatchCompleted(t *testing.T) {
	now := time.Now().UTC()
	evt := &domainEvent.SoloMatchCompleted{
		MatchID:       "match_1",
		ExerciseID:    "ex_pushup",
		ChallengeType: "REPS",
		WinnerID:      "usr_bot_001",
		IsDraw:        false,
		Player1ID:     "user_1",
		Player1Score:  30,
		Player2ID:     "usr_bot_001",
		Player2Score:  35,
		BotUserID:     "usr_bot_001",
		CompletedAt:   now,
	}

	eventID, eventType, payload, partitionKey, err := infraEvent.PackCloudEvent(evt)
	if err != nil {
		t.Fatalf("PackCloudEvent failed: %v", err)
	}

	if eventID == "" {
		t.Errorf("expected non-empty eventID")
	}
	if eventType != "contracts.core.solo_match.v1.soloMatchCompleted" {
		t.Errorf("unexpected eventType: %s", eventType)
	}
	if partitionKey != "usr_bot_001" {
		t.Errorf("got partitionKey %s, want usr_bot_001", partitionKey)
	}

	var envelope infraEvent.CloudEventEnvelope
	if err := json.Unmarshal(payload, &envelope); err != nil {
		t.Fatalf("unmarshal payload failed: %v", err)
	}
	if envelope.Type != "contracts.core.solo_match.v1.soloMatchCompleted" {
		t.Errorf("unexpected envelope type: %s", envelope.Type)
	}

	var dataMap map[string]any
	if err := json.Unmarshal(envelope.Data, &dataMap); err != nil {
		t.Fatalf("unmarshal envelope data: %v", err)
	}
	if dataMap["botUserId"] != "usr_bot_001" {
		t.Errorf("expected botUserId usr_bot_001, got %v", dataMap["botUserId"])
	}
}

func TestPackCloudEvent_NilEvent(t *testing.T) {
	_, _, _, _, err := infraEvent.PackCloudEvent(nil)
	if err == nil {
		t.Errorf("expected error for nil event, got nil")
	}
}

package event

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	domainEvent "github.com/viethung213/gym-companion/internal/solo_match/domain/event"
)

// CloudEventEnvelope tuân thủ đặc tả CloudEvents 1.0 JSON format
type CloudEventEnvelope struct {
	SpecVersion     string          `json:"specversion"`
	ID              string          `json:"id"`
	Source          string          `json:"source"`
	Type            string          `json:"type"`
	Time            string          `json:"time"`
	DataContentType string          `json:"datacontenttype"`
	Data            json.RawMessage `json:"data"`
}

// PackCloudEvent đóng gói một Domain Event thành CloudEvents 1.0 JSON kèm partition key
func PackCloudEvent(ev domainEvent.DomainEvent) (eventID string, eventType string, payload []byte, partitionKey string, err error) {
	if ev == nil {
		return "", "", nil, "", fmt.Errorf("nil domain event")
	}

	eventID = uuid.New().String()
	eventType = ev.EventName()
	occurredAt := ev.OccurredAt()
	if occurredAt.IsZero() {
		occurredAt = time.Now().UTC()
	}

	var dataBytes []byte
	switch e := ev.(type) {
	case *domainEvent.SoloMatchStarted:
		partitionKey = e.Player1ID
		dataBytes, err = json.Marshal(map[string]any{
			"matchId":       e.MatchID,
			"exerciseId":    e.ExerciseID,
			"exerciseName":  e.ExerciseName,
			"challengeType": e.ChallengeType,
			"player1Id":     e.Player1ID,
			"player2Id":     e.Player2ID,
			"botUserId":     e.BotUserID,
			"startedAt":     e.StartedAt.Format(time.RFC3339),
		})
	case *domainEvent.SoloMatchCompleted:
		if e.WinnerID != "" {
			partitionKey = e.WinnerID
		} else {
			partitionKey = e.Player1ID
		}
		dataBytes, err = json.Marshal(map[string]any{
			"matchId":       e.MatchID,
			"exerciseId":    e.ExerciseID,
			"challengeType": e.ChallengeType,
			"winnerId":      e.WinnerID,
			"isDraw":        e.IsDraw,
			"player1Id":     e.Player1ID,
			"player1Score":  e.Player1Score,
			"player2Id":     e.Player2ID,
			"player2Score":  e.Player2Score,
			"botUserId":     e.BotUserID,
			"completedAt":   e.CompletedAt.Format(time.RFC3339),
		})
	case *domainEvent.SoloMatchForfeited:
		partitionKey = e.ForfeitedByID
		dataBytes, err = json.Marshal(map[string]any{
			"matchId":       e.MatchID,
			"forfeitedById": e.ForfeitedByID,
			"winnerId":      e.WinnerID,
			"botUserId":     e.BotUserID,
			"reason":        e.Reason,
			"forfeitedAt":   e.ForfeitedAt.Format(time.RFC3339),
		})
	default:
		// Fallback mặc định
		partitionKey = "default"
		dataBytes, err = json.Marshal(ev)
	}

	if err != nil {
		return "", "", nil, "", fmt.Errorf("marshal event data: %w", err)
	}

	envelope := CloudEventEnvelope{
		SpecVersion:     "1.0",
		ID:              eventID,
		Source:          "services/solo-match-service",
		Type:            eventType,
		Time:            occurredAt.Format(time.RFC3339),
		DataContentType: "application/json",
		Data:            json.RawMessage(dataBytes),
	}

	payload, err = json.Marshal(envelope)
	if err != nil {
		return "", "", nil, "", fmt.Errorf("marshal cloudevent envelope: %w", err)
	}

	return eventID, eventType, payload, partitionKey, nil
}

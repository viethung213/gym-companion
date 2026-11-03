package event

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	solomatchv1event "github.com/viethung213/gym-companion/internal/gen/go/contracts/core/solo_match/v1/event"
	domainEvent "github.com/viethung213/gym-companion/internal/solo_match/domain/event"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"
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
func PackCloudEvent(ev domainEvent.DomainEvent) (eventID, eventType string, payload []byte, partitionKey string, err error) {
	if ev == nil {
		return "", "", nil, "", errors.New("nil domain event")
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
		protoMsg := &solomatchv1event.SoloMatchStarted{
			MatchId:       e.MatchID,
			ExerciseId:    e.ExerciseID,
			ExerciseName:  e.ExerciseName,
			ChallengeType: e.ChallengeType,
			Player1Id:     e.Player1ID,
			Player2Id:     e.Player2ID,
			BotUserId:     e.BotUserID,
			StartedAt:     timestamppb.New(e.StartedAt),
		}
		dataBytes, err = protojson.Marshal(protoMsg)
	case *domainEvent.SoloMatchCompleted:
		if e.WinnerID != "" {
			partitionKey = e.WinnerID
		} else {
			partitionKey = e.Player1ID
		}
		protoMsg := &solomatchv1event.SoloMatchCompleted{
			MatchId:       e.MatchID,
			ExerciseId:    e.ExerciseID,
			ChallengeType: e.ChallengeType,
			WinnerId:      e.WinnerID,
			IsDraw:        e.IsDraw,
			Player1Id:     e.Player1ID,
			Player1Score:  float32(e.Player1Score),
			Player2Id:     e.Player2ID,
			Player2Score:  float32(e.Player2Score),
			BotUserId:     e.BotUserID,
			CompletedAt:   timestamppb.New(e.CompletedAt),
		}
		dataBytes, err = protojson.Marshal(protoMsg)
	case *domainEvent.SoloMatchForfeited:
		partitionKey = e.ForfeitedByID
		protoMsg := &solomatchv1event.SoloMatchForfeited{
			MatchId:       e.MatchID,
			ForfeitedById: e.ForfeitedByID,
			WinnerId:      e.WinnerID,
			BotUserId:     e.BotUserID,
			Reason:        e.Reason,
			ForfeitedAt:   timestamppb.New(e.ForfeitedAt),
		}
		dataBytes, err = protojson.Marshal(protoMsg)
	case *domainEvent.SoloMatchCancelled:
		partitionKey = e.MatchID
		protoMsg := &solomatchv1event.SoloMatchCancelled{
			MatchId:     e.MatchID,
			Reason:      e.Reason,
			CancelledAt: timestamppb.New(e.CancelledAt),
		}
		dataBytes, err = protojson.Marshal(protoMsg)
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

package consumer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/viethung213/gym-companion/internal/solo_match/application/port"
	"github.com/viethung213/gym-companion/internal/solo_match/domain/derror"
	"github.com/viethung213/gym-companion/internal/solo_match/domain/repository"
)

type soloMatchRoomReadyData struct {
	MatchID   string `json:"matchId"`
	StartedAt string `json:"startedAt,omitempty"`
}

// SoloMatchRoomReadyConsumer listens to SoloMatchRoomReady events from workout_execution
// and activates the match from PENDING_READY to IN_PROGRESS.
type SoloMatchRoomReadyConsumer struct {
	matchRepo     repository.SoloMatchRepository
	outboxLogRepo port.OutboxLogRepository
}

// NewSoloMatchRoomReadyConsumer constructs a new SoloMatchRoomReadyConsumer.
func NewSoloMatchRoomReadyConsumer(
	matchRepo repository.SoloMatchRepository,
	outboxLogRepo port.OutboxLogRepository,
) *SoloMatchRoomReadyConsumer {
	return &SoloMatchRoomReadyConsumer{
		matchRepo:     matchRepo,
		outboxLogRepo: outboxLogRepo,
	}
}

func isSoloMatchRoomReadyEvent(eventType string) bool {
	return strings.HasSuffix(eventType, "soloMatchRoomReady") ||
		strings.HasSuffix(eventType, "SoloMatchRoomReady") ||
		eventType == "contracts.core.workout_execution.v1.soloMatchRoomReady"
}

// HandleMessage parses SoloMatchRoomReady CloudEvent and transitions match to IN_PROGRESS.
// Enforces Terminal State Rule: If match was already CANCELLED, it is NEVER reactivated.
func (c *SoloMatchRoomReadyConsumer) HandleMessage(ctx context.Context, rawPayload []byte) error {
	var env cloudEventEnvelope
	if err := json.Unmarshal(rawPayload, &env); err != nil {
		return fmt.Errorf("solo match room ready consumer: unmarshal cloudevent: %w", err)
	}

	if !isSoloMatchRoomReadyEvent(env.Type) {
		return nil
	}

	if c.outboxLogRepo != nil && env.ID != "" {
		processed, err := c.outboxLogRepo.IsProcessed(ctx, env.ID)
		if err != nil {
			log.Printf("[SoloMatch] OutboxLog IsProcessed error: %v", err)
		} else if processed {
			log.Printf("[SoloMatch] Event %s already processed, skipping (Idempotent)", env.ID)
			return nil
		}
	}

	var data soloMatchRoomReadyData
	if err := json.Unmarshal(env.Data, &data); err != nil {
		return fmt.Errorf("solo match room ready consumer: unmarshal data: %w", err)
	}

	if data.MatchID == "" {
		return nil
	}

	startedAt := time.Now().UTC()
	if data.StartedAt != "" {
		if parsed, err := time.Parse(time.RFC3339, data.StartedAt); err == nil {
			startedAt = parsed
		}
	}

	var procErr error
	match, err := c.matchRepo.FindByID(ctx, data.MatchID)
	if err != nil {
		procErr = fmt.Errorf("find match by id: %w", err)
	} else if match == nil {
		log.Printf("[SoloMatch] Match %s not found for SoloMatchRoomReady event, skipping", data.MatchID)
	} else if actErr := match.ActivateRoom(startedAt); actErr != nil {
		if errors.Is(actErr, derror.ErrMatchNotEligibleForActivation) {
			log.Printf("[SoloMatch] Match %s is not eligible for activation (current status: %s), skipping late room ready event", match.ID(), match.Status())
		} else {
			procErr = fmt.Errorf("activate room: %w", actErr)
		}
	} else if saveErr := c.matchRepo.Save(ctx, match); saveErr != nil {
		procErr = fmt.Errorf("save activated match: %w", saveErr)
	} else {
		log.Printf("[SoloMatch] Match %s activated: started_at=%v", match.ID(), match.StartedAt())
	}

	if c.outboxLogRepo != nil && env.ID != "" {
		status := "PROCESSED"
		errMsg := ""
		if procErr != nil {
			status = "FAILED"
			errMsg = procErr.Error()
		}
		if saveErr := c.outboxLogRepo.Save(ctx, env.ID, env.Type, rawPayload, data.MatchID, status, errMsg); saveErr != nil {
			log.Printf("[SoloMatch] Failed to save outbox log for event %s: %v", env.ID, saveErr)
		}
	}

	return procErr
}

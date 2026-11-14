package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	gamificationv1event "github.com/viethung213/gym-companion/internal/gen/go/contracts/supporting/gamification/v1/event"
	"github.com/viethung213/gym-companion/internal/solo_match/application/command"
	"github.com/viethung213/gym-companion/internal/solo_match/application/port"
	"google.golang.org/protobuf/encoding/protojson"
)

type XpEarnedConsumer struct {
	handler       *command.SyncUserSnapshotHandler
	outboxLogRepo port.OutboxLogRepository
}

func NewXpEarnedConsumer(
	handler *command.SyncUserSnapshotHandler,
	outboxLogRepo port.OutboxLogRepository,
) *XpEarnedConsumer {
	return &XpEarnedConsumer{
		handler:       handler,
		outboxLogRepo: outboxLogRepo,
	}
}

func (c *XpEarnedConsumer) HandleMessage(ctx context.Context, rawPayload []byte) error {
	var env cloudEventEnvelope
	if err := json.Unmarshal(rawPayload, &env); err != nil {
		return fmt.Errorf("xp earned consumer: unmarshal cloudevent: %w", err)
	}

	if !strings.HasSuffix(env.Type, "XpEarned") &&
		!strings.HasSuffix(env.Type, "xpEarned") {
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

	var pb gamificationv1event.XpEarned
	unmarshaler := protojson.UnmarshalOptions{DiscardUnknown: true}
	if err := unmarshaler.Unmarshal(env.Data, &pb); err != nil {
		return fmt.Errorf("xp earned consumer: unmarshal data: %w", err)
	}

	userID := strings.TrimSpace(pb.GetUserId())
	if userID == "" {
		return nil
	}

	syncedAt := time.Now().UTC()
	if pb.GetEarnedAt() != nil && pb.GetEarnedAt().IsValid() {
		syncedAt = pb.GetEarnedAt().AsTime()
	}

	log.Printf("[SoloMatch] Syncing XpEarned snapshot: %s (xp=%d)", userID, pb.GetNewXp())

	err := c.handler.HandleXP(ctx, command.SyncUserXPCommand{
		UserID:   userID,
		XP:       pb.GetNewXp(),
		SyncedAt: syncedAt,
	})

	if c.outboxLogRepo != nil && env.ID != "" {
		status := "PROCESSED"
		errMsg := ""
		if err != nil {
			status = "FAILED"
			errMsg = err.Error()
		}
		if saveErr := c.outboxLogRepo.Save(ctx, env.ID, env.Type, rawPayload, userID, status, errMsg); saveErr != nil {
			log.Printf("[SoloMatch] Failed to save outbox log for event %s: %v", env.ID, saveErr)
		}
	}

	return err
}

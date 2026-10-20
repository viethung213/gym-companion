package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/viethung213/gym-companion/internal/solo_match/application/command"
	"github.com/viethung213/gym-companion/internal/solo_match/application/port"
)

type userIdentityUpdatedData struct {
	UserID    string `json:"userId"`
	FullName  string `json:"fullName"`
	AvatarURL string `json:"avatarUrl"`
	UpdatedAt string `json:"updatedAt"`
}

type UserIdentityUpdatedConsumer struct {
	handler       *command.SyncUserSnapshotHandler
	outboxLogRepo port.OutboxLogRepository
}

func NewUserIdentityUpdatedConsumer(
	handler *command.SyncUserSnapshotHandler,
	outboxLogRepo port.OutboxLogRepository,
) *UserIdentityUpdatedConsumer {
	return &UserIdentityUpdatedConsumer{
		handler:       handler,
		outboxLogRepo: outboxLogRepo,
	}
}

func (c *UserIdentityUpdatedConsumer) HandleMessage(ctx context.Context, rawPayload []byte) error {
	var env cloudEventEnvelope
	if err := json.Unmarshal(rawPayload, &env); err != nil {
		return fmt.Errorf("user identity updated consumer: unmarshal cloudevent: %w", err)
	}

	if !strings.HasSuffix(env.Type, "UserIdentityUpdated") &&
		!strings.HasSuffix(env.Type, "userIdentityUpdated") {
		return nil
	}

	// Idempotency check: Bỏ qua nếu event này đã được xử lý thành công trước đó
	if c.outboxLogRepo != nil && env.ID != "" {
		processed, err := c.outboxLogRepo.IsProcessed(ctx, env.ID)
		if err != nil {
			log.Printf("[SoloMatch] OutboxLog IsProcessed error: %v", err)
		} else if processed {
			log.Printf("[SoloMatch] Event %s already processed, skipping (Idempotent)", env.ID)
			return nil
		}
	}

	var data userIdentityUpdatedData
	if err := json.Unmarshal(env.Data, &data); err != nil {
		return fmt.Errorf("user identity updated consumer: unmarshal data: %w", err)
	}

	if data.UserID == "" {
		return nil
	}

	syncedAt := time.Now().UTC()
	if data.UpdatedAt != "" {
		if t, err := time.Parse(time.RFC3339, data.UpdatedAt); err == nil {
			syncedAt = t
		}
	}

	log.Printf("[SoloMatch] Syncing UserIdentityUpdated snapshot: %s (%s)", data.UserID, data.FullName)

	err := c.handler.Handle(ctx, command.SyncUserSnapshotCommand{
		UserID:      data.UserID,
		DisplayName: data.FullName,
		AvatarURL:   data.AvatarURL,
		SyncedAt:    syncedAt,
	})

	if c.outboxLogRepo != nil && env.ID != "" {
		if err != nil {
			_ = c.outboxLogRepo.Save(ctx, env.ID, env.Type, rawPayload, data.UserID, "FAILED", err.Error())
		} else {
			_ = c.outboxLogRepo.Save(ctx, env.ID, env.Type, rawPayload, data.UserID, "PROCESSED", "")
		}
	}

	return err
}

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

type userRegisteredData struct {
	UserID       string `json:"userId"`
	FullName     string `json:"fullName"`
	AvatarURL    string `json:"avatarUrl"`
	RegisteredAt string `json:"registeredAt"`
}

type UserRegisteredConsumer struct {
	handler       *command.SyncUserSnapshotHandler
	outboxLogRepo port.OutboxLogRepository
}

func NewUserRegisteredConsumer(
	handler *command.SyncUserSnapshotHandler,
	outboxLogRepo port.OutboxLogRepository,
) *UserRegisteredConsumer {
	return &UserRegisteredConsumer{
		handler:       handler,
		outboxLogRepo: outboxLogRepo,
	}
}

func (c *UserRegisteredConsumer) HandleMessage(ctx context.Context, rawPayload []byte) error {
	var env cloudEventEnvelope
	if err := json.Unmarshal(rawPayload, &env); err != nil {
		return fmt.Errorf("user registered consumer: unmarshal cloudevent: %w", err)
	}

	if !strings.HasSuffix(env.Type, "UserRegistered") &&
		!strings.HasSuffix(env.Type, "userRegistered") {
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

	var data userRegisteredData
	if err := json.Unmarshal(env.Data, &data); err != nil {
		return fmt.Errorf("user registered consumer: unmarshal data: %w", err)
	}

	if data.UserID == "" {
		return nil
	}

	syncedAt := time.Now().UTC()
	if data.RegisteredAt != "" {
		if t, err := time.Parse(time.RFC3339, data.RegisteredAt); err == nil {
			syncedAt = t
		}
	}

	log.Printf("[SoloMatch] Syncing new UserRegistered snapshot: %s (%s)", data.UserID, data.FullName)

	err := c.handler.Handle(ctx, command.SyncUserSnapshotCommand{
		UserID:      data.UserID,
		DisplayName: data.FullName,
		AvatarURL:   data.AvatarURL,
		SyncedAt:    syncedAt,
	})

	if c.outboxLogRepo != nil && env.ID != "" {
		status := "PROCESSED"
		errMsg := ""
		if err != nil {
			status = "FAILED"
			errMsg = err.Error()
		}
		if saveErr := c.outboxLogRepo.Save(ctx, env.ID, env.Type, rawPayload, data.UserID, status, errMsg); saveErr != nil {
			log.Printf("[SoloMatch] Failed to save outbox log for event %s: %v", env.ID, saveErr)
		}
	}

	return err
}

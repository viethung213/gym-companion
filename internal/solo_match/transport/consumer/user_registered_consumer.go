package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
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
			slog.WarnContext(ctx, "OutboxLog IsProcessed error", slog.String("event_id", env.ID), slog.String("error", err.Error()))
		} else if processed {
			slog.DebugContext(ctx, "Event already processed, skipping (Idempotent)", slog.String("event_id", env.ID))
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

	slog.InfoContext(ctx, "Syncing new UserRegistered snapshot", slog.String("user_id", data.UserID), slog.String("full_name", data.FullName))

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
			slog.WarnContext(ctx, "Failed to save outbox log for event", slog.String("event_id", env.ID), slog.String("error", saveErr.Error()))
		}
	}

	return err
}

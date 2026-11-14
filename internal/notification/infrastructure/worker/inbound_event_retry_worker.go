package worker

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/viethung213/gym-companion/internal/notification/application/command"
	"github.com/viethung213/gym-companion/internal/notification/application/port"
)

type InboundEventRetryWorker struct {
	outboxLogRepo   port.OutboxLogRepository
	sendPushHandler *command.SendPushNotificationHandler
	interval        time.Duration
}

type cloudEventEnvelope struct {
	ID     string          `json:"id"`
	Source string          `json:"source"`
	Type   string          `json:"type"`
	Time   string          `json:"time"`
	Data   json.RawMessage `json:"data"`
}

type defaultEventData struct {
	UserID  string `json:"userId"`
	Title   string `json:"title"`
	Message string `json:"message"`
	Body    string `json:"body"`
}

func NewInboundEventRetryWorker(
	outboxLogRepo port.OutboxLogRepository,
	sendPushHandler *command.SendPushNotificationHandler,
	interval time.Duration,
) *InboundEventRetryWorker {
	if interval <= 0 {
		interval = 10 * time.Second
	}
	return &InboundEventRetryWorker{
		outboxLogRepo:   outboxLogRepo,
		sendPushHandler: sendPushHandler,
		interval:        interval,
	}
}

func (w *InboundEventRetryWorker) Start(ctx context.Context) error {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	slog.InfoContext(ctx, "Started Notification InboundEventRetryWorker", slog.Duration("interval", w.interval))

	for {
		select {
		case <-ctx.Done():
			slog.InfoContext(ctx, "Stopping Notification InboundEventRetryWorker due to context cancellation")
			return nil
		case <-ticker.C:
			if err := w.processFailedLogs(ctx); err != nil {
				slog.ErrorContext(ctx, "Notification InboundEventRetryWorker processing error", slog.String("error", err.Error()))
			}
		}
	}
}

func (w *InboundEventRetryWorker) processFailedLogs(ctx context.Context) error {
	if w.outboxLogRepo == nil || w.sendPushHandler == nil {
		return nil
	}

	failedLogs, err := w.outboxLogRepo.FetchFailedLogs(ctx, 50)
	if err != nil {
		return err
	}

	if len(failedLogs) == 0 {
		return nil
	}

	slog.InfoContext(ctx, "Found failed inbound events to retry", slog.Int("count", len(failedLogs)))

	for _, logRec := range failedLogs {
		var env cloudEventEnvelope
		if err := json.Unmarshal(logRec.Payload, &env); err != nil {
			slog.WarnContext(ctx, "Unmarshal payload failed for inbound log", slog.String("log_id", logRec.ID), slog.String("error", err.Error()))
			_ = w.outboxLogRepo.UpdateLogStatus(ctx, logRec.ID, "FAILED_UNPARSEABLE", err.Error())
			continue
		}

		var payload defaultEventData
		if err := json.Unmarshal(env.Data, &payload); err != nil {
			slog.WarnContext(ctx, "Unmarshal event data failed for inbound log", slog.String("log_id", logRec.ID), slog.String("error", err.Error()))
			_ = w.outboxLogRepo.UpdateLogStatus(ctx, logRec.ID, "FAILED_UNPARSEABLE", err.Error())
			continue
		}

		userID := payload.UserID
		if userID == "" {
			userID = logRec.PartitionKey
		}

		title := payload.Title
		if title == "" {
			title = "Gym Companion Thông Báo"
		}
		body := payload.Body
		if body == "" {
			body = payload.Message
		}

		_, sendErr := w.sendPushHandler.Handle(ctx, command.SendPushNotificationCommand{
			UserID: userID,
			Title:  title,
			Body:   body,
			Data: map[string]string{
				"eventId":   env.ID,
				"eventType": env.Type,
				"source":    env.Source,
				"isRetry":   "true",
			},
		})

		if sendErr != nil {
			slog.WarnContext(ctx, "Retry failed for inbound log", slog.String("log_id", logRec.ID), slog.String("error", sendErr.Error()))
			_ = w.outboxLogRepo.UpdateLogStatus(ctx, logRec.ID, "FAILED", sendErr.Error())
		} else {
			slog.InfoContext(ctx, "Retry SUCCESS for inbound log", slog.String("log_id", logRec.ID), slog.String("event_id", env.ID))
			_ = w.outboxLogRepo.UpdateLogStatus(ctx, logRec.ID, "SUCCESS", "")
		}
	}

	return nil
}

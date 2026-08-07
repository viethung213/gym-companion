package worker

import (
	"context"
	"encoding/json"
	"log"
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

	log.Printf("[Notification InboundEventRetryWorker] Started background retry worker (interval: %v)...", w.interval)

	for {
		select {
		case <-ctx.Done():
			log.Println("[Notification InboundEventRetryWorker] Stopping worker due to context cancellation.")
			return nil
		case <-ticker.C:
			if err := w.processFailedLogs(ctx); err != nil {
				log.Printf("[Notification InboundEventRetryWorker] Processing error: %v", err)
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

	log.Printf("[Notification InboundEventRetryWorker] Found %d failed inbound events to retry...", len(failedLogs))

	for _, logRec := range failedLogs {
		var env cloudEventEnvelope
		if err := json.Unmarshal(logRec.Payload, &env); err != nil {
			log.Printf("[Notification InboundEventRetryWorker] Unmarshal payload failed for log ID '%s': %v", logRec.ID, err)
			_ = w.outboxLogRepo.UpdateLogStatus(ctx, logRec.ID, "FAILED_UNPARSEABLE", err.Error())
			continue
		}

		var payload defaultEventData
		if err := json.Unmarshal(env.Data, &payload); err != nil {
			log.Printf("[Notification InboundEventRetryWorker] Unmarshal event data failed for log ID '%s': %v", logRec.ID, err)
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
			log.Printf("[Notification InboundEventRetryWorker] Retry failed for log ID '%s': %v", logRec.ID, sendErr)
			_ = w.outboxLogRepo.UpdateLogStatus(ctx, logRec.ID, "FAILED", sendErr.Error())
		} else {
			log.Printf("[Notification InboundEventRetryWorker] Retry SUCCESS for log ID '%s' (EventID: %s)", logRec.ID, env.ID)
			_ = w.outboxLogRepo.UpdateLogStatus(ctx, logRec.ID, "SUCCESS", "")
		}
	}

	return nil
}

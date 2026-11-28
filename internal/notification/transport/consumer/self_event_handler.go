package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/viethung213/gym-companion/internal/notification/application/command"
	"github.com/viethung213/gym-companion/internal/notification/application/port"
)

const defaultDispatchTimeout = 5 * time.Second

type genericNotificationPayload struct {
	Target *struct {
		UserID   string `json:"userId"`
		AllUsers bool   `json:"allUsers"`
		GroupID  string `json:"groupId"`
		UserList *struct {
			UserIDs []string `json:"userIds"`
		} `json:"userList"`
	} `json:"target"`
	Title       string            `json:"title"`
	Body        string            `json:"body"`
	Data        map[string]string `json:"data"`
	Channels    []string          `json:"channels"`
	RequestedAt string            `json:"requestedAt"`
	UserID      string            `json:"userId"`
}

type selfEventRule struct {
	DefaultTitle   string
	DefaultBody    string
	IsHighPriority bool
}

// SelfEventHandler handles notification events originated from the Notification Service itself (e.g. multi-channel dispatch).
type SelfEventHandler struct {
	sendNotificationHandler *command.SendNotificationHandler
	outboxLogRepo           port.OutboxLogRepository
	rules                   map[string]selfEventRule
}

func NewSelfEventHandler(
	sendNotificationHandler *command.SendNotificationHandler,
	outboxLogRepo port.OutboxLogRepository,
) *SelfEventHandler {
	return &SelfEventHandler{
		sendNotificationHandler: sendNotificationHandler,
		outboxLogRepo:           outboxLogRepo,
		rules: map[string]selfEventRule{
			// 1. NORMAL PRIORITY NOTIFICATION REQUEST EVENT
			"contracts.generic.notification.v1.event.NormalPriorityNotificationRequested": {
				DefaultTitle:   "Gym Companion Thông Báo",
				DefaultBody:    "Bạn có thông báo mới từ ứng dụng.",
				IsHighPriority: false,
			},

			// 2. HIGH PRIORITY NOTIFICATION REQUEST EVENT
			"contracts.generic.notification.v1.event.HighPriorityNotificationRequested": {
				DefaultTitle:   "Thông Báo Quan Trọng ⚠️",
				DefaultBody:    "Bạn có thông báo quan trọng từ ứng dụng.",
				IsHighPriority: true,
			},
		},
	}
}

func (h *SelfEventHandler) CanHandle(eventType string) bool {
	_, ok := h.rules[eventType]
	return ok
}

func (h *SelfEventHandler) Handle(
	ctx context.Context,
	env *cloudEventEnvelope,
	msgValue []byte,
) error {
	rule, ok := h.rules[env.Type]
	if !ok {
		return nil
	}

	if env.ID == "" {
		return fmt.Errorf("cloudevent missing required event id for type: %s", env.Type)
	}
	if h.outboxLogRepo == nil {
		return fmt.Errorf("outbox log repository is required for self event handler")
	}

	var payload genericNotificationPayload
	if err := json.Unmarshal(env.Data, &payload); err != nil {
		slog.ErrorContext(ctx, "Unmarshal generic notification data failed", slog.String("event_type", env.Type), slog.Any("error", err))
		return err
	}

	title := payload.Title
	if title == "" {
		title = rule.DefaultTitle
	}
	body := payload.Body
	if body == "" {
		body = rule.DefaultBody
	}

	channels := payload.Channels
	if len(channels) == 0 {
		channels = []string{"IN_APP", "PUSH"}
	}

	isHighPriority := rule.IsHighPriority

	// 1. Resolve Target & Determine Partition Key
	target, partitionKey := resolveTarget(payload)

	// 2. Idempotency Check & Atomic Claim Lock
	logRec, logErr := h.outboxLogRepo.LogProcessed(ctx, env.ID, env.Type, partitionKey, msgValue)
	if logErr != nil {
		slog.ErrorContext(ctx, "LogProcessed error for event", slog.String("event_id", env.ID), slog.Any("error", logErr))
	} else if logRec == nil {
		slog.DebugContext(ctx, "Duplicate event already processed, skipping", slog.String("event_type", env.Type), slog.String("event_id", env.ID))
		return nil
	}

	// 3. Check previously dispatched channels for Multi-Channel retry
	var dispatchedChannels []string
	if logRec != nil && len(logRec.Metadata) > 0 {
		var meta port.OutboxLogMetadata
		if err := json.Unmarshal(logRec.Metadata, &meta); err == nil {
			dispatchedChannels = meta.ChannelsDispatched
		}
	}

	dispatchedSet := make(map[string]bool, len(dispatchedChannels))
	for _, ch := range dispatchedChannels {
		dispatchedSet[strings.ToUpper(strings.TrimSpace(ch))] = true
	}

	// Filter channels that haven't been dispatched yet
	var finalDispatched []string
	channelsToDispatch := make([]string, 0, len(channels))
	for _, ch := range channels {
		chUpper := strings.ToUpper(strings.TrimSpace(ch))
		if dispatchedSet[chUpper] {
			slog.InfoContext(ctx, "Channel already dispatched, skipping",
				slog.String("event_id", env.ID),
				slog.String("channel", chUpper),
			)
			finalDispatched = append(finalDispatched, chUpper)
		} else {
			channelsToDispatch = append(channelsToDispatch, chUpper)
		}
	}

	// 4. Dispatch via SendNotificationHandler (Strategy Pattern across channels)
	var lastErr error
	if len(channelsToDispatch) > 0 && h.sendNotificationHandler != nil {
		resp, nErr := h.sendNotificationHandler.Handle(ctx, command.SendNotificationCommand{
			Target:         target,
			EventID:        env.ID,
			Title:          title,
			Body:           body,
			Data:           payload.Data,
			Channels:       channelsToDispatch,
			IsHighPriority: isHighPriority,
		})
		lastErr = nErr
		if resp != nil {
			finalDispatched = append(finalDispatched, resp.Dispatched...)
		}
	}

	// 5. Persist final dispatch progress to outbox log
	status := "SUCCESS"
	errMsg := ""
	retryCount := 0
	var nextRetryAt time.Time

	if lastErr != nil {
		errMsg = lastErr.Error()
		currentRetry := 0
		if logRec != nil {
			currentRetry = logRec.RetryCount
		}
		retryCount, status, nextRetryAt = calculateRetryProgression(currentRetry)
		if status == "DEAD_LETTER" {
			slog.ErrorContext(ctx, "Event processing reached max retry attempts (3), moved to DEAD_LETTER",
				slog.String("event_id", env.ID),
				slog.String("event_type", env.Type),
				slog.Int("attempts", retryCount),
				slog.Any("error", lastErr),
			)
		} else {
			slog.WarnContext(ctx, "Event dispatch failed, scheduled for retry",
				slog.String("event_id", env.ID),
				slog.String("event_type", env.Type),
				slog.Int("attempt", retryCount),
				slog.Time("next_retry_at", nextRetryAt),
				slog.Any("error", lastErr),
			)
		}
	}

	var metadataBytes []byte
	if len(finalDispatched) > 0 {
		metadataBytes, _ = json.Marshal(port.OutboxLogMetadata{
			ChannelsDispatched: finalDispatched,
		})
	}

	_ = h.outboxLogRepo.SaveLog(ctx, &port.OutboxLogRecord{
		EventID:      env.ID,
		EventType:    env.Type,
		Payload:      msgValue,
		PartitionKey: partitionKey,
		Status:       status,
		ErrorMessage: errMsg,
		RetryCount:   retryCount,
		NextRetryAt:  nextRetryAt,
		Metadata:     metadataBytes,
	})

	return lastErr
}

func resolveTarget(payload genericNotificationPayload) (port.NotificationTarget, string) {
	if payload.Target != nil {
		if payload.Target.AllUsers {
			return port.NotificationTarget{
				Scope: port.ScopeBroadcast,
			}, "broadcast"
		}

		if payload.Target.GroupID != "" || payload.Target.UserList != nil {
			var userIDs []string
			if payload.Target.UserList != nil {
				userIDs = payload.Target.UserList.UserIDs
			}
			partitionKey := payload.Target.GroupID
			if partitionKey == "" {
				partitionKey = "group"
			}
			return port.NotificationTarget{
				Scope:   port.ScopeGroup,
				GroupID: payload.Target.GroupID,
				UserIDs: userIDs,
			}, partitionKey
		}

		if payload.Target.UserID != "" {
			return port.NotificationTarget{
				Scope:  port.ScopeDirect,
				UserID: payload.Target.UserID,
			}, payload.Target.UserID
		}
	}

	// Fallback for tests or legacy payloads
	userID := payload.UserID
	if userID == "" && payload.Data != nil {
		userID = payload.Data["userId"]
	}
	partitionKey := userID
	if partitionKey == "" {
		partitionKey = "direct"
	}
	return port.NotificationTarget{
		Scope:  port.ScopeDirect,
		UserID: userID,
	}, partitionKey
}

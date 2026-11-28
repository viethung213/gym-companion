package consumer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/viethung213/gym-companion/internal/notification/application/command"
	"github.com/viethung213/gym-companion/internal/notification/application/port"
)

type externalEventData struct {
	UserID      string  `json:"userId"`
	Title       string  `json:"title"`
	Message     string  `json:"message"`
	Body        string  `json:"body"`
	MealName    string  `json:"mealName"`
	MealType    string  `json:"mealType"`
	WorkoutName string  `json:"workoutName"`
	ExerciseID  string  `json:"exerciseId"`
	OneRepMax   float32 `json:"oneRepMax"`
	Weight      float32 `json:"weight"`
	Reps        int32   `json:"reps"`
}

type ExternalEventRule struct {
	DefaultTitle string
	DefaultBody  string
	Formatter    func(data *externalEventData) (title, body string, dataMap map[string]string)
}

func (r ExternalEventRule) Format(data *externalEventData) (title, body string, dataMap map[string]string) {
	if r.Formatter != nil {
		title, body, dataMap = r.Formatter(data)
	}
	if title == "" {
		if data != nil && data.Title != "" {
			title = data.Title
		} else {
			title = r.DefaultTitle
		}
	}
	if body == "" {
		switch {
		case data != nil && data.Body != "":
			body = data.Body
		case data != nil && data.Message != "":
			body = data.Message
		default:
			body = r.DefaultBody
		}
	}
	if dataMap == nil {
		dataMap = make(map[string]string)
	}
	return title, body, dataMap
}

// ExternalEventHandler handles domain events published by external services (e.g. workout_execution, nutrition, coaching).
type ExternalEventHandler struct {
	sendNotificationHandler *command.SendNotificationHandler
	outboxLogRepo           port.OutboxLogRepository
	rules                   map[string]ExternalEventRule
}

func NewExternalEventHandler(
	sendNotificationHandler *command.SendNotificationHandler,
	outboxLogRepo port.OutboxLogRepository,
) *ExternalEventHandler {
	handler := &ExternalEventHandler{
		sendNotificationHandler: sendNotificationHandler,
		outboxLogRepo:           outboxLogRepo,
		rules:                   make(map[string]ExternalEventRule),
	}

	handler.registerDefaultRules()
	return handler
}

func (h *ExternalEventHandler) registerDefaultRules() {
	// ==========================================
	// 1. SERVICE: WORKOUT EXECUTION
	// ==========================================
	prRule := ExternalEventRule{
		DefaultTitle: "Kỷ lục cá nhân mới! 🏆",
		DefaultBody:  "Chúc mừng bạn vừa xác lập một kỷ lục cá nhân (PR) mới!",
		Formatter: func(data *externalEventData) (string, string, map[string]string) {
			title := "Kỷ lục cá nhân mới! 🏆"
			body := "Chúc mừng bạn vừa xác lập một kỷ lục cá nhân (PR) mới!"
			switch {
			case data.Weight > 0 && data.Reps > 0 && data.OneRepMax > 0:
				body = fmt.Sprintf("Chúc mừng bạn vừa đạt PR mới: %.1f kg x %d reps (1RM ước tính: %.1f kg)!", data.Weight, data.Reps, data.OneRepMax)
			case data.Weight > 0 && data.Reps > 0:
				body = fmt.Sprintf("Chúc mừng bạn vừa đạt PR mới: %.1f kg x %d reps!", data.Weight, data.Reps)
			case data.OneRepMax > 0:
				body = fmt.Sprintf("Chúc mừng bạn vừa đạt mức 1RM kỷ lục mới: %.1f kg!", data.OneRepMax)
			case data.Body != "":
				body = data.Body
			case data.Message != "":
				body = data.Message
			}

			if data.Title != "" {
				title = data.Title
			}

			m := make(map[string]string)
			if data.ExerciseID != "" {
				m["exerciseId"] = data.ExerciseID
			}
			if data.OneRepMax > 0 {
				m["oneRepMax"] = fmt.Sprintf("%.1f", data.OneRepMax)
			}
			if data.Weight > 0 {
				m["weight"] = fmt.Sprintf("%.1f", data.Weight)
			}
			if data.Reps > 0 {
				m["reps"] = strconv.Itoa(int(data.Reps))
			}
			return title, body, m
		},
	}
	h.RegisterRule("contracts.core.workout_execution.v1.event.NewPersonalRecordAchieved", prRule)

	// ==========================================
	// 2. SERVICE: NUTRITION
	// ==========================================
	mealRule := ExternalEventRule{
		DefaultTitle: "Nhắc nhở bữa ăn 🥗",
		DefaultBody:  "Sắp đến giờ ăn theo lịch dinh dưỡng (trước 30 phút). Nhớ chuẩn bị bữa ăn nhé!",
		Formatter: func(data *externalEventData) (string, string, map[string]string) {
			title := "Nhắc nhở bữa ăn 🥗"
			body := "Sắp đến giờ ăn theo lịch dinh dưỡng (trước 30 phút). Nhớ chuẩn bị bữa ăn nhé!"
			switch {
			case data.MealName != "":
				title = "Nhắc nhở bữa ăn 🥗: " + data.MealName
				if data.MealType != "" {
					body = fmt.Sprintf("Sắp đến giờ ăn %s (%s). Nhớ chuẩn bị bữa ăn nhé!", data.MealName, data.MealType)
				} else {
					body = fmt.Sprintf("Sắp đến giờ ăn %s. Nhớ chuẩn bị bữa ăn nhé!", data.MealName)
				}
			case data.Body != "":
				body = data.Body
			case data.Message != "":
				body = data.Message
			}

			if data.Title != "" {
				title = data.Title
			}

			m := make(map[string]string)
			if data.MealName != "" {
				m["mealName"] = data.MealName
			}
			if data.MealType != "" {
				m["mealType"] = data.MealType
			}
			return title, body, m
		},
	}
	h.RegisterRule("contracts.core.nutrition.v1.event.UpcomingMealReminder", mealRule)

	// ==========================================
	// 3. SERVICE: COACHING
	// ==========================================
	coachingRule := ExternalEventRule{
		DefaultTitle: "Nhắc nhở buổi tập ⏰",
		DefaultBody:  "Sắp đến giờ tập luyện theo lịch HLV (trước 1 tiếng). Chuẩn bị sẵn sàng nhé!",
		Formatter: func(data *externalEventData) (string, string, map[string]string) {
			title := "Nhắc nhở buổi tập ⏰"
			body := "Sắp đến giờ tập luyện theo lịch HLV (trước 1 tiếng). Chuẩn bị sẵn sàng nhé!"
			switch {
			case data.WorkoutName != "":
				title = "Nhắc nhở buổi tập ⏰: " + data.WorkoutName
				body = fmt.Sprintf("Sắp đến giờ tập %s. Chuẩn bị sẵn sàng nhé!", data.WorkoutName)
			case data.Body != "":
				body = data.Body
			case data.Message != "":
				body = data.Message
			}

			if data.Title != "" {
				title = data.Title
			}

			m := make(map[string]string)
			if data.WorkoutName != "" {
				m["workoutName"] = data.WorkoutName
			}
			return title, body, m
		},
	}
	h.RegisterRule("contracts.core.coaching.v1.event.UpcomingWorkoutReminder", coachingRule)
}

// RegisterRule allows easily registering event rules for new external services in the future.
func (h *ExternalEventHandler) RegisterRule(eventType string, rule ExternalEventRule) {
	h.rules[eventType] = rule
}

func (h *ExternalEventHandler) CanHandle(eventType string) bool {
	_, ok := h.rules[eventType]
	return ok
}

func (h *ExternalEventHandler) Handle(
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
		return errors.New("outbox log repository is required for external event handler")
	}

	var payload externalEventData
	if err := json.Unmarshal(env.Data, &payload); err != nil {
		slog.ErrorContext(ctx, "Unmarshal CloudEvent data failed", slog.String("event_type", env.Type), slog.Any("error", err))
		return err
	}

	if payload.UserID == "" {
		return nil
	}

	logRec, logErr := h.outboxLogRepo.LogProcessed(ctx, env.ID, env.Type, payload.UserID, msgValue)
	if logErr != nil {
		slog.ErrorContext(ctx, "LogProcessed error for event", slog.String("event_id", env.ID), slog.Any("error", logErr))
	} else if logRec == nil {
		slog.DebugContext(ctx, "Duplicate event already recorded in outbox_log, skipping", slog.String("event_type", env.Type), slog.String("event_id", env.ID))
		return nil
	}

	title, body, dataMap := rule.Format(&payload)
	dataMap["eventId"] = env.ID
	dataMap["eventType"] = env.Type
	dataMap["source"] = env.Source

	slog.InfoContext(ctx, "Targeted Event matched for user, dispatching notification",
		slog.String("event_type", env.Type),
		slog.String("user_id", payload.UserID),
	)

	var sendErr error
	if h.sendNotificationHandler != nil {
		target := port.NotificationTarget{
			Scope:  port.ScopeDirect,
			UserID: payload.UserID,
		}
		resp, nErr := h.sendNotificationHandler.Handle(ctx, &command.SendNotificationCommand{
			Target:  target,
			EventID: env.ID,
			Title:   title,
			Body:    body,
			Data:    dataMap,
		})

		inAppSucceeded := false
		if resp != nil {
			for _, ch := range resp.Dispatched {
				if ch == "IN_APP" {
					inAppSucceeded = true
					break
				}
			}
		}

		if inAppSucceeded {
			if nErr != nil {
				slog.WarnContext(ctx, "External event push dispatch failed, but in-app saved successfully",
					slog.String("event_id", env.ID),
					slog.String("event_type", env.Type),
					slog.Any("error", nErr),
				)
			}
			sendErr = nil
		} else {
			sendErr = nErr
		}
	}

	if sendErr != nil {
		slog.ErrorContext(ctx, "Error sending notification for event", slog.String("event_id", env.ID), slog.Any("error", sendErr))
	}

	status := "SUCCESS"
	errMsg := ""
	retryCount := 0
	var nextRetryAt time.Time

	if sendErr != nil {
		errMsg = sendErr.Error()
		currentRetry := 0
		if logRec != nil {
			currentRetry = logRec.RetryCount
		}
		retryCount, status, nextRetryAt = calculateRetryProgression(currentRetry)
		if status == "DEAD_LETTER" {
			slog.ErrorContext(ctx, "External event reached max retry attempts (3), moved to DEAD_LETTER",
				slog.String("event_id", env.ID),
				slog.String("event_type", env.Type),
				slog.Int("attempts", retryCount),
				slog.Any("error", sendErr),
			)
		} else {
			slog.WarnContext(ctx, "External event dispatch failed, scheduled for retry",
				slog.String("event_id", env.ID),
				slog.String("event_type", env.Type),
				slog.Int("attempt", retryCount),
				slog.Time("next_retry_at", nextRetryAt),
				slog.Any("error", sendErr),
			)
		}
	}

	_ = h.outboxLogRepo.SaveLog(ctx, &port.OutboxLogRecord{
		EventID:      env.ID,
		EventType:    env.Type,
		Payload:      msgValue,
		PartitionKey: payload.UserID,
		Status:       status,
		ErrorMessage: errMsg,
		RetryCount:   retryCount,
		NextRetryAt:  nextRetryAt,
	})

	return sendErr
}

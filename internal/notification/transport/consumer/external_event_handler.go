package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/viethung213/gym-companion/internal/notification/application/command"
	"github.com/viethung213/gym-companion/internal/notification/application/port"
)

type externalEventData struct {
	UserID      string `json:"userId"`
	Title       string `json:"title"`
	Message     string `json:"message"`
	Body        string `json:"body"`
	MealName    string `json:"mealName"`
	MealType    string `json:"mealType"`
	WorkoutName string `json:"workoutName"`
}

type ExternalEventRule struct {
	DefaultTitle string
	DefaultBody  string
	// Formatter optionally customizes title and body based on the parsed event data
	Formatter func(data *externalEventData) (title, body string)
}

// ExternalEventHandler handles domain events published by external services (e.g. workout_execution, nutrition, coaching).
type ExternalEventHandler struct {
	sendPushHandler *command.SendPushNotificationHandler
	outboxLogRepo   port.OutboxLogRepository
	rules           map[string]ExternalEventRule
}

func NewExternalEventHandler(
	sendPushHandler *command.SendPushNotificationHandler,
	outboxLogRepo port.OutboxLogRepository,
) *ExternalEventHandler {
	handler := &ExternalEventHandler{
		sendPushHandler: sendPushHandler,
		outboxLogRepo:   outboxLogRepo,
		rules:           make(map[string]ExternalEventRule),
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
	}
	h.RegisterRule("contracts.core.workout_execution.v1.event.NewPersonalRecordAchieved", prRule)
	h.RegisterRule("contracts.core.workout_execution.v1.newPersonalRecordAchieved", prRule)

	// ==========================================
	// 2. SERVICE: NUTRITION
	// ==========================================
	mealRule := ExternalEventRule{
		DefaultTitle: "Nhắc nhở bữa ăn 🥗",
		DefaultBody:  "Sắp đến giờ ăn theo lịch dinh dưỡng (trước 30 phút). Nhớ chuẩn bị bữa ăn nhé!",
		Formatter: func(data *externalEventData) (string, string) {
			if data.MealName == "" {
				return "", ""
			}
			title := "Nhắc nhở bữa ăn 🥗: " + data.MealName
			var body string
			if data.MealType != "" {
				body = fmt.Sprintf("Sắp đến giờ ăn %s (%s). Nhớ chuẩn bị bữa ăn nhé!", data.MealName, data.MealType)
			} else {
				body = fmt.Sprintf("Sắp đến giờ ăn %s. Nhớ chuẩn bị bữa ăn nhé!", data.MealName)
			}
			return title, body
		},
	}
	h.RegisterRule("contracts.core.nutrition.v1.event.UpcomingMealReminder", mealRule)
	h.RegisterRule("contracts.core.nutrition.v1.upcomingMealReminder", mealRule)

	// ==========================================
	// 3. SERVICE: COACHING
	// ==========================================
	coachingRule := ExternalEventRule{
		DefaultTitle: "Nhắc nhở buổi tập ⏰",
		DefaultBody:  "Sắp đến giờ tập luyện theo lịch HLV (trước 1 tiếng). Chuẩn bị sẵn sàng nhé!",
		Formatter: func(data *externalEventData) (string, string) {
			if data.WorkoutName == "" {
				return "", ""
			}
			title := "Nhắc nhở buổi tập ⏰: " + data.WorkoutName
			body := fmt.Sprintf("Sắp đến giờ tập %s. Chuẩn bị sẵn sàng nhé!", data.WorkoutName)
			return title, body
		},
	}
	h.RegisterRule("contracts.core.coaching.v1.event.UpcomingWorkoutReminder", coachingRule)
	h.RegisterRule("contracts.core.coaching.v1.upcomingWorkoutReminder", coachingRule)
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

	var payload externalEventData
	if err := json.Unmarshal(env.Data, &payload); err != nil {
		slog.ErrorContext(ctx, "Unmarshal CloudEvent data failed", slog.String("event_type", env.Type), slog.Any("error", err))
		return err
	}

	if payload.UserID == "" {
		return nil
	}

	if h.outboxLogRepo != nil && env.ID != "" {
		fresh, logErr := h.outboxLogRepo.LogProcessed(ctx, env.ID, env.Type, payload.UserID, msgValue, "PROCESSING", "")
		if logErr != nil {
			slog.ErrorContext(ctx, "LogProcessed error for event", slog.String("event_id", env.ID), slog.Any("error", logErr))
		} else if !fresh {
			slog.DebugContext(ctx, "Duplicate event already recorded in outbox_log, skipping", slog.String("event_type", env.Type), slog.String("event_id", env.ID))
			return nil
		}
	}

	title := payload.Title
	body := payload.Body
	if body == "" {
		body = payload.Message
	}

	if rule.Formatter != nil {
		customTitle, customBody := rule.Formatter(&payload)
		if customTitle != "" {
			title = customTitle
		}
		if customBody != "" {
			body = customBody
		}
	}

	if title == "" {
		title = rule.DefaultTitle
	}
	if body == "" {
		body = rule.DefaultBody
	}

	dataMap := map[string]string{
		"eventId":   env.ID,
		"eventType": env.Type,
		"source":    env.Source,
	}
	if payload.MealName != "" {
		dataMap["mealName"] = payload.MealName
	}
	if payload.WorkoutName != "" {
		dataMap["workoutName"] = payload.WorkoutName
	}

	slog.InfoContext(ctx, "Targeted Event matched for user, dispatching push", slog.String("event_type", env.Type), slog.String("user_id", payload.UserID))
	var sendErr error
	if h.sendPushHandler != nil {
		_, sendErr = h.sendPushHandler.Handle(ctx, command.SendPushNotificationCommand{
			UserID: payload.UserID,
			Title:  title,
			Body:   body,
			Data:   dataMap,
		})
	}

	if sendErr != nil {
		slog.ErrorContext(ctx, "Error sending push notification for event", slog.String("event_id", env.ID), slog.Any("error", sendErr))
		if h.outboxLogRepo != nil && env.ID != "" {
			_ = h.outboxLogRepo.SaveLog(ctx, &port.OutboxLogRecord{
				EventID:      env.ID,
				EventType:    env.Type,
				Payload:      msgValue,
				PartitionKey: payload.UserID,
				Status:       "FAILED",
				ErrorMessage: sendErr.Error(),
			})
		}
		return sendErr
	}

	if h.outboxLogRepo != nil && env.ID != "" {
		_ = h.outboxLogRepo.SaveLog(ctx, &port.OutboxLogRecord{
			EventID:      env.ID,
			EventType:    env.Type,
			Payload:      msgValue,
			PartitionKey: payload.UserID,
			Status:       "SUCCESS",
			ErrorMessage: "",
		})
	}

	return nil
}

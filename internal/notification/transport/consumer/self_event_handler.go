package consumer

import (
	"context"
	"encoding/json"
	"log"
	"strings"

	"github.com/viethung213/gym-companion/internal/notification/application/command"
	"github.com/viethung213/gym-companion/internal/notification/application/port"
)

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
	sendPushHandler  *command.SendPushNotificationHandler
	sendEmailHandler *command.SendEmailNotificationHandler
	sendSMSHandler   *command.SendSMSNotificationHandler
	outboxLogRepo    port.OutboxLogRepository
	rules            map[string]selfEventRule
}

func NewSelfEventHandler(
	sendPushHandler *command.SendPushNotificationHandler,
	sendEmailHandler *command.SendEmailNotificationHandler,
	sendSMSHandler *command.SendSMSNotificationHandler,
	outboxLogRepo port.OutboxLogRepository,
) *SelfEventHandler {
	return &SelfEventHandler{
		sendPushHandler:  sendPushHandler,
		sendEmailHandler: sendEmailHandler,
		sendSMSHandler:   sendSMSHandler,
		outboxLogRepo:    outboxLogRepo,
		rules: map[string]selfEventRule{
			// 1. NORMAL PRIORITY NOTIFICATION REQUEST EVENT
			"contracts.generic.notification.v1.event.NormalPriorityNotificationRequested": {
				DefaultTitle:   "Gym Companion Thông Báo",
				DefaultBody:    "Bạn có thông báo mới từ ứng dụng.",
				IsHighPriority: false,
			},
			"contracts.generic.notification.v1.normalPriorityNotificationRequested": {
				DefaultTitle:   "Gym Companion Thông Báo",
				DefaultBody:    "Bạn có thông báo mới từ ứng dụng.",
				IsHighPriority: false,
			},

			// 3. HIGH PRIORITY NOTIFICATION REQUEST EVENT
			"contracts.generic.notification.v1.event.HighPriorityNotificationRequested": {
				DefaultTitle:   "Thông Báo Quan Trọng ⚠️",
				DefaultBody:    "Bạn có thông báo quan trọng từ ứng dụng.",
				IsHighPriority: true,
			},
			"contracts.generic.notification.v1.highPriorityNotificationRequested": {
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

	var payload genericNotificationPayload
	if err := json.Unmarshal(env.Data, &payload); err != nil {
		log.Printf("[Self Event Handler] Unmarshal generic notification data failed for '%s': %v", env.Type, err)
		return err
	}

	var userIDs []string
	if payload.Target != nil {
		if payload.Target.UserID != "" {
			userIDs = append(userIDs, payload.Target.UserID)
		} else if payload.Target.UserList != nil {
			userIDs = append(userIDs, payload.Target.UserList.UserIDs...)
		}
	}
	if len(userIDs) == 0 && payload.UserID != "" {
		userIDs = append(userIDs, payload.UserID)
	}

	partitionKey := "broadcast"
	if len(userIDs) > 0 {
		partitionKey = userIDs[0]
	} else if payload.Data != nil {
		if email, ok := payload.Data["email"]; ok {
			partitionKey = email
		} else if phone, ok := payload.Data["phone"]; ok {
			partitionKey = phone
		}
	}

	if h.outboxLogRepo != nil && env.ID != "" {
		fresh, logErr := h.outboxLogRepo.LogProcessed(ctx, env.ID, env.Type, partitionKey, msgValue, "PROCESSING", "")
		if logErr != nil {
			log.Printf("[Self Event Handler] LogProcessed error for event '%s': %v", env.ID, logErr)
		} else if !fresh {
			log.Printf("[Self Event Handler] Duplicate event '%s' (ID: %s) already processed, skipping...", env.Type, env.ID)
			return nil
		}
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
		channels = []string{"PUSH"}
	}

	isHighPriority := rule.IsHighPriority

	var lastErr error
	for _, ch := range channels {
		switch strings.ToUpper(strings.TrimSpace(ch)) {
		case "PUSH":
			if h.sendPushHandler != nil {
				for _, uid := range userIDs {
					_, err := h.sendPushHandler.Handle(ctx, command.SendPushNotificationCommand{
						UserID:         uid,
						Title:          title,
						Body:           body,
						Data:           payload.Data,
						IsHighPriority: isHighPriority,
					})
					if err != nil {
						lastErr = err
						log.Printf("[Self Event Handler] Push dispatch error for user %s: %v", uid, err)
					}
				}
			}

		case "EMAIL":
			if h.sendEmailHandler != nil {
				emailAddr := ""
				if payload.Data != nil {
					emailAddr = payload.Data["email"]
					if emailAddr == "" {
						emailAddr = payload.Data["recipient_email"]
					}
					if emailAddr == "" {
						emailAddr = payload.Data["to_email"]
					}
				}

				if emailAddr != "" {
					if len(userIDs) > 0 {
						for _, uid := range userIDs {
							_, err := h.sendEmailHandler.Handle(ctx, command.SendEmailNotificationCommand{
								UserID:         uid,
								To:             emailAddr,
								Subject:        title,
								Body:           body,
								IsHighPriority: isHighPriority,
							})
							if err != nil {
								lastErr = err
								log.Printf("[Self Event Handler] Email dispatch error to %s: %v", emailAddr, err)
							}
						}
					} else {
						_, err := h.sendEmailHandler.Handle(ctx, command.SendEmailNotificationCommand{
							UserID:         "",
							To:             emailAddr,
							Subject:        title,
							Body:           body,
							IsHighPriority: isHighPriority,
						})
						if err != nil {
							lastErr = err
							log.Printf("[Self Event Handler] Email dispatch error to %s: %v", emailAddr, err)
						}
					}
				} else {
					log.Printf("[Self Event Handler] EMAIL requested but no recipient email in data (event ID: %s)", env.ID)
				}
			}

		case "SMS":
			if h.sendSMSHandler != nil {
				phoneNum := ""
				if payload.Data != nil {
					phoneNum = payload.Data["phone"]
					if phoneNum == "" {
						phoneNum = payload.Data["phone_number"]
					}
					if phoneNum == "" {
						phoneNum = payload.Data["recipient_phone"]
					}
					if phoneNum == "" {
						phoneNum = payload.Data["to_phone"]
					}
				}

				if phoneNum != "" {
					if len(userIDs) > 0 {
						for _, uid := range userIDs {
							_, err := h.sendSMSHandler.Handle(ctx, command.SendSMSNotificationCommand{
								UserID:         uid,
								PhoneNumber:    phoneNum,
								Message:        body,
								IsHighPriority: isHighPriority,
							})
							if err != nil {
								lastErr = err
								log.Printf("[Self Event Handler] SMS dispatch error to %s: %v", phoneNum, err)
							}
						}
					} else {
						_, err := h.sendSMSHandler.Handle(ctx, command.SendSMSNotificationCommand{
							UserID:         "",
							PhoneNumber:    phoneNum,
							Message:        body,
							IsHighPriority: isHighPriority,
						})
						if err != nil {
							lastErr = err
							log.Printf("[Self Event Handler] SMS dispatch error to %s: %v", phoneNum, err)
						}
					}
				} else {
					log.Printf("[Self Event Handler] SMS requested but no recipient phone in data (event ID: %s)", env.ID)
				}
			}
		}
	}

	if lastErr != nil {
		if h.outboxLogRepo != nil && env.ID != "" {
			_ = h.outboxLogRepo.SaveLog(ctx, &port.OutboxLogRecord{
				EventID:      env.ID,
				EventType:    env.Type,
				Payload:      msgValue,
				PartitionKey: partitionKey,
				Status:       "FAILED",
				ErrorMessage: lastErr.Error(),
			})
		}
		return lastErr
	}

	if h.outboxLogRepo != nil && env.ID != "" {
		_ = h.outboxLogRepo.SaveLog(ctx, &port.OutboxLogRecord{
			EventID:      env.ID,
			EventType:    env.Type,
			Payload:      msgValue,
			PartitionKey: partitionKey,
			Status:       "SUCCESS",
			ErrorMessage: "",
		})
	}

	return nil
}

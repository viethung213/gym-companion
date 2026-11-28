package command

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/viethung213/gym-companion/internal/notification/application/port"
)

// NotificationMessage encapsulates the notification content and metadata for channel dispatch.
type NotificationMessage struct {
	EventID        string
	Title          string
	Body           string
	Data           map[string]string
	IsHighPriority bool
	InAppFailed    bool
}

// ChannelDispatcher is the Strategy interface for an atomic communication channel.
// It is defined in the command package where it is consumed by SendNotificationHandler (Go idiom: interfaces defined by consumer).
type ChannelDispatcher interface {
	Channel() string // e.g., "PUSH", "IN_APP", "EMAIL", "SMS"
	Dispatch(ctx context.Context, target port.NotificationTarget, msg NotificationMessage) (*NotificationMessage, error)
}

// SendNotificationCommand is the unified command for dispatching notifications
// across any recipient scope (Direct, Group, Broadcast) and any communication channel.
type SendNotificationCommand struct {
	Target         port.NotificationTarget
	EventID        string
	Title          string
	Body           string
	Data           map[string]string
	Channels       []string
	IsHighPriority bool
}

// SendNotificationResponse returns the successfully dispatched channels.
type SendNotificationResponse struct {
	Dispatched []string
}

// SendNotificationHandler orchestrates multi-channel notifications via the Channel-Strategy pattern.
type SendNotificationHandler struct {
	dispatchers map[string]ChannelDispatcher
}

// NewSendNotificationHandler creates an orchestrator initialized with channel dispatchers.
func NewSendNotificationHandler(dispatchers ...ChannelDispatcher) *SendNotificationHandler {
	m := make(map[string]ChannelDispatcher, len(dispatchers))
	for _, d := range dispatchers {
		if d != nil {
			m[strings.ToUpper(strings.TrimSpace(d.Channel()))] = d
		}
	}
	return &SendNotificationHandler{dispatchers: m}
}

// RegisterDispatcher dynamically registers or overrides a channel dispatcher.
func (h *SendNotificationHandler) RegisterDispatcher(d ChannelDispatcher) {
	if d != nil {
		h.dispatchers[strings.ToUpper(strings.TrimSpace(d.Channel()))] = d
	}
}

// Handle executes the notification dispatch across all specified channels.
func (h *SendNotificationHandler) Handle(
	ctx context.Context,
	cmd SendNotificationCommand,
) (*SendNotificationResponse, error) {
	var dispatched []string
	var lastErr error
	var mu sync.Mutex

	channels := cmd.Channels
	if len(channels) == 0 {
		channels = []string{"IN_APP", "PUSH"}
	}

	currentMsg := NotificationMessage{
		EventID:        cmd.EventID,
		Title:          cmd.Title,
		Body:           cmd.Body,
		Data:           cmd.Data,
		IsHighPriority: cmd.IsHighPriority,
		InAppFailed:    false,
	}

	// 1. Separate IN_APP (which acts as SSOT / enrichment) from other outbound channels
	var hasInApp bool
	var outboundChannels []string
	for _, ch := range channels {
		chUpper := strings.ToUpper(strings.TrimSpace(ch))
		if chUpper == "IN_APP" {
			hasInApp = true
		} else {
			outboundChannels = append(outboundChannels, chUpper)
		}
	}

	// 2. Phase 1: Execute IN_APP first to persist state & compute rollups in RAM
	if hasInApp {
		if inAppDisp, exists := h.dispatchers["IN_APP"]; exists {
			dispatchCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			updatedMsg, err := inAppDisp.Dispatch(dispatchCtx, cmd.Target, currentMsg)
			cancel()

			if err != nil {
				lastErr = err
				currentMsg.InAppFailed = true // Pass inAppFailed in RAM via message!
				slog.ErrorContext(ctx, "Channel dispatch error",
					slog.String("channel", "IN_APP"),
					slog.String("scope", string(cmd.Target.Scope)),
					slog.Any("error", err),
				)
			} else {
				if updatedMsg != nil {
					currentMsg = *updatedMsg
				}
				dispatched = append(dispatched, "IN_APP")
			}
		}
	}

	// 3. Phase 2: Dispatch remaining outbound channels (PUSH, EMAIL, SMS) concurrently in PARALLEL
	if len(outboundChannels) > 0 {
		var wg sync.WaitGroup
		for _, chUpper := range outboundChannels {
			dispatcher, exists := h.dispatchers[chUpper]
			if !exists {
				continue
			}

			wg.Add(1)
			go func(channelName string, d ChannelDispatcher) {
				defer wg.Done()

				dispatchCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
				defer cancel()

				_, err := d.Dispatch(dispatchCtx, cmd.Target, currentMsg)
				if err != nil {
					slog.ErrorContext(ctx, "Channel dispatch error",
						slog.String("channel", channelName),
						slog.String("scope", string(cmd.Target.Scope)),
						slog.Any("error", err),
					)
					mu.Lock()
					if lastErr == nil {
						lastErr = err
					}
					mu.Unlock()
				} else {
					mu.Lock()
					dispatched = append(dispatched, channelName)
					mu.Unlock()
				}
			}(chUpper, dispatcher)
		}
		wg.Wait()
	}

	return &SendNotificationResponse{
		Dispatched: dispatched,
	}, lastErr
}

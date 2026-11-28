package command

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/viethung213/gym-companion/internal/notification/application/port"
	"github.com/viethung213/gym-companion/internal/notification/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/notification/domain/repository"
)

// PushChannelDispatcher handles push notifications across Broadcast, Group, and Direct targets as a pure transport channel.
type PushChannelDispatcher struct {
	deviceRepo      repository.DeviceRepository
	pushProvider    port.PushProvider
	settingRepo     repository.SettingRepository
	cooldownService port.PushCooldownService
}

func NewPushChannelDispatcher(
	deviceRepo repository.DeviceRepository,
	pushProvider port.PushProvider,
	settingRepo repository.SettingRepository,
	cooldownService port.PushCooldownService,
) *PushChannelDispatcher {
	return &PushChannelDispatcher{
		deviceRepo:      deviceRepo,
		pushProvider:    pushProvider,
		settingRepo:     settingRepo,
		cooldownService: cooldownService,
	}
}

func (d *PushChannelDispatcher) Channel() string {
	return "PUSH"
}

// targetRecipients resolves the recipient identifiers for cooldown tracking across all target scopes.
func targetRecipients(t port.NotificationTarget) []string {
	switch t.Scope {
	case port.ScopeBroadcast:
		return []string{"all_users"}
	case port.ScopeDirect:
		if t.UserID == "" {
			return nil
		}
		return []string{t.UserID}
	case port.ScopeGroup:
		return t.UserIDs
	default:
		return nil
	}
}

// applyCooldownPolicy evaluates anti-spam cooldown for social entities (e.g. feed comments/reactions).
// It returns:
// - allowedRecipients: recipient IDs permitted to receive push notification.
// - releaseCooldown: closure to release only the locks acquired in this call (Self-Healing on FCM delivery failure).
func (d *PushChannelDispatcher) applyCooldownPolicy(
	ctx context.Context,
	msg NotificationMessage,
	target port.NotificationTarget,
) ([]string, func(ctx context.Context)) {
	recipients := targetRecipients(target)
	noRelease := func(context.Context) {}

	feedID := ""
	if msg.Data != nil {
		feedID = msg.Data["feedItemId"]
	}

	// Non-social notifications or high priority alerts bypass cooldown completely (e.g. OTP, PR achievements, workout reminders)
	if feedID == "" || msg.IsHighPriority || d.cooldownService == nil || len(recipients) == 0 {
		return recipients, noRelease
	}

	allowed, err := d.cooldownService.CheckAndLockBatch(ctx, feedID, recipients, 60*time.Second)
	if err != nil {
		slog.WarnContext(ctx, "Push cooldown batch check error, fail-open to all recipients",
			slog.String("feed_id", feedID),
			slog.String("scope", string(target.Scope)),
			slog.Int("recipient_count", len(recipients)),
			slog.Any("error", err),
		)
		return recipients, noRelease
	}

	if len(allowed) < len(recipients) {
		slog.InfoContext(ctx, "Push cooldown filtered recipients",
			slog.String("feed_id", feedID),
			slog.Int("total", len(recipients)),
			slog.Int("allowed_count", len(allowed)),
			slog.Int("suppressed_count", len(recipients)-len(allowed)),
		)
	}

	if len(allowed) == 0 {
		return nil, noRelease
	}

	release := func(relCtx context.Context) {
		if err := d.cooldownService.ReleaseLockBatch(relCtx, feedID, allowed); err != nil {
			slog.WarnContext(relCtx, "Push cooldown release lock batch error",
				slog.String("feed_id", feedID),
				slog.String("scope", string(target.Scope)),
				slog.Any("error", err),
			)
		}
	}

	return allowed, release
}

// filterAllowedBySettings filters out recipients who have disabled push notifications or are in quiet hours.
// High priority notifications (security alerts, OTP, etc.) bypass all privacy restrictions.
func (d *PushChannelDispatcher) filterAllowedBySettings(
	ctx context.Context,
	userIDs []string,
	isHighPriority bool,
) []string {
	if isHighPriority || d.settingRepo == nil || len(userIDs) == 0 {
		return userIDs
	}

	settings, err := d.settingRepo.GetByUserIDs(ctx, userIDs)
	if err != nil {
		slog.WarnContext(ctx, "Get notification settings for push failed, failing open",
			slog.Int("user_count", len(userIDs)),
			slog.Any("error", err),
		)
		return userIDs // Graceful fail-open
	}

	settingsMap := make(map[string]*aggregate.Setting, len(settings))
	for _, s := range settings {
		if s != nil {
			settingsMap[s.UserID()] = s
		}
	}

	now := time.Now()
	allowed := make([]string, 0, len(userIDs))
	for _, uid := range userIDs {
		s, exists := settingsMap[uid]
		if !exists {
			// Users without explicit setting record default to enabled push and no quiet hours
			allowed = append(allowed, uid)
			continue
		}

		if !s.EnablePush() {
			slog.InfoContext(ctx, "Push disabled by user setting", slog.String("user_id", uid))
			continue
		}
		if s.IsInQuietHours(now) {
			slog.InfoContext(ctx, "Push suppressed during quiet hours", slog.String("user_id", uid))
			continue
		}

		allowed = append(allowed, uid)
	}

	return allowed
}

func (d *PushChannelDispatcher) Dispatch(
	ctx context.Context,
	target port.NotificationTarget,
	msg NotificationMessage,
) (*NotificationMessage, error) {
	// If IN_APP persistence failed in this run, skip PUSH to preserve SSOT consistency
	if msg.InAppFailed {
		slog.WarnContext(ctx, "Push skipped because in-app persistence failed (will retry together on next attempt)",
			slog.String("event_id", msg.EventID),
			slog.String("scope", string(target.Scope)),
			slog.String("user_id", target.UserID),
		)
		return nil, errors.New("push skipped: in-app persistence failed")
	}

	if d.pushProvider == nil {
		return nil, nil
	}

	// 1. Evaluate Cooldown Policy: Determines allowed recipients and self-healing release closure
	recipients, releaseCooldown := d.applyCooldownPolicy(ctx, msg, target)
	if len(recipients) == 0 {
		slog.InfoContext(ctx, "Push notification suppressed by cooldown",
			slog.String("scope", string(target.Scope)),
			slog.String("status", "SKIPPED_COOLDOWN"),
		)
		return nil, nil
	}

	switch target.Scope {
	case port.ScopeBroadcast:
		if err := d.pushProvider.SendPushToTopic(ctx, "all_users", msg.Title, msg.Body, msg.Data); err != nil {
			releaseCooldown(ctx)
			slog.ErrorContext(ctx, "Broadcast push delivery error (best-effort, skipping retry)",
				slog.String("event_id", msg.EventID),
				slog.String("title", msg.Title),
				slog.String("topic", "all_users"),
				slog.Any("error", err),
			)
		}
		return nil, nil

	case port.ScopeGroup:
		if len(recipients) == 0 || d.deviceRepo == nil {
			return nil, nil
		}

		// Filter out users who disabled push or are in quiet hours (unless high priority)
		recipients = d.filterAllowedBySettings(ctx, recipients, msg.IsHighPriority)
		if len(recipients) == 0 {
			return nil, nil
		}

		devices, err := d.deviceRepo.GetActiveDevicesByUserIDs(ctx, recipients)
		if err != nil {
			slog.ErrorContext(ctx, "Get active devices for group push failed (skipping retry)",
				slog.String("event_id", msg.EventID),
				slog.String("title", msg.Title),
				slog.Any("error", err),
			)
			return nil, nil
		}

		tokens := make([]string, 0, len(devices))
		for _, dev := range devices {
			tokens = append(tokens, dev.DeviceToken())
		}
		if len(tokens) == 0 {
			return nil, nil
		}

		const chunkSize = 500
		var allInvalidTokens []string

		for i := 0; i < len(tokens); i += chunkSize {
			end := i + chunkSize
			if end > len(tokens) {
				end = len(tokens)
			}
			pushResp, pErr := d.pushProvider.SendPush(ctx, tokens[i:end], msg.Title, msg.Body, msg.Data)
			if pErr != nil {
				releaseCooldown(ctx)
				slog.ErrorContext(ctx, "Group multicast push chunk error (best-effort, skipping retry)",
					slog.String("event_id", msg.EventID),
					slog.String("title", msg.Title),
					slog.Int("chunk_start", i),
					slog.Int("tokens_count", len(tokens[i:end])),
					slog.Any("error", pErr),
				)
			}
			if pushResp != nil && len(pushResp.InvalidTokens) > 0 {
				allInvalidTokens = append(allInvalidTokens, pushResp.InvalidTokens...)
			}
		}

		if len(allInvalidTokens) > 0 {
			if deactErr := d.deviceRepo.DeactivateTokens(ctx, allInvalidTokens); deactErr != nil {
				slog.WarnContext(ctx, "Failed to deactivate invalid tokens from group push",
					slog.String("event_id", msg.EventID),
					slog.Any("error", deactErr),
				)
			}
		}
		return nil, nil

	case port.ScopeDirect:
		if target.UserID == "" || d.deviceRepo == nil {
			return nil, nil
		}

		// Filter by privacy settings (EnablePush & QuietHours) unless high priority
		allowed := d.filterAllowedBySettings(ctx, []string{target.UserID}, msg.IsHighPriority)
		if len(allowed) == 0 {
			return nil, nil
		}

		devices, err := d.deviceRepo.GetActiveDevicesByUserID(ctx, target.UserID)
		if err != nil {
			slog.ErrorContext(ctx, "Get user active devices failed for direct push",
				slog.String("event_id", msg.EventID),
				slog.String("user_id", target.UserID),
				slog.Any("error", err),
			)
			return nil, nil
		}

		tokens := make([]string, 0, len(devices))
		for _, dev := range devices {
			tokens = append(tokens, dev.DeviceToken())
		}
		if len(tokens) == 0 {
			return nil, nil
		}

		pushResp, pErr := d.pushProvider.SendPush(ctx, tokens, msg.Title, msg.Body, msg.Data)
		if pErr != nil {
			releaseCooldown(ctx)
			slog.ErrorContext(ctx, "Direct push provider delivery failed (skipping retry)",
				slog.String("event_id", msg.EventID),
				slog.String("title", msg.Title),
				slog.String("user_id", target.UserID),
				slog.Any("error", pErr),
			)
			return nil, nil
		}

		if pushResp != nil {
			if pushResp.SuccessCount == 0 && pushResp.FailureCount > 0 {
				releaseCooldown(ctx)
				slog.WarnContext(ctx, "Direct push delivery failed for all user devices",
					slog.String("event_id", msg.EventID),
					slog.String("user_id", target.UserID),
					slog.Int("failures", pushResp.FailureCount),
				)
			} else {
				slog.InfoContext(ctx, "Direct push sent successfully",
					slog.String("event_id", msg.EventID),
					slog.String("user_id", target.UserID),
					slog.Int("success_count", pushResp.SuccessCount),
				)
			}

			if len(pushResp.InvalidTokens) > 0 {
				_ = d.deviceRepo.DeactivateTokens(ctx, pushResp.InvalidTokens)
			}
		}

		return nil, nil

	default:
		slog.ErrorContext(ctx, "Unsupported target scope for push dispatch",
			slog.String("event_id", msg.EventID),
			slog.String("scope", string(target.Scope)),
		)
		return nil, nil
	}
}

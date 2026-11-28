package command

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/viethung213/gym-companion/internal/notification/application/port"
	"github.com/viethung213/gym-companion/internal/notification/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/notification/domain/repository"
)

// InAppChannelDispatcher handles persistent in-app notifications across Broadcast, Group, and Direct targets.
type InAppChannelDispatcher struct {
	notificationRepo repository.NotificationRepository
}

func NewInAppChannelDispatcher(
	notificationRepo repository.NotificationRepository,
) *InAppChannelDispatcher {
	return &InAppChannelDispatcher{
		notificationRepo: notificationRepo,
	}
}

func (d *InAppChannelDispatcher) Channel() string {
	return "IN_APP"
}

func (d *InAppChannelDispatcher) Dispatch(
	ctx context.Context,
	target port.NotificationTarget,
	msg NotificationMessage,
) (*NotificationMessage, error) {
	if d.notificationRepo == nil {
		return &msg, nil
	}

	switch target.Scope {
	case port.ScopeBroadcast:
		announcementID := uuid.New().String()
		announcement, err := aggregate.NewSystemAnnouncement(
			announcementID,
			msg.EventID,
			msg.Title,
			msg.Body,
			msg.Data,
		)
		if err != nil {
			return nil, fmt.Errorf("create system announcement: %w", err)
		}
		if saveErr := d.notificationRepo.SaveSystemAnnouncement(ctx, announcement); saveErr != nil {
			return nil, saveErr
		}
		return &msg, nil

	case port.ScopeGroup:
		if len(target.UserIDs) == 0 {
			return &msg, nil
		}

		uniqueUserIDs := make([]string, 0, len(target.UserIDs))
		seen := make(map[string]struct{}, len(target.UserIDs))
		for _, uid := range target.UserIDs {
			if uid == "" {
				continue
			}
			if _, exists := seen[uid]; exists {
				continue
			}
			seen[uid] = struct{}{}
			uniqueUserIDs = append(uniqueUserIDs, uid)
		}

		if len(uniqueUserIDs) == 0 {
			return &msg, nil
		}

		isSocial := msg.Data != nil && msg.Data["type"] == "POST_REACTION" && msg.Data["feedItemId"] != ""

		// 1. NON-SOCIAL: Zero queries! Directly create and batch save
		if !isSocial {
			items := make([]*aggregate.InAppNotification, 0, len(uniqueUserIDs))
			for _, uid := range uniqueUserIDs {
				item, itemErr := aggregate.NewInAppNotification(
					uuid.New().String(),
					uid,
					msg.Title,
					msg.Body,
					msg.Data,
				)
				if itemErr == nil {
					if msg.EventID != "" {
						item.WithEventID(msg.EventID)
					}
					items = append(items, item)
				}
			}
			if len(items) > 0 {
				if batchErr := d.notificationRepo.BatchSave(ctx, items); batchErr != nil {
					return nil, batchErr
				}
			}
			return &msg, nil
		}

		// 2. SOCIAL NOTIFICATION: Single batch query to find unread reactions for rollup
		var unreadByUser map[string]*aggregate.InAppNotification
		feedItemID := msg.Data["feedItemId"]
		if rollupRepo, ok := d.notificationRepo.(repository.ReactionRollupRepository); ok {
			unreads, findErr := rollupRepo.FindUnreadReactionsByPostAndUsers(ctx, uniqueUserIDs, feedItemID)
			if findErr != nil {
				return nil, fmt.Errorf("find unread reactions for group: %w", findErr)
			}
			if len(unreads) > 0 {
				unreadByUser = make(map[string]*aggregate.InAppNotification, len(unreads))
				for _, unread := range unreads {
					if _, exists := unreadByUser[unread.UserID()]; !exists {
						unreadByUser[unread.UserID()] = unread
					}
				}
			}
		}

		itemsToCreate := make([]*aggregate.InAppNotification, 0, len(uniqueUserIDs))
		itemsToUpdate := make([]*aggregate.InAppNotification, 0, len(unreadByUser))

		for _, uid := range uniqueUserIDs {
			existingUnread := unreadByUser[uid]
			if existingUnread != nil {
				// Compute rollup in memory (pure in-RAM calculation)
				updatedItem, _ := d.computeRollup(existingUnread, uid, msg)
				itemsToUpdate = append(itemsToUpdate, updatedItem)
			} else {
				// Recipient + post + already read (or new): insert new row
				item, itemErr := aggregate.NewInAppNotification(
					uuid.New().String(),
					uid,
					msg.Title,
					msg.Body,
					msg.Data,
				)
				if itemErr == nil {
					if msg.EventID != "" {
						item.WithEventID(msg.EventID)
					}
					itemsToCreate = append(itemsToCreate, item)
				}
			}
		}

		// 1. Batch UPDATE all rolled up items in 1 query!
		if len(itemsToUpdate) > 0 {
			if rollupRepo, ok := d.notificationRepo.(repository.ReactionRollupRepository); ok {
				if updErr := rollupRepo.BatchUpdateNotificationContent(ctx, itemsToUpdate); updErr != nil {
					return nil, updErr
				}
			}
		}

		// 2. Batch INSERT all new/read items in 1 query!
		if len(itemsToCreate) > 0 {
			if batchErr := d.notificationRepo.BatchSave(ctx, itemsToCreate); batchErr != nil {
				return nil, batchErr
			}
		}

		return &msg, nil

	case port.ScopeDirect:
		if target.UserID == "" {
			return &msg, nil
		}

		// Check if this is a social notification supporting rollup
		isSocial := msg.Data != nil && msg.Data["type"] == "POST_REACTION" && msg.Data["feedItemId"] != ""
		if isSocial {
			if rollupRepo, ok := d.notificationRepo.(repository.ReactionRollupRepository); ok {
				existing, findErr := rollupRepo.FindUnreadReactionByPost(ctx, target.UserID, msg.Data["feedItemId"])
				if findErr != nil {
					return nil, findErr
				}
				if existing != nil {
					updatedItem, rolledMsg := d.computeRollup(existing, target.UserID, msg)
					if updErr := rollupRepo.UpdateNotificationContent(ctx, updatedItem); updErr != nil {
						return nil, updErr
					}
					return rolledMsg, nil
				}
			}
		}

		// Non-social, or social with no unread notification: insert new row
		notifID := uuid.New().String()
		item, err := aggregate.NewInAppNotification(notifID, target.UserID, msg.Title, msg.Body, msg.Data)
		if err != nil {
			return nil, fmt.Errorf("create in-app notification: %w", err)
		}
		if msg.EventID != "" {
			item.WithEventID(msg.EventID)
		}
		if saveErr := d.notificationRepo.Save(ctx, item); saveErr != nil {
			return nil, saveErr
		}
		return &msg, nil

	default:
		return nil, fmt.Errorf("unsupported target scope: %s", target.Scope)
	}
}

func (d *InAppChannelDispatcher) computeRollup(
	existing *aggregate.InAppNotification,
	userID string,
	msg NotificationMessage,
) (*aggregate.InAppNotification, *NotificationMessage) {
	prevCount := 1
	if rawCount := existing.Data()["rollupCount"]; rawCount != "" {
		if parsed, pErr := strconv.Atoi(rawCount); pErr == nil && parsed > 0 {
			prevCount = parsed
		}
	}
	newCount := prevCount + 1
	mergedData := make(map[string]string, len(msg.Data)+1)
	for k, v := range msg.Data {
		mergedData[k] = v
	}
	mergedData["rollupCount"] = strconv.Itoa(newCount)

	actorName := mergedData["actorName"]
	if actorName == "" {
		actorName = "Một người dùng"
	}
	rolledBody := fmt.Sprintf("%s và %d người khác đã bày tỏ cảm xúc về bài viết của bạn", actorName, newCount-1)
	updated := aggregate.ReconstituteInAppNotification(
		existing.ID(),
		existing.EventID(),
		userID,
		msg.Title,
		rolledBody,
		mergedData,
		false,
		time.Now().UTC(),
	)

	effectiveMsg := msg
	effectiveMsg.Body = rolledBody
	effectiveMsg.Data = mergedData
	return updated, &effectiveMsg
}

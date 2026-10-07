package event

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	notificationv1event "github.com/viethung213/gym-companion/internal/gen/go/contracts/generic/notification/v1/event"
	"github.com/viethung213/gym-companion/internal/social/application/port"
	domainEvent "github.com/viethung213/gym-companion/internal/social/domain/event"
	"google.golang.org/protobuf/encoding/protojson"
)

type OutboxWriter struct {
	outboxRepo port.OutboxRepository
}

var _ port.EventPublisher = (*OutboxWriter)(nil)

func NewOutboxWriter(outboxRepo port.OutboxRepository) *OutboxWriter {
	return &OutboxWriter{outboxRepo: outboxRepo}
}

func (w *OutboxWriter) PublishEvents(ctx context.Context, events []any) error {
	for _, ev := range events {
		if err := w.publishSingleEvent(ctx, ev); err != nil {
			return err
		}
	}
	return nil
}

type UserFollowedEvent = domainEvent.UserFollowedEvent
type PostCreatedEvent = domainEvent.PostCreatedEvent
type PostReactedEvent = domainEvent.PostReactedEvent
type PostCommentedEvent = domainEvent.PostCommentedEvent

func (w *OutboxWriter) publishSingleEvent(ctx context.Context, ev any) error {
	switch e := ev.(type) {
	case *domainEvent.UserFollowedEvent:
		return w.handleUserFollowedNotification(ctx, e)
	case *domainEvent.PostCreatedEvent:
		return w.handlePostCreatedNotification(ctx, e)
	case *domainEvent.PostCommentedEvent:
		return w.handlePostCommentedNotification(ctx, e)
	case *domainEvent.PostReactedEvent:
		return w.handlePostReactedNotification(ctx, e)
	default:
		return nil
	}
}

func (w *OutboxWriter) handleUserFollowedNotification(ctx context.Context, e *domainEvent.UserFollowedEvent) error {
	if e.FollowerID == e.FollowingID || e.FollowingID == "" {
		return nil
	}

	actorName := e.FollowerName
	if actorName == "" {
		actorName = "Một người dùng"
	}

	dataMap := map[string]string{
		"followerId":        e.FollowerID,
		"followerName":      e.FollowerName,
		"followerAvatarUrl": e.FollowerAvatarURL,
		"type":              "USER_FOLLOWED",
	}
	body := actorName + " đã bắt đầu theo dõi bạn"
	return w.publishNotificationRequested(ctx, []string{e.FollowingID}, "Người theo dõi mới", body, dataMap)
}

func (w *OutboxWriter) handlePostCreatedNotification(ctx context.Context, e *domainEvent.PostCreatedEvent) error {
	if len(e.FollowerUserIDs) == 0 {
		return nil
	}

	actorName := e.UserName
	if actorName == "" {
		actorName = "Một người dùng"
	}

	dataMap := map[string]string{
		"feedItemId":     e.PostID,
		"actorId":        e.UserID,
		"actorName":      e.UserName,
		"actorAvatarUrl": e.UserAvatarURL,
		"type":           "POST_CREATED",
	}
	title := "Bài viết mới"
	body := actorName + " vừa đăng một bài viết mới"

	return w.publishNotificationRequested(ctx, e.FollowerUserIDs, title, body, dataMap)
}

func (w *OutboxWriter) handlePostCommentedNotification(ctx context.Context, e *domainEvent.PostCommentedEvent) error {
	actorName := e.UserName
	if actorName == "" {
		actorName = "Một người dùng"
	}

	isReply := e.ParentID != nil && e.ParentCommentOwnerID != nil && *e.ParentCommentOwnerID != ""
	if isReply {
		parentOwner := *e.ParentCommentOwnerID
		if parentOwner == e.PostOwnerID {
			// Chủ bài viết đồng thời là người viết comment cha: chỉ nhận 1 thông báo phản hồi
			if e.UserID != e.PostOwnerID {
				dataMap := map[string]string{
					"feedItemId":     e.FeedItemID,
					"commentId":      e.CommentID,
					"parentId":       *e.ParentID,
					"type":           "COMMENT_REPLY",
					"actorId":        e.UserID,
					"actorName":      e.UserName,
					"actorAvatarUrl": e.UserAvatarURL,
				}
				body := actorName + " đã trả lời bình luận của bạn"
				if err := w.publishNotificationRequested(ctx, []string{e.PostOwnerID}, "Phản hồi mới", body, dataMap); err != nil {
					return err
				}
			}
		} else {
			// Hai đối tượng khác nhau: người viết comment cha và chủ bài viết
			if e.UserID != parentOwner {
				dataMap := map[string]string{
					"feedItemId":     e.FeedItemID,
					"commentId":      e.CommentID,
					"parentId":       *e.ParentID,
					"type":           "COMMENT_REPLY",
					"actorId":        e.UserID,
					"actorName":      e.UserName,
					"actorAvatarUrl": e.UserAvatarURL,
				}
				body := actorName + " đã trả lời bình luận của bạn"
				if err := w.publishNotificationRequested(ctx, []string{parentOwner}, "Phản hồi mới", body, dataMap); err != nil {
					return err
				}
			}
			if e.UserID != e.PostOwnerID {
				dataMap := map[string]string{
					"feedItemId":     e.FeedItemID,
					"commentId":      e.CommentID,
					"parentId":       *e.ParentID,
					"type":           "POST_COMMENT",
					"actorId":        e.UserID,
					"actorName":      e.UserName,
					"actorAvatarUrl": e.UserAvatarURL,
				}
				body := actorName + " đã bình luận trên bài viết của bạn"
				if err := w.publishNotificationRequested(ctx, []string{e.PostOwnerID}, "Bình luận mới", body, dataMap); err != nil {
					return err
				}
			}
		}
		return nil
	}

	// Bình luận trực tiếp vào bài viết (root comment)
	if e.UserID != e.PostOwnerID {
		dataMap := map[string]string{
			"feedItemId":     e.FeedItemID,
			"commentId":      e.CommentID,
			"type":           "POST_COMMENT",
			"actorId":        e.UserID,
			"actorName":      e.UserName,
			"actorAvatarUrl": e.UserAvatarURL,
		}
		body := actorName + " đã bình luận bài viết của bạn"
		if err := w.publishNotificationRequested(ctx, []string{e.PostOwnerID}, "Bình luận mới", body, dataMap); err != nil {
			return err
		}
	}

	return nil
}

func (w *OutboxWriter) handlePostReactedNotification(ctx context.Context, e *domainEvent.PostReactedEvent) error {
	if e.UserID == e.PostOwnerID {
		return nil
	}

	actorName := e.UserName
	if actorName == "" {
		actorName = "Một người dùng"
	}

	dataMap := map[string]string{
		"feedItemId":     e.FeedItemID,
		"type":           "POST_REACTION",
		"reactionType":   string(e.ReactionType),
		"actorId":        e.UserID,
		"actorName":      e.UserName,
		"actorAvatarUrl": e.UserAvatarURL,
	}
	body := actorName + " đã bày tỏ cảm xúc về bài viết của bạn"
	return w.publishNotificationRequested(ctx, []string{e.PostOwnerID}, "Tương tác mới", body, dataMap)
}

func (w *OutboxWriter) publishNotificationRequested(
	ctx context.Context,
	recipientIDs []string,
	title string,
	body string,
	dataMap map[string]string,
) error {
	if len(recipientIDs) == 0 {
		return nil
	}

	var target *notificationv1event.NotificationTarget
	if len(recipientIDs) == 1 {
		target = &notificationv1event.NotificationTarget{
			Recipient: &notificationv1event.NotificationTarget_UserId{
				UserId: recipientIDs[0],
			},
		}
	} else {
		target = &notificationv1event.NotificationTarget{
			Recipient: &notificationv1event.NotificationTarget_UserList{
				UserList: &notificationv1event.UserList{
					UserIds: recipientIDs,
				},
			},
		}
	}

	now := time.Now().UTC()
	notifProto := &notificationv1event.NormalPriorityNotificationRequested{
		Target:      target,
		Title:       title,
		Body:        body,
		Data:        dataMap,
		Channels:    []string{"PUSH"},
		RequestedAt: now.Format(time.RFC3339),
	}

	dataBytes, err := protojson.Marshal(notifProto)
	if err != nil {
		return fmt.Errorf("marshal NormalPriorityNotificationRequested proto: %w", err)
	}

	eventID := uuid.New().String()
	eventType := "contracts.generic.notification.v1.event.NormalPriorityNotificationRequested"
	cloudEvent := map[string]any{
		"specversion":     "1.0",
		"id":              eventID,
		"source":          "services/social-service",
		"type":            eventType,
		"time":            now.Format(time.RFC3339),
		"datacontenttype": "application/json",
		"data":            json.RawMessage(dataBytes),
	}

	envelopeBytes, err := json.Marshal(cloudEvent)
	if err != nil {
		return fmt.Errorf("marshal cloud event envelope: %w", err)
	}

	return w.outboxRepo.Save(ctx, &port.OutboxRecord{
		ID:           uuid.New().String(),
		EventID:      eventID,
		EventType:    eventType,
		Payload:      envelopeBytes,
		PartitionKey: recipientIDs[0],
	})
}

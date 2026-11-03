package event

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	notificationv1event "github.com/viethung213/gym-companion/internal/gen/go/contracts/generic/notification/v1/event"
	domainEvent "github.com/viethung213/gym-companion/internal/social/domain/event"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// CloudEventEnvelope represents the standard CloudEvents 1.0 JSON format envelope.
type CloudEventEnvelope struct {
	SpecVersion     string          `json:"specversion"`
	ID              string          `json:"id"`
	Source          string          `json:"source"`
	Type            string          `json:"type"`
	Time            string          `json:"time"`
	DataContentType string          `json:"datacontenttype"`
	Data            json.RawMessage `json:"data"`
}

// NotificationRecordData holds packaged event data ready for outbox persistence.
type NotificationRecordData struct {
	EventID       string
	AggregateType string
	AggregateID   string
	EventType     string
	PartitionKey  string
	OccurredAt    time.Time
	Payload       []byte
}

// PackNotifications transforms a social domain event into a slice of outbox notification payloads.
func PackNotifications(ev any) ([]NotificationRecordData, error) {
	switch e := ev.(type) {
	case *domainEvent.UserFollowedEvent:
		return packUserFollowedNotification(e)
	case *domainEvent.PostCreatedEvent:
		return packPostCreatedNotification(e)
	case *domainEvent.PostCommentedEvent:
		return packPostCommentedNotification(e)
	case *domainEvent.PostReactedEvent:
		return packPostReactedNotification(e)
	default:
		return nil, nil
	}
}

func packUserFollowedNotification(e *domainEvent.UserFollowedEvent) ([]NotificationRecordData, error) {
	if e.FollowerID == e.FollowingID || e.FollowingID == "" {
		return nil, nil
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

	rec, err := buildNotificationCloudEvent([]string{e.FollowingID}, "Người theo dõi mới", body, dataMap)
	if err != nil {
		return nil, err
	}
	if rec == nil {
		return nil, nil
	}
	return []NotificationRecordData{*rec}, nil
}

func packPostCreatedNotification(e *domainEvent.PostCreatedEvent) ([]NotificationRecordData, error) {
	if len(e.FollowerUserIDs) == 0 {
		return nil, nil
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
	body := actorName + " vừa đăng bài viết mới"

	rec, err := buildNotificationCloudEvent(e.FollowerUserIDs, title, body, dataMap)
	if err != nil {
		return nil, err
	}
	if rec == nil {
		return nil, nil
	}
	return []NotificationRecordData{*rec}, nil
}

func packPostCommentedNotification(e *domainEvent.PostCommentedEvent) ([]NotificationRecordData, error) {
	actorName := e.UserName
	if actorName == "" {
		actorName = "Một người dùng"
	}

	var results []NotificationRecordData
	isReply := e.ParentID != nil && e.ParentCommentOwnerID != nil && *e.ParentCommentOwnerID != ""

	if isReply {
		parentOwner := *e.ParentCommentOwnerID
		if parentOwner == e.PostOwnerID {
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
				rec, err := buildNotificationCloudEvent([]string{e.PostOwnerID}, "Phản hồi mới", body, dataMap)
				if err != nil {
					return nil, err
				}
				if rec != nil {
					results = append(results, *rec)
				}
			}
		} else {
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
				rec, err := buildNotificationCloudEvent([]string{parentOwner}, "Phản hồi mới", body, dataMap)
				if err != nil {
					return nil, err
				}
				if rec != nil {
					results = append(results, *rec)
				}
			}

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
				rec, err := buildNotificationCloudEvent([]string{e.PostOwnerID}, "Bình luận mới", body, dataMap)
				if err != nil {
					return nil, err
				}
				if rec != nil {
					results = append(results, *rec)
				}
			}
		}
	} else if e.UserID != e.PostOwnerID {
		dataMap := map[string]string{
			"feedItemId":     e.FeedItemID,
			"commentId":      e.CommentID,
			"type":           "POST_COMMENT",
			"actorId":        e.UserID,
			"actorName":      e.UserName,
			"actorAvatarUrl": e.UserAvatarURL,
		}
		body := actorName + " đã bình luận bài viết của bạn"
		rec, err := buildNotificationCloudEvent([]string{e.PostOwnerID}, "Bình luận mới", body, dataMap)
		if err != nil {
			return nil, err
		}
		if rec != nil {
			results = append(results, *rec)
		}
	}

	return results, nil
}

func packPostReactedNotification(e *domainEvent.PostReactedEvent) ([]NotificationRecordData, error) {
	if e.UserID == e.PostOwnerID {
		return nil, nil
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

	rec, err := buildNotificationCloudEvent([]string{e.PostOwnerID}, "Tương tác mới", body, dataMap)
	if err != nil {
		return nil, err
	}
	if rec == nil {
		return nil, nil
	}
	return []NotificationRecordData{*rec}, nil
}

func buildNotificationCloudEvent(
	recipientIDs []string,
	title string,
	body string,
	dataMap map[string]string,
) (*NotificationRecordData, error) {
	if len(recipientIDs) == 0 {
		return nil, nil
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
		RequestedAt: timestamppb.New(now),
	}

	dataBytes, err := protojson.Marshal(notifProto)
	if err != nil {
		return nil, fmt.Errorf("marshal NormalPriorityNotificationRequested proto: %w", err)
	}

	eventType := "contracts.generic.notification.v1.event.NormalPriorityNotificationRequested"
	eventID := uuid.New().String()

	cloudEvent := CloudEventEnvelope{
		SpecVersion:     "1.0",
		ID:              eventID,
		Source:          "services/social-service",
		Type:            eventType,
		Time:            now.Format(time.RFC3339),
		DataContentType: "application/json",
		Data:            json.RawMessage(dataBytes),
	}

	envelopeBytes, err := json.Marshal(cloudEvent)
	if err != nil {
		return nil, fmt.Errorf("marshal cloud event envelope: %w", err)
	}

	return &NotificationRecordData{
		EventID:       eventID,
		AggregateType: "SocialNotification",
		AggregateID:   recipientIDs[0],
		EventType:     eventType,
		PartitionKey:  recipientIDs[0],
		OccurredAt:    now,
		Payload:       envelopeBytes,
	}, nil
}

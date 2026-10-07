package event_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	notificationv1event "github.com/viethung213/gym-companion/internal/gen/go/contracts/generic/notification/v1/event"
	"github.com/viethung213/gym-companion/internal/social/application/port"
	domainEvent "github.com/viethung213/gym-companion/internal/social/domain/event"
	"github.com/viethung213/gym-companion/internal/social/domain/vo"
	"github.com/viethung213/gym-companion/internal/social/infrastructure/event"
	"google.golang.org/protobuf/encoding/protojson"
)

type mockOutboxRepo struct {
	records []*port.OutboxRecord
}

func (m *mockOutboxRepo) Save(ctx context.Context, record *port.OutboxRecord) error {
	m.records = append(m.records, record)
	return nil
}

func (m *mockOutboxRepo) FetchUnpublished(ctx context.Context, limit int) ([]*port.OutboxRecord, error) {
	return nil, nil
}

func (m *mockOutboxRepo) ClaimBatch(ctx context.Context, limit int, lockDuration time.Duration) ([]*port.OutboxRecord, error) {
	return nil, nil
}

func (m *mockOutboxRepo) MarkAsPublished(ctx context.Context, ids []string) error {
	return nil
}

type cloudEventEnvelope struct {
	SpecVersion     string          `json:"specversion"`
	ID              string          `json:"id"`
	Source          string          `json:"source"`
	Type            string          `json:"type"`
	Time            string          `json:"time"`
	DataContentType string          `json:"datacontenttype"`
	Data            json.RawMessage `json:"data"`
}

func assertCloudEventEnvelope(t *testing.T, payload []byte, expectedType string) []byte {
	t.Helper()
	var env cloudEventEnvelope
	if err := json.Unmarshal(payload, &env); err != nil {
		t.Fatalf("failed to unmarshal cloud event envelope: %v", err)
	}
	if env.SpecVersion != "1.0" {
		t.Errorf("expected specversion '1.0', got '%s'", env.SpecVersion)
	}
	if env.Source != "services/social-service" {
		t.Errorf("expected source 'services/social-service', got '%s'", env.Source)
	}
	if env.Type != expectedType {
		t.Errorf("expected type '%s', got '%s'", expectedType, env.Type)
	}
	if env.DataContentType != "application/json" {
		t.Errorf("expected datacontenttype 'application/json', got '%s'", env.DataContentType)
	}
	if env.ID == "" {
		t.Errorf("expected non-empty event id")
	}
	if env.Time == "" {
		t.Errorf("expected non-empty event time")
	}
	return env.Data
}

func TestOutboxWriter_PublishUserFollowedEvent(t *testing.T) {
	repo := &mockOutboxRepo{}
	writer := event.NewOutboxWriter(repo)

	ev := &domainEvent.UserFollowedEvent{
		FollowerID:        "follower-123",
		FollowingID:       "following-456",
		FollowedAt:        time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC),
		FollowerName:      "Nguyen Van A",
		FollowerAvatarURL: "https://cdn.example.com/avatar.jpg",
	}

	err := writer.PublishEvents(context.Background(), []any{ev})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(repo.records) != 1 {
		t.Fatalf("expected 1 notification outbox record, got %d", len(repo.records))
	}

	notifRec := repo.records[0]
	if notifRec.EventType != "contracts.generic.notification.v1.event.NormalPriorityNotificationRequested" {
		t.Errorf("unexpected notification event type: %s", notifRec.EventType)
	}
	if notifRec.PartitionKey != "following-456" {
		t.Errorf("expected partition key 'following-456', got '%s'", notifRec.PartitionKey)
	}

	notifData := assertCloudEventEnvelope(t, notifRec.Payload, notifRec.EventType)
	var notifProto notificationv1event.NormalPriorityNotificationRequested
	if err := protojson.Unmarshal(notifData, &notifProto); err != nil {
		t.Fatalf("failed to unmarshal notification proto: %v", err)
	}
	if notifProto.GetTarget().GetUserId() != "following-456" {
		t.Errorf("expected target 'following-456', got '%s'", notifProto.GetTarget().GetUserId())
	}
	if notifProto.Title != "Người theo dõi mới" {
		t.Errorf("unexpected notification title: %s", notifProto.Title)
	}
}

func TestOutboxWriter_PublishPostCreatedEvent(t *testing.T) {
	repo := &mockOutboxRepo{}
	writer := event.NewOutboxWriter(repo)

	ev := &domainEvent.PostCreatedEvent{
		PostID:          "post-100",
		UserID:          "user-creator",
		UserName:        "Author User",
		Content:         "First post!",
		MediaURLs:       []string{"https://img.com/1.png"},
		Visibility:      "PUBLIC",
		FollowerUserIDs: []string{"follower-1", "follower-2"},
		CreatedAt:       time.Date(2026, 10, 5, 12, 1, 0, 0, time.UTC),
	}

	err := writer.PublishEvents(context.Background(), []any{ev})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(repo.records) != 1 {
		t.Fatalf("expected 1 notification outbox record, got %d", len(repo.records))
	}

	notifRec := repo.records[0]
	if notifRec.EventType != "contracts.generic.notification.v1.event.NormalPriorityNotificationRequested" {
		t.Errorf("unexpected notification event type: %s", notifRec.EventType)
	}
	if notifRec.PartitionKey != "follower-1" {
		t.Errorf("expected partition key 'follower-1', got '%s'", notifRec.PartitionKey)
	}

	notifData := assertCloudEventEnvelope(t, notifRec.Payload, notifRec.EventType)
	var notifProto notificationv1event.NormalPriorityNotificationRequested
	if err := protojson.Unmarshal(notifData, &notifProto); err != nil {
		t.Fatalf("failed to unmarshal notification proto: %v", err)
	}
	userList := notifProto.GetTarget().GetUserList()
	if userList == nil || len(userList.GetUserIds()) != 2 {
		t.Fatalf("expected 2 users in userList, got %+v", userList)
	}
	if notifProto.Title != "Bài viết mới" {
		t.Errorf("unexpected title: %s", notifProto.Title)
	}
}

func TestOutboxWriter_PublishPostReactedEvent(t *testing.T) {
	repo := &mockOutboxRepo{}
	writer := event.NewOutboxWriter(repo)

	ev := &domainEvent.PostReactedEvent{
		FeedItemID:    "feed-1",
		PostOwnerID:   "author-1",
		UserID:        "user-react",
		UserName:      "Tran Thi B",
		UserAvatarURL: "https://cdn.example.com/b.jpg",
		ReactionType:  vo.ReactionTypeLike,
		ReactedAt:     time.Date(2026, 10, 5, 12, 5, 0, 0, time.UTC),
	}

	err := writer.PublishEvents(context.Background(), []any{ev})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(repo.records) != 1 {
		t.Fatalf("expected 1 notification outbox record, got %d", len(repo.records))
	}

	notifRec := repo.records[0]
	if notifRec.EventType != "contracts.generic.notification.v1.event.NormalPriorityNotificationRequested" {
		t.Errorf("unexpected notification event type: %s", notifRec.EventType)
	}
	if notifRec.PartitionKey != "author-1" {
		t.Errorf("unexpected partition key for notification: %s", notifRec.PartitionKey)
	}

	notifData := assertCloudEventEnvelope(t, notifRec.Payload, notifRec.EventType)
	var notifProto notificationv1event.NormalPriorityNotificationRequested
	if err := protojson.Unmarshal(notifData, &notifProto); err != nil {
		t.Fatalf("failed to unmarshal notification proto: %v", err)
	}
	if notifProto.GetTarget().GetUserId() != "author-1" {
		t.Errorf("expected target user 'author-1', got '%s'", notifProto.GetTarget().GetUserId())
	}
	if notifProto.Title != "Tương tác mới" {
		t.Errorf("unexpected notification title: %s", notifProto.Title)
	}
}

func TestOutboxWriter_PublishPostCommentedEvent_RootComment(t *testing.T) {
	repo := &mockOutboxRepo{}
	writer := event.NewOutboxWriter(repo)

	ev := &domainEvent.PostCommentedEvent{
		CommentID:     "c-1",
		FeedItemID:    "feed-1",
		PostOwnerID:   "author-1",
		UserID:        "user-comm",
		UserName:      "Le Van C",
		UserAvatarURL: "https://cdn.example.com/c.jpg",
		Content:       "Keep it up!",
		ParentID:      nil,
		CommentedAt:   time.Date(2026, 10, 5, 12, 10, 0, 0, time.UTC),
	}

	err := writer.PublishEvents(context.Background(), []any{ev})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(repo.records) != 1 {
		t.Fatalf("expected 1 notification outbox record, got %d", len(repo.records))
	}

	notifRec := repo.records[0]
	if notifRec.EventType != "contracts.generic.notification.v1.event.NormalPriorityNotificationRequested" {
		t.Errorf("unexpected notification event type: %s", notifRec.EventType)
	}
	if notifRec.PartitionKey != "author-1" {
		t.Errorf("expected partition key 'author-1', got '%s'", notifRec.PartitionKey)
	}
	notifData := assertCloudEventEnvelope(t, notifRec.Payload, notifRec.EventType)
	var notifProto notificationv1event.NormalPriorityNotificationRequested
	if err := protojson.Unmarshal(notifData, &notifProto); err != nil {
		t.Fatalf("failed to unmarshal notification proto: %v", err)
	}
	if notifProto.GetTarget().GetUserId() != "author-1" {
		t.Errorf("expected target 'author-1', got '%s'", notifProto.GetTarget().GetUserId())
	}
	if notifProto.Title != "Bình luận mới" {
		t.Errorf("unexpected title: %s", notifProto.Title)
	}
}

func TestOutboxWriter_PublishPostCommentedEvent_ReplyToComment(t *testing.T) {
	repo := &mockOutboxRepo{}
	writer := event.NewOutboxWriter(repo)

	parentID := "c-parent"
	parentOwner := "parent-author"
	ev := &domainEvent.PostCommentedEvent{
		CommentID:            "c-reply",
		FeedItemID:           "feed-1",
		PostOwnerID:          "post-owner",
		ParentCommentOwnerID: &parentOwner,
		UserID:               "user-replyer",
		UserName:             "Hoang D",
		UserAvatarURL:        "https://cdn.example.com/d.jpg",
		Content:              "I agree!",
		ParentID:             &parentID,
		CommentedAt:          time.Date(2026, 10, 5, 12, 15, 0, 0, time.UTC),
	}

	err := writer.PublishEvents(context.Background(), []any{ev})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(repo.records) != 2 {
		t.Fatalf("expected 2 notification outbox records (reply notif + post owner notif), got %d", len(repo.records))
	}

	// Record 0: reply notification to parent comment author
	recReply := repo.records[0]
	if recReply.PartitionKey != "parent-author" {
		t.Errorf("record 0 recipient should be 'parent-author', got '%s'", recReply.PartitionKey)
	}
	dataReply := assertCloudEventEnvelope(t, recReply.Payload, recReply.EventType)
	var protoReply notificationv1event.NormalPriorityNotificationRequested
	if err := protojson.Unmarshal(dataReply, &protoReply); err != nil {
		t.Fatalf("unmarshal proto reply failed: %v", err)
	}
	if protoReply.GetTarget().GetUserId() != "parent-author" {
		t.Errorf("target user should be parent-author, got %s", protoReply.GetTarget().GetUserId())
	}
	if protoReply.Title != "Phản hồi mới" {
		t.Errorf("expected title 'Phản hồi mới', got '%s'", protoReply.Title)
	}

	// Record 1: comment notification to post owner
	recPostOwner := repo.records[1]
	if recPostOwner.PartitionKey != "post-owner" {
		t.Errorf("record 1 recipient should be 'post-owner', got '%s'", recPostOwner.PartitionKey)
	}
	dataPostOwner := assertCloudEventEnvelope(t, recPostOwner.Payload, recPostOwner.EventType)
	var protoPostOwner notificationv1event.NormalPriorityNotificationRequested
	if err := protojson.Unmarshal(dataPostOwner, &protoPostOwner); err != nil {
		t.Fatalf("unmarshal proto post owner failed: %v", err)
	}
	if protoPostOwner.GetTarget().GetUserId() != "post-owner" {
		t.Errorf("target user should be post-owner, got %s", protoPostOwner.GetTarget().GetUserId())
	}
	if protoPostOwner.Title != "Bình luận mới" {
		t.Errorf("expected title 'Bình luận mới', got '%s'", protoPostOwner.Title)
	}
}

func TestOutboxWriter_PublishSelfInteraction_NoNotification(t *testing.T) {
	repo := &mockOutboxRepo{}
	writer := event.NewOutboxWriter(repo)

	// Self-react
	reactEv := &domainEvent.PostReactedEvent{
		FeedItemID:   "feed-1",
		PostOwnerID:  "same-user",
		UserID:       "same-user",
		ReactionType: vo.ReactionTypeFire,
	}
	if err := writer.PublishEvents(context.Background(), []any{reactEv}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(repo.records) != 0 {
		t.Errorf("expected 0 records for self-react, got %d", len(repo.records))
	}

	// Self-comment
	commentEv := &domainEvent.PostCommentedEvent{
		CommentID:   "c-self",
		FeedItemID:  "feed-1",
		PostOwnerID: "same-user",
		UserID:      "same-user",
		Content:     "My own comment",
	}
	if err := writer.PublishEvents(context.Background(), []any{commentEv}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(repo.records) != 0 {
		t.Errorf("expected 0 records for self-comment, got %d", len(repo.records))
	}
}

func TestOutboxWriter_PublishReplyToSelfComment_OnlyNotifiesPostOwner(t *testing.T) {
	repo := &mockOutboxRepo{}
	writer := event.NewOutboxWriter(repo)

	parentID := "c-parent"
	parentOwner := "user-replyer" // user tự rep chính comment của mình
	ev := &domainEvent.PostCommentedEvent{
		CommentID:            "c-reply-self",
		FeedItemID:           "feed-1",
		PostOwnerID:          "post-owner",
		ParentCommentOwnerID: &parentOwner,
		UserID:               "user-replyer",
		UserName:             "Hoang D",
		Content:              "Adding more thoughts to my own comment",
		ParentID:             &parentID,
		CommentedAt:          time.Date(2026, 10, 5, 12, 20, 0, 0, time.UTC),
	}

	err := writer.PublishEvents(context.Background(), []any{ev})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Mong đợi 1 record duy nhất cho post-owner
	if len(repo.records) != 1 {
		t.Fatalf("expected 1 outbox record (post-owner notif only), got %d", len(repo.records))
	}

	notifRec := repo.records[0]
	if notifRec.PartitionKey != "post-owner" {
		t.Errorf("notification should be sent to 'post-owner', got '%s'", notifRec.PartitionKey)
	}

	notifData := assertCloudEventEnvelope(t, notifRec.Payload, notifRec.EventType)
	var notifProto notificationv1event.NormalPriorityNotificationRequested
	if err := protojson.Unmarshal(notifData, &notifProto); err != nil {
		t.Fatalf("unmarshal proto failed: %v", err)
	}
	if notifProto.GetTarget().GetUserId() != "post-owner" {
		t.Errorf("target user should be post-owner, got %s", notifProto.GetTarget().GetUserId())
	}
	if notifProto.Title != "Bình luận mới" {
		t.Errorf("expected title 'Bình luận mới', got '%s'", notifProto.Title)
	}
}

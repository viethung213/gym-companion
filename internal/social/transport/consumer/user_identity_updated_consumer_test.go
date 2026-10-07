package consumer_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/segmentio/kafka-go"
	profilev1event "github.com/viethung213/gym-companion/internal/gen/go/contracts/supporting/profile/v1/event"
	"github.com/viethung213/gym-companion/internal/social/application/command"
	"github.com/viethung213/gym-companion/internal/social/domain/entity"
	socialConsumer "github.com/viethung213/gym-companion/internal/social/transport/consumer"
	"google.golang.org/protobuf/encoding/protojson"
)

type mockSnapshotRepoForUserIdentityUpdated struct {
	users map[string]*entity.UserSnapshot
}

func (m *mockSnapshotRepoForUserIdentityUpdated) GetByID(ctx context.Context, id string) (*entity.UserSnapshot, error) {
	return m.users[id], nil
}

func (m *mockSnapshotRepoForUserIdentityUpdated) GetByIDs(ctx context.Context, ids []string) (map[string]*entity.UserSnapshot, error) {
	res := make(map[string]*entity.UserSnapshot)
	for _, id := range ids {
		if u, ok := m.users[id]; ok {
			res[id] = u
		}
	}
	return res, nil
}

func (m *mockSnapshotRepoForUserIdentityUpdated) Upsert(ctx context.Context, user *entity.UserSnapshot) error {
	m.users[user.ID()] = user
	return nil
}

func (m *mockSnapshotRepoForUserIdentityUpdated) UpdateRole(ctx context.Context, userID, newRole string) error {
	if u, ok := m.users[userID]; ok {
		u.UpdateRole(newRole)
	}
	return nil
}

func (m *mockSnapshotRepoForUserIdentityUpdated) SearchUsers(ctx context.Context, query, role string, limit int, cursor string) ([]*entity.UserSnapshot, string, int32, error) {
	return nil, "", 0, nil
}

type mockTxForUserIdentityUpdated struct{}

func (m *mockTxForUserIdentityUpdated) WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(ctx)
}

func TestUserIdentityUpdatedConsumer_ProcessMessage(t *testing.T) {
	repo := &mockSnapshotRepoForUserIdentityUpdated{users: make(map[string]*entity.UserSnapshot)}
	tx := &mockTxForUserIdentityUpdated{}
	handler := command.NewSyncUserSnapshotHandler(repo, tx)
	consumer := socialConsumer.NewUserIdentityUpdatedConsumer(nil, handler, nil)

	data := &profilev1event.UserIdentityUpdated{
		UserId:    "u-updated-1",
		FullName:  "Bob Builder",
		AvatarUrl: "https://avatar.com/bob.png",
	}
	dataBytes, _ := protojson.Marshal(data)

	cloudEvent := map[string]any{
		"specversion":     "1.0",
		"id":              "evt-2",
		"source":          "profile.service",
		"type":            "contracts.supporting.profile.v1.event.UserIdentityUpdated",
		"datacontenttype": "application/json",
		"data":            json.RawMessage(dataBytes),
	}
	envelopeBytes, _ := json.Marshal(cloudEvent)

	msg := kafka.Message{
		Key:   []byte("u-updated-1"),
		Value: envelopeBytes,
	}

	err := consumer.ProcessMessage(context.Background(), msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	u, _ := repo.GetByID(context.Background(), "u-updated-1")
	if u == nil {
		t.Fatalf("expected user snapshot to be updated")
	}
	if u.FullName() != "Bob Builder" || u.AvatarURL() != "https://avatar.com/bob.png" {
		t.Errorf("unexpected user data: %+v", u)
	}
}

func TestUserIdentityUpdatedConsumer_StartCancellation(t *testing.T) {
	consumer := socialConsumer.NewUserIdentityUpdatedConsumer(nil, nil, nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	done := make(chan struct{})
	go func() {
		consumer.Start(ctx)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatalf("consumer did not stop upon cancelled context")
	}
}

func TestUserIdentityUpdatedConsumer_EdgeCases(t *testing.T) {
	consumer := socialConsumer.NewUserIdentityUpdatedConsumer(nil, nil, nil)

	// Invalid JSON
	err := consumer.ProcessMessage(context.Background(), kafka.Message{
		Value: []byte("{not-json"),
	})
	if err == nil {
		t.Errorf("expected error for invalid json, got nil")
	}

	// Ignored Event Type
	b, _ := json.Marshal(map[string]any{"type": "other.event"})
	err = consumer.ProcessMessage(context.Background(), kafka.Message{Value: b})
	if err != nil {
		t.Errorf("expected nil for ignored event type, got %v", err)
	}
}

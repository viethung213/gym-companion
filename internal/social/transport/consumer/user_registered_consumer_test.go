package consumer_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/segmentio/kafka-go"
	authv1event "github.com/viethung213/gym-companion/internal/gen/go/contracts/generic/auth/v1/event"
	"github.com/viethung213/gym-companion/internal/social/application/command"
	"github.com/viethung213/gym-companion/internal/social/domain/entity"
	socialConsumer "github.com/viethung213/gym-companion/internal/social/transport/consumer"
	"google.golang.org/protobuf/encoding/protojson"
)

type mockSnapshotRepoForUserRegistered struct {
	users map[string]*entity.UserSnapshot
}

func (m *mockSnapshotRepoForUserRegistered) GetByID(ctx context.Context, id string) (*entity.UserSnapshot, error) {
	return m.users[id], nil
}

func (m *mockSnapshotRepoForUserRegistered) GetByIDs(ctx context.Context, ids []string) (map[string]*entity.UserSnapshot, error) {
	res := make(map[string]*entity.UserSnapshot)
	for _, id := range ids {
		if u, ok := m.users[id]; ok {
			res[id] = u
		}
	}
	return res, nil
}

func (m *mockSnapshotRepoForUserRegistered) Upsert(ctx context.Context, user *entity.UserSnapshot) error {
	m.users[user.ID()] = user
	return nil
}

func (m *mockSnapshotRepoForUserRegistered) UpdateRole(ctx context.Context, userID, newRole string) error {
	if u, ok := m.users[userID]; ok {
		u.UpdateRole(newRole)
	}
	return nil
}

func (m *mockSnapshotRepoForUserRegistered) SearchUsers(ctx context.Context, query, role string, limit int, cursor string) ([]*entity.UserSnapshot, string, int32, error) {
	return nil, "", 0, nil
}

type mockTxForUserRegistered struct{}

func (m *mockTxForUserRegistered) WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(ctx)
}

func TestUserRegisteredConsumer_ProcessMessage(t *testing.T) {
	repo := &mockSnapshotRepoForUserRegistered{users: make(map[string]*entity.UserSnapshot)}
	tx := &mockTxForUserRegistered{}
	handler := command.NewSyncUserSnapshotHandler(repo, tx)
	consumer := socialConsumer.NewUserRegisteredConsumer(nil, handler, nil)

	data := &authv1event.UserRegistered{
		UserId:    "u-new-1",
		FullName:  "Alice Wonderland",
		AvatarUrl: "https://avatar.com/alice.png",
	}
	dataBytes, _ := protojson.Marshal(data)

	cloudEvent := map[string]any{
		"specversion":     "1.0",
		"id":              "evt-1",
		"source":          "auth.service",
		"type":            "contracts.generic.auth.v1.userRegistered",
		"datacontenttype": "application/json",
		"data":            json.RawMessage(dataBytes),
	}
	envelopeBytes, _ := json.Marshal(cloudEvent)

	msg := kafka.Message{
		Key:   []byte("u-new-1"),
		Value: envelopeBytes,
	}

	err := consumer.ProcessMessage(context.Background(), msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	u, _ := repo.GetByID(context.Background(), "u-new-1")
	if u == nil {
		t.Fatalf("expected user snapshot to be created")
	}
	if u.FullName() != "Alice Wonderland" || u.AvatarURL() != "https://avatar.com/alice.png" {
		t.Errorf("unexpected user data: %+v", u)
	}
}

func TestUserRegisteredConsumer_StartCancellation(t *testing.T) {
	consumer := socialConsumer.NewUserRegisteredConsumer(nil, nil, nil)
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

func TestUserRegisteredConsumer_EdgeCases(t *testing.T) {
	consumer := socialConsumer.NewUserRegisteredConsumer(nil, nil, nil)

	// 1. Invalid JSON
	err := consumer.ProcessMessage(context.Background(), kafka.Message{
		Value: []byte("{invalid-json"),
	})
	if err == nil {
		t.Errorf("expected error for invalid json, got nil")
	}

	// 2. Ignored Event Type
	cloudEvent := map[string]any{
		"id":   "evt-ignored",
		"type": "unrelated.event.type",
	}
	b, _ := json.Marshal(cloudEvent)
	err = consumer.ProcessMessage(context.Background(), kafka.Message{Value: b})
	if err != nil {
		t.Errorf("expected nil for ignored event type, got %v", err)
	}
}

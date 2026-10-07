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
	"google.golang.org/protobuf/types/known/timestamppb"
)

type mockSnapshotRepoForUserRoleUpdated struct {
	users map[string]*entity.UserSnapshot
}

func (m *mockSnapshotRepoForUserRoleUpdated) GetByID(ctx context.Context, id string) (*entity.UserSnapshot, error) {
	return m.users[id], nil
}

func (m *mockSnapshotRepoForUserRoleUpdated) GetByIDs(ctx context.Context, ids []string) (map[string]*entity.UserSnapshot, error) {
	return nil, nil
}

func (m *mockSnapshotRepoForUserRoleUpdated) Upsert(ctx context.Context, user *entity.UserSnapshot) error {
	m.users[user.ID()] = user
	return nil
}

func (m *mockSnapshotRepoForUserRoleUpdated) UpdateRole(ctx context.Context, userID, newRole string) error {
	if u, ok := m.users[userID]; ok {
		u.UpdateRole(newRole)
	}
	return nil
}

func (m *mockSnapshotRepoForUserRoleUpdated) SearchUsers(ctx context.Context, query string, role string, limit int, cursor string) ([]*entity.UserSnapshot, string, int32, error) {
	return nil, "", 0, nil
}

type mockTxForUserRoleUpdated struct{}

func (m *mockTxForUserRoleUpdated) WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(ctx)
}

func TestUserRoleUpdatedConsumer_ProcessMessage(t *testing.T) {
	repo := &mockSnapshotRepoForUserRoleUpdated{users: make(map[string]*entity.UserSnapshot)}
	repo.users["u-100"] = entity.NewUserSnapshot("u-100", "Brand User", "https://brand.com/logo.png", "user", time.Now())
	tx := &mockTxForUserRoleUpdated{}
	handler := command.NewUpdateUserRoleHandler(repo, tx)
	consumer := socialConsumer.NewUserRoleUpdatedConsumer(nil, handler, nil)

	data := &authv1event.UserRoleUpdated{
		UserId:    "u-100",
		OldRole:   "user",
		NewRole:   "brand",
		UpdatedAt: timestamppb.New(time.Now()),
	}
	dataBytes, _ := protojson.Marshal(data)

	cloudEvent := map[string]any{
		"specversion":     "1.0",
		"id":              "evt-role-1",
		"source":          "auth.service",
		"type":            "contracts.generic.auth.v1.userRoleUpdated",
		"datacontenttype": "application/json",
		"data":            json.RawMessage(dataBytes),
	}
	envelopeBytes, _ := json.Marshal(cloudEvent)

	msg := kafka.Message{
		Key:   []byte("u-100"),
		Value: envelopeBytes,
	}

	err := consumer.ProcessMessage(context.Background(), msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	u := repo.users["u-100"]
	if u.Role() != "brand" {
		t.Errorf("expected role 'brand', got %s", u.Role())
	}
}

func TestUserRoleUpdatedConsumer_StartCancellation(t *testing.T) {
	consumer := socialConsumer.NewUserRoleUpdatedConsumer(nil, nil, nil)
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

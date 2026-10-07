//go:build unit

package event_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	profilev1event "github.com/viethung213/gym-companion/internal/gen/go/contracts/supporting/profile/v1/event"
	"github.com/viethung213/gym-companion/internal/profile/application/port"
	"github.com/viethung213/gym-companion/internal/profile/domain/entity"
	domainEvent "github.com/viethung213/gym-companion/internal/profile/domain/event"
	"github.com/viethung213/gym-companion/internal/profile/domain/vo"
	"github.com/viethung213/gym-companion/internal/profile/infrastructure/event"
	"google.golang.org/protobuf/encoding/protojson"
)

type mockOutboxRepo struct {
	records  []*port.OutboxRecord
	failSave bool
}

func (m *mockOutboxRepo) Save(ctx context.Context, record *port.OutboxRecord) error {
	if m.failSave {
		return errors.New("outbox save error")
	}
	m.records = append(m.records, record)
	return nil
}

func (m *mockOutboxRepo) FetchUnpublished(ctx context.Context, limit int) ([]*port.OutboxRecord, error) {
	return m.records, nil
}

func (m *mockOutboxRepo) ClaimBatch(ctx context.Context, _ int, _ time.Duration) ([]*port.OutboxRecord, error) {
	return m.records, nil
}
func (m *mockOutboxRepo) MarkAsPublished(ctx context.Context, ids []string) error {
	return nil
}

func (m *mockOutboxRepo) ProcessBatch(ctx context.Context, _ int, publishFn func(ctx context.Context, records []*port.OutboxRecord) error) error {
	return publishFn(ctx, m.records)
}

func TestOutboxWriter(t *testing.T) {
	repo := &mockOutboxRepo{}
	writer := event.NewOutboxWriter(repo)
	ctx := context.Background()

	t.Run("Publish ProfileCompletedEvent", func(t *testing.T) {
		bio, err := vo.NewBiologicalMetrics(70.0, 175.0, 25, "MALE")
		require.NoError(t, err)

		inj, _ := entity.NewInjury("inj-compl", "Back", "MILD", "", time.Now())
		ev := domainEvent.NewProfileCompletedEvent("user-ev-1", bio, []string{"FAT_LOSS"}, []*entity.Injury{inj}, []string{"MORNING"}, time.Now())
		err = writer.PublishEvents(ctx, []any{ev})
		require.NoError(t, err)
		assert.Len(t, repo.records, 1)
		assert.Equal(t, "contracts.supporting.profile.v1.event.ProfileCompleted", repo.records[0].EventType)

		// Fail save
		repo.failSave = true
		err = writer.PublishEvents(ctx, []any{ev})
		assert.Error(t, err)
		repo.failSave = false
	})

	t.Run("Publish InjuryReportedEvent", func(t *testing.T) {
		inj, err := entity.NewInjury("inj-ev-1", "Knee", "MODERATE", "Pain", time.Now())
		require.NoError(t, err)

		ev := domainEvent.NewInjuryReportedEvent("user-ev-1", inj)
		err = writer.PublishEvents(ctx, []any{ev})
		require.NoError(t, err)
		assert.Len(t, repo.records, 2)
		assert.Equal(t, "contracts.supporting.profile.v1.event.InjuryReported", repo.records[1].EventType)

		// Fail save
		repo.failSave = true
		err = writer.PublishEvents(ctx, []any{ev})
		assert.Error(t, err)
		repo.failSave = false
	})

	t.Run("Publish InjuryRecoveredEvent", func(t *testing.T) {
		inj, err := entity.NewInjury("inj-ev-2", "Shoulder", "SEVERE", "Tear", time.Now())
		require.NoError(t, err)

		ev := domainEvent.NewInjuryRecoveredEvent("user-ev-1", inj, time.Now())
		err = writer.PublishEvents(ctx, []any{ev})
		require.NoError(t, err)
		assert.Len(t, repo.records, 3)
		assert.Equal(t, "contracts.supporting.profile.v1.event.InjuryRecovered", repo.records[2].EventType)

		// Fail save
		repo.failSave = true
		err = writer.PublishEvents(ctx, []any{ev})
		assert.Error(t, err)
		repo.failSave = false
	})

	t.Run("Publish UserIdentityUpdatedEvent", func(t *testing.T) {
		ev := domainEvent.NewUserIdentityUpdatedEvent(
			"user-identity-event-1",
			"Tran Van C",
			"https://example.com/avatar_c.png",
			time.Now(),
		)
		err := writer.PublishEvents(ctx, []any{ev})
		require.NoError(t, err)
		assert.Len(t, repo.records, 4)

		lastRecord := repo.records[3]
		assert.Equal(t, "contracts.supporting.profile.v1.event.UserIdentityUpdated", lastRecord.EventType)
		assert.Equal(t, "user-identity-event-1", lastRecord.PartitionKey)

		// Verify CloudEvent envelope structure
		var cloudEvent struct {
			SpecVersion string          `json:"specversion"`
			ID          string          `json:"id"`
			Source      string          `json:"source"`
			Type        string          `json:"type"`
			Time        string          `json:"time"`
			Data        json.RawMessage `json:"data"`
		}
		require.NoError(t, json.Unmarshal(lastRecord.Payload, &cloudEvent))
		assert.Equal(t, "1.0", cloudEvent.SpecVersion)
		assert.Equal(t, "services/profile-service", cloudEvent.Source)
		assert.Equal(t, "contracts.supporting.profile.v1.event.UserIdentityUpdated", cloudEvent.Type)

		// Verify protobuf payload
		var protoData profilev1event.UserIdentityUpdated
		require.NoError(t, protojson.Unmarshal(cloudEvent.Data, &protoData))
		assert.Equal(t, "user-identity-event-1", protoData.GetUserId())
		assert.Equal(t, "Tran Van C", protoData.GetFullName())
		assert.Equal(t, "https://example.com/avatar_c.png", protoData.GetAvatarUrl())

		// Fail save
		repo.failSave = true
		err = writer.PublishEvents(ctx, []any{ev})
		assert.Error(t, err)
		repo.failSave = false
	})

	t.Run("Publish unsupported event returns error", func(t *testing.T) {
		err := writer.PublishEvents(ctx, []any{"unsupported-string-event"})
		assert.Error(t, err)
	})
}

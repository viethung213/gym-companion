package event_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/gamification/application/port"
	domainEvent "github.com/viethung213/gym-companion/internal/gamification/domain/event"
	infraEvent "github.com/viethung213/gym-companion/internal/gamification/infrastructure/event"
)

type mockOutboxRepository struct {
	records []*port.OutboxRecord
	err     error
}

func (m *mockOutboxRepository) Save(ctx context.Context, record *port.OutboxRecord) error {
	if m.err != nil {
		return m.err
	}
	m.records = append(m.records, record)
	return nil
}

func (m *mockOutboxRepository) ClaimBatch(
	ctx context.Context,
	limit int,
	lockDuration time.Duration,
) ([]*port.OutboxRecord, error) {
	return nil, nil
}

func (m *mockOutboxRepository) MarkAsPublished(ctx context.Context, ids []string) error {
	return nil
}

func (m *mockOutboxRepository) MarkAsFailed(ctx context.Context, ids []string, lastErr error) error {
	return nil
}

func (m *mockOutboxRepository) ProcessBatch(
	ctx context.Context,
	limit int,
	publishFn func(ctx context.Context, records []*port.OutboxRecord) error,
) error {
	return nil
}

func TestOutboxWriter_WriteEvents(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 10, 13, 10, 0, 0, 0, time.UTC)

	tests := []struct {
		name          string
		aggregateType string
		aggregateID   string
		events        []any
		wantType      string
		wantPartition string
	}{
		{
			name:          "xp earned event",
			aggregateType: "user_xp",
			aggregateID:   "user-123",
			events: []any{
				domainEvent.XpEarned{
					UserID:   "user-123",
					DeltaXp:  50,
					NewXp:    150,
					NewLevel: 2,
					Source:   "WORKOUT",
					EarnedAt: now,
				},
			},
			wantType:      "contracts.supporting.gamification.v1.xpEarned",
			wantPartition: "user-123",
		},
		{
			name:          "user leveled up event",
			aggregateType: "user_xp",
			aggregateID:   "user-123",
			events: []any{
				domainEvent.UserLeveledUp{
					UserID:      "user-123",
					OldLevel:    1,
					NewLevel:    2,
					TotalXp:     150,
					LeveledUpAt: now,
				},
			},
			wantType:      "contracts.supporting.gamification.v1.userLeveledUp",
			wantPartition: "user-123",
		},
		{
			name:          "streak advanced event",
			aggregateType: "user_streak",
			aggregateID:   "user-456",
			events: []any{
				domainEvent.StreakAdvanced{
					UserID:        "user-456",
					NewStreak:     7,
					LongestStreak: 7,
					ActivityDate:  "2026-10-13",
					ActivityType:  "WORKOUT",
					AdvancedAt:    now,
				},
			},
			wantType:      "contracts.supporting.gamification.v1.streakAdvanced",
			wantPartition: "user-456",
		},
		{
			name:          "streak reset event",
			aggregateType: "user_streak",
			aggregateID:   "user-456",
			events: []any{
				domainEvent.StreakReset{
					UserID:         "user-456",
					PreviousStreak: 5,
					ResetDate:      "2026-10-13",
					ResetAt:        now,
				},
			},
			wantType:      "contracts.supporting.gamification.v1.streakReset",
			wantPartition: "user-456",
		},
		{
			name:          "streak milestone reached event",
			aggregateType: "user_streak",
			aggregateID:   "user-789",
			events: []any{
				domainEvent.StreakMilestoneReached{
					UserID:        "user-789",
					MilestoneDays: 30,
					ReachedAt:     now,
				},
			},
			wantType:      "contracts.supporting.gamification.v1.streakMilestoneReached",
			wantPartition: "user-789",
		},
		{
			name:          "badge unlocked event",
			aggregateType: "user_badges",
			aggregateID:   "user-999",
			events: []any{
				domainEvent.BadgeUnlocked{
					UserID:     "user-999",
					BadgeCode:  "FIRST_WORKOUT",
					BadgeName:  "First Workout",
					Category:   "MILESTONE",
					UnlockedAt: now,
				},
			},
			wantType:      "contracts.supporting.gamification.v1.badgeUnlocked",
			wantPartition: "user-999",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			repo := &mockOutboxRepository{}
			writer := infraEvent.NewOutboxWriter(repo)

			err := writer.WriteEvents(ctx, tt.aggregateType, tt.aggregateID, tt.events)
			if err != nil {
				t.Fatalf("WriteEvents() error = %v, want nil", err)
			}

			if got := len(repo.records); got != len(tt.events) {
				t.Fatalf("saved records count = %d, want %d", got, len(tt.events))
			}

			rec := repo.records[0]
			if got := rec.EventType; got != tt.wantType {
				t.Errorf("EventType = %q, want %q", got, tt.wantType)
			}
			if got := rec.PartitionKey; got != tt.wantPartition {
				t.Errorf("PartitionKey = %q, want %q", got, tt.wantPartition)
			}

			// Validate CloudEvent 1.0 envelope
			var envelope map[string]any
			if err := json.Unmarshal(rec.Payload, &envelope); err != nil {
				t.Fatalf("unmarshal cloudevent payload: %v", err)
			}

			if got := envelope["specversion"]; got != "1.0" {
				t.Errorf("specversion = %v, want '1.0'", got)
			}
			if got := envelope["source"]; got != "services/gamification-service" {
				t.Errorf("source = %v, want 'services/gamification-service'", got)
			}
			if got := envelope["type"]; got != tt.wantType {
				t.Errorf("type = %v, want %q", got, tt.wantType)
			}
			if got := envelope["datacontenttype"]; got != "application/json" {
				t.Errorf("datacontenttype = %v, want 'application/json'", got)
			}
			if envelope["data"] == nil {
				t.Errorf("data field in cloudevent must not be nil")
			}
		})
	}
}

func TestOutboxWriter_RepositoryError(t *testing.T) {
	ctx := context.Background()
	wantErr := errors.New("database connection failed")
	repo := &mockOutboxRepository{err: wantErr}
	writer := infraEvent.NewOutboxWriter(repo)

	events := []any{
		domainEvent.XpEarned{UserID: "user-1", DeltaXp: 10},
	}

	err := writer.WriteEvents(ctx, "user_xp", "user-1", events)
	if !errors.Is(err, wantErr) {
		t.Errorf("WriteEvents() error = %v, want %v", err, wantErr)
	}
}

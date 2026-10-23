package event

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/viethung213/gym-companion/internal/gamification/application/port"
	domainEvent "github.com/viethung213/gym-companion/internal/gamification/domain/event"
	gamificationv1event "github.com/viethung213/gym-companion/internal/gen/go/contracts/supporting/gamification/v1/event"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// NameProvider is implemented by events that provide their event type name.
type NameProvider interface {
	EventName() string
}

// OutboxWriter implements port.OutboxWriter.
type OutboxWriter struct {
	outboxRepo port.OutboxRepository
}

// Compile-time interface check.
var _ port.OutboxWriter = (*OutboxWriter)(nil)

// NewOutboxWriter constructs a new OutboxWriter.
func NewOutboxWriter(outboxRepo port.OutboxRepository) *OutboxWriter {
	return &OutboxWriter{outboxRepo: outboxRepo}
}

// WriteEvents serializes domain events to CloudEvents 1.0 format and stores them in the Outbox.
func (w *OutboxWriter) WriteEvents(
	ctx context.Context,
	aggregateType string,
	aggregateID string,
	events []any,
) error {
	for _, ev := range events {
		if err := w.writeSingleEvent(ctx, aggregateType, aggregateID, ev); err != nil {
			return err
		}
	}
	return nil
}

func (w *OutboxWriter) writeSingleEvent(
	ctx context.Context,
	aggregateType string,
	aggregateID string,
	ev any,
) error {
	eventType := "contracts.supporting.gamification.v1.unknownEvent"
	if np, ok := ev.(NameProvider); ok {
		eventType = np.EventName()
	}

	var payloadBytes []byte
	var err error

	if protoMsg := mapToProtoEvent(ev); protoMsg != nil {
		payloadBytes, err = protojson.Marshal(protoMsg)
		if err != nil {
			return fmt.Errorf("marshal proto event payload: %w", err)
		}
	} else {
		payloadBytes, err = json.Marshal(ev)
		if err != nil {
			return fmt.Errorf("marshal event payload: %w", err)
		}
	}

	outboxID := uuid.NewString()
	eventID := uuid.NewString()
	now := time.Now().UTC().Format(time.RFC3339)

	cloudEvent := map[string]any{
		"specversion":     "1.0",
		"id":              eventID,
		"source":          "services/gamification-service",
		"type":            eventType,
		"time":            now,
		"datacontenttype": "application/json",
		"data":            json.RawMessage(payloadBytes),
	}

	envelopeBytes, err := json.Marshal(cloudEvent)
	if err != nil {
		return fmt.Errorf("marshal cloudevent envelope: %w", err)
	}

	partitionKey := extractUserID(ev)
	if partitionKey == "" {
		partitionKey = aggregateID
	}

	record := &port.OutboxRecord{
		ID:            outboxID,
		EventID:       eventID,
		AggregateType: aggregateType,
		AggregateID:   aggregateID,
		EventType:     eventType,
		Payload:       envelopeBytes,
		PartitionKey:  partitionKey,
		Published:     false,
		CreatedAt:     time.Now().UTC(),
	}

	if err := w.outboxRepo.Save(ctx, record); err != nil {
		return fmt.Errorf("save outbox record: %w", err)
	}

	return nil
}

func extractUserID(ev any) string {
	if ev == nil {
		return ""
	}

	if de, ok := ev.(domainEvent.Event); ok {
		return de.PartitionKey()
	}

	switch e := ev.(type) {
	case domainEvent.XpEarned:
		return e.UserID
	case *domainEvent.XpEarned:
		if e != nil {
			return e.UserID
		}
	case domainEvent.UserLeveledUp:
		return e.UserID
	case *domainEvent.UserLeveledUp:
		if e != nil {
			return e.UserID
		}
	case domainEvent.StreakAdvanced:
		return e.UserID
	case *domainEvent.StreakAdvanced:
		if e != nil {
			return e.UserID
		}
	case domainEvent.StreakReset:
		return e.UserID
	case *domainEvent.StreakReset:
		if e != nil {
			return e.UserID
		}
	case domainEvent.StreakMilestoneReached:
		return e.UserID
	case *domainEvent.StreakMilestoneReached:
		if e != nil {
			return e.UserID
		}
	case domainEvent.BadgeUnlocked:
		return e.UserID
	case *domainEvent.BadgeUnlocked:
		if e != nil {
			return e.UserID
		}
	case domainEvent.CoinsEarned:
		return e.UserID
	case *domainEvent.CoinsEarned:
		if e != nil {
			return e.UserID
		}
	case domainEvent.CoinsSpent:
		return e.UserID
	case *domainEvent.CoinsSpent:
		if e != nil {
			return e.UserID
		}
	}

	return ""
}

func mapToProtoEvent(ev any) proto.Message {
	switch e := ev.(type) {
	case domainEvent.XpEarned:
		return &gamificationv1event.XpEarned{
			UserId:   e.UserID,
			DeltaXp:  e.DeltaXp,
			NewXp:    e.NewXp,
			NewLevel: e.NewLevel,
			Source:   e.Source,
			EarnedAt: timestamppb.New(e.EarnedAt),
		}
	case *domainEvent.XpEarned:
		if e == nil {
			return nil
		}
		return &gamificationv1event.XpEarned{
			UserId:   e.UserID,
			DeltaXp:  e.DeltaXp,
			NewXp:    e.NewXp,
			NewLevel: e.NewLevel,
			Source:   e.Source,
			EarnedAt: timestamppb.New(e.EarnedAt),
		}
	case domainEvent.UserLeveledUp:
		return &gamificationv1event.UserLeveledUp{
			UserId:      e.UserID,
			OldLevel:    e.OldLevel,
			NewLevel:    e.NewLevel,
			TotalXp:     e.TotalXp,
			LeveledUpAt: timestamppb.New(e.LeveledUpAt),
		}
	case *domainEvent.UserLeveledUp:
		if e == nil {
			return nil
		}
		return &gamificationv1event.UserLeveledUp{
			UserId:      e.UserID,
			OldLevel:    e.OldLevel,
			NewLevel:    e.NewLevel,
			TotalXp:     e.TotalXp,
			LeveledUpAt: timestamppb.New(e.LeveledUpAt),
		}
	case domainEvent.StreakAdvanced:
		return &gamificationv1event.StreakAdvanced{
			UserId:        e.UserID,
			NewStreak:     e.NewStreak,
			LongestStreak: e.LongestStreak,
			ActivityDate:  e.ActivityDate,
			ActivityType:  e.ActivityType,
			AdvancedAt:    timestamppb.New(e.AdvancedAt),
		}
	case *domainEvent.StreakAdvanced:
		if e == nil {
			return nil
		}
		return &gamificationv1event.StreakAdvanced{
			UserId:        e.UserID,
			NewStreak:     e.NewStreak,
			LongestStreak: e.LongestStreak,
			ActivityDate:  e.ActivityDate,
			ActivityType:  e.ActivityType,
			AdvancedAt:    timestamppb.New(e.AdvancedAt),
		}
	case domainEvent.StreakReset:
		return &gamificationv1event.StreakReset{
			UserId:         e.UserID,
			PreviousStreak: e.PreviousStreak,
			ResetDate:      e.ResetDate,
			ResetAt:        timestamppb.New(e.ResetAt),
		}
	case *domainEvent.StreakReset:
		if e == nil {
			return nil
		}
		return &gamificationv1event.StreakReset{
			UserId:         e.UserID,
			PreviousStreak: e.PreviousStreak,
			ResetDate:      e.ResetDate,
			ResetAt:        timestamppb.New(e.ResetAt),
		}
	case domainEvent.StreakMilestoneReached:
		return mapStreakMilestoneReached(&e)
	case *domainEvent.StreakMilestoneReached:
		if e == nil {
			return nil
		}
		return mapStreakMilestoneReached(e)
	case domainEvent.BadgeUnlocked:
		return &gamificationv1event.BadgeUnlocked{
			UserId:     e.UserID,
			BadgeCode:  e.BadgeCode,
			BadgeName:  e.BadgeName,
			Category:   e.Category,
			UnlockedAt: timestamppb.New(e.UnlockedAt),
		}
	case *domainEvent.BadgeUnlocked:
		if e == nil {
			return nil
		}
		return &gamificationv1event.BadgeUnlocked{
			UserId:     e.UserID,
			BadgeCode:  e.BadgeCode,
			BadgeName:  e.BadgeName,
			Category:   e.Category,
			UnlockedAt: timestamppb.New(e.UnlockedAt),
		}
	case domainEvent.CoinsEarned:
		return &gamificationv1event.CoinsEarned{
			UserId:       e.UserID,
			Amount:       e.Amount,
			BalanceAfter: e.BalanceAfter,
			Reason:       e.Reason,
			ReferenceId:  e.ReferenceID,
			EarnedAt:     timestamppb.New(e.EarnedAt),
		}
	case *domainEvent.CoinsEarned:
		if e == nil {
			return nil
		}
		return &gamificationv1event.CoinsEarned{
			UserId:       e.UserID,
			Amount:       e.Amount,
			BalanceAfter: e.BalanceAfter,
			Reason:       e.Reason,
			ReferenceId:  e.ReferenceID,
			EarnedAt:     timestamppb.New(e.EarnedAt),
		}
	case domainEvent.CoinsSpent:
		return &gamificationv1event.CoinsSpent{
			UserId:       e.UserID,
			Amount:       e.Amount,
			BalanceAfter: e.BalanceAfter,
			Reason:       e.Reason,
			ReferenceId:  e.ReferenceID,
			SpentAt:      timestamppb.New(e.SpentAt),
		}
	case *domainEvent.CoinsSpent:
		if e == nil {
			return nil
		}
		return &gamificationv1event.CoinsSpent{
			UserId:       e.UserID,
			Amount:       e.Amount,
			BalanceAfter: e.BalanceAfter,
			Reason:       e.Reason,
			ReferenceId:  e.ReferenceID,
			SpentAt:      timestamppb.New(e.SpentAt),
		}
	default:
		return nil
	}
}

func mapStreakMilestoneReached(e *domainEvent.StreakMilestoneReached) *gamificationv1event.StreakMilestoneReached {
	var ms gamificationv1event.StreakMilestone
	switch e.MilestoneDays {
	case 7:
		ms = gamificationv1event.StreakMilestone_STREAK_MILESTONE_7_DAYS
	case 14:
		ms = gamificationv1event.StreakMilestone_STREAK_MILESTONE_14_DAYS
	case 30:
		ms = gamificationv1event.StreakMilestone_STREAK_MILESTONE_30_DAYS
	case 50:
		ms = gamificationv1event.StreakMilestone_STREAK_MILESTONE_50_DAYS
	case 100:
		ms = gamificationv1event.StreakMilestone_STREAK_MILESTONE_100_DAYS
	case 365:
		ms = gamificationv1event.StreakMilestone_STREAK_MILESTONE_365_DAYS
	default:
		ms = gamificationv1event.StreakMilestone_STREAK_MILESTONE_UNSPECIFIED
	}

	return &gamificationv1event.StreakMilestoneReached{
		UserId:        e.UserID,
		Milestone:     ms,
		MilestoneDays: e.MilestoneDays,
		ReachedAt:     timestamppb.New(e.ReachedAt),
	}
}

package command

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/viethung213/gym-companion/internal/gamification/application/port"
	"github.com/viethung213/gym-companion/internal/gamification/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/gamification/domain/event"
	"github.com/viethung213/gym-companion/internal/gamification/domain/repository"
	"github.com/viethung213/gym-companion/internal/gamification/domain/vo"
)

type EvaluateBadgesCommand struct {
	UserID     string
	Events     []event.Event
	OccurredAt time.Time
}

type EvaluateBadgesHandler struct {
	badgeRepo    repository.BadgeRepository
	txManager    port.TransactionManager
	outboxWriter port.OutboxWriter
}

func NewEvaluateBadgesHandler(
	badgeRepo repository.BadgeRepository,
	txManager port.TransactionManager,
	outboxWriter port.OutboxWriter,
) *EvaluateBadgesHandler {
	return &EvaluateBadgesHandler{
		badgeRepo:    badgeRepo,
		txManager:    txManager,
		outboxWriter: outboxWriter,
	}
}

func (h *EvaluateBadgesHandler) HandleEvents(
	ctx context.Context,
	userID string,
	occurredAt time.Time,
	events []event.Event,
) ([]event.Event, error) {
	return h.Handle(ctx, EvaluateBadgesCommand{
		UserID:     userID,
		Events:     events,
		OccurredAt: occurredAt,
	})
}

type returningBadgeBatchSaver interface {
	SaveBatchReturningInserted(ctx context.Context, badges []*aggregate.UserBadge) ([]string, error)
}

func (h *EvaluateBadgesHandler) Handle(ctx context.Context, cmd EvaluateBadgesCommand) ([]event.Event, error) {
	if h == nil || h.badgeRepo == nil || h.txManager == nil || h.outboxWriter == nil {
		return nil, nil
	}

	candidates := collectBadgeCandidates(cmd.Events)
	if len(candidates) == 0 {
		return nil, nil
	}

	now := cmd.OccurredAt
	if now.IsZero() {
		now = time.Now().UTC()
	}

	var emittedEvents []event.Event
	err := h.txManager.WithTransaction(ctx, func(txCtx context.Context) error {
		existingBadges, err := h.badgeRepo.FindByUserID(txCtx, cmd.UserID)
		if err != nil {
			return fmt.Errorf("find existing badges for user %s: %w", cmd.UserID, err)
		}
		unlockedSet := make(map[string]struct{}, len(existingBadges))
		for _, b := range existingBadges {
			unlockedSet[b.BadgeCode()] = struct{}{}
		}

		badgesToUnlock := make([]*aggregate.UserBadge, 0, len(candidates))
		eventByCode := make(map[string]event.Event, len(candidates))
		orderedCodes := make([]string, 0, len(candidates))

		for _, c := range candidates {
			if _, alreadyUnlocked := unlockedSet[c.code]; alreadyUnlocked {
				continue
			}

			def, _ := vo.GetBadgeDefinition(c.code)
			badge, ev, err := aggregate.UnlockBadge(
				uuid.NewString(),
				cmd.UserID,
				c.code,
				def.Name,
				def.Category,
				c.meta,
				now,
			)
			if err != nil {
				return fmt.Errorf("unlock badge %s for user %s: %w", c.code, cmd.UserID, err)
			}

			badgesToUnlock = append(badgesToUnlock, badge)
			eventByCode[c.code] = ev
			orderedCodes = append(orderedCodes, c.code)
		}

		if len(badgesToUnlock) == 0 {
			return nil
		}

		var insertedCodes []string
		if retRepo, ok := h.badgeRepo.(returningBadgeBatchSaver); ok {
			insertedCodes, err = retRepo.SaveBatchReturningInserted(txCtx, badgesToUnlock)
			if err != nil {
				return fmt.Errorf("batch save badges: %w", err)
			}
		} else {
			if err := h.badgeRepo.SaveBatch(txCtx, badgesToUnlock); err != nil {
				return fmt.Errorf("batch save badges: %w", err)
			}
			insertedCodes = orderedCodes
		}

		if len(insertedCodes) == 0 {
			return nil
		}

		emittedEvents = make([]event.Event, 0, len(insertedCodes))
		outboxEvents := make([]any, 0, len(insertedCodes))
		for _, code := range insertedCodes {
			if ev, ok := eventByCode[code]; ok {
				emittedEvents = append(emittedEvents, ev)
				outboxEvents = append(outboxEvents, ev)
			}
		}

		if err := h.outboxWriter.WriteEvents(txCtx, "user_badges", cmd.UserID, outboxEvents); err != nil {
			return fmt.Errorf("write badge outbox events: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("save and write badge events: %w", err)
	}

	return emittedEvents, nil
}

type badgeCandidate struct {
	code string
	meta map[string]string
}

func collectBadgeCandidates(events []event.Event) []badgeCandidate {
	seen := make(map[string]struct{})
	var list []badgeCandidate

	add := func(code string, meta map[string]string) {
		if _, exists := seen[code]; exists {
			return
		}
		seen[code] = struct{}{}
		list = append(list, badgeCandidate{code: code, meta: meta})
	}

	for _, ev := range events {
		switch e := ev.(type) {
		case *event.WorkoutActivityIngested:
			add("FIRST_WORKOUT", nil)
			if e.HasNewPR {
				add("PR_BREAKER", nil)
			}
		case *event.PersonalRecordIngested:
			add("PR_BREAKER", map[string]string{
				"exerciseId": e.ExerciseID,
				"weight":     fmt.Sprintf("%.1f", e.Weight),
				"reps":       strconv.Itoa(int(e.Reps)),
			})
		case *event.StreakAdvanced:
			if code := streakBadgeCode(e.NewStreak); code != "" {
				add(code, nil)
			}
		case *event.StreakMilestoneReached:
			if code := streakBadgeCode(e.MilestoneDays); code != "" {
				add(code, nil)
			}
		case *event.UserLeveledUp:
			if e.OldLevel < 10 && e.NewLevel >= 10 {
				add("LEVEL_10", nil)
			}
			if e.OldLevel < 25 && e.NewLevel >= 25 {
				add("LEVEL_25", nil)
			}
			if e.OldLevel < 50 && e.NewLevel >= 50 {
				add("LEVEL_50", nil)
			}
			if e.OldLevel < 100 && e.NewLevel >= 100 {
				add("LEVEL_100", nil)
			}
		}
	}

	return list
}

func streakBadgeCode(days int32) string {
	switch days {
	case 7:
		return "STREAK_7"
	case 14:
		return "STREAK_14"
	case 30:
		return "STREAK_30"
	case 50:
		return "STREAK_50"
	case 100:
		return "STREAK_100"
	case 365:
		return "STREAK_365"
	default:
		return ""
	}
}

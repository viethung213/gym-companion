package command

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/viethung213/gym-companion/internal/gamification/application/port"
	"github.com/viethung213/gym-companion/internal/gamification/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/gamification/domain/entity"
	"github.com/viethung213/gym-companion/internal/gamification/domain/event"
	"github.com/viethung213/gym-companion/internal/gamification/domain/repository"
	"github.com/viethung213/gym-companion/internal/gamification/domain/vo"
)

type AwardCoinsHandler struct {
	userWalletRepo repository.UserWalletRepository
	txManager      port.TransactionManager
	outboxWriter   port.OutboxWriter
}

var _ port.WalletRewarder = (*AwardCoinsHandler)(nil)

func NewAwardCoinsHandler(
	userWalletRepo repository.UserWalletRepository,
	txManager port.TransactionManager,
	outboxWriter port.OutboxWriter,
) *AwardCoinsHandler {
	return &AwardCoinsHandler{
		userWalletRepo: userWalletRepo,
		txManager:      txManager,
		outboxWriter:   outboxWriter,
	}
}

//nolint:gocritic // hugeParam: input struct is an immutable value object
func (h *AwardCoinsHandler) AwardCoins(
	ctx context.Context,
	in port.AwardCoinsInput,
) (*port.AwardCoinsOutput, error) {
	outputs, err := h.AwardCoinsBatch(ctx, in.UserID, []port.AwardCoinsInput{in})
	if err != nil {
		return nil, err
	}
	if len(outputs) == 0 {
		return &port.AwardCoinsOutput{Skipped: true}, nil
	}
	return outputs[0], nil
}

func (h *AwardCoinsHandler) HandleEvents(
	ctx context.Context,
	userID, activityDate string,
	occurredAt time.Time,
	events []event.Event,
) ([]event.Event, error) {
	items := buildCoinRewardItems(userID, activityDate, occurredAt, events)
	if len(items) == 0 {
		return nil, nil
	}
	_, emitted, err := h.executeBatch(ctx, userID, items)
	return emitted, err
}

func (h *AwardCoinsHandler) AwardCoinsBatch(
	ctx context.Context,
	userID string,
	items []port.AwardCoinsInput,
) ([]*port.AwardCoinsOutput, error) {
	outputs, _, err := h.executeBatch(ctx, userID, items)
	return outputs, err
}

type batchLedgerKeyFinder interface {
	FindExistingLedgerEntriesByIdempotencyKeys(ctx context.Context, userID string, keys []string) (map[string]*entity.CoinLedgerEntry, error)
}

func (h *AwardCoinsHandler) executeBatch(
	ctx context.Context,
	userID string,
	items []port.AwardCoinsInput,
) ([]*port.AwardCoinsOutput, []event.Event, error) {
	if userID == "" {
		return nil, nil, aggregate.ErrEmptyUserID
	}
	if len(items) == 0 {
		return nil, nil, nil
	}
	for i := range items {
		if items[i].IdempotencyKey == "" || len(items[i].IdempotencyKey) > aggregate.MaxIdempotencyKeyLength {
			return nil, nil, aggregate.ErrEmptyIdempotencyKey
		}
	}

	outputs := make([]*port.AwardCoinsOutput, 0, len(items))
	emitted := make([]event.Event, 0, len(items))

	err := h.txManager.WithTransaction(ctx, func(txCtx context.Context) error {
		wallet, err := h.userWalletRepo.FindByIDForUpdate(txCtx, userID)
		if err != nil {
			return fmt.Errorf("find user_wallet for update: %w", err)
		}
		if wallet == nil {
			wallet, err = aggregate.NewUserWallet(userID)
			if err != nil {
				return fmt.Errorf("initialize user_wallet: %w", err)
			}
		}

		entriesToSave := make([]*entity.CoinLedgerEntry, 0, len(items))
		outboxEvents := make([]any, 0, len(items))
		seenKeys := make(map[string]bool, len(items))

		var existingByKey map[string]*entity.CoinLedgerEntry
		if batchFinder, ok := h.userWalletRepo.(batchLedgerKeyFinder); ok {
			keysToCheck := make([]string, 0, len(items))
			for i := range items {
				if !wallet.IsDailyCapReached(items[i].Reason, items[i].ActivityDate) {
					keysToCheck = append(keysToCheck, items[i].IdempotencyKey)
				}
			}
			if len(keysToCheck) > 0 {
				existingByKey, err = batchFinder.FindExistingLedgerEntriesByIdempotencyKeys(txCtx, userID, keysToCheck)
				if err != nil {
					return fmt.Errorf("batch check idempotency keys: %w", err)
				}
			}
		}

		for i := range items {
			in := items[i]
			if seenKeys[in.IdempotencyKey] {
				outputs = append(outputs, &port.AwardCoinsOutput{
					BalanceAfter: wallet.Balance(),
					Skipped:      true,
				})
				continue
			}
			seenKeys[in.IdempotencyKey] = true

			if wallet.IsDailyCapReached(in.Reason, in.ActivityDate) {
				outputs = append(outputs, &port.AwardCoinsOutput{
					BalanceAfter: wallet.Balance(),
					Skipped:      true,
				})
				continue
			}

			var existing *entity.CoinLedgerEntry
			if batchFinder, ok := h.userWalletRepo.(batchLedgerKeyFinder); ok {
				_ = batchFinder
				existing = existingByKey[in.IdempotencyKey]
			} else {
				existing, err = h.userWalletRepo.FindLedgerEntryByIdempotencyKey(txCtx, userID, in.IdempotencyKey)
				if err != nil {
					return fmt.Errorf("check idempotency key: %w", err)
				}
			}
			if existing != nil {
				outputs = append(outputs, &port.AwardCoinsOutput{
					TransactionID: existing.ID,
					BalanceAfter:  existing.BalanceAfter,
					Skipped:       true,
				})
				continue
			}

			occurredAt := in.OccurredAt
			if occurredAt.IsZero() {
				occurredAt = time.Now().UTC()
			}
			occurredAt = occurredAt.Add(time.Duration(len(entriesToSave)) * time.Microsecond)

			entry, ev, err := wallet.Award(
				uuid.NewString(),
				in.Amount,
				in.Reason,
				in.ReferenceID,
				in.IdempotencyKey,
				in.ActivityDate,
				occurredAt,
			)
			if err != nil {
				if errors.Is(err, aggregate.ErrDailyCapReached) {
					outputs = append(outputs, &port.AwardCoinsOutput{
						BalanceAfter: wallet.Balance(),
						Skipped:      true,
					})
					continue
				}
				return fmt.Errorf("award coins on wallet aggregate: %w", err)
			}

			entriesToSave = append(entriesToSave, entry)
			emitted = append(emitted, ev)
			outboxEvents = append(outboxEvents, ev)
			outputs = append(outputs, &port.AwardCoinsOutput{
				TransactionID: entry.ID,
				BalanceAfter:  wallet.Balance(),
				Skipped:       false,
			})
		}

		if len(entriesToSave) == 0 {
			return nil
		}

		if err := h.userWalletRepo.Save(txCtx, wallet); err != nil {
			return fmt.Errorf("save user_wallet: %w", err)
		}

		if err := h.userWalletRepo.SaveLedgerEntries(txCtx, entriesToSave); err != nil {
			return fmt.Errorf("batch save coin_ledger entries: %w", err)
		}

		if err := h.outboxWriter.WriteEvents(txCtx, "user_wallet", userID, outboxEvents); err != nil {
			return fmt.Errorf("write user_wallet outbox events: %w", err)
		}

		return nil
	})
	if err != nil {
		return nil, nil, err
	}

	return outputs, emitted, nil
}

func buildCoinRewardItems(
	userID, activityDate string,
	occurredAt time.Time,
	events []event.Event,
) []port.AwardCoinsInput {
	items := make([]port.AwardCoinsInput, 0, len(events))

	for _, ev := range events {
		switch e := ev.(type) {
		case *event.WorkoutActivityIngested:
			sessionRef := e.SessionID
			items = append(items, port.AwardCoinsInput{
				UserID:         userID,
				Amount:         vo.WorkoutCoinReward,
				Reason:         vo.ReasonEarnWorkout,
				ReferenceID:    &sessionRef,
				IdempotencyKey: fmt.Sprintf("WORKOUT:%s:%s", userID, activityDate),
				ActivityDate:   activityDate,
				OccurredAt:     occurredAt,
			})
			if e.HasNewPR {
				items = append(items, port.AwardCoinsInput{
					UserID:         userID,
					Amount:         vo.PersonalRecordCoinReward,
					Reason:         vo.ReasonEarnPR,
					ReferenceID:    &sessionRef,
					IdempotencyKey: fmt.Sprintf("PR:%s:%s", userID, activityDate),
					ActivityDate:   activityDate,
					OccurredAt:     occurredAt,
				})
			}
		case *event.NutritionActivityIngested:
			items = append(items, port.AwardCoinsInput{
				UserID:         userID,
				Amount:         vo.NutritionDailyCoinReward,
				Reason:         vo.ReasonEarnNutrition,
				ReferenceID:    nil,
				IdempotencyKey: fmt.Sprintf("NUTRITION:%s:%s", userID, activityDate),
				ActivityDate:   activityDate,
				OccurredAt:     occurredAt,
			})
		case *event.PersonalRecordIngested:
			exerciseRef := e.ExerciseID
			items = append(items, port.AwardCoinsInput{
				UserID:         userID,
				Amount:         vo.PersonalRecordCoinReward,
				Reason:         vo.ReasonEarnPR,
				ReferenceID:    &exerciseRef,
				IdempotencyKey: fmt.Sprintf("PR:%s:%s", userID, activityDate),
				ActivityDate:   activityDate,
				OccurredAt:     occurredAt,
			})
		case *event.StreakMilestoneReached:
			if reward := vo.StreakMilestoneCoinReward(e.MilestoneDays); reward > 0 {
				milestoneRef := fmt.Sprintf("STREAK_%d", e.MilestoneDays)
				items = append(items, port.AwardCoinsInput{
					UserID:         userID,
					Amount:         reward,
					Reason:         vo.ReasonEarnStreakMilestone,
					ReferenceID:    &milestoneRef,
					IdempotencyKey: fmt.Sprintf("STREAK_MILESTONE:%s:%d:%s", userID, e.MilestoneDays, activityDate),
					ActivityDate:   activityDate,
					OccurredAt:     occurredAt,
				})
			}
		case *event.BadgeUnlocked:
			codeRef := e.BadgeCode
			items = append(items, port.AwardCoinsInput{
				UserID:         userID,
				Amount:         vo.BadgeUnlockedCoinReward,
				Reason:         vo.ReasonEarnBadge,
				ReferenceID:    &codeRef,
				IdempotencyKey: fmt.Sprintf("BADGE:%s:%s", userID, e.BadgeCode),
				ActivityDate:   activityDate,
				OccurredAt:     occurredAt,
			})
		}
	}

	return items
}

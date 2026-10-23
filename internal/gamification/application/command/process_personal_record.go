package command

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/viethung213/gym-companion/internal/gamification/application/port"
	"github.com/viethung213/gym-companion/internal/gamification/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/gamification/domain/entity"
	"github.com/viethung213/gym-companion/internal/gamification/domain/repository"
	"github.com/viethung213/gym-companion/internal/gamification/domain/vo"
)

type ProcessPersonalRecordCommand struct {
	UserID       string
	ExerciseID   string
	Weight       float64
	Reps         int32
	ActivityDate string
	AchievedAt   time.Time
}

type ProcessPersonalRecordHandler struct {
	userXpRepo     repository.UserXpRepository
	badgeRepo      repository.BadgeRepository
	txManager      port.TransactionManager
	outboxWriter   port.OutboxWriter
	walletRewarder port.WalletRewarder
}

func NewProcessPersonalRecordHandler(
	userXpRepo repository.UserXpRepository,
	badgeRepo repository.BadgeRepository,
	txManager port.TransactionManager,
	outboxWriter port.OutboxWriter,
) *ProcessPersonalRecordHandler {
	return &ProcessPersonalRecordHandler{
		userXpRepo:   userXpRepo,
		badgeRepo:    badgeRepo,
		txManager:    txManager,
		outboxWriter: outboxWriter,
	}
}

// WithWalletRewarder attaches an optional WalletRewarder for Personal Record FitCoins rewards.
func (h *ProcessPersonalRecordHandler) WithWalletRewarder(
	walletRewarder port.WalletRewarder,
) *ProcessPersonalRecordHandler {
	h.walletRewarder = walletRewarder
	return h
}

//nolint:gocritic // hugeParam: command struct is an immutable value object
func (h *ProcessPersonalRecordHandler) Handle(ctx context.Context, cmd ProcessPersonalRecordCommand) error {
	return h.txManager.WithTransaction(ctx, func(txCtx context.Context) error {
		userXp, err := h.userXpRepo.FindByIDForUpdate(txCtx, cmd.UserID)
		if err != nil {
			return fmt.Errorf("find user xp: %w", err)
		}
		if userXp == nil {
			userXp, err = aggregate.NewUserXp(cmd.UserID)
			if err != nil {
				return fmt.Errorf("create user xp: %w", err)
			}
		}

		xpEvents, err := userXp.EarnXp(25, "PERSONAL_RECORD", cmd.AchievedAt)
		if err != nil {
			return fmt.Errorf("earn pr xp: %w", err)
		}

		if saveErr := h.userXpRepo.Save(txCtx, userXp); saveErr != nil {
			return fmt.Errorf("save user xp: %w", saveErr)
		}

		history := entity.NewXpHistoryEntry(
			uuid.NewString(),
			cmd.UserID,
			25,
			"PERSONAL_RECORD",
			nil,
			cmd.ActivityDate,
			cmd.AchievedAt,
		)
		if histErr := h.userXpRepo.SaveHistory(txCtx, history); histErr != nil {
			return fmt.Errorf("save xp history: %w", histErr)
		}

		outboxEvents := make([]any, len(xpEvents))
		for i, ev := range xpEvents {
			outboxEvents[i] = ev
		}

		if writeErr := h.outboxWriter.WriteEvents(txCtx, "user_xp", cmd.UserID, outboxEvents); writeErr != nil {
			return fmt.Errorf("write outbox events: %w", writeErr)
		}

		has, hasErr := h.badgeRepo.HasBadge(txCtx, cmd.UserID, "PR_BREAKER")
		if hasErr != nil {
			return fmt.Errorf("check badge PR_BREAKER for user %s: %w", cmd.UserID, hasErr)
		}
		if !has {
			def, _ := vo.GetBadgeDefinition("PR_BREAKER")
			meta := map[string]string{
				"exerciseId": cmd.ExerciseID,
				"weight":     fmt.Sprintf("%.1f", cmd.Weight),
				"reps":       strconv.Itoa(int(cmd.Reps)),
			}
			badge, badgeEv, bErr := aggregate.UnlockBadge(
				uuid.NewString(),
				cmd.UserID,
				"PR_BREAKER",
				def.Name,
				def.Category,
				meta,
				cmd.AchievedAt,
			)
			if bErr != nil {
				return fmt.Errorf("unlock badge PR_BREAKER for user %s: %w", cmd.UserID, bErr)
			}
			if saveBadgeErr := h.badgeRepo.Save(txCtx, badge); saveBadgeErr != nil {
				return fmt.Errorf("save badge PR_BREAKER for user %s: %w", cmd.UserID, saveBadgeErr)
			}
			if writeBadgeErr := h.outboxWriter.WriteEvents(txCtx, "user_badges", cmd.UserID, []any{badgeEv}); writeBadgeErr != nil {
				return fmt.Errorf("write outbox badge event for user %s: %w", cmd.UserID, writeBadgeErr)
			}
		}

		if h.walletRewarder != nil {
			exerciseRef := cmd.ExerciseID
			if _, prCoinErr := h.walletRewarder.AwardCoins(txCtx, port.AwardCoinsInput{
				UserID:         cmd.UserID,
				Amount:         vo.PersonalRecordCoinReward,
				Reason:         vo.ReasonEarnPR,
				ReferenceID:    &exerciseRef,
				IdempotencyKey: fmt.Sprintf("PR:%s:%s", cmd.UserID, cmd.ActivityDate),
				ActivityDate:   cmd.ActivityDate,
				OccurredAt:     cmd.AchievedAt,
			}); prCoinErr != nil {
				return fmt.Errorf("award pr coins: %w", prCoinErr)
			}

			if !has {
				badgeCode := "PR_BREAKER"
				if _, badgeCoinErr := h.walletRewarder.AwardCoins(txCtx, port.AwardCoinsInput{
					UserID:         cmd.UserID,
					Amount:         vo.BadgeUnlockedCoinReward,
					Reason:         vo.ReasonEarnBadge,
					ReferenceID:    &badgeCode,
					IdempotencyKey: fmt.Sprintf("BADGE:%s:%s", cmd.UserID, badgeCode),
					ActivityDate:   cmd.ActivityDate,
					OccurredAt:     cmd.AchievedAt,
				}); badgeCoinErr != nil {
					return fmt.Errorf("award pr badge coins: %w", badgeCoinErr)
				}
			}
		}

		return nil
	})
}

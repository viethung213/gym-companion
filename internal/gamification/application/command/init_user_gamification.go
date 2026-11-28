package command

import (
	"context"
	"fmt"
	"time"

	"github.com/viethung213/gym-companion/internal/gamification/application/port"
	"github.com/viethung213/gym-companion/internal/gamification/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/gamification/domain/entity"
	"github.com/viethung213/gym-companion/internal/gamification/domain/repository"
)

// InitUserGamificationCommand initializes a fresh profile upon user registration.
type InitUserGamificationCommand struct {
	UserID      string
	DisplayName string
	AvatarURL   string
}

// InitUserGamificationHandler creates baseline records for a new user.
type InitUserGamificationHandler struct {
	userXpRepo     repository.UserXpRepository
	userStreakRepo repository.UserStreakRepository
	snapshotRepo   repository.UserSnapshotRepository
	userWalletRepo repository.UserWalletRepository
	txManager      port.TransactionManager
	leaderboard    *ProjectLeaderboardScoreHandler
}

// NewInitUserGamificationHandler constructs InitUserGamificationHandler.
func NewInitUserGamificationHandler(
	userXpRepo repository.UserXpRepository,
	userStreakRepo repository.UserStreakRepository,
	snapshotRepo repository.UserSnapshotRepository,
	txManager port.TransactionManager,
) *InitUserGamificationHandler {
	return &InitUserGamificationHandler{
		userXpRepo:     userXpRepo,
		userStreakRepo: userStreakRepo,
		snapshotRepo:   snapshotRepo,
		txManager:      txManager,
	}
}

// WithLeaderboardProjector enables adding newly created users to the leaderboard read model
// after commit.
func (h *InitUserGamificationHandler) WithLeaderboardProjector(
	projector *ProjectLeaderboardScoreHandler,
) *InitUserGamificationHandler {
	h.leaderboard = projector
	return h
}

// WithWalletRepository attaches an optional UserWalletRepository for eager wallet initialization.
func (h *InitUserGamificationHandler) WithWalletRepository(
	userWalletRepo repository.UserWalletRepository,
) *InitUserGamificationHandler {
	h.userWalletRepo = userWalletRepo
	return h
}

// Handle sets up initial UserStreak, UserXp and UserWallet records in one transaction. After it
// commits, the UserSnapshot is saved and the user's XP is projected onto the leaderboard; both are
// idempotent, so a redelivered registration event repairs a failure of either step.
func (h *InitUserGamificationHandler) Handle(ctx context.Context, cmd InitUserGamificationCommand) error {
	var userXp *aggregate.UserXp
	txErr := h.txManager.WithTransaction(ctx, func(txCtx context.Context) error {
		userXp = nil
		existingStreak, findStreakErr := h.userStreakRepo.FindByIDForUpdate(txCtx, cmd.UserID)
		if findStreakErr != nil {
			return findStreakErr
		}
		if existingStreak == nil {
			newStreak, createStreakErr := aggregate.NewUserStreak(cmd.UserID)
			if createStreakErr != nil {
				return createStreakErr
			}
			if saveStreakErr := h.userStreakRepo.Save(txCtx, newStreak); saveStreakErr != nil {
				return saveStreakErr
			}
		}

		existingXp, findXpErr := h.userXpRepo.FindByIDForUpdate(txCtx, cmd.UserID)
		if findXpErr != nil {
			return findXpErr
		}
		userXp = existingXp
		if userXp == nil {
			newXp, createXpErr := aggregate.NewUserXp(cmd.UserID)
			if createXpErr != nil {
				return createXpErr
			}
			if saveXpErr := h.userXpRepo.Save(txCtx, newXp); saveXpErr != nil {
				return saveXpErr
			}
			userXp = newXp
		}

		if h.userWalletRepo != nil {
			existingWallet, findWalletErr := h.userWalletRepo.FindByIDForUpdate(txCtx, cmd.UserID)
			if findWalletErr != nil {
				return findWalletErr
			}
			if existingWallet == nil {
				newWallet, createWalletErr := aggregate.NewUserWallet(cmd.UserID)
				if createWalletErr != nil {
					return createWalletErr
				}
				if saveWalletErr := h.userWalletRepo.Save(txCtx, newWallet); saveWalletErr != nil {
					return saveWalletErr
				}
			}
		}
		return nil
	})
	if txErr != nil {
		return fmt.Errorf("init user gamification: %w", txErr)
	}

	// Saved after commit: the snapshot repository may write through to a cache, which must never
	// hold a profile for a user whose initialization rolled back.
	displayName := entity.DisplayNameOrDefault(cmd.DisplayName)
	snapshot := entity.NewUserSnapshot(cmd.UserID, displayName, cmd.AvatarURL, time.Now().UTC())
	if err := h.snapshotRepo.SaveSnapshot(ctx, snapshot); err != nil {
		return fmt.Errorf("init user gamification: save snapshot: %w", err)
	}

	// Projected whether or not the row was created here: the repository may seed a baseline row
	// itself, and re-projecting the current score is idempotent.
	h.leaderboard.ProjectBestEffort(ctx, ProjectLeaderboardScoreCommand{
		UserID:    userXp.UserID(),
		Xp:        userXp.Xp(),
		UpdatedAt: userXp.UpdatedAt(),
	})
	return nil
}

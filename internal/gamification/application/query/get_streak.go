package query

import (
	"context"
	"fmt"
	"time"

	"github.com/viethung213/gym-companion/internal/gamification/domain/repository"
)

// GetStreakQuery defines parameters for retrieving current streak status.
type GetStreakQuery struct {
	UserID           string
	CurrentLocalDate string // YYYY-MM-DD
}

// GetStreakResult contains the user's streak status.
type GetStreakResult struct {
	CurrentStreak    int32
	LongestStreak    int32
	LastActivityDate string
	IsActiveToday    bool
	UpdatedAt        time.Time
}

// GetStreakHandler handles GetStreakQuery.
type GetStreakHandler struct {
	userStreakRepo repository.UserStreakRepository
}

// NewGetStreakHandler constructs GetStreakHandler.
func NewGetStreakHandler(userStreakRepo repository.UserStreakRepository) *GetStreakHandler {
	return &GetStreakHandler{userStreakRepo: userStreakRepo}
}

// Handle retrieves streak status without altering any database records (BR-ST-01, ADR-0001).
// Computes virtual streak display: if inactivity delta >= 2 days, displays CurrentStreak = 0.
func (h *GetStreakHandler) Handle(ctx context.Context, q GetStreakQuery) (*GetStreakResult, error) {
	streak, err := h.userStreakRepo.FindByID(ctx, q.UserID)
	if err != nil {
		return nil, fmt.Errorf("find user streak: %w", err)
	}

	if streak == nil {
		return &GetStreakResult{
			CurrentStreak:    0,
			LongestStreak:    0,
			LastActivityDate: "",
			IsActiveToday:    false,
			UpdatedAt:        time.Now().UTC(),
		}, nil
	}

	currentStreak := streak.CurrentStreak()
	isActiveToday := false

	if q.CurrentLocalDate != "" && streak.LastActivityDate() != "" {
		lastDate, errLast := time.Parse("2006-01-02", streak.LastActivityDate())
		currDate, errCurr := time.Parse("2006-01-02", q.CurrentLocalDate)
		if errLast == nil && errCurr == nil {
			daysDiff := int(currDate.Sub(lastDate).Hours() / 24)
			if daysDiff == 0 {
				isActiveToday = true
			} else if daysDiff >= 2 {
				// The streak was broken yesterday! Virtual display shows 0 to be truthful to user (ADR-0001)
				currentStreak = 0
			}
		}
	} else if q.CurrentLocalDate != "" && streak.LastActivityDate() == q.CurrentLocalDate {
		isActiveToday = true
	}

	return &GetStreakResult{
		CurrentStreak:    currentStreak,
		LongestStreak:    streak.LongestStreak(),
		LastActivityDate: streak.LastActivityDate(),
		IsActiveToday:    isActiveToday,
		UpdatedAt:        streak.UpdatedAt(),
	}, nil
}

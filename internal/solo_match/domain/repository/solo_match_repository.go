package repository

import (
	"context"
	"time"

	"github.com/viethung213/gym-companion/internal/solo_match/domain/aggregate"
)

type SoloMatchRepository interface {
	Save(ctx context.Context, match *aggregate.SoloMatch) error
	FindByID(ctx context.Context, id string) (*aggregate.SoloMatch, error)
	FindActiveByUserID(ctx context.Context, userID string) (*aggregate.SoloMatch, error)
	FindUnreadyExpiredMatches(ctx context.Context, now time.Time, limit int) ([]*aggregate.SoloMatch, error)
	FindExpiredInProgressMatches(ctx context.Context, startedBefore time.Time, limit int) ([]*aggregate.SoloMatch, error)
	ListByUserID(ctx context.Context, userID string, offset, limit int) ([]*aggregate.SoloMatch, error)
}

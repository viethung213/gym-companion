package repository

import (
	"context"

	"github.com/viethung213/gym-companion/internal/notification/domain/aggregate"
)

type SettingRepository interface {
	GetByUserID(ctx context.Context, userID string) (*aggregate.Setting, error)
	Save(ctx context.Context, setting *aggregate.Setting) error
}

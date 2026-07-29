package persistence

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"github.com/viethung213/gym-companion/internal/profile/application/port"
)

type txKey struct{}

type SQLTransactionManager struct {
	db *gorm.DB
}

var _ port.TransactionManager = (*SQLTransactionManager)(nil)

func NewSQLTransactionManager(db *gorm.DB) *SQLTransactionManager {
	return &SQLTransactionManager{db: db}
}

func (tm *SQLTransactionManager) WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	if _, ok := ctx.Value(txKey{}).(*gorm.DB); ok {
		return fn(ctx)
	}

	return tm.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		txCtx := context.WithValue(ctx, txKey{}, tx)
		if err := fn(txCtx); err != nil {
			return fmt.Errorf("transaction execution failed: %w", err)
		}
		return nil
	})
}

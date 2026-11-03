package persistence

import (
	"context"

	"github.com/viethung213/gym-companion/internal/nutrition/application/port"
	"gorm.io/gorm"
)

type SQLTransactionManager struct {
	db *gorm.DB
}

var _ port.TransactionManager = (*SQLTransactionManager)(nil)

func NewSQLTransactionManager(db *gorm.DB) *SQLTransactionManager {
	return &SQLTransactionManager{db: db}
}

func (tm *SQLTransactionManager) WithTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	if existingTx := GetTx(ctx); existingTx != nil {
		return fn(ctx)
	}

	return tm.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		txCtx := WithTx(ctx, tx)
		if err := fn(txCtx); err != nil {
			return err
		}
		return nil
	})
}

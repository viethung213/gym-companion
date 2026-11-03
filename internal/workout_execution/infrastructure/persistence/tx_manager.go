package persistence

import (
	"context"

	"github.com/viethung213/gym-companion/internal/workout_execution/application/port"
	"gorm.io/gorm"
)

// SQLTransactionManager implements port.TransactionManager using GORM transaction.
type SQLTransactionManager struct {
	db *gorm.DB
}

// Compile-time interface check
var _ port.TransactionManager = (*SQLTransactionManager)(nil)

// NewSQLTransactionManager creates a new instance of SQLTransactionManager.
func NewSQLTransactionManager(db *gorm.DB) *SQLTransactionManager {
	return &SQLTransactionManager{db: db}
}

// WithTransaction runs the given function inside a database transaction context.
func (m *SQLTransactionManager) WithTransaction(
	ctx context.Context,
	fn func(txCtx context.Context) error,
) error {
	if existingTx := GetTx(ctx); existingTx != nil {
		return fn(ctx)
	}

	return m.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		txCtx := WithTx(ctx, tx)
		if err := fn(txCtx); err != nil {
			return err
		}
		return nil
	})
}

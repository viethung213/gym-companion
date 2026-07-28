package persistence

import (
	"context"
	"fmt"

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
	tx := m.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return fmt.Errorf("begin gorm transaction: %w", tx.Error)
	}

	defer func() {
		if p := recover(); p != nil {
			tx.Rollback()
			panic(p) // Re-throw panic after rollback
		}
	}()

	txCtx := WithTx(ctx, tx)
	if err := fn(txCtx); err != nil {
		tx.Rollback()
		return err
	}

	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("commit gorm transaction: %w", err)
	}

	return nil
}

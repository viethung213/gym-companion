package persistence

import (
	"context"
	"fmt"

	"github.com/viethung213/gym-companion/internal/gamification/application/port"
	"gorm.io/gorm"
)

// SQLTransactionManager implements port.TransactionManager using GORM transaction.
type SQLTransactionManager struct {
	db *gorm.DB
}

// Compile-time interface check.
var _ port.TransactionManager = (*SQLTransactionManager)(nil)

// NewSQLTransactionManager creates a new instance of SQLTransactionManager.
func NewSQLTransactionManager(db *gorm.DB) *SQLTransactionManager {
	return &SQLTransactionManager{db: db}
}

// WithTransaction executes the callback function inside a database transaction context.
func (m *SQLTransactionManager) WithTransaction(
	ctx context.Context,
	fn func(txCtx context.Context) error,
) error {
	tx := m.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return fmt.Errorf("begin transaction: %w", tx.Error)
	}

	defer func() {
		if p := recover(); p != nil {
			tx.Rollback()
			panic(p)
		}
	}()

	txCtx := WithTx(ctx, tx)
	if err := fn(txCtx); err != nil {
		tx.Rollback()
		return err
	}

	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

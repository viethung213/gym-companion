package postgres

import (
	"context"
	"fmt"

	"github.com/viethung213/gym-companion/internal/auth/application"
	"gorm.io/gorm"
)

// SQLTransactionManager implements application.TransactionManager port using GORM transaction.
type SQLTransactionManager struct {
	db *gorm.DB
}

// Compile-time interface verification
var _ application.TransactionManager = (*SQLTransactionManager)(nil)

// NewSQLTransactionManager creates a new instance of SQLTransactionManager.
func NewSQLTransactionManager(db *gorm.DB) *SQLTransactionManager {
	return &SQLTransactionManager{db: db}
}

// WithTransaction runs the given function inside a database transaction context.
func (m *SQLTransactionManager) WithTransaction(
	ctx context.Context,
	fn func(ctx context.Context) error,
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

	// Inject transaction into context
	ctxWithTx := WithTx(ctx, tx)

	if err := fn(ctxWithTx); err != nil {
		tx.Rollback()
		return err
	}

	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("commit gorm transaction: %w", err)
	}

	return nil
}

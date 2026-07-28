package persistence

import (
	"context"
	"fmt"

	"github.com/viethung213/gym-companion/internal/coaching/application/port"
	"gorm.io/gorm"
)

// SQLTransactionManager implements port.TransactionManager using GORM.
type SQLTransactionManager struct {
	db *gorm.DB
}

var _ port.TransactionManager = (*SQLTransactionManager)(nil)

// NewSQLTransactionManager constructs the tx manager.
func NewSQLTransactionManager(db *gorm.DB) *SQLTransactionManager {
	return &SQLTransactionManager{db: db}
}

// WithTransaction runs fn inside a GORM transaction. If fn returns an error,
// the transaction is rolled back. Panics are re-thrown after rollback.
func (m *SQLTransactionManager) WithTransaction(ctx context.Context, fn func(txCtx context.Context) error) error {
	tx := m.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return fmt.Errorf("begin coaching tx: %w", tx.Error)
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
		return fmt.Errorf("commit coaching tx: %w", err)
	}
	return nil
}

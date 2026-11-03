package persistence

import (
	"context"

	"github.com/viethung213/gym-companion/internal/notification/application/port"
	"gorm.io/gorm"
)

// SQLTransactionManager implements port.TransactionManager and port.TxManager using GORM transaction.
type SQLTransactionManager struct {
	db *gorm.DB
}

var _ port.TransactionManager = (*SQLTransactionManager)(nil)
var _ port.TxManager = (*SQLTransactionManager)(nil)

// NewSQLTransactionManager creates a new SQLTransactionManager instance.
func NewSQLTransactionManager(db *gorm.DB) *SQLTransactionManager {
	return &SQLTransactionManager{db: db}
}

// NewTxManager is an alias constructor for backward compatibility.
func NewTxManager(db *gorm.DB) *SQLTransactionManager {
	return NewSQLTransactionManager(db)
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

// ExecTx satisfies the legacy port.TxManager interface.
func (m *SQLTransactionManager) ExecTx(ctx context.Context, fn func(ctx context.Context) error) error {
	return m.WithTransaction(ctx, fn)
}

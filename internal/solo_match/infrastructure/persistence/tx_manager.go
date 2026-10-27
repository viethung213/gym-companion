package persistence

import (
	"context"
	"fmt"

	"github.com/viethung213/gym-companion/internal/solo_match/application/port"
	"gorm.io/gorm"
)

type SQLTransactionManager struct {
	db *gorm.DB
}

var _ port.TransactionManager = (*SQLTransactionManager)(nil)

func NewSQLTransactionManager(db *gorm.DB) *SQLTransactionManager {
	return &SQLTransactionManager{db: db}
}

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
			panic(p)
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

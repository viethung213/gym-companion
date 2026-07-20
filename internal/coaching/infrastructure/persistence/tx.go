package persistence

import (
	"context"
	"fmt"

	"github.com/viethung213/gym-companion/internal/coaching/application/port"
	"gorm.io/gorm"
)

type txKey struct{}

func WithTx(ctx context.Context, tx *gorm.DB) context.Context {
	return context.WithValue(ctx, txKey{}, tx)
}

func GetTx(ctx context.Context) *gorm.DB {
	if tx, ok := ctx.Value(txKey{}).(*gorm.DB); ok {
		return tx
	}
	return nil
}

type UnitOfWork struct {
	db *gorm.DB
}

var _ port.UnitOfWork = (*UnitOfWork)(nil)

func NewUnitOfWork(db *gorm.DB) *UnitOfWork {
	return &UnitOfWork{db: db}
}

func (u *UnitOfWork) WithinTransaction(
	ctx context.Context,
	fn func(context.Context) error,
) error {
	if GetTx(ctx) != nil {
		return fn(ctx)
	}

	if err := u.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(WithTx(ctx, tx))
	}); err != nil {
		return fmt.Errorf("execute unit of work: %w", err)
	}

	return nil
}

func withinTransaction(
	ctx context.Context,
	db *gorm.DB,
	fn func(*gorm.DB) error,
) error {
	if tx := GetTx(ctx); tx != nil {
		return fn(tx.WithContext(ctx))
	}

	return db.WithContext(ctx).Transaction(fn)
}

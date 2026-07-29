package persistence

import (
	"context"

	"gorm.io/gorm"
)

type txKey struct{}

// WithTx embeds an active GORM transaction into the context.

func WithTx(ctx context.Context, tx *gorm.DB) context.Context {

	return context.WithValue(ctx, txKey{}, tx)

}

// GetTx returns the transaction from context, or nil.

func GetTx(ctx context.Context) *gorm.DB {

	if tx, ok := ctx.Value(txKey{}).(*gorm.DB); ok {

		return tx

	}

	return nil

}

// GetDB returns the transaction if present, otherwise defaultDB.WithContext(ctx).

func GetDB(ctx context.Context, defaultDB *gorm.DB) *gorm.DB {

	if tx := GetTx(ctx); tx != nil {

		return tx

	}

	return defaultDB.WithContext(ctx)

}

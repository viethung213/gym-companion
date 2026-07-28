package persistence

import (
	"context"

	"gorm.io/gorm"
)

type txKey struct{}

// WithTx embeds an active database transaction (GORM DB instance) into the context.
func WithTx(ctx context.Context, tx *gorm.DB) context.Context {
	return context.WithValue(ctx, txKey{}, tx)
}

// GetTx extracts the GORM transaction from context if it exists.
func GetTx(ctx context.Context) *gorm.DB {
	if tx, ok := ctx.Value(txKey{}).(*gorm.DB); ok {
		return tx
	}
	return nil
}

// GetDB returns the transaction if present in context, otherwise returns defaultDB with context.
func GetDB(ctx context.Context, defaultDB *gorm.DB) *gorm.DB {
	if tx := GetTx(ctx); tx != nil {
		return tx
	}
	return defaultDB.WithContext(ctx)
}

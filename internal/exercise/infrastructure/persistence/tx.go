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

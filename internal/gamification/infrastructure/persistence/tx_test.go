package persistence_test

import (
	"context"
	"testing"

	"github.com/viethung213/gym-companion/internal/gamification/infrastructure/persistence"
	"gorm.io/gorm"
)

func TestTxContextHelpers(t *testing.T) {
	ctx := context.Background()

	// GetTx on empty context
	if got := persistence.GetTx(ctx); got != nil {
		t.Errorf("GetTx(ctx) = %v, want nil", got)
	}

	dummyDB := &gorm.DB{Config: &gorm.Config{}, Statement: &gorm.Statement{Context: ctx}}

	// WithTx and GetTx
	txCtx := persistence.WithTx(ctx, dummyDB)
	if got := persistence.GetTx(txCtx); got != dummyDB {
		t.Errorf("GetTx(txCtx) = %v, want %v", got, dummyDB)
	}

	// GetDB with tx in context returns tx
	if got := persistence.GetDB(txCtx, nil); got != dummyDB {
		t.Errorf("GetDB(txCtx, nil) = %v, want %v", got, dummyDB)
	}

	// GetDB without tx returns defaultDB.WithContext(ctx)
	gotDB := persistence.GetDB(ctx, dummyDB)
	if gotDB == nil {
		t.Errorf("GetDB(ctx, dummyDB) = nil, want non-nil")
	}

	// GetDB with nil defaultDB
	if gotNil := persistence.GetDB(ctx, nil); gotNil != nil {
		t.Errorf("GetDB(ctx, nil) = %v, want nil", gotNil)
	}
}

func TestSQLTransactionManager_Constructor(t *testing.T) {
	dummyDB := &gorm.DB{Config: &gorm.Config{}, Statement: &gorm.Statement{}}
	tm := persistence.NewSQLTransactionManager(dummyDB)
	if tm == nil {
		t.Fatal("NewSQLTransactionManager() = nil, want non-nil")
	}
}

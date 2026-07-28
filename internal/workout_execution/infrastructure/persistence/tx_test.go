package persistence_test

import (
	"context"
	"testing"

	"github.com/viethung213/gym-companion/internal/workout_execution/infrastructure/persistence"
	"gorm.io/gorm"
)

func TestTxContextHelpers(t *testing.T) {
	ctx := context.Background()

	// GetTx on empty context
	if got := persistence.GetTx(ctx); got != nil {
		t.Errorf("got %v, want nil", got)
	}

	dummyDB := &gorm.DB{Config: &gorm.Config{}, Statement: &gorm.Statement{Context: ctx}}

	// WithTx and GetTx
	txCtx := persistence.WithTx(ctx, dummyDB)
	if got := persistence.GetTx(txCtx); got != dummyDB {
		t.Errorf("got %v, want %v", got, dummyDB)
	}

	// GetDB with tx in context returns tx (does not call defaultDB.WithContext)
	if got := persistence.GetDB(txCtx, nil); got != dummyDB {
		t.Errorf("got %v, want %v", got, dummyDB)
	}
}

func TestSQLTransactionManager_Constructor(t *testing.T) {
	dummyDB := &gorm.DB{Config: &gorm.Config{}, Statement: &gorm.Statement{}}
	tm := persistence.NewSQLTransactionManager(dummyDB)
	if tm == nil {
		t.Fatal("got nil transaction manager, want non-nil")
	}
}

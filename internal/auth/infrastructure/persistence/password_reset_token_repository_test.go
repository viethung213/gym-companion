package persistence

import (
	"context"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/viethung213/gym-companion/internal/auth/domain/derror"
	"github.com/viethung213/gym-companion/internal/auth/domain/entity"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}

	if err := db.Exec("ATTACH DATABASE ':memory:' AS auth;").Error; err != nil {
		t.Fatalf("failed to attach auth schema: %v", err)
	}

	if err := db.Exec(`
		CREATE TABLE auth.password_reset_tokens (
			token TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			expires_at DATETIME NOT NULL,
			is_used BOOLEAN NOT NULL DEFAULT 0,
			created_at DATETIME NOT NULL
		);
	`).Error; err != nil {
		t.Fatalf("failed to create password_reset_tokens table: %v", err)
	}

	return db
}

func TestPasswordResetTokenRepository_CRUD(t *testing.T) {
	db := setupTestDB(t)
	repo := NewPasswordResetTokenRepository(db)
	ctx := context.Background()

	tokenStr := "test-token-123"
	userID := "user-456"
	tokenEntity := entity.NewPasswordResetToken(tokenStr, userID, 15*time.Minute)

	// 1. Save
	if err := repo.Save(ctx, tokenEntity); err != nil {
		t.Fatalf("Save() unexpected error: %v", err)
	}

	// 2. FindByToken
	found, err := repo.FindByToken(ctx, tokenStr)
	if err != nil {
		t.Fatalf("FindByToken() unexpected error: %v", err)
	}
	if found.Token() != tokenStr {
		t.Errorf("got token %s, want %s", found.Token(), tokenStr)
	}
	if found.UserID() != userID {
		t.Errorf("got userID %s, want %s", found.UserID(), userID)
	}
	if found.IsUsed() {
		t.Errorf("got isUsed true, want false")
	}

	// 3. FindByTokenForUpdate
	foundForUpdate, err := repo.FindByTokenForUpdate(ctx, tokenStr)
	if err != nil {
		t.Fatalf("FindByTokenForUpdate() unexpected error: %v", err)
	}
	if foundForUpdate.Token() != tokenStr {
		t.Errorf("got token %s, want %s", foundForUpdate.Token(), tokenStr)
	}

	// 4. Update
	if err := tokenEntity.Use(time.Now()); err != nil {
		t.Fatalf("Use() unexpected error: %v", err)
	}
	if err := repo.Update(ctx, tokenEntity); err != nil {
		t.Fatalf("Update() unexpected error: %v", err)
	}

	foundUpdated, err := repo.FindByToken(ctx, tokenStr)
	if err != nil {
		t.Fatalf("FindByToken() after update unexpected error: %v", err)
	}
	if !foundUpdated.IsUsed() {
		t.Errorf("got isUsed false, want true")
	}

	// 5. Not found
	_, err = repo.FindByToken(ctx, "non-existent-token")
	if err != derror.ErrNotFound {
		t.Errorf("got error %v, want %v", err, derror.ErrNotFound)
	}
}

func TestPasswordResetTokenRepository_TransactionRollback(t *testing.T) {
	db := setupTestDB(t)
	repo := NewPasswordResetTokenRepository(db)
	ctx := context.Background()

	tokenStr := "tx-token-789"
	userID := "user-789"
	tokenEntity := entity.NewPasswordResetToken(tokenStr, userID, 15*time.Minute)

	if err := repo.Save(ctx, tokenEntity); err != nil {
		t.Fatalf("Save() unexpected error: %v", err)
	}

	// Start a transaction and embed in context via WithTx
	tx := db.Begin()
	txCtx := WithTx(ctx, tx)

	// Mark token as used inside transaction
	if err := tokenEntity.Use(time.Now()); err != nil {
		t.Fatalf("Use() unexpected error: %v", err)
	}
	if err := repo.Update(txCtx, tokenEntity); err != nil {
		t.Fatalf("Update(txCtx) unexpected error: %v", err)
	}

	// Rollback the transaction (simulating failure in another aggregate, e.g. user update failure)
	if err := tx.Rollback().Error; err != nil {
		t.Fatalf("Rollback() unexpected error: %v", err)
	}

	// Verify token is NOT burned in DB: is_used must remain false!
	foundAfterRollback, err := repo.FindByToken(ctx, tokenStr)
	if err != nil {
		t.Fatalf("FindByToken() unexpected error: %v", err)
	}
	if foundAfterRollback.IsUsed() {
		t.Fatalf("CRIT-AUTH-03 reproduced: token was burned outside transaction, got isUsed=true, want false")
	}
}

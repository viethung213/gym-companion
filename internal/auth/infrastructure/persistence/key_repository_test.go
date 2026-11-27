package persistence

import (
	"context"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/viethung213/gym-companion/internal/auth/application/port"
	"gorm.io/gorm"
)

func TestKeyRepository_WithSQLite(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}

	if err := db.Exec("ATTACH DATABASE ':memory:' AS auth;").Error; err != nil {
		t.Fatalf("failed to attach auth schema: %v", err)
	}

	if err := db.Exec(`
		CREATE TABLE auth.jwk_keys (
			id TEXT PRIMARY KEY,
			private_key_pem TEXT NOT NULL,
			public_key_pem TEXT NOT NULL,
			algorithm TEXT NOT NULL,
			status TEXT NOT NULL,
			created_at DATETIME,
			expires_at DATETIME
		);
	`).Error; err != nil {
		t.Fatalf("failed to create auth.jwk_keys table: %v", err)
	}

	repo := NewKeyRepository(db)
	ctx := context.Background()

	now := time.Now()
	activeKey := &port.JWKRecord{
		ID:            "key-active",
		PrivateKeyPEM: "mock-priv",
		PublicKeyPEM:  "mock-pub",
		Algorithm:     "RS256",
		Status:        port.KeyStatusActive,
		CreatedAt:     now,
		ExpiresAt:     now.Add(7 * 24 * time.Hour),
	}
	if err := repo.Save(ctx, activeKey); err != nil {
		t.Fatalf("failed to save active key: %v", err)
	}

	// 1. Deactivate with grace period
	graceExpiresAt := now.Add(24 * time.Hour)
	if err := repo.DeactivateAllActiveKeys(ctx, graceExpiresAt); err != nil {
		t.Fatalf("failed to deactivate active keys: %v", err)
	}

	// 2. Add an already-expired inactive key
	expiredKey := &port.JWKRecord{
		ID:            "key-expired-inactive",
		PrivateKeyPEM: "mock-priv-2",
		PublicKeyPEM:  "mock-pub-2",
		Algorithm:     "RS256",
		Status:        port.KeyStatusInactive,
		CreatedAt:     now.Add(-48 * time.Hour),
		ExpiresAt:     now.Add(-1 * time.Hour),
	}
	if err := repo.Save(ctx, expiredKey); err != nil {
		t.Fatalf("failed to save expired key: %v", err)
	}

	// 3. GetAllActiveAndInactiveKeys should return key-active (now inactive with graceExpiresAt > now)
	// and exclude key-expired-inactive (inactive with expires_at < now)
	keys, err := repo.GetAllActiveAndInactiveKeys(ctx)
	if err != nil {
		t.Fatalf("failed to get active and inactive keys: %v", err)
	}

	var foundActiveAsGrace, foundExpired bool
	for _, k := range keys {
		if k.ID == "key-active" {
			foundActiveAsGrace = true
			if k.Status != port.KeyStatusInactive {
				t.Errorf("expected status inactive, got %s", k.Status)
			}
		}
		if k.ID == "key-expired-inactive" {
			foundExpired = true
		}
	}

	if !foundActiveAsGrace {
		t.Error("expected key-active within grace period to be included in GetAllActiveAndInactiveKeys")
	}
	if foundExpired {
		t.Error("expected expired inactive key to be excluded from GetAllActiveAndInactiveKeys")
	}

	// 4. DeleteExpiredKeys should purge key-expired-inactive but keep key-active
	if err := repo.DeleteExpiredKeys(ctx); err != nil {
		t.Fatalf("failed to delete expired keys: %v", err)
	}

	var count int64
	db.Table("auth.jwk_keys").Where("id = ?", "key-expired-inactive").Count(&count)
	if count != 0 {
		t.Error("expected expired key to be deleted from database")
	}

	db.Table("auth.jwk_keys").Where("id = ?", "key-active").Count(&count)
	if count != 1 {
		t.Error("expected grace period key to remain in database")
	}
}

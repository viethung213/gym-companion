package persistence

import (
	"context"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/viethung213/gym-companion/internal/auth/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/auth/domain/derror"
	"gorm.io/gorm"
)

func TestFindByIdentity_WithSQLite(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}

	// Attach schema auth for sqlite
	if err := db.Exec("ATTACH DATABASE ':memory:' AS auth;").Error; err != nil {
		t.Fatalf("failed to attach auth schema: %v", err)
	}

	// Create tables
	if err := db.Exec(`
		CREATE TABLE auth.users (
			id TEXT PRIMARY KEY,
			full_name TEXT,
			role_id TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'active',
			created_at DATETIME,
			updated_at DATETIME
		);
		CREATE TABLE auth.user_identities (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			identity_type TEXT NOT NULL,
			identifier TEXT NOT NULL,
			credential_data TEXT,
			metadata BLOB,
			created_at DATETIME,
			updated_at DATETIME
		);
	`).Error; err != nil {
		t.Fatalf("failed to create tables: %v", err)
	}

	repo := NewUserRepository(db)
	ctx := context.Background()

	// 1. Test Not Found
	_, err = repo.FindByIdentity(ctx, aggregate.IdentityTypeEmail, "nonexistent@example.com")
	if err != derror.ErrUserNotFound {
		t.Fatalf("expected ErrUserNotFound, got: %v", err)
	}

	// 2. Insert test user & identity
	now := time.Now().UTC().Truncate(time.Second)
	testUser := aggregate.RegisterUser(
		"user-123",
		"Test User",
		aggregate.NewIdentity("ident-123", aggregate.IdentityTypeEmail, "test@example.com", "hashed_pwd", nil, now, now),
		"",
	)
	if err := repo.Create(ctx, testUser); err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	// 3. FindByIdentity
	var checkUser UserModel
	err = db.Joins("Identity").
		Where(`"Identity".identity_type = ? AND "Identity".identifier = ?`, "email", "test@example.com").
		First(&checkUser).Error
	if err != nil {
		t.Fatalf("failed joins query: %v", err)
	}
	if checkUser.Identity == nil {
		t.Fatalf("expected Identity to be populated, got nil")
	}

	found, err := repo.FindByIdentity(ctx, aggregate.IdentityTypeEmail, "test@example.com")
	if err != nil {
		t.Fatalf("failed to find by identity: %v", err)
	}

	if found.ID() != "user-123" {
		t.Errorf("expected user ID 'user-123', got '%s'", found.ID())
	}
	if found.FullName() != "Test User" {
		t.Errorf("expected full name 'Test User', got '%s'", found.FullName())
	}
	if found.Identity().Identifier() != "test@example.com" {
		t.Errorf("expected identifier 'test@example.com', got '%s'", found.Identity().Identifier())
	}
	// 4. Test phone identity
	phoneUser := aggregate.RegisterUser(
		"user-phone",
		"Phone User",
		aggregate.NewIdentity("ident-phone", aggregate.IdentityTypePhone, "+84987654321", "hashed_phone_pwd", nil, now, now),
		"",
	)
	if err := repo.Create(ctx, phoneUser); err != nil {
		t.Fatalf("failed to create phone user: %v", err)
	}

	foundPhone, err := repo.FindByIdentity(ctx, aggregate.IdentityTypePhone, "+84987654321")
	if err != nil {
		t.Fatalf("failed to find by phone identity: %v", err)
	}
	if foundPhone.ID() != "user-phone" {
		t.Errorf("expected user ID 'user-phone', got '%s'", foundPhone.ID())
	}
	if foundPhone.Identity().Identifier() != "+84987654321" {
		t.Errorf("expected identifier '+84987654321', got '%s'", foundPhone.Identity().Identifier())
	}
	if foundPhone.Identity().CredentialData() != "hashed_phone_pwd" {
		t.Errorf("expected credential 'hashed_phone_pwd', got '%s'", foundPhone.Identity().CredentialData())
	}
}

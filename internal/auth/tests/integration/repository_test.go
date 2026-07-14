//go:build integration

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/viethung213/gym-companion/internal/auth/application/port"
	"github.com/viethung213/gym-companion/internal/auth/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/auth/domain/derror"
	"github.com/viethung213/gym-companion/internal/auth/domain/vo"
	infraPostgres "github.com/viethung213/gym-companion/internal/auth/infrastructure/persistence/postgres"
	"github.com/viethung213/gym-companion/internal/shared/database"
)

func getTestDB(t *testing.T) *gorm.DB {
	sqlDB, err := database.GetRegistry().GetPool("auth")
	if err != nil {
		t.Fatalf("Failed to initialize auth test database from monolith registry: %v", err)
	}

	db, err := gorm.Open(postgres.New(postgres.Config{
		Conn: sqlDB,
	}), &gorm.Config{
		SkipDefaultTransaction: true,
	})
	if err != nil {
		t.Fatalf("Failed to wrap database in gorm: %v", err)
	}

	return db
}

func truncateTables(db *gorm.DB) {
	db.Exec("TRUNCATE TABLE auth.users CASCADE")
	db.Exec("TRUNCATE TABLE auth.jwk_keys CASCADE")
	db.Exec("TRUNCATE TABLE auth.sessions CASCADE")
	db.Exec("TRUNCATE TABLE auth.outbox CASCADE")
}

func TestUserRepository_Integration(t *testing.T) {
	db := getTestDB(t)
	truncateTables(db)
	defer truncateTables(db)

	repo := infraPostgres.NewUserRepository(db)
	ctx := context.Background()

	userID, _ := vo.NewUserID(uuid.New().String())
	email, _ := vo.NewEmail("integration-test@example.com")
	role, _ := vo.NewRole("user")
	user, err := aggregate.RegisterUser(userID.Value(), email.Value(), "John Doe", role.Value())
	if err != nil {
		t.Fatalf("Failed to register user: %v", err)
	}

	// 1. Create User
	err = repo.Create(ctx, user)
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// 2. Find By ID
	found, err := repo.FindByID(ctx, user.ID())
	if err != nil {
		t.Fatalf("Failed to find user by ID: %v", err)
	}
	if found.Email() != user.Email() {
		t.Errorf("Expected email %s, got %s", user.Email(), found.Email())
	}

	// 3. Find By Email
	foundByEmail, err := repo.FindByEmail(ctx, user.Email())
	if err != nil {
		t.Fatalf("Failed to find user by email: %v", err)
	}
	if foundByEmail.ID() != user.ID() {
		t.Errorf("Expected ID %s, got %s", user.ID(), foundByEmail.ID())
	}

	// 4. Update User Social Links
	user.LinkGoogle("google-social-id-123")
	user.LinkFacebook("facebook-social-id-456")
	err = repo.Update(ctx, user)
	if err != nil {
		t.Fatalf("Failed to update user: %v", err)
	}

	// 5. Query via Social ID
	foundByGoogle, err := repo.FindByGoogleID(ctx, "google-social-id-123")
	if err != nil {
		t.Fatalf("Failed to find user by Google ID: %v", err)
	}
	if foundByGoogle.ID() != user.ID() {
		t.Errorf("Expected ID %s, got %s", user.ID(), foundByGoogle.ID())
	}

	foundByFacebook, err := repo.FindByFacebookID(ctx, "facebook-social-id-456")
	if err != nil {
		t.Fatalf("Failed to find user by Facebook ID: %v", err)
	}
	if foundByFacebook.ID() != user.ID() {
		t.Errorf("Expected ID %s, got %s", user.ID(), foundByFacebook.ID())
	}
}

func TestKeyRepository_Integration(t *testing.T) {
	db := getTestDB(t)
	truncateTables(db)
	defer truncateTables(db)

	repo := infraPostgres.NewKeyRepository(db)
	ctx := context.Background()

	keyRecord := &port.JWKRecord{
		ID:            "key-1",
		PrivateKeyPEM: "mock-priv-key-pem",
		PublicKeyPEM:  "mock-pub-key-pem",
		Algorithm:     "RS256",
		Status:        port.KeyStatusActive,
		CreatedAt:     time.Now(),
		ExpiresAt:     time.Now().Add(24 * time.Hour),
	}

	// 1. Save Key
	err := repo.Save(ctx, keyRecord)
	if err != nil {
		t.Fatalf("Failed to save JWK key: %v", err)
	}

	// 2. Get Active Key
	active, err := repo.GetActiveKey(ctx)
	if err != nil {
		t.Fatalf("Failed to get active key: %v", err)
	}
	if active.ID != "key-1" {
		t.Errorf("Expected active key-1, got %s", active.ID)
	}

	// 3. Update Status to Inactive
	err = repo.UpdateStatus(ctx, "key-1", port.KeyStatusInactive)
	if err != nil {
		t.Fatalf("Failed to update status: %v", err)
	}

	// Verify no active key now
	_, err = repo.GetActiveKey(ctx)
	if err == nil {
		t.Error("Expected error fetching active key when none exists, got nil")
	}

	// 4. Get all active and inactive keys
	all, err := repo.GetAllActiveAndInactiveKeys(ctx)
	if err != nil {
		t.Fatalf("Failed to get all keys: %v", err)
	}
	if len(all) != 1 {
		t.Errorf("Expected 1 key, got %d", len(all))
	}
	if all[0].Status != port.KeyStatusInactive {
		t.Errorf("Expected status to be inactive, got %s", all[0].Status)
	}
}

func TestSessionRepository_Integration(t *testing.T) {
	db := getTestDB(t)
	truncateTables(db)
	defer truncateTables(db)

	userRepo := infraPostgres.NewUserRepository(db)
	repo := infraPostgres.NewSessionRepository(db)
	ctx := context.Background()

	// Create user first because of foreign key constraint
	userID := uuid.New().String()
	email, _ := vo.NewEmail("session-test@example.com")
	role, _ := vo.NewRole("user")
	user, err := aggregate.RegisterUser(userID, email.Value(), "John Doe", role.Value())
	if err != nil {
		t.Fatalf("Failed to register user: %v", err)
	}
	err = userRepo.Create(ctx, user)
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	token := "refresh-token-123"
	expiresAt := time.Now().Add(1 * time.Hour)

	// 1. Save Session
	err = repo.Save(ctx, token, userID, expiresAt)
	if err != nil {
		t.Fatalf("Failed to save session: %v", err)
	}

	// 2. Find by Token
	sess, err := repo.FindByToken(ctx, token)
	if err != nil {
		t.Fatalf("Failed to find session: %v", err)
	}
	if sess.UserID != userID {
		t.Errorf("Expected user ID %s, got %s", userID, sess.UserID)
	}

	// 3. Delete Session
	err = repo.Delete(ctx, token)
	if err != nil {
		t.Fatalf("Failed to delete session: %v", err)
	}

	// Verify deleted
	_, err = repo.FindByToken(ctx, token)
	if err == nil {
		t.Error("Expected error finding deleted session, got nil")
	}

	// 4. Delete All By User ID
	_ = repo.Save(ctx, "token-a", userID, expiresAt)
	_ = repo.Save(ctx, "token-b", userID, expiresAt)
	err = repo.DeleteAllByUserID(ctx, userID)
	if err != nil {
		t.Fatalf("Failed to delete all by user id: %v", err)
	}

	_, err = repo.FindByToken(ctx, "token-a")
	if err == nil {
		t.Error("Expected token-a to be deleted")
	}
}

func TestOutboxRepository_Integration(t *testing.T) {
	db := getTestDB(t)
	truncateTables(db)
	defer truncateTables(db)

	repo := infraPostgres.NewOutboxRepository(db)
	ctx := context.Background()

	eventID := uuid.New().String()

	// 1. Save Event (eventID must be a valid UUID)
	err := repo.SaveEvent(ctx, eventID, "user.registered", []byte(`{"userId":"123"}`), "123")
	if err != nil {
		t.Fatalf("Failed to save outbox event: %v", err)
	}

	// 2. Fetch Unpublished
	events, err := repo.FetchUnpublished(ctx, 10)
	if err != nil {
		t.Fatalf("Failed to fetch unpublished events: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("Expected 1 unpublished event, got %d", len(events))
	}
	if events[0].EventID != eventID {
		t.Errorf("Expected event ID %s, got %s", eventID, events[0].EventID)
	}

	// 3. Mark Published
	err = repo.MarkPublished(ctx, []string{events[0].ID})
	if err != nil {
		t.Fatalf("Failed to mark event as published: %v", err)
	}

	// Verify no unpublished remaining
	events, err = repo.FetchUnpublished(ctx, 10)
	if err != nil {
		t.Fatalf("Failed to fetch unpublished events: %v", err)
	}
	if len(events) != 0 {
		t.Errorf("Expected 0 unpublished events, got %d", len(events))
	}
}

func TestOutboxRepository_ExecuteInLock_Integration(t *testing.T) {
	db := getTestDB(t)
	repo := infraPostgres.NewOutboxRepository(db)
	ctx := context.Background()

	var count int
	err := repo.ExecuteInLock(ctx, 99887766, func(txCtx context.Context) error {
		count++
		return nil
	})
	if err != nil {
		t.Fatalf("Failed to execute in lock: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected count to be 1, got %d", count)
	}
}

func TestSQLTransactionManager_Integration(t *testing.T) {
	db := getTestDB(t)
	truncateTables(db)
	defer truncateTables(db)

	txManager := infraPostgres.NewSQLTransactionManager(db)
	userRepo := infraPostgres.NewUserRepository(db)
	ctx := context.Background()

	userID := uuid.New().String()
	email := "tx-test@example.com"
	role := "user"

	// 1. Rollback case: error inside transaction
	err := txManager.WithTransaction(ctx, func(txCtx context.Context) error {
		user, _ := aggregate.RegisterUser(userID, email, "Test User", role)
		if err := userRepo.Create(txCtx, user); err != nil {
			return err
		}
		// Return error to force rollback
		return derror.ErrUserNotFound
	})

	if err == nil {
		t.Fatal("Expected transaction error to propagate, got nil")
	}

	// Check that user was NOT created due to rollback
	_, err = userRepo.FindByID(ctx, userID)
	if err == nil {
		t.Error("Expected user not to exist in DB due to transaction rollback")
	}

	// 2. Commit case: success inside transaction
	err = txManager.WithTransaction(ctx, func(txCtx context.Context) error {
		user, _ := aggregate.RegisterUser(userID, email, "Test User", role)
		if err := userRepo.Create(txCtx, user); err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		t.Fatalf("Transaction failed: %v", err)
	}

	// Check that user WAS created
	found, err := userRepo.FindByID(ctx, userID)
	if err != nil {
		t.Fatalf("Failed to find committed user: %v", err)
	}
	if found.Email() != email {
		t.Errorf("Expected email %s, got %s", email, found.Email())
	}
}

package command

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/viethung213/gym-companion/internal/auth/application/port"
)

// RotateKeysHandler generates a new key pair, sets it active, and deactivates old active ones.
type RotateKeysHandler struct {
	keyRepo        port.KeyRepository
	keyGen         port.KeyGenerator
	txManager      port.TransactionManager
	keyTTL         time.Duration
	gracePeriodTTL time.Duration
}

// NewRotateKeysHandler constructs a RotateKeysHandler instance with durations loaded from environment.
func NewRotateKeysHandler(
	keyRepo port.KeyRepository,
	keyGen port.KeyGenerator,
	txManager port.TransactionManager,
	keyTTL time.Duration,
	gracePeriodTTL time.Duration,
) *RotateKeysHandler {
	if keyTTL <= 0 {
		keyTTL = 7 * 24 * time.Hour
	}
	if gracePeriodTTL <= 0 {
		gracePeriodTTL = 24 * time.Hour
	}
	return &RotateKeysHandler{
		keyRepo:        keyRepo,
		keyGen:         keyGen,
		txManager:      txManager,
		keyTTL:         keyTTL,
		gracePeriodTTL: gracePeriodTTL,
	}
}

// Handle executes the key rotation logic using configured key and grace period TTLs.
func (h *RotateKeysHandler) Handle(ctx context.Context) (string, error) {
	// 1. Generate new active key in memory
	privPEM, pubPEM, err := h.keyGen.Generate(ctx)
	if err != nil {
		return "", fmt.Errorf("generate key pair: %w", err)
	}

	now := time.Now()
	newKey := &port.JWKRecord{
		ID:            uuid.New().String(),
		PrivateKeyPEM: privPEM,
		PublicKeyPEM:  pubPEM,
		Algorithm:     "RS256",
		Status:        port.KeyStatusActive,
		CreatedAt:     now,
		ExpiresAt:     now.Add(h.keyTTL),
	}

	executeRotation := func(txCtx context.Context) error {
		// 2. Deactivate all previous active keys and set their expiration to grace period
		graceExpiresAt := now.Add(h.gracePeriodTTL)
		if err := h.keyRepo.DeactivateAllActiveKeys(txCtx, graceExpiresAt); err != nil {
			return fmt.Errorf("deactivate active keys: %w", err)
		}

		// 3. Save new key
		if err := h.keyRepo.Save(txCtx, newKey); err != nil {
			return fmt.Errorf("save new key: %w", err)
		}

		// 4. Purge expired keys (expired inactive keys)
		if err := h.keyRepo.DeleteExpiredKeys(txCtx); err != nil {
			slog.WarnContext(txCtx, "Failed to purge expired keys", slog.Any("error", err))
		}

		return nil
	}

	if h.txManager != nil {
		if err := h.txManager.WithTransaction(ctx, executeRotation); err != nil {
			return "", fmt.Errorf("rotate keys transaction: %w", err)
		}
	} else {
		if err := executeRotation(ctx); err != nil {
			return "", err
		}
	}

	return newKey.ID, nil
}

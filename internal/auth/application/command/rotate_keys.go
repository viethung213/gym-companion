package command

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/viethung213/gym-companion/internal/auth/application/port"
)

// RotateKeysCommand represents the command to rotate JWT signing keys.
type RotateKeysCommand struct {
	KeyTTL         time.Duration
	GracePeriodTTL time.Duration
}

// RotateKeysHandler generates a new key pair, sets it active, and deactivates old active ones.
type RotateKeysHandler struct {
	keyRepo   port.KeyRepository
	keyGen    port.KeyGenerator
	txManager port.TransactionManager
}

// NewRotateKeysHandler constructs a RotateKeysHandler instance.
func NewRotateKeysHandler(
	keyRepo port.KeyRepository,
	keyGen port.KeyGenerator,
	txManager port.TransactionManager,
) *RotateKeysHandler {
	return &RotateKeysHandler{
		keyRepo:   keyRepo,
		keyGen:    keyGen,
		txManager: txManager,
	}
}

// Handle executes the key rotation logic.
func (h *RotateKeysHandler) Handle(ctx context.Context, cmd RotateKeysCommand) (string, error) {
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
		ExpiresAt:     now.Add(cmd.KeyTTL),
	}

	executeRotation := func(txCtx context.Context) error {
		// 2. Deactivate all previous active keys
		if err := h.keyRepo.DeactivateAllActiveKeys(txCtx); err != nil {
			return fmt.Errorf("deactivate active keys: %w", err)
		}

		// 3. Save new key
		if err := h.keyRepo.Save(txCtx, newKey); err != nil {
			return fmt.Errorf("save new key: %w", err)
		}

		// 4. Purge expired keys (expired inactive keys)
		if err := h.keyRepo.DeleteExpiredKeys(txCtx); err != nil {
			log.Printf("WARNING: Failed to purge expired keys: %v", err)
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

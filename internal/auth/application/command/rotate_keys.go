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
	keyRepo port.KeyRepository
	keyGen  port.KeyGenerator
}

// NewRotateKeysHandler constructs a RotateKeysHandler instance.
func NewRotateKeysHandler(
	keyRepo port.KeyRepository,
	keyGen port.KeyGenerator,
) *RotateKeysHandler {
	return &RotateKeysHandler{
		keyRepo: keyRepo,
		keyGen:  keyGen,
	}
}

// Handle executes the key rotation logic.
func (h *RotateKeysHandler) Handle(ctx context.Context, cmd RotateKeysCommand) (string, error) {
	// 1. Generate new active key
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

	// 2. Save new key
	if err := h.keyRepo.Save(ctx, newKey); err != nil {
		return "", fmt.Errorf("save new key: %w", err)
	}

	// 3. Find and deprecate all previous active keys to inactive status
	keys, err := h.keyRepo.GetAllActiveAndInactiveKeys(ctx)
	if err == nil {
		for _, key := range keys {
			if key.ID != newKey.ID && key.Status == port.KeyStatusActive {
				if err := h.keyRepo.UpdateStatus(ctx, key.ID, port.KeyStatusInactive); err != nil {
					log.Printf("WARNING: Failed to deprecate key %s: %v", key.ID, err)
				}
			}
		}
	}

	// 4. Purge expired keys (expired inactive keys)
	if err := h.keyRepo.DeleteExpiredKeys(ctx); err != nil {
		log.Printf("WARNING: Failed to purge expired keys: %v", err)
	}

	return newKey.ID, nil
}

//go:build unit

package command

import (
	"context"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/auth/application/port"
)

func TestRotateKeysHandler(t *testing.T) {
	ctx := context.Background()
	keyRepo := &mockKeyRepo{}
	keyGen := &mockKeyGenerator{privPEM: "mock-priv-pem", pubPEM: "mock-pub-pem"}
	txManager := &mockTxManager{}
	defaultTTL := 2 * time.Hour
	defaultGraceTTL := 30 * time.Minute
	handler := NewRotateKeysHandler(keyRepo, keyGen, txManager, defaultTTL, defaultGraceTTL)

	// --- First rotation ---
	kid, err := handler.Handle(ctx)
	if err != nil {
		t.Fatalf("first rotation failed: %v", err)
	}

	activeKey, err := keyRepo.GetActiveKey(ctx)
	if err != nil {
		t.Fatalf("could not find active key after first rotation: %v", err)
	}
	if activeKey.ID != kid {
		t.Errorf("active key mismatch: got %s, want %s", activeKey.ID, kid)
	}

	// --- Second rotation ---
	beforeRotate2 := time.Now()
	kid2, err := handler.Handle(ctx)
	if err != nil {
		t.Fatalf("second rotation failed: %v", err)
	}

	activeKey2, err := keyRepo.GetActiveKey(ctx)
	if err != nil {
		t.Fatalf("could not find active key after second rotation: %v", err)
	}
	if activeKey2.ID != kid2 {
		t.Errorf("active key mismatch: got %s, want %s", activeKey2.ID, kid2)
	}

	// First key must be marked inactive AND have ExpiresAt updated to GracePeriodTTL
	var key1Found, key1Inactive bool
	var key1ExpiresAt time.Time
	for _, k := range keyRepo.keys {
		if k.ID == kid {
			key1Found = true
			key1Inactive = k.Status == port.KeyStatusInactive
			key1ExpiresAt = k.ExpiresAt
		}
	}
	if !key1Found {
		t.Error("first key should still exist in repository after second rotation")
	}
	if !key1Inactive {
		t.Error("first key should have been marked inactive after second rotation")
	}
	expectedGraceExpiry := beforeRotate2.Add(defaultGraceTTL)
	if key1ExpiresAt.Before(expectedGraceExpiry.Add(-time.Minute)) || key1ExpiresAt.After(expectedGraceExpiry.Add(time.Minute)) {
		t.Errorf("expected grace expiry close to %v, got %v", expectedGraceExpiry, key1ExpiresAt)
	}
}

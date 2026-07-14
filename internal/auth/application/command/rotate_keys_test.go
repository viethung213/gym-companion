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
	handler := NewRotateKeysHandler(keyRepo, keyGen)

	// --- First rotation ---
	kid, err := handler.Handle(ctx, RotateKeysCommand{KeyTTL: 1 * time.Hour})
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
	kid2, err := handler.Handle(ctx, RotateKeysCommand{KeyTTL: 1 * time.Hour})
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

	// First key must be marked inactive
	var key1Found, key1Inactive bool
	for _, k := range keyRepo.keys {
		if k.ID == kid {
			key1Found = true
			key1Inactive = k.Status == port.KeyStatusInactive
		}
	}
	if !key1Found {
		t.Error("first key should still exist in repository after second rotation")
	}
	if !key1Inactive {
		t.Error("first key should have been marked inactive after second rotation")
	}
}

package crypto_test

import (
	"errors"
	"testing"

	"github.com/viethung213/gym-companion/internal/auth/application/port"
	"github.com/viethung213/gym-companion/internal/auth/infrastructure/crypto"
)

func TestArgon2Hasher_HashAndCompare(t *testing.T) {
	var hasher port.Hasher = crypto.NewCustomArgon2Hasher(crypto.TestArgon2Params())

	password := "SecretPassword!123"
	hashed, err := hasher.Hash(password)
	if err != nil {
		t.Fatalf("unexpected hash error: %v", err)
	}

	if hashed == "" {
		t.Fatal("expected non-empty hash string")
	}

	// Successful comparison
	if err := hasher.Compare(hashed, password); err != nil {
		t.Errorf("expected comparison to pass, got err: %v", err)
	}

	// Failed comparison with wrong password
	err = hasher.Compare(hashed, "WrongPassword")
	if !errors.Is(err, crypto.ErrMismatchedHash) {
		t.Errorf("got %v, want %v", err, crypto.ErrMismatchedHash)
	}
}

func TestArgon2Hasher_InvalidHashFormat(t *testing.T) {
	var hasher port.Hasher = crypto.NewCustomArgon2Hasher(crypto.TestArgon2Params())

	tests := []struct {
		name    string
		give    string
		wantErr error
	}{
		{
			name:    "Not enough parts",
			give:    "$argon2id$v=19$m=65536,t=3,p=2",
			wantErr: crypto.ErrInvalidHash,
		},
		{
			name:    "Invalid algorithm",
			give:    "$bcrypt$v=19$m=65536,t=3,p=2$salt$hash",
			wantErr: crypto.ErrInvalidHash,
		},
		{
			name:    "Invalid version",
			give:    "$argon2id$v=99$m=65536,t=3,p=2$salt$hash",
			wantErr: crypto.ErrIncompatibleVersion,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := hasher.Compare(tt.give, "password")
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("got err %v, want %v", err, tt.wantErr)
			}
		})
	}
}

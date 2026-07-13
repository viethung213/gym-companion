package grpc

import (
	"context"
	"fmt"

	"github.com/viethung213/gym-companion/internal/auth/application/port"
)

// AuthKeyProvider bridges the KeyRepository to the shared middleware KeyProvider interface.
type AuthKeyProvider struct {
	KeyRepo port.KeyRepository
}

// GetPublicKeyPEM retrieves the PEM string of a public key by its Key ID (kid).
func (p *AuthKeyProvider) GetPublicKeyPEM(ctx context.Context, kid string) (string, error) {
	keys, err := p.KeyRepo.GetAllActiveAndInactiveKeys(ctx)
	if err != nil {
		return "", err
	}
	for _, key := range keys {
		if key.ID == kid {
			return key.PublicKeyPEM, nil
		}
	}
	return "", fmt.Errorf("public key not found for kid: %s", kid)
}

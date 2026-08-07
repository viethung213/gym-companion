package grpc

import (
	"context"
	"fmt"

	"github.com/viethung213/gym-companion/internal/auth/application/port"
)

type AuthKeyProvider struct {
	KeyRepo port.KeyRepository
}

func (p *AuthKeyProvider) GetPublicKeyPEM(ctx context.Context, kid string) (string, error) {
	keys, err := p.KeyRepo.GetAllActiveAndInactiveKeys(ctx)
	if err != nil {
		return "", fmt.Errorf("find key by kid: %w", err)
	}
	for _, k := range keys {
		if k.ID == kid {
			return k.PublicKeyPEM, nil
		}
	}
	return "", fmt.Errorf("key with kid %s not found", kid)
}

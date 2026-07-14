package crypto

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"

	"github.com/viethung213/gym-companion/internal/auth/application/port"
)

// RSAKeyGenerator implements the port.KeyGenerator interface for RSA-2048 keys.
type RSAKeyGenerator struct{}

// Compile-time interface verification
var _ port.KeyGenerator = (*RSAKeyGenerator)(nil)

// NewRSAKeyGenerator creates a new instance of RSAKeyGenerator.
func NewRSAKeyGenerator() *RSAKeyGenerator {
	return &RSAKeyGenerator{}
}

// Generate implements the port to output PEM encoded public/private RSA-2048 key pairs.
func (g *RSAKeyGenerator) Generate(ctx context.Context) (string, string, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return "", "", fmt.Errorf("generate rsa private key: %w", err)
	}

	// Marshal private key to PKCS#8 DER
	privBytes, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return "", "", fmt.Errorf("marshal pkcs8 private key: %w", err)
	}
	privBlock := &pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: privBytes,
	}
	privatePEM := pem.EncodeToMemory(privBlock)

	// Marshal public key to PKIX DER
	pubBytes, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		return "", "", fmt.Errorf("marshal pkix public key: %w", err)
	}
	pubBlock := &pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubBytes,
	}
	publicPEM := pem.EncodeToMemory(pubBlock)

	return string(privatePEM), string(publicPEM), nil
}

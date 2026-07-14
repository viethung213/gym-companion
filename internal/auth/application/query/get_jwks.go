package query

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"

	"github.com/viethung213/gym-companion/internal/auth/application/port"
)

// JWKKeyDTO represents a single JSON Web Key in JWKS format.
type JWKKeyDTO struct {
	Kty string `json:"kty"`
	Use string `json:"use"`
	Alg string `json:"alg"`
	Kid string `json:"kid"`
	N   string `json:"n"`
	E   string `json:"e"`
}

// GetJWKSQuery represents a query to retrieve current JWKS.
type GetJWKSQuery struct{}

// GetJWKSHandler handles the query to fetch public key sets formatted as JWKS.
type GetJWKSHandler struct {
	keyRepo port.KeyRepository
}

// NewGetJWKSHandler creates a new instance of GetJWKSHandler.
func NewGetJWKSHandler(keyRepo port.KeyRepository) *GetJWKSHandler {
	return &GetJWKSHandler{keyRepo: keyRepo}
}

// Handle handles the query and converts key records to JWKS public key format.
func (h *GetJWKSHandler) Handle(ctx context.Context, _ GetJWKSQuery) ([]JWKKeyDTO, error) {
	keys, err := h.keyRepo.GetAllActiveAndInactiveKeys(ctx)
	if err != nil {
		return nil, fmt.Errorf("get active and inactive keys: %w", err)
	}

	jwkKeys := make([]JWKKeyDTO, 0, len(keys))

	for _, key := range keys {
		rsaPub, err := parseRSAPublicKey(key.PublicKeyPEM)
		if err != nil {
			// In production, we log error and skip invalid keys rather than failing the whole JWKS
			continue
		}

		nStr := base64RawURLEncode(rsaPub.N.Bytes())
		eStr := base64RawURLEncode(intToBytes(rsaPub.E))

		jwkKeys = append(jwkKeys, JWKKeyDTO{
			Kty: "RSA",
			Use: "sig",
			Alg: "RS256",
			Kid: key.ID,
			N:   nStr,
			E:   eStr,
		})
	}

	return jwkKeys, nil
}

func parseRSAPublicKey(pemStr string) (*rsa.PublicKey, error) {
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return nil, errors.New("failed to decode PEM block")
	}

	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse DER public key: %w", err)
	}

	rsaPub, ok := pub.(*rsa.PublicKey)
	if !ok {
		return nil, errors.New("key is not of type *rsa.PublicKey")
	}

	return rsaPub, nil
}

func base64RawURLEncode(b []byte) string {
	return base64.RawURLEncoding.EncodeToString(b)
}

func intToBytes(n int) []byte {
	if n == 65537 {
		return []byte{1, 0, 1}
	}
	buf := make([]byte, 4)
	buf[0] = byte(n >> 24)
	buf[1] = byte(n >> 16)
	buf[2] = byte(n >> 8)
	buf[3] = byte(n)
	i := 0
	for i < len(buf) && buf[i] == 0 {
		i++
	}
	return buf[i:]
}

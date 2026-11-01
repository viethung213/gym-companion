package middleware

import (
	"context"
	"crypto/rsa"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/viethung213/gym-companion/internal/gen/go/contracts/generic/auth/v1/service/authv1serviceconnect"
	"github.com/viethung213/gym-companion/internal/shared/internaltoken"
	"golang.org/x/net/http2"
)

// KeyProvider defines the interface to fetch public key PEM for JWT verification.
type KeyProvider interface {
	GetPublicKeyPEM(ctx context.Context, kid string) (string, error)
}

// jwksResponse represents standard RFC 7517 JSON Web Key Set structure.
type jwksResponse struct {
	Keys []struct {
		Kty string `json:"kty"`
		Kid string `json:"kid"`
		Use string `json:"use"`
		Alg string `json:"alg"`
		N   string `json:"n"`
		E   string `json:"e"`
	} `json:"keys"`
}

type cachedKey struct {
	key       *rsa.PublicKey
	expiresAt time.Time
}

// RemoteJWKSKeyProvider fetches public keys dynamically from Auth Service's GetJWKS endpoint.
type RemoteJWKSKeyProvider struct {
	internalSecret string
	mu             sync.RWMutex
	upstreamFunc   func() string
	cache          map[string]*cachedKey
	ttl            time.Duration
	client         *http.Client
}

// NewRemoteJWKSKeyProvider initializes a stateless key provider caching public keys in RAM.
func NewRemoteJWKSKeyProvider(upstreamFunc func() string, internalSecret string, ttl time.Duration) *RemoteJWKSKeyProvider {
	if ttl <= 0 {
		ttl = 15 * time.Minute
	}

	h2cTransport := &http2.Transport{
		AllowHTTP: true,
		DialTLSContext: func(ctx context.Context, network, addr string, _ *tls.Config) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, network, addr)
		},
		ReadIdleTimeout: 30 * time.Second,
		PingTimeout:     5 * time.Second,
	}

	return &RemoteJWKSKeyProvider{
		upstreamFunc:   upstreamFunc,
		internalSecret: internalSecret,
		cache:          make(map[string]*cachedKey),
		ttl:            ttl,
		client: &http.Client{
			Transport: h2cTransport,
			Timeout:   5 * time.Second,
		},
	}
}

// GetPublicKey fetches and returns an RSA public key by its key ID (kid).
func (p *RemoteJWKSKeyProvider) GetPublicKey(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	p.mu.RLock()
	if item, found := p.cache[kid]; found && time.Now().Before(item.expiresAt) {
		p.mu.RUnlock()
		return item.key, nil
	}
	p.mu.RUnlock()

	// Cache miss or expired: fetch fresh keys from Auth Service
	if err := p.refresh(ctx); err != nil {
		return nil, fmt.Errorf("refresh jwks: %w", err)
	}

	p.mu.RLock()
	defer p.mu.RUnlock()
	if item, found := p.cache[kid]; found {
		return item.key, nil
	}

	return nil, fmt.Errorf("public key not found for kid: %s", kid)
}

// GetPublicKeyPEM satisfies the KeyProvider interface.
func (p *RemoteJWKSKeyProvider) GetPublicKeyPEM(ctx context.Context, kid string) (string, error) {
	_, err := p.GetPublicKey(ctx, kid)
	if err != nil {
		return "", err
	}
	return "", nil
}

func (p *RemoteJWKSKeyProvider) refresh(ctx context.Context) error {
	baseURL := p.upstreamFunc()
	if baseURL == "" {
		return errors.New("no auth service upstream available to fetch JWKS")
	}

	endpoint := strings.TrimRight(baseURL, "/") + authv1serviceconnect.AuthServiceGetJWKSProcedure
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader("{}"))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if p.internalSecret != "" {
		if tok, tokErr := internaltoken.Generate("gateway", "service", "jwks-refresh", 1*time.Minute, p.internalSecret); tokErr == nil {
			req.Header.Set(HeaderInternalToken, tok)
		}
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("http request to jwks endpoint: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("jwks endpoint returned status: %d", resp.StatusCode)
	}

	var jwks jwksResponse
	if err := json.NewDecoder(resp.Body).Decode(&jwks); err != nil {
		return fmt.Errorf("decode jwks json: %w", err)
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	now := time.Now()
	for _, k := range jwks.Keys {
		if k.Kty != "RSA" || k.Kid == "" {
			continue
		}

		nBytes, err := base64.RawURLEncoding.DecodeString(k.N)
		if err != nil {
			continue
		}
		eBytes, err := base64.RawURLEncoding.DecodeString(k.E)
		if err != nil {
			continue
		}

		var eInt int
		for _, b := range eBytes {
			eInt = (eInt << 8) | int(b)
		}

		pubKey := &rsa.PublicKey{
			N: new(big.Int).SetBytes(nBytes),
			E: eInt,
		}

		p.cache[k.Kid] = &cachedKey{
			key:       pubKey,
			expiresAt: now.Add(p.ttl),
		}
	}

	return nil
}

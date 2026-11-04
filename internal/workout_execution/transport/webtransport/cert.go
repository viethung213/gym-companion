package webtransport

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"fmt"
	"math/big"
	"net"
	"time"
)

const (
	// Browsers only honour serverCertificateHashes for certificates valid for at most 14 days.
	certValidity = 10 * 24 * time.Hour
	// Renew when fewer than this much validity remains, so a freshly issued config hash always has runway.
	certRenewBefore = 3 * 24 * time.Hour
)

// certBundle is an in-memory self-signed ECDSA P-256 certificate plus its SHA-256 pin for
// WebTransport `serverCertificateHashes`. It is never written to disk.
type certBundle struct {
	tlsCert   tls.Certificate
	hash      [sha256.Size]byte
	notAfter  time.Time
	hashB64   string
	dnsNames  []string
	ipAddress []net.IP
}

func newCertBundle(now time.Time, hosts []string) (*certBundle, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate ecdsa key: %w", err)
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 127))
	if err != nil {
		return nil, fmt.Errorf("generate serial: %w", err)
	}

	dnsNames := []string{"localhost"}
	ips := []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")}
	for _, h := range hosts {
		if h == "" {
			continue
		}
		if ip := net.ParseIP(h); ip != nil {
			ips = append(ips, ip)
		} else {
			dnsNames = append(dnsNames, h)
		}
	}

	// Backdate slightly to tolerate client clock skew.
	notBefore := now.Add(-5 * time.Minute)
	notAfter := notBefore.Add(certValidity)
	tmpl := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: "gym-companion-webtransport"},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              dnsNames,
		IPAddresses:           ips,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return nil, fmt.Errorf("create certificate: %w", err)
	}

	sum := sha256.Sum256(der)
	return &certBundle{
		tlsCert:   tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key},
		hash:      sum,
		notAfter:  notAfter,
		hashB64:   base64.StdEncoding.EncodeToString(sum[:]),
		dnsNames:  dnsNames,
		ipAddress: ips,
	}, nil
}

func (c *certBundle) needsRenewal(now time.Time) bool {
	return c.notAfter.Sub(now) < certRenewBefore
}

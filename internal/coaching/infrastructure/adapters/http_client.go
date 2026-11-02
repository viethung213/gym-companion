// Package adapters provides infrastructure adapters for coaching ports.
package adapters

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"time"

	"golang.org/x/net/http2"
)

// newDefaultHTTPClient returns an http.Client configured for HTTP/2 cleartext (h2c)
// with standard connection and request timeouts.
func newDefaultHTTPClient() *http.Client {
	return &http.Client{
		Transport: &http2.Transport{
			AllowHTTP: true,
			DialTLSContext: func(ctx context.Context, network, addr string, _ *tls.Config) (net.Conn, error) {
				var d net.Dialer
				return d.DialContext(ctx, network, addr)
			},
			ReadIdleTimeout: 30 * time.Second,
			PingTimeout:     5 * time.Second,
		},
		Timeout: 10 * time.Second,
	}
}

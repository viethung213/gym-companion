// Package adapters provides infrastructure adapters for coaching ports.
package adapters

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"time"

	"github.com/viethung213/gym-companion/internal/shared/telemetry"
	"golang.org/x/net/http2"
)

// newDefaultHTTPClient returns an http.Client configured for HTTP/2 cleartext (h2c)
// with standard connection and request timeouts instrumented with OpenTelemetry tracing.
func newDefaultHTTPClient() *http.Client {
	h2cTransport := &http2.Transport{
		AllowHTTP: true,
		DialTLSContext: func(ctx context.Context, network, addr string, _ *tls.Config) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, network, addr)
		},
		ReadIdleTimeout: 30 * time.Second,
		PingTimeout:     5 * time.Second,
	}
	return &http.Client{
		Transport: telemetry.WrapHTTPTransport(h2cTransport),
		Timeout:   10 * time.Second,
	}
}

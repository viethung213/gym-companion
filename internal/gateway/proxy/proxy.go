package proxy

import (
	"context"
	"crypto/tls"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"github.com/viethung213/gym-companion/internal/gateway/gwerror"
	"golang.org/x/net/http2"
)

// Engine manages reverse proxies with strict HTTP/2 (h2c) cleartext multiplexing.
type Engine struct {
	h2cTransport *http2.Transport
}

// NewEngine initializes an upstream transport engine strictly enforcing HTTP/2 (h2c) for microservices.
func NewEngine() *Engine {
	return &Engine{
		h2cTransport: &http2.Transport{
			AllowHTTP: true,
			DialTLSContext: func(ctx context.Context, network, addr string, _ *tls.Config) (net.Conn, error) {
				var d net.Dialer
				return d.DialContext(ctx, network, addr)
			},
			ReadIdleTimeout: 30 * time.Second,
			PingTimeout:     5 * time.Second,
		},
	}
}

// CreateProxy creates an httputil.ReverseProxy with strict HTTP/2 h2c transport.
func (pe *Engine) CreateProxy(target *url.URL) *httputil.ReverseProxy {
	p := httputil.NewSingleHostReverseProxy(target)
	p.Transport = pe.h2cTransport
	p.FlushInterval = -1 // Flush immediately for real-time ConnectRPC & AI streaming

	originalDirector := p.Director
	p.Director = func(req *http.Request) {
		originalDirector(req)
		req.Host = target.Host
	}

	p.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		reqID := r.Header.Get("X-Request-ID")
		log.Printf("[Gateway Proxy Error] [ReqID: %s] %s %s -> %s: %v", reqID, r.Method, r.URL.Path, target.String(), err)
		gwerror.BadGateway(w, r, "Service temporarily unavailable. Please try again later.")
	}

	return p
}

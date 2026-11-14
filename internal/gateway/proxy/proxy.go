package proxy

import (
	"context"
	"crypto/tls"
	"log/slog"
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

// Hooks defines optional lifecycle callbacks invoked before response headers are written to client.
type Hooks struct {
	OnResponse func(statusCode int)
	OnError    func(err error)
}

// CreateProxy creates an httputil.ReverseProxy with strict HTTP/2 h2c transport and optional lifecycle hooks.
func (pe *Engine) CreateProxy(target *url.URL, hooks ...Hooks) *httputil.ReverseProxy {
	p := httputil.NewSingleHostReverseProxy(target)
	p.Transport = pe.h2cTransport
	p.FlushInterval = -1 // Flush immediately for real-time ConnectRPC & AI streaming

	originalDirector := p.Director
	p.Director = func(req *http.Request) {
		originalDirector(req)
		req.Host = target.Host
	}

	var hook Hooks
	if len(hooks) > 0 {
		hook = hooks[0]
	}

	if hook.OnResponse != nil {
		p.ModifyResponse = func(resp *http.Response) error {
			hook.OnResponse(resp.StatusCode)
			return nil
		}
	}

	p.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		reqID := r.Header.Get("X-Request-ID")
		slog.ErrorContext(r.Context(), "Gateway proxy error",
			"req_id", reqID,
			"method", r.Method,
			"path", r.URL.Path,
			"target", target.String(),
			"error", err,
		)
		if hook.OnError != nil {
			hook.OnError(err)
		}
		gwerror.BadGateway(w, r, "Service temporarily unavailable. Please try again later.")
	}

	return p
}

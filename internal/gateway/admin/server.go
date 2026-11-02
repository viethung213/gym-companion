package admin

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"time"

	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

// Server encapsulates the Gateway Admin Control Plane HTTP server.
type Server struct {
	httpServer *http.Server
	addr       string
}

// NewServer creates a new admin control plane HTTP server.
func NewServer(port, adminKey string, inspector Inspector) *Server {
	if port == "" {
		port = "8001"
	}
	addr := net.JoinHostPort("", port)

	handler := NewHandler(adminKey, inspector)
	h2cHandler := h2c.NewHandler(handler.Routes(), &http2.Server{})

	srv := &http.Server{
		Addr:              addr,
		Handler:           h2cHandler,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       30 * time.Second,
	}

	return &Server{
		httpServer: srv,
		addr:       addr,
	}
}

// Start begins listening and serving incoming admin requests.
func (s *Server) Start() error {
	log.Printf("🛡️ [Gateway Admin] Control Plane listening on http://0.0.0.0%s", s.addr)
	if err := s.httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("gateway admin server: %w", err)
	}
	return nil
}

// Serve accepts incoming HTTP connections on the listener l.
func (s *Server) Serve(l net.Listener) error {
	if err := s.httpServer.Serve(l); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("gateway admin server serve: %w", err)
	}
	return nil
}

// Handler returns the HTTP handler used by the admin server.
func (s *Server) Handler() http.Handler {
	return s.httpServer.Handler
}

// Shutdown gracefully stops the admin server.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

package discovery

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/http2"
)

// Config configures dynamic registration with the API Gateway.
type Config struct {
	GatewayAdminURL   string
	AdminKey          string
	ServiceID         string
	ServicePort       string
	UpstreamURL       string
	Prefixes          []string
	HeartbeatInterval time.Duration
	HTTPClient        *http.Client
}

// Registrar coordinates background registration and heartbeats with the API Gateway.
type Registrar struct {
	cfg         Config
	adminURL    string
	upstreamURL string
	client      *http.Client
	stopChan    chan struct{}
	wg          sync.WaitGroup
	registered  bool
	mu          sync.Mutex
}

// NewRegistrar creates a new service registrar instance.
func NewRegistrar(cfg Config) *Registrar {
	if cfg.GatewayAdminURL == "" {
		cfg.GatewayAdminURL = "http://localhost:8001"
	}
	adminURL := strings.TrimRight(cfg.GatewayAdminURL, "/")

	if cfg.AdminKey == "" {
		cfg.AdminKey = "gym-companion-admin-secret-2026"
	}
	if cfg.HeartbeatInterval <= 0 {
		cfg.HeartbeatInterval = 5 * time.Second
	}

	upstream := cfg.UpstreamURL
	if upstream == "" {
		upstream = ResolveSelfAddress(cfg.ServicePort)
	}

	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{
			Transport: &http2.Transport{
				AllowHTTP: true,
				DialTLSContext: func(ctx context.Context, network, addr string, _ *tls.Config) (net.Conn, error) {
					var d net.Dialer
					return d.DialContext(ctx, network, addr)
				},
				ReadIdleTimeout: 30 * time.Second,
				PingTimeout:     5 * time.Second,
			},
			Timeout: 5 * time.Second,
		}
	}

	return &Registrar{
		cfg:         cfg,
		adminURL:    adminURL,
		upstreamURL: upstream,
		client:      httpClient,
		stopChan:    make(chan struct{}),
	}
}

// UpstreamURL returns the auto-detected or configured upstream address of this instance.
func (r *Registrar) UpstreamURL() string {
	return r.upstreamURL
}

// Start initiates registration and starts the heartbeat goroutine.
func (r *Registrar) Start(ctx context.Context) {
	log.Printf("📡 [Service Discovery] Self upstream: %s -> Gateway Admin: %s (Service ID: %s)", r.upstreamURL, r.adminURL, r.cfg.ServiceID)

	// Attempt initial registration immediately
	if err := r.register(ctx); err != nil {
		log.Printf("⚠️ [Service Discovery] Initial registration for %s failed (%v); will retry in %v",
			r.cfg.ServiceID, err, r.cfg.HeartbeatInterval)
	} else {
		r.mu.Lock()
		r.registered = true
		r.mu.Unlock()
		log.Printf("✅ [Service Discovery] Successfully registered %s (%s) with API Gateway",
			r.cfg.ServiceID, r.upstreamURL)
	}

	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		ticker := time.NewTicker(r.cfg.HeartbeatInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-r.stopChan:
				return
			case <-ticker.C:
				if err := r.register(ctx); err != nil {
					log.Printf("⚠️ [Service Discovery] Heartbeat registration failed for %s: %v", r.cfg.ServiceID, err)
				} else {
					r.mu.Lock()
					if !r.registered {
						r.registered = true
						log.Printf("✅ [Service Discovery] Recovered and registered %s (%s) with API Gateway",
							r.cfg.ServiceID, r.upstreamURL)
					}
					r.mu.Unlock()
				}
			}
		}
	}()
}

// Stop stops the heartbeat loop and deregisters from the Gateway.
func (r *Registrar) Stop(ctx context.Context) error {
	close(r.stopChan)
	r.wg.Wait()

	if err := r.deregister(ctx); err != nil {
		return fmt.Errorf("deregister from gateway: %w", err)
	}
	log.Printf("👋 [Service Discovery] Deregistered %s (%s) from API Gateway", r.cfg.ServiceID, r.upstreamURL)
	return nil
}

func (r *Registrar) register(ctx context.Context) error {
	payload := map[string]any{
		"service_id":   r.cfg.ServiceID,
		"upstream_url": r.upstreamURL,
		"prefixes":     r.cfg.Prefixes,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal register payload: %w", err)
	}

	endpoint := fmt.Sprintf("%s/admin/discovery/register", r.adminURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create register request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if r.cfg.AdminKey != "" {
		req.Header.Set("X-Admin-Key", r.cfg.AdminKey)
	}

	resp, err := r.client.Do(req)
	if err != nil {
		return fmt.Errorf("send register request to %s: %w", endpoint, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("gateway responded with status %d from %s", resp.StatusCode, endpoint)
	}
	return nil
}

func (r *Registrar) deregister(ctx context.Context) error {
	payload := map[string]string{
		"service_id":   r.cfg.ServiceID,
		"upstream_url": r.upstreamURL,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal deregister payload: %w", err)
	}

	endpoint := fmt.Sprintf("%s/admin/discovery/deregister", r.adminURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create deregister request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if r.cfg.AdminKey != "" {
		req.Header.Set("X-Admin-Key", r.cfg.AdminKey)
	}

	resp, err := r.client.Do(req)
	if err != nil {
		return fmt.Errorf("send deregister request to %s: %w", endpoint, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("gateway responded with status %d from %s", resp.StatusCode, endpoint)
	}
	return nil
}


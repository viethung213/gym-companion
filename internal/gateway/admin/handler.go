package admin

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/viethung213/gym-companion/internal/gateway/config"
	"github.com/viethung213/gym-companion/internal/gateway/discovery"
	"github.com/viethung213/gym-companion/internal/gateway/gwerror"
	"github.com/viethung213/gym-companion/internal/gateway/telemetry"
)

// HeaderAdminKey defines the request header required for authenticating with the Admin API.
const HeaderAdminKey = "X-Admin-Key"

// Inspector định nghĩa các phương thức cần thiết để Control Plane truy xuất số liệu và quản lý discovery.
type Inspector interface {
	GetOverview() telemetry.OverviewStats
	GetUpstreamStats() []telemetry.UpstreamStats
	QueryTraces(filter telemetry.TraceFilter) []telemetry.RequestJourney
	FormatPrometheus() string
	GetConfig() *config.Config
	RegisterService(serviceID, upstreamURL string, prefixes []string) error
	DeregisterService(serviceID, upstreamURL string) error
	GetDiscoveryInstances() []discovery.Instance
}

// Handler handles administrative and telemetry endpoints.
type Handler struct {
	adminKey  string
	inspector Inspector
}

// NewHandler initializes an Admin Handler.
func NewHandler(adminKey string, inspector Inspector) *Handler {
	return &Handler{
		adminKey:  adminKey,
		inspector: inspector,
	}
}

// Routes builds and returns an http.Handler with all registered admin endpoints.
func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()

	// Health check (accessible without auth)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"OK","service":"gateway-admin"}`))
	})

	// Protected endpoints
	mux.HandleFunc("/admin/overview", h.handleOverview)
	mux.HandleFunc("/admin/upstreams", h.handleUpstreams)
	mux.HandleFunc("/admin/config", h.handleConfig)
	mux.HandleFunc("/admin/traces", h.handleTraces)
	mux.HandleFunc("/admin/discovery/register", h.handleDiscoveryRegister)
	mux.HandleFunc("/admin/discovery/deregister", h.handleDiscoveryDeregister)
	mux.HandleFunc("/admin/discovery/instances", h.handleDiscoveryInstances)
	mux.HandleFunc("/metrics", h.handleMetrics)

	return h.authMiddleware(mux)
}

func (h *Handler) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Bypass health check
		if r.URL.Path == "/healthz" {
			next.ServeHTTP(w, r)
			return
		}

		key := r.Header.Get(HeaderAdminKey)
		if key == "" || subtle.ConstantTimeCompare([]byte(key), []byte(h.adminKey)) != 1 {
			gwerror.Unauthenticated(w, r, "Invalid or missing admin key")
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (h *Handler) handleOverview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		gwerror.MethodNotAllowed(w, r)
		return
	}

	stats := h.inspector.GetOverview()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(stats)
}

func (h *Handler) handleUpstreams(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		gwerror.MethodNotAllowed(w, r)
		return
	}

	upstreams := h.inspector.GetUpstreamStats()
	if upstreams == nil {
		upstreams = []telemetry.UpstreamStats{}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(upstreams)
}

func (h *Handler) handleConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		gwerror.MethodNotAllowed(w, r)
		return
	}

	cfg := h.inspector.GetConfig()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(cfg)
}

func (h *Handler) handleTraces(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		gwerror.MethodNotAllowed(w, r)
		return
	}

	q := r.URL.Query()
	filter := telemetry.TraceFilter{
		RequestID:  q.Get("requestId"),
		Path:       q.Get("path"),
		Slowest:    strings.EqualFold(q.Get("slowest"), "true") || q.Get("slowest") == "1",
		ErrorsOnly: strings.EqualFold(q.Get("errors"), "true") || q.Get("errors") == "1",
	}

	if limitStr := q.Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			filter.Limit = l
		}
	}

	traces := h.inspector.QueryTraces(filter)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(traces)
}

func (h *Handler) handleMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		gwerror.MethodNotAllowed(w, r)
		return
	}

	prom := h.inspector.FormatPrometheus()
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	_, _ = w.Write([]byte(prom))
}

type discoveryRequest struct {
	ServiceID      string   `json:"service_id"`
	ServiceIDAlt   string   `json:"serviceId"`
	UpstreamURL    string   `json:"upstream_url"`
	UpstreamURLAlt string   `json:"upstreamUrl"`
	Prefixes       []string `json:"prefixes"`
}

func (h *Handler) handleDiscoveryRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		gwerror.MethodNotAllowed(w, r)
		return
	}

	var req discoveryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		gwerror.BadRequest(w, r, "Invalid JSON payload")
		return
	}

	serviceID := req.ServiceID
	if serviceID == "" {
		serviceID = req.ServiceIDAlt
	}
	upstreamURL := req.UpstreamURL
	if upstreamURL == "" {
		upstreamURL = req.UpstreamURLAlt
	}

	if serviceID == "" || upstreamURL == "" {
		gwerror.BadRequest(w, r, "service_id and upstream_url are required")
		return
	}

	if err := h.inspector.RegisterService(serviceID, upstreamURL, req.Prefixes); err != nil {
		gwerror.Write(w, r, http.StatusBadRequest, "REGISTRATION_FAILED", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status":       "OK",
		"message":      "Registered successfully",
		"service_id":   serviceID,
		"upstream_url": upstreamURL,
	})
}

func (h *Handler) handleDiscoveryDeregister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		gwerror.MethodNotAllowed(w, r)
		return
	}

	var req discoveryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		gwerror.BadRequest(w, r, "Invalid JSON payload")
		return
	}

	serviceID := req.ServiceID
	if serviceID == "" {
		serviceID = req.ServiceIDAlt
	}
	upstreamURL := req.UpstreamURL
	if upstreamURL == "" {
		upstreamURL = req.UpstreamURLAlt
	}

	if serviceID == "" || upstreamURL == "" {
		gwerror.BadRequest(w, r, "service_id and upstream_url are required")
		return
	}

	if err := h.inspector.DeregisterService(serviceID, upstreamURL); err != nil {
		gwerror.Write(w, r, http.StatusBadRequest, "DEREGISTRATION_FAILED", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status":  "OK",
		"message": "Deregistered successfully",
	})
}

func (h *Handler) handleDiscoveryInstances(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		gwerror.MethodNotAllowed(w, r)
		return
	}

	instances := h.inspector.GetDiscoveryInstances()
	if instances == nil {
		instances = []discovery.Instance{}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(instances)
}

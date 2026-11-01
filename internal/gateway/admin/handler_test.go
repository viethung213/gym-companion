package admin

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/viethung213/gym-companion/internal/gateway/config"
	"github.com/viethung213/gym-companion/internal/gateway/telemetry"
)

type mockInspector struct {
	overview  telemetry.OverviewStats
	upstreams []telemetry.UpstreamStats
	traces    []telemetry.RequestJourney
	prom      string
	cfg       *config.Config
}

func (m *mockInspector) GetOverview() telemetry.OverviewStats {
	return m.overview
}

func (m *mockInspector) GetUpstreamStats() []telemetry.UpstreamStats {
	return m.upstreams
}

func (m *mockInspector) QueryTraces(filter telemetry.TraceFilter) []telemetry.RequestJourney {
	result := make([]telemetry.RequestJourney, 0, len(m.traces))
	for _, t := range m.traces {
		if filter.RequestID != "" && t.RequestID != filter.RequestID {
			continue
		}
		if filter.ErrorsOnly && t.StatusCode < 400 {
			continue
		}
		result = append(result, t)
	}
	if filter.Limit > 0 && len(result) > filter.Limit {
		result = result[:filter.Limit]
	}
	return result
}

func (m *mockInspector) FormatPrometheus() string {
	return m.prom
}

func (m *mockInspector) GetConfig() *config.Config {
	return m.cfg
}

func TestAdminHandler_Healthz(t *testing.T) {
	mock := &mockInspector{}
	handler := NewHandler("secret-key", mock).Routes()

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 OK for healthz, got %d", rec.Code)
	}
}

func TestAdminHandler_Authentication(t *testing.T) {
	mock := &mockInspector{}
	handler := NewHandler("my-admin-key", mock).Routes()

	tests := []struct {
		name       string
		headerKey  string
		wantStatus int
	}{
		{
			name:       "Missing header",
			headerKey:  "",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "Invalid key",
			headerKey:  "wrong-key",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "Valid key",
			headerKey:  "my-admin-key",
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/admin/overview", nil)
			if tt.headerKey != "" {
				req.Header.Set(HeaderAdminKey, tt.headerKey)
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("got status %d, want %d", rec.Code, tt.wantStatus)
			}
		})
	}
}

func TestAdminHandler_Endpoints(t *testing.T) {
	mock := &mockInspector{
		overview: telemetry.OverviewStats{
			TotalRequests: 100,
			QPS:           10.5,
		},
		upstreams: []telemetry.UpstreamStats{
			{TargetHost: "http://127.0.0.1:50051", Healthy: true},
		},
		traces: []telemetry.RequestJourney{
			{RequestID: "req-1", StatusCode: 200, DurationMs: 12.5},
			{RequestID: "req-2", StatusCode: 500, DurationMs: 45.0},
		},
		prom: "# HELP gateway_requests_total\ngateway_requests_total 100\n",
		cfg: &config.Config{
			AllowedHosts: []string{"localhost"},
		},
	}

	adminKey := "test-key"
	handler := NewHandler(adminKey, mock).Routes()

	// 1. Overview
	{
		req := httptest.NewRequest(http.MethodGet, "/admin/overview", nil)
		req.Header.Set(HeaderAdminKey, adminKey)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("Overview got status %d, want 200", rec.Code)
		}
		var ov telemetry.OverviewStats
		if err := json.NewDecoder(rec.Body).Decode(&ov); err != nil {
			t.Fatalf("failed to decode overview JSON: %v", err)
		}
		if ov.TotalRequests != 100 {
			t.Errorf("TotalRequests got %d, want 100", ov.TotalRequests)
		}
	}

	// 2. Upstreams
	{
		req := httptest.NewRequest(http.MethodGet, "/admin/upstreams", nil)
		req.Header.Set(HeaderAdminKey, adminKey)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("Upstreams got status %d, want 200", rec.Code)
		}
		var list []telemetry.UpstreamStats
		if err := json.NewDecoder(rec.Body).Decode(&list); err != nil {
			t.Fatalf("failed to decode upstreams JSON: %v", err)
		}
		if len(list) != 1 || list[0].TargetHost != "http://127.0.0.1:50051" {
			t.Errorf("Upstreams data mismatch: %+v", list)
		}
	}

	// 3. Config
	{
		req := httptest.NewRequest(http.MethodGet, "/admin/config", nil)
		req.Header.Set(HeaderAdminKey, adminKey)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("Config got status %d, want 200", rec.Code)
		}
		var cfg config.Config
		if err := json.NewDecoder(rec.Body).Decode(&cfg); err != nil {
			t.Fatalf("failed to decode config JSON: %v", err)
		}
		if len(cfg.AllowedHosts) != 1 || cfg.AllowedHosts[0] != "localhost" {
			t.Errorf("Config data mismatch: %+v", cfg)
		}
	}

	// 4. Traces with Query params
	{
		req := httptest.NewRequest(http.MethodGet, "/admin/traces?errors=true&limit=10", nil)
		req.Header.Set(HeaderAdminKey, adminKey)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("Traces got status %d, want 200", rec.Code)
		}
		var traces []telemetry.RequestJourney
		if err := json.NewDecoder(rec.Body).Decode(&traces); err != nil {
			t.Fatalf("failed to decode traces JSON: %v", err)
		}
		if len(traces) != 1 || traces[0].RequestID != "req-2" {
			t.Errorf("Traces filtered data mismatch: %+v", traces)
		}
	}

	// 5. Metrics
	{
		req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
		req.Header.Set(HeaderAdminKey, adminKey)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("Metrics got status %d, want 200", rec.Code)
		}
		body := rec.Body.String()
		if !strings.Contains(body, "gateway_requests_total 100") {
			t.Errorf("Metrics response mismatch: %s", body)
		}
	}

	// 6. Method Not Allowed
	{
		req := httptest.NewRequest(http.MethodPost, "/admin/overview", nil)
		req.Header.Set(HeaderAdminKey, adminKey)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected 405 Method Not Allowed, got %d", rec.Code)
		}
	}
}

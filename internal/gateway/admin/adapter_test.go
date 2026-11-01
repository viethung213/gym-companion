package admin

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/viethung213/gym-companion/internal/gateway/config"
	"github.com/viethung213/gym-companion/internal/gateway/telemetry"
)

func TestAdapter(t *testing.T) {
	t.Parallel()

	tel := telemetry.NewTelemetry(10)
	done := tel.Metrics.RecordRequestStart("http://upstream:8080")
	done(200, nil)

	cfg := &config.Config{
		AllowedHosts: []string{"localhost"},
	}
	cfgProvider := &mockConfigProvider{cfg: cfg}

	adapter := NewAdapter(tel, cfgProvider)

	// Overview
	ov := adapter.GetOverview()
	assert.Equal(t, uint64(1), ov.TotalRequests)

	// Upstreams
	upstreams := adapter.GetUpstreamStats()
	assert.Len(t, upstreams, 1)

	// Traces
	traces := adapter.QueryTraces(telemetry.TraceFilter{})
	assert.Empty(t, traces)

	// Prometheus
	prom := adapter.FormatPrometheus()
	assert.Contains(t, prom, "gateway_requests_total")

	// Config
	retrievedCfg := adapter.GetConfig()
	assert.Equal(t, cfg, retrievedCfg)
}

func TestAdapter_NilDependencies(t *testing.T) {
	t.Parallel()

	adapter := NewAdapter(nil, nil)

	assert.Equal(t, uint64(0), adapter.GetOverview().TotalRequests)
	assert.Empty(t, adapter.GetUpstreamStats())
	assert.Empty(t, adapter.QueryTraces(telemetry.TraceFilter{}))
	assert.Empty(t, adapter.FormatPrometheus())
	assert.Nil(t, adapter.GetConfig())
}

type mockConfigProvider struct {
	cfg *config.Config
}

func (m *mockConfigProvider) GetConfig() *config.Config {
	return m.cfg
}

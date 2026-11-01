package telemetry

import (
	"strings"
	"testing"
)

func TestTelemetry_Initialization(t *testing.T) {
	tel := NewTelemetry(100)
	if tel.Metrics == nil {
		t.Fatal("Metrics should not be nil")
	}
	if tel.Traces == nil {
		t.Fatal("Traces should not be nil")
	}
}

func TestTelemetry_PrometheusFormat(t *testing.T) {
	tel := NewTelemetry(100)
	done := tel.Metrics.RecordRequestStart("http://auth-service:50051")
	done(200, nil)

	prom := tel.FormatPrometheus()
	if !strings.Contains(prom, "gateway_requests_total 1") {
		t.Errorf("Prometheus missing gateway_requests_total: %s", prom)
	}
	if !strings.Contains(prom, "gateway_upstream_requests_total") {
		t.Errorf("Prometheus missing upstream metrics: %s", prom)
	}
}

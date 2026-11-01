package telemetry

import (
	"fmt"
	"strings"
)

// Telemetry groups real-time metrics collection and request journey tracing.
type Telemetry struct {
	Metrics *MetricsCollector
	Traces  *TraceStore
}

// NewTelemetry creates and initializes a unified Telemetry instance.
func NewTelemetry(traceCapacity int) *Telemetry {
	return &Telemetry{
		Metrics: NewMetricsCollector(),
		Traces:  NewTraceStore(traceCapacity),
	}
}

// GetOverview returns a snapshot of gateway health, traffic, and Go runtime stats.
func (t *Telemetry) GetOverview() OverviewStats {
	if t.Metrics == nil {
		return OverviewStats{}
	}
	return t.Metrics.GetOverview()
}

// GetUpstreamStats returns latency, health, and error metrics for each backend upstream.
func (t *Telemetry) GetUpstreamStats() []UpstreamStats {
	if t.Metrics == nil {
		return []UpstreamStats{}
	}
	return t.Metrics.GetUpstreamStats()
}

// QueryTraces searches the ring buffer of recent request journeys.
func (t *Telemetry) QueryTraces(filter TraceFilter) []RequestJourney {
	if t.Traces == nil {
		return []RequestJourney{}
	}
	return t.Traces.Query(filter)
}

// FormatPrometheus generates Prometheus text exposition format metrics.
func (t *Telemetry) FormatPrometheus() string {
	ov := t.Metrics.GetOverview()
	upstreams := t.Metrics.GetUpstreamStats()

	var sb strings.Builder

	sb.WriteString("# HELP gateway_requests_total Total number of HTTP requests processed by the gateway.\n")
	sb.WriteString("# TYPE gateway_requests_total counter\n")
	fmt.Fprintf(&sb, "gateway_requests_total %d\n\n", ov.TotalRequests)

	sb.WriteString("# HELP gateway_active_requests Number of requests currently being processed.\n")
	sb.WriteString("# TYPE gateway_active_requests gauge\n")
	fmt.Fprintf(&sb, "gateway_active_requests %d\n\n", ov.ActiveRequests)

	sb.WriteString("# HELP gateway_requests_qps Current calculated requests per second.\n")
	sb.WriteString("# TYPE gateway_requests_qps gauge\n")
	fmt.Fprintf(&sb, "gateway_requests_qps %.2f\n\n", ov.QPS)

	sb.WriteString("# HELP gateway_requests_duration_ms_avg Average request processing latency in milliseconds.\n")
	sb.WriteString("# TYPE gateway_requests_duration_ms_avg gauge\n")
	fmt.Fprintf(&sb, "gateway_requests_duration_ms_avg %.2f\n\n", ov.AvgLatencyMs)

	sb.WriteString("# HELP gateway_requests_success_total Total successful 2xx responses.\n")
	sb.WriteString("# TYPE gateway_requests_success_total counter\n")
	fmt.Fprintf(&sb, "gateway_requests_success_total %d\n\n", ov.Success2xx)

	sb.WriteString("# HELP gateway_client_errors_total Total 4xx client errors.\n")
	sb.WriteString("# TYPE gateway_client_errors_total counter\n")
	fmt.Fprintf(&sb, "gateway_client_errors_total %d\n\n", ov.ClientErrors4xx)

	sb.WriteString("# HELP gateway_server_errors_total Total 5xx server/upstream errors.\n")
	sb.WriteString("# TYPE gateway_server_errors_total counter\n")
	fmt.Fprintf(&sb, "gateway_server_errors_total %d\n\n", ov.ServerErrors5xx)

	sb.WriteString("# HELP gateway_rate_limited_total Total requests rejected due to rate limit.\n")
	sb.WriteString("# TYPE gateway_rate_limited_total counter\n")
	fmt.Fprintf(&sb, "gateway_rate_limited_total %d\n\n", ov.RateLimitedCount)

	sb.WriteString("# HELP gateway_host_rejected_total Total requests rejected due to host whitelist.\n")
	sb.WriteString("# TYPE gateway_host_rejected_total counter\n")
	fmt.Fprintf(&sb, "gateway_host_rejected_total %d\n\n", ov.HostRejectedCount)

	sb.WriteString("# HELP gateway_auth_rejected_total Total requests rejected due to authentication failure.\n")
	sb.WriteString("# TYPE gateway_auth_rejected_total counter\n")
	fmt.Fprintf(&sb, "gateway_auth_rejected_total %d\n\n", ov.AuthRejectedCount)

	sb.WriteString("# HELP gateway_uptime_seconds Total runtime of the gateway in seconds.\n")
	sb.WriteString("# TYPE gateway_uptime_seconds counter\n")
	fmt.Fprintf(&sb, "gateway_uptime_seconds %d\n\n", ov.UptimeSeconds)

	sb.WriteString("# HELP go_goroutines Number of goroutines currently existing.\n")
	sb.WriteString("# TYPE go_goroutines gauge\n")
	fmt.Fprintf(&sb, "go_goroutines %d\n\n", ov.Resources.NumGoroutines)

	sb.WriteString("# HELP go_memstats_alloc_bytes Number of bytes allocated and in use.\n")
	sb.WriteString("# TYPE go_memstats_alloc_bytes gauge\n")
	fmt.Fprintf(&sb, "go_memstats_alloc_bytes %.0f\n\n", ov.Resources.AllocMB*1024*1024)

	if len(upstreams) > 0 {
		sb.WriteString("# HELP gateway_upstream_requests_total Total requests forwarded to backend upstream.\n")
		sb.WriteString("# TYPE gateway_upstream_requests_total counter\n")
		for _, u := range upstreams {
			fmt.Fprintf(&sb, "gateway_upstream_requests_total{upstream=%q} %d\n", u.TargetHost, u.TotalRequests)
		}
		sb.WriteString("\n")

		sb.WriteString("# HELP gateway_upstream_active_conns Active connections to backend upstream.\n")
		sb.WriteString("# TYPE gateway_upstream_active_conns gauge\n")
		for _, u := range upstreams {
			fmt.Fprintf(&sb, "gateway_upstream_active_conns{upstream=%q} %d\n", u.TargetHost, u.ActiveConns)
		}
		sb.WriteString("\n")

		sb.WriteString("# HELP gateway_upstream_latency_avg_ms Average latency for backend upstream in milliseconds.\n")
		sb.WriteString("# TYPE gateway_upstream_latency_avg_ms gauge\n")
		for _, u := range upstreams {
			fmt.Fprintf(&sb, "gateway_upstream_latency_avg_ms{upstream=%q} %.2f\n", u.TargetHost, u.AvgLatencyMs)
		}
		sb.WriteString("\n")

		sb.WriteString("# HELP gateway_upstream_healthy Health status of backend upstream (1 = healthy, 0 = unhealthy).\n")
		sb.WriteString("# TYPE gateway_upstream_healthy gauge\n")
		for _, u := range upstreams {
			healthVal := 0
			if u.Healthy {
				healthVal = 1
			}
			fmt.Fprintf(&sb, "gateway_upstream_healthy{upstream=%q} %d\n", u.TargetHost, healthVal)
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

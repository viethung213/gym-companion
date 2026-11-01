package telemetry

import (
	"errors"
	"testing"
	"time"
)

func TestMetricsCollector_BasicFlow(t *testing.T) {
	mc := NewMetricsCollector()

	host := "http://localhost:50051"

	// Successful request
	done1 := mc.RecordRequestStart(host)
	time.Sleep(5 * time.Millisecond)
	done1(200, nil)

	// Client error request
	done2 := mc.RecordRequestStart(host)
	time.Sleep(2 * time.Millisecond)
	done2(400, nil)

	// Server error request
	done3 := mc.RecordRequestStart(host)
	done3(500, errors.New("upstream connection reset"))

	// Special rejects
	mc.RecordRateLimited()
	mc.RecordHostRejected()
	mc.RecordAuthRejected()

	ov := mc.GetOverview()
	if got, want := ov.TotalRequests, uint64(3); got != want {
		t.Errorf("TotalRequests got %d, want %d", got, want)
	}
	if got, want := ov.ActiveRequests, int64(0); got != want {
		t.Errorf("ActiveRequests got %d, want %d", got, want)
	}
	if got, want := ov.Success2xx, uint64(1); got != want {
		t.Errorf("Success2xx got %d, want %d", got, want)
	}
	if got, want := ov.ClientErrors4xx, uint64(1); got != want {
		t.Errorf("ClientErrors4xx got %d, want %d", got, want)
	}
	if got, want := ov.ServerErrors5xx, uint64(1); got != want {
		t.Errorf("ServerErrors5xx got %d, want %d", got, want)
	}
	if got, want := ov.RateLimitedCount, uint64(1); got != want {
		t.Errorf("RateLimitedCount got %d, want %d", got, want)
	}
	if got, want := ov.HostRejectedCount, uint64(1); got != want {
		t.Errorf("HostRejectedCount got %d, want %d", got, want)
	}
	if got, want := ov.AuthRejectedCount, uint64(1); got != want {
		t.Errorf("AuthRejectedCount got %d, want %d", got, want)
	}
	if ov.AvgLatencyMs <= 0 {
		t.Errorf("AvgLatencyMs got %.2f, want > 0", ov.AvgLatencyMs)
	}
	if ov.Resources.NumGoroutines <= 0 {
		t.Errorf("NumGoroutines got %d, want > 0", ov.Resources.NumGoroutines)
	}

	upstreams := mc.GetUpstreamStats()
	if len(upstreams) != 1 {
		t.Fatalf("UpstreamStats length got %d, want 1", len(upstreams))
	}
	u := upstreams[0]
	if got, want := u.TargetHost, host; got != want {
		t.Errorf("TargetHost got %s, want %s", got, want)
	}
	if got, want := u.TotalRequests, uint64(3); got != want {
		t.Errorf("Upstream TotalRequests got %d, want %d", got, want)
	}
	if !u.Healthy {
		t.Errorf("Healthy got false, want true (only 1 consecutive error)")
	}
	if u.LastError != "upstream connection reset" {
		t.Errorf("LastError got %s, want 'upstream connection reset'", u.LastError)
	}
}

func TestMetricsCollector_UnhealthyUpstream(t *testing.T) {
	mc := NewMetricsCollector()
	host := "http://unhealthy-service:50052"

	for i := 0; i < 5; i++ {
		done := mc.RecordRequestStart(host)
		done(503, errors.New("service unavailable"))
	}

	upstreams := mc.GetUpstreamStats()
	if len(upstreams) != 1 {
		t.Fatalf("Upstreams len got %d, want 1", len(upstreams))
	}
	if upstreams[0].Healthy {
		t.Errorf("Upstream should be unhealthy after 5 consecutive errors")
	}

	// Recovery
	done := mc.RecordRequestStart(host)
	done(200, nil)

	upstreams = mc.GetUpstreamStats()
	if !upstreams[0].Healthy {
		t.Errorf("Upstream should recover to healthy after 200 OK")
	}
}

func TestMetricsCollector_TrackUpstreamConn(t *testing.T) {
	mc := NewMetricsCollector()
	host := "http://upstream-1:50051"

	dec := mc.TrackUpstreamConn(host)
	stats := mc.GetUpstreamStats()
	if len(stats) != 1 || stats[0].ActiveConns != 1 {
		t.Errorf("ActiveConns got %d, want 1", stats[0].ActiveConns)
	}

	dec()
	stats = mc.GetUpstreamStats()
	if stats[0].ActiveConns != 0 {
		t.Errorf("ActiveConns after decrement got %d, want 0", stats[0].ActiveConns)
	}
}

package telemetry

import (
	"math"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

// SystemResourceUsage contains Go runtime memory and goroutine stats.
type SystemResourceUsage struct {
	AllocMB       float64 `json:"allocMB"`
	TotalAllocMB  float64 `json:"totalAllocMB"`
	SysMB         float64 `json:"sysMB"`
	NumGoroutines int     `json:"numGoroutines"`
	NumGC         uint32  `json:"numGC"`
}

// OverviewStats represents a top-level snapshot of gateway health, traffic, and resources.
type OverviewStats struct {
	UptimeSeconds     int64               `json:"uptimeSeconds"`
	TotalRequests     uint64              `json:"totalRequests"`
	ActiveRequests    int64               `json:"activeRequests"`
	QPS               float64             `json:"qps"`
	AvgLatencyMs      float64             `json:"avgLatencyMs"`
	Success2xx        uint64              `json:"success2xx"`
	ClientErrors4xx   uint64              `json:"clientErrors4xx"`
	ServerErrors5xx   uint64              `json:"serverErrors5xx"`
	RateLimitedCount  uint64              `json:"rateLimitedCount"`
	HostRejectedCount uint64              `json:"hostRejectedCount"`
	AuthRejectedCount uint64              `json:"authRejectedCount"`
	Resources         SystemResourceUsage `json:"resources"`
}

// UpstreamStats represents aggregated health and performance metrics for a specific backend host.
type UpstreamStats struct {
	TargetHost        string  `json:"targetHost"`
	ActiveConns       int64   `json:"activeConns"`
	TotalRequests     uint64  `json:"totalRequests"`
	Success2xx        uint64  `json:"success2xx"`
	ClientErrors4xx   uint64  `json:"clientErrors4xx"`
	ServerErrors5xx   uint64  `json:"serverErrors5xx"`
	AvgLatencyMs      float64 `json:"avgLatencyMs"`
	MinLatencyMs      float64 `json:"minLatencyMs"`
	MaxLatencyMs      float64 `json:"maxLatencyMs"`
	LastError         string  `json:"lastError,omitempty"`
	ConsecutiveErrors int     `json:"consecutiveErrors"`
	Healthy           bool    `json:"healthy"`
}

type upstreamData struct {
	mu                sync.RWMutex
	targetHost        string
	activeConns       atomic.Int64
	totalRequests     atomic.Uint64
	success2xx        atomic.Uint64
	clientErrors4xx   atomic.Uint64
	serverErrors5xx   atomic.Uint64
	totalDurationNs   atomic.Int64
	minDurationNs     atomic.Int64
	maxDurationNs     atomic.Int64
	lastError         string
	consecutiveErrors int
}

// MetricsCollector tracks real-time traffic, latency, errors, and system resources.
type MetricsCollector struct {
	startTime         time.Time
	totalRequests     atomic.Uint64
	activeRequests    atomic.Int64
	success2xx        atomic.Uint64
	clientErrors4xx   atomic.Uint64
	serverErrors5xx   atomic.Uint64
	rateLimitedCount  atomic.Uint64
	hostRejectedCount atomic.Uint64
	authRejectedCount atomic.Uint64
	totalDurationNs   atomic.Int64
	upstreams         sync.Map // map[string]*upstreamData
}

// NewMetricsCollector initializes a thread-safe metrics collector.
func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		startTime: time.Now(),
	}
}

// TrackUpstreamConn increments active connections to an upstream and returns a function to decrement it.
func (mc *MetricsCollector) TrackUpstreamConn(upstreamHost string) func() {
	if upstreamHost == "" {
		return func() {}
	}
	val, _ := mc.upstreams.LoadOrStore(upstreamHost, &upstreamData{
		targetHost: upstreamHost,
	})
	uData, ok := val.(*upstreamData)
	if !ok {
		return func() {}
	}
	uData.activeConns.Add(1)
	return func() {
		uData.activeConns.Add(-1)
	}
}

// RecordUpstreamResult records response metrics for a specific backend host.
func (mc *MetricsCollector) RecordUpstreamResult(upstreamHost string, durNs int64, statusCode int, err error) {
	if upstreamHost == "" {
		return
	}
	val, _ := mc.upstreams.LoadOrStore(upstreamHost, &upstreamData{
		targetHost: upstreamHost,
	})
	uData, ok := val.(*upstreamData)
	if !ok {
		return
	}
	uData.totalRequests.Add(1)
	uData.totalDurationNs.Add(durNs)

	// Update min/max latency atomically using CAS loops
	for {
		currentMin := uData.minDurationNs.Load()
		if currentMin == 0 || durNs < currentMin {
			if uData.minDurationNs.CompareAndSwap(currentMin, durNs) {
				break
			}
		} else {
			break
		}
	}

	for {
		currentMax := uData.maxDurationNs.Load()
		if durNs > currentMax {
			if uData.maxDurationNs.CompareAndSwap(currentMax, durNs) {
				break
			}
		} else {
			break
		}
	}

	switch {
	case statusCode >= 200 && statusCode < 300:
		uData.success2xx.Add(1)
		uData.mu.Lock()
		uData.consecutiveErrors = 0
		uData.mu.Unlock()
	case statusCode >= 400 && statusCode < 500:
		uData.clientErrors4xx.Add(1)
	case statusCode >= 500 || err != nil:
		uData.serverErrors5xx.Add(1)
		uData.mu.Lock()
		uData.consecutiveErrors++
		if err != nil {
			uData.lastError = err.Error()
		}
		uData.mu.Unlock()
	}
}

// RecordRequestStart increments active requests and returns a completion callback to record results.
func (mc *MetricsCollector) RecordRequestStart(upstreamHost string) func(statusCode int, err error) {
	mc.totalRequests.Add(1)
	mc.activeRequests.Add(1)

	var uDone func()
	if upstreamHost != "" {
		uDone = mc.TrackUpstreamConn(upstreamHost)
	}

	start := time.Now()

	return func(statusCode int, err error) {
		mc.activeRequests.Add(-1)
		durNs := time.Since(start).Nanoseconds()
		mc.totalDurationNs.Add(durNs)

		switch {
		case statusCode >= 200 && statusCode < 300:
			mc.success2xx.Add(1)
		case statusCode >= 400 && statusCode < 500:
			mc.clientErrors4xx.Add(1)
		case statusCode >= 500 || err != nil:
			mc.serverErrors5xx.Add(1)
		}

		if upstreamHost != "" {
			if uDone != nil {
				uDone()
			}
			mc.RecordUpstreamResult(upstreamHost, durNs, statusCode, err)
		}
	}
}

// RecordRateLimited increments the count of requests dropped by rate limiting.
func (mc *MetricsCollector) RecordRateLimited() {
	mc.rateLimitedCount.Add(1)
}

// RecordHostRejected increments the count of requests dropped by the host whitelist filter.
func (mc *MetricsCollector) RecordHostRejected() {
	mc.hostRejectedCount.Add(1)
}

// RecordAuthRejected increments the count of requests dropped by JWT authentication.
func (mc *MetricsCollector) RecordAuthRejected() {
	mc.authRejectedCount.Add(1)
}

// GetOverview builds and returns a complete system health overview.
func (mc *MetricsCollector) GetOverview() OverviewStats {
	uptime := time.Since(mc.startTime)
	uptimeSec := int64(uptime.Seconds())
	if uptimeSec <= 0 {
		uptimeSec = 1
	}

	totalReq := mc.totalRequests.Load()
	qps := math.Round((float64(totalReq)/float64(uptimeSec))*100) / 100

	var avgLatency float64
	if totalReq > 0 {
		totalMs := float64(mc.totalDurationNs.Load()) / 1e6
		avgLatency = math.Round((totalMs/float64(totalReq))*100) / 100
	}

	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	resources := SystemResourceUsage{
		AllocMB:       math.Round((float64(m.Alloc)/1024/1024)*100) / 100,
		TotalAllocMB:  math.Round((float64(m.TotalAlloc)/1024/1024)*100) / 100,
		SysMB:         math.Round((float64(m.Sys)/1024/1024)*100) / 100,
		NumGoroutines: runtime.NumGoroutine(),
		NumGC:         m.NumGC,
	}

	return OverviewStats{
		UptimeSeconds:     uptimeSec,
		TotalRequests:     totalReq,
		ActiveRequests:    mc.activeRequests.Load(),
		QPS:               qps,
		AvgLatencyMs:      avgLatency,
		Success2xx:        mc.success2xx.Load(),
		ClientErrors4xx:   mc.clientErrors4xx.Load(),
		ServerErrors5xx:   mc.serverErrors5xx.Load(),
		RateLimitedCount:  mc.rateLimitedCount.Load(),
		HostRejectedCount: mc.hostRejectedCount.Load(),
		AuthRejectedCount: mc.authRejectedCount.Load(),
		Resources:         resources,
	}
}

// GetUpstreamStats returns a slice of all tracked backend upstream hosts with their metrics.
func (mc *MetricsCollector) GetUpstreamStats() []UpstreamStats {
	var list []UpstreamStats

	mc.upstreams.Range(func(_, val any) bool {
		u, ok := val.(*upstreamData)
		if !ok {
			return true
		}
		total := u.totalRequests.Load()

		var avgMs, minMs, maxMs float64
		if total > 0 {
			avgMs = math.Round((float64(u.totalDurationNs.Load())/1e6/float64(total))*100) / 100
			minMs = math.Round((float64(u.minDurationNs.Load())/1e6)*100) / 100
			maxMs = math.Round((float64(u.maxDurationNs.Load())/1e6)*100) / 100
		}

		u.mu.RLock()
		lastErr := u.lastError
		consecErrs := u.consecutiveErrors
		u.mu.RUnlock()

		// A host is marked unhealthy if it suffered >= 5 consecutive 5xx/network errors
		healthy := consecErrs < 5

		list = append(list, UpstreamStats{
			TargetHost:        u.targetHost,
			ActiveConns:       u.activeConns.Load(),
			TotalRequests:     total,
			Success2xx:        u.success2xx.Load(),
			ClientErrors4xx:   u.clientErrors4xx.Load(),
			ServerErrors5xx:   u.serverErrors5xx.Load(),
			AvgLatencyMs:      avgMs,
			MinLatencyMs:      minMs,
			MaxLatencyMs:      maxMs,
			LastError:         lastErr,
			ConsecutiveErrors: consecErrs,
			Healthy:           healthy,
		})
		return true
	})

	return list
}

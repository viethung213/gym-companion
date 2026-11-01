package gateway

import (
	"fmt"
	"math"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/rs/cors"
	"github.com/viethung213/gym-companion/internal/gateway/balancer"
	"github.com/viethung213/gym-companion/internal/gateway/config"
	"github.com/viethung213/gym-companion/internal/gateway/middleware"
	"github.com/viethung213/gym-companion/internal/gateway/proxy"
	"github.com/viethung213/gym-companion/internal/gateway/telemetry"
	"github.com/viethung213/gym-companion/internal/gen/go/contracts/generic/auth/v1/service/authv1serviceconnect"
)

// RuntimeRoute represents an in-memory prefix route to a balanced upstream service.
type RuntimeRoute struct {
	ServiceID  string
	PathPrefix string
	LB         *balancer.LoadBalancer
	Proxies    sync.Map // map[string]*httputil.ReverseProxy
}

// Gateway manages dynamic routing, reverse proxying, and authentication.
type Gateway struct {
	mu          sync.RWMutex
	routes      []*RuntimeRoute
	configMgr   *config.Manager
	proxyEngine *proxy.Engine
	hostFilter  *middleware.HostFilter
	pipeline    *middleware.Pipeline
	telemetry   *telemetry.Telemetry
}

// Options contains parameters to initialize the Gateway.
type Options struct {
	ConfigPath      string
	KeyProvider     middleware.KeyProvider
	InternalSecret  string
	TokenTTL        time.Duration
	WatchInterval   time.Duration
	EnforceTLS      bool
	TraceBufferSize int
}

// NewGateway creates and initializes a Gateway instance.
//
//nolint:gocritic // opts is startup configuration struct passed by value
func NewGateway(opts Options) (*Gateway, error) {
	if opts.TokenTTL <= 0 {
		opts.TokenTTL = time.Minute
	}
	if opts.WatchInterval <= 0 {
		opts.WatchInterval = 3 * time.Second
	}

	bufSize := opts.TraceBufferSize
	if bufSize <= 0 {
		bufSize = 2000
	}

	gw := &Gateway{
		proxyEngine: proxy.NewEngine(),
		hostFilter:  middleware.NewHostFilter(),
		telemetry:   telemetry.NewTelemetry(bufSize),
	}

	// If no KeyProvider is supplied, auto-bind to dynamic Auth upstream from routes!
	kp := opts.KeyProvider
	if kp == nil {
		kp = middleware.NewRemoteJWKSKeyProvider(gw.GetAuthUpstreamURL, opts.InternalSecret, 15*time.Minute)
	}

	gw.pipeline = middleware.NewPipeline(middleware.PipelineOptions{
		HostFilter:     gw.hostFilter,
		KeyProvider:    kp,
		InternalSecret: opts.InternalSecret,
		TokenTTL:       opts.TokenTTL,
		EnforceTLS:     opts.EnforceTLS,
	})

	cm, err := config.NewManager(opts.ConfigPath, gw.onConfigChange)
	if err != nil {
		return nil, fmt.Errorf("init config manager: %w", err)
	}
	gw.configMgr = cm
	cm.StartWatcher(opts.WatchInterval)

	return gw, nil
}

func (gw *Gateway) onConfigChange(cfg *config.Config) {
	newRoutes := make([]*RuntimeRoute, 0)

	for _, svc := range cfg.Services {
		lb, err := balancer.New(svc.Upstreams)
		if err != nil {
			continue
		}

		for _, prefix := range svc.Prefixes {
			newRoutes = append(newRoutes, &RuntimeRoute{
				ServiceID:  svc.ID,
				PathPrefix: prefix,
				LB:         lb,
			})
		}
	}

	// Sort routes longest prefix first for unambiguous matching
	sort.Slice(newRoutes, func(i, j int) bool {
		return len(newRoutes[i].PathPrefix) > len(newRoutes[j].PathPrefix)
	})

	// Update allowed hosts in HostFilter
	gw.hostFilter.SetAllowedHosts(cfg.AllowedHosts)

	gw.mu.Lock()
	gw.routes = newRoutes
	gw.mu.Unlock()
}

// AddAllowedHost dynamically registers a new domain into the gateway's whitelist.
func (gw *Gateway) AddAllowedHost(host string) {
	gw.hostFilter.AddHost(host)
}

// RemoveAllowedHost removes a domain name from the gateway's whitelist.
func (gw *Gateway) RemoveAllowedHost(host string) bool {
	return gw.hostFilter.RemoveHost(host)
}

// GetAllowedHosts returns all currently permitted hosts in the gateway.
func (gw *Gateway) GetAllowedHosts() []string {
	return gw.hostFilter.GetAllowedHosts()
}

// Telemetry returns the underlying telemetry engine for metric and trace inspection.
func (gw *Gateway) Telemetry() *telemetry.Telemetry {
	return gw.telemetry
}

// ConfigManager returns the configuration manager instance.
func (gw *Gateway) ConfigManager() *config.Manager {
	return gw.configMgr
}

// Close cleans up resources such as configuration file watchers.
func (gw *Gateway) Close() {
	if gw.configMgr != nil {
		gw.configMgr.Close()
	}
}

// GetAuthUpstreamURL finds an upstream URL capable of serving AuthService.
func (gw *Gateway) GetAuthUpstreamURL() string {
	route := gw.MatchRoute(authv1serviceconnect.AuthServiceGetJWKSProcedure)
	if route == nil || route.LB == nil {
		return ""
	}
	target := route.LB.Next()
	if target == nil {
		return ""
	}
	return target.String()
}

// MatchRoute matches an incoming URL path against configured runtime routes.
func (gw *Gateway) MatchRoute(path string) *RuntimeRoute {
	gw.mu.RLock()
	defer gw.mu.RUnlock()

	for _, r := range gw.routes {
		if strings.HasPrefix(path, r.PathPrefix) {
			return r
		}
	}
	return nil
}

type statusRecorder struct {
	http.ResponseWriter
	statusCode int
	written    bool
}

func (r *statusRecorder) WriteHeader(code int) {
	if !r.written {
		r.statusCode = code
		r.written = true
		r.ResponseWriter.WriteHeader(code)
	}
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	if !r.written {
		r.statusCode = http.StatusOK
		r.written = true
	}
	return r.ResponseWriter.Write(b)
}

func (r *statusRecorder) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (r *statusRecorder) Unwrap() http.ResponseWriter {
	return r.ResponseWriter
}

// Handler returns the full http.Handler with CORS, Pipeline, Routing, and Telemetry.
func (gw *Gateway) Handler() http.Handler {
	mux := http.NewServeMux()

	// Health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"OK","service":"gateway"}`))
	})

	// Reverse proxy dispatcher
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		route := gw.MatchRoute(r.URL.Path)
		if route == nil {
			http.Error(w, `{"code":"NOT_FOUND","message":"Route not found"}`, http.StatusNotFound)
			return
		}

		if route.LB == nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"code":"UNAVAILABLE","message":"No load balancer configured for service"}`))
			return
		}

		target := route.LB.Next()
		if target == nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"code":"UNAVAILABLE","message":"No healthy upstream instances available for service"}`))
			return
		}

		key := target.String()

		// Record selected upstream for telemetry
		r.Header.Set("X-Selected-Upstream", key)

		uDone := gw.telemetry.Metrics.TrackUpstreamConn(key)
		defer uDone()

		uStart := time.Now()

		val, ok := route.Proxies.Load(key)
		var p http.Handler
		if !ok {
			created := gw.proxyEngine.CreateProxy(target)
			route.Proxies.Store(key, created)
			p = created
		} else if handler, isHandler := val.(http.Handler); isHandler {
			p = handler
		} else {
			created := gw.proxyEngine.CreateProxy(target)
			route.Proxies.Store(key, created)
			p = created
		}

		p.ServeHTTP(w, r)

		if rec, ok := w.(*statusRecorder); ok {
			uDurNs := time.Since(uStart).Nanoseconds()
			gw.telemetry.Metrics.RecordUpstreamResult(key, uDurNs, rec.statusCode, nil)
		}
	})

	// Wrap in Security & Translation Pipeline
	wrappedPipeline := gw.pipeline.Wrap(mux)

	// Wrap in CORS
	corsHandler := cors.New(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{
			"Accept", "Content-Type", "Content-Length", "Accept-Encoding",
			"Authorization", "Connect-Protocol-Version", "Connect-Timeout-Ms",
			"Connect-Accept-Encoding", "Connect-Content-Encoding", "X-User-Id", "X-GRPC-Web",
			middleware.HeaderRequestID,
		},
		ExposedHeaders: []string{
			"Content-Type", "Content-Length", "Connect-Content-Encoding",
			"Grpc-Status", "Grpc-Message", "Grpc-Status-Details-Bin",
			middleware.HeaderRequestID,
		},
		AllowCredentials: true,
		MaxAge:           7200,
	}).Handler(wrappedPipeline)

	// Outer Telemetry wrapper
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}

		done := gw.telemetry.Metrics.RecordRequestStart("")
		defer func() {
			durNs := time.Since(start).Nanoseconds()
			durMs := math.Round(float64(durNs)/1e6*100) / 100
			done(rec.statusCode, nil)

			switch rec.statusCode {
			case http.StatusTooManyRequests:
				gw.telemetry.Metrics.RecordRateLimited()
			case http.StatusForbidden:
				gw.telemetry.Metrics.RecordHostRejected()
			case http.StatusUnauthorized:
				gw.telemetry.Metrics.RecordAuthRejected()
			}

			reqID := r.Header.Get(middleware.HeaderRequestID)
			if reqID == "" {
				reqID = w.Header().Get(middleware.HeaderRequestID)
			}
			upstreamHost := r.Header.Get("X-Selected-Upstream")
			if upstreamHost == "" {
				upstreamHost = "gateway"
			}

			gw.telemetry.Traces.Record(&telemetry.RequestJourney{
				RequestID:        reqID,
				Timestamp:        start,
				ClientIP:         middleware.ExtractClientIP(r),
				Method:           r.Method,
				Path:             r.URL.Path,
				SelectedUpstream: upstreamHost,
				StatusCode:       rec.statusCode,
				DurationMs:       durMs,
			})
		}()

		corsHandler.ServeHTTP(rec, r)
	})
}

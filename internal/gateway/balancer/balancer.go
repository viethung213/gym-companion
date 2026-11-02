package balancer

import (
	"errors"
	"net/url"
	"sync"
	"sync/atomic"
	"time"
)

// ErrNoUpstreamsAvailable occurs when a route has empty upstreams.
var ErrNoUpstreamsAvailable = errors.New("no upstream servers available")

// State represents the circuit breaker state of an upstream node.
type State int32

const (
	// StateClosed indicates normal operation; requests flow through.
	StateClosed State = iota
	// StateOpen indicates the circuit is broken; requests are fast-failed or bypassed.
	StateOpen
	// StateHalfOpen indicates a trial state; a limited probe request is tested.
	StateHalfOpen
)

func (s State) String() string {
	switch s {
	case StateClosed:
		return "CLOSED"
	case StateOpen:
		return "OPEN"
	case StateHalfOpen:
		return "HALF-OPEN"
	default:
		return "UNKNOWN"
	}
}

// Config defines the parameters for the circuit breaker.
type Config struct {
	FailureThreshold    int           // Consecutive failures required to trip the circuit (default: 5)
	CooldownTimeout     time.Duration // Time to wait in StateOpen before transitioning to StateHalfOpen (default: 10s)
	HalfOpenSuccessGoal int           // Successful probe requests in StateHalfOpen to close circuit (default: 1)
}

// DefaultConfig returns recommended production defaults.
func DefaultConfig() Config {
	return Config{
		FailureThreshold:    5,
		CooldownTimeout:     10 * time.Second,
		HalfOpenSuccessGoal: 1,
	}
}

// UpstreamNode represents a managed upstream server with circuit breaker tracking.
type UpstreamNode struct {
	URL                 *url.URL
	State               atomic.Int32 // StateClosed, StateOpen, StateHalfOpen
	ConsecutiveFailures atomic.Int32
	HalfOpenSuccesses   atomic.Int32
	LastFailureTime     atomic.Int64 // Unix nanoseconds
}

// LoadBalancer distributes traffic across healthy upstream servers using round-robin and per-node circuit breakers.
type LoadBalancer struct {
	mu      sync.RWMutex
	nodes   []*UpstreamNode
	counter atomic.Uint64
	cfg     Config
}

// New parses upstream URLs and initializes the round-robin load balancer with default circuit breaker config.
func New(upstreams []string) (*LoadBalancer, error) {
	return NewWithConfig(upstreams, DefaultConfig())
}

// NewWithConfig initializes the load balancer with custom circuit breaker settings.
func NewWithConfig(upstreams []string, cfg Config) (*LoadBalancer, error) {
	if len(upstreams) == 0 {
		return nil, ErrNoUpstreamsAvailable
	}

	if cfg.FailureThreshold <= 0 {
		cfg.FailureThreshold = 5
	}
	if cfg.CooldownTimeout <= 0 {
		cfg.CooldownTimeout = 10 * time.Second
	}
	if cfg.HalfOpenSuccessGoal <= 0 {
		cfg.HalfOpenSuccessGoal = 1
	}

	nodes := make([]*UpstreamNode, 0, len(upstreams))
	for _, raw := range upstreams {
		parsed, err := url.Parse(raw)
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, &UpstreamNode{
			URL: parsed,
		})
	}

	return &LoadBalancer{
		nodes: nodes,
		cfg:   cfg,
	}, nil
}

// NewEmpty initializes an empty LoadBalancer for dynamic registration.
func NewEmpty() *LoadBalancer {
	return &LoadBalancer{
		nodes: make([]*UpstreamNode, 0),
		cfg:   DefaultConfig(),
	}
}

// Next selects the next healthy upstream server using round-robin with circuit breaker checks.
// If all nodes are tripped (StateOpen), it returns nil to allow the caller to Fast-Fail immediately.
func (lb *LoadBalancer) Next() *url.URL {
	lb.mu.RLock()
	defer lb.mu.RUnlock()

	n := len(lb.nodes)
	if n == 0 {
		return nil
	}

	startIdx := lb.counter.Add(1) - 1
	now := time.Now().UnixNano()

	for i := 0; i < n; i++ {
		idx := (startIdx + uint64(i)) % uint64(n)
		node := lb.nodes[idx]
		state := State(node.State.Load())

		switch state {
		case StateClosed:
			return node.URL

		case StateHalfOpen:
			// In half-open trial, permit request through to probe service health
			return node.URL

		case StateOpen:
			lastFail := node.LastFailureTime.Load()
			if now-lastFail >= lb.cfg.CooldownTimeout.Nanoseconds() {
				// Cooldown expired; atomically transition to Half-Open for probe
				if node.State.CompareAndSwap(int32(StateOpen), int32(StateHalfOpen)) {
					node.HalfOpenSuccesses.Store(0)
					return node.URL
				}
			}
			// Node still in cooldown; skip and inspect next node
		}
	}

	// All upstream nodes are currently OPEN and cooling down
	return nil
}

// ReportSuccess notifies the load balancer that a request to target succeeded.
// Resets consecutive failures and transitions Half-Open nodes back to Closed.
func (lb *LoadBalancer) ReportSuccess(target *url.URL) {
	if target == nil {
		return
	}
	lb.mu.RLock()
	defer lb.mu.RUnlock()

	targetStr := target.String()
	for _, node := range lb.nodes {
		if node.URL.String() == targetStr {
			node.ConsecutiveFailures.Store(0)
			state := State(node.State.Load())
			if state == StateHalfOpen {
				successes := node.HalfOpenSuccesses.Add(1)
				if int(successes) >= lb.cfg.HalfOpenSuccessGoal {
					node.State.Store(int32(StateClosed))
				}
			}
			return
		}
	}
}

// ReportFailure notifies the load balancer that a request to target failed (e.g. 502/503/504 or network timeout).
// Increments failure count and trips the circuit to StateOpen if threshold is exceeded.
func (lb *LoadBalancer) ReportFailure(target *url.URL) {
	if target == nil {
		return
	}
	lb.mu.RLock()
	defer lb.mu.RUnlock()

	targetStr := target.String()
	now := time.Now().UnixNano()
	for _, node := range lb.nodes {
		if node.URL.String() == targetStr {
			node.LastFailureTime.Store(now)
			failures := node.ConsecutiveFailures.Add(1)
			state := State(node.State.Load())

			if state == StateHalfOpen {
				// Any failure in Half-Open immediately trips back to Open
				node.State.Store(int32(StateOpen))
			} else if state == StateClosed && int(failures) >= lb.cfg.FailureThreshold {
				// Reached failure threshold in Closed, trip circuit to Open
				node.State.Store(int32(StateOpen))
			}
			return
		}
	}
}

// AddServer dynamically registers a new upstream URL into the load balancer.
func (lb *LoadBalancer) AddServer(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return err
	}

	lb.mu.Lock()
	defer lb.mu.Unlock()

	// Prevent duplicates
	for _, n := range lb.nodes {
		if n.URL.String() == parsed.String() {
			return nil
		}
	}

	lb.nodes = append(lb.nodes, &UpstreamNode{
		URL: parsed,
	})
	return nil
}

// RemoveServer dynamically deregisters an upstream URL from the load balancer.
func (lb *LoadBalancer) RemoveServer(rawURL string) {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	newNodes := make([]*UpstreamNode, 0, len(lb.nodes))
	for _, n := range lb.nodes {
		if n.URL.String() != rawURL {
			newNodes = append(newNodes, n)
		}
	}
	lb.nodes = newNodes
}

// ServerCount returns the current count of registered upstream servers.
func (lb *LoadBalancer) ServerCount() int {
	lb.mu.RLock()
	defer lb.mu.RUnlock()
	return len(lb.nodes)
}

// HealthyCount returns the count of upstream servers that are not currently tripped (StateClosed or StateHalfOpen).
func (lb *LoadBalancer) HealthyCount() int {
	lb.mu.RLock()
	defer lb.mu.RUnlock()
	count := 0
	for _, n := range lb.nodes {
		if State(n.State.Load()) != StateOpen {
			count++
		}
	}
	return count
}

// GetNodeState returns the current circuit breaker state of a specific target URL.
func (lb *LoadBalancer) GetNodeState(target *url.URL) State {
	if target == nil {
		return StateClosed
	}
	lb.mu.RLock()
	defer lb.mu.RUnlock()

	targetStr := target.String()
	for _, node := range lb.nodes {
		if node.URL.String() == targetStr {
			return State(node.State.Load())
		}
	}
	return StateClosed
}

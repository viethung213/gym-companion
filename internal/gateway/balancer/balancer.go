package balancer

import (
	"errors"
	"net/url"
	"sync"
	"sync/atomic"
)

// ErrNoUpstreamsAvailable occurs when a route has empty upstreams.
var ErrNoUpstreamsAvailable = errors.New("no upstream servers available")

// LoadBalancer distributes traffic evenly across upstream servers using dynamic round-robin.
type LoadBalancer struct {
	mu      sync.RWMutex
	servers []*url.URL
	counter atomic.Uint64
}

// New parses upstream URLs and initializes the round-robin counter.
func New(upstreams []string) (*LoadBalancer, error) {
	if len(upstreams) == 0 {
		return nil, ErrNoUpstreamsAvailable
	}

	servers := make([]*url.URL, 0, len(upstreams))
	for _, raw := range upstreams {
		parsed, err := url.Parse(raw)
		if err != nil {
			return nil, err
		}
		servers = append(servers, parsed)
	}

	return &LoadBalancer{
		servers: servers,
	}, nil
}

// Next selects the next upstream server using atomic counter increment.
func (lb *LoadBalancer) Next() *url.URL {
	lb.mu.RLock()
	defer lb.mu.RUnlock()

	if len(lb.servers) == 0 {
		return nil
	}

	idx := lb.counter.Add(1) - 1
	return lb.servers[idx%uint64(len(lb.servers))]
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
	for _, s := range lb.servers {
		if s.String() == parsed.String() {
			return nil
		}
	}

	lb.servers = append(lb.servers, parsed)
	return nil
}

// RemoveServer dynamically deregisters an upstream URL from the load balancer.
func (lb *LoadBalancer) RemoveServer(rawURL string) {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	newServers := make([]*url.URL, 0, len(lb.servers))
	for _, s := range lb.servers {
		if s.String() != rawURL {
			newServers = append(newServers, s)
		}
	}
	lb.servers = newServers
}

// ServerCount returns the current count of active upstream servers.
func (lb *LoadBalancer) ServerCount() int {
	lb.mu.RLock()
	defer lb.mu.RUnlock()
	return len(lb.servers)
}

package discovery

import (
	"context"
	"sync"
	"time"
)

// Instance represents a dynamically registered microservice instance.
type Instance struct {
	ServiceID     string    `json:"service_id"`
	UpstreamURL   string    `json:"upstream_url"`
	Prefixes      []string  `json:"prefixes"`
	LastHeartbeat time.Time `json:"last_heartbeat"`
}

// EvictCallback defines a function to be invoked when an instance expires.
type EvictCallback func(serviceID, upstreamURL string)

// Registry manages dynamically discovered microservice instances and heartbeats.
type Registry struct {
	mu        sync.RWMutex
	instances map[string]*Instance
	ttl       time.Duration
	onEvict   EvictCallback
	cancel    context.CancelFunc
	wg        sync.WaitGroup
}

// NewRegistry initializes an in-memory discovery registry.
func NewRegistry(ttl time.Duration, onEvict EvictCallback) *Registry {
	if ttl <= 0 {
		ttl = 15 * time.Second
	}
	return &Registry{
		instances: make(map[string]*Instance),
		ttl:       ttl,
		onEvict:   onEvict,
	}
}

// Register registers or refreshes a service instance heartbeat and prefixes.
// Returns true if the instance was newly registered, false if refreshed.
func (r *Registry) Register(serviceID, upstreamURL string, prefixes []string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := serviceID + "::" + upstreamURL
	now := time.Now()
	if inst, exists := r.instances[key]; exists {
		inst.LastHeartbeat = now
		if len(prefixes) > 0 {
			inst.Prefixes = prefixes
		}
		return false
	}

	r.instances[key] = &Instance{
		ServiceID:     serviceID,
		UpstreamURL:   upstreamURL,
		Prefixes:      prefixes,
		LastHeartbeat: now,
	}
	return true
}

// Deregister removes a service instance.
func (r *Registry) Deregister(serviceID, upstreamURL string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := serviceID + "::" + upstreamURL
	if _, exists := r.instances[key]; exists {
		delete(r.instances, key)
		return true
	}
	return false
}

// GetInstances returns a copy of all currently registered instances.
func (r *Registry) GetInstances() []Instance {
	r.mu.RLock()
	defer r.mu.RUnlock()

	list := make([]Instance, 0, len(r.instances))
	for _, inst := range r.instances {
		cp := *inst
		cpPrefixes := make([]string, len(inst.Prefixes))
		copy(cpPrefixes, inst.Prefixes)
		cp.Prefixes = cpPrefixes
		list = append(list, cp)
	}
	return list
}

// StartWatcher starts background periodic eviction of expired instances.
func (r *Registry) StartWatcher(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = 5 * time.Second
	}

	watchCtx, cancel := context.WithCancel(ctx)
	r.cancel = cancel
	r.wg.Add(1)

	go func() {
		defer r.wg.Done()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-watchCtx.Done():
				return
			case now := <-ticker.C:
				r.evictStale(now)
			}
		}
	}()
}

func (r *Registry) evictStale(now time.Time) {
	r.mu.Lock()
	var evicted []*Instance
	for key, inst := range r.instances {
		if now.Sub(inst.LastHeartbeat) > r.ttl {
			evicted = append(evicted, inst)
			delete(r.instances, key)
		}
	}
	r.mu.Unlock()

	if r.onEvict != nil {
		for _, inst := range evicted {
			r.onEvict(inst.ServiceID, inst.UpstreamURL)
		}
	}
}

// Stop shuts down the watcher and waits for background goroutines to finish.
func (r *Registry) Stop() {
	if r.cancel != nil {
		r.cancel()
	}
	r.wg.Wait()
}

package discovery

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRegistry_RegisterAndGetInstances(t *testing.T) {
	t.Parallel()

	reg := NewRegistry(15*time.Second, nil)
	isNew := reg.Register("service-a", "http://localhost:8080", []string{"/prefix.a."})
	assert.True(t, isNew)

	// Refresh heartbeat
	isNew = reg.Register("service-a", "http://localhost:8080", []string{"/prefix.a."})
	assert.False(t, isNew)

	// Register another instance
	isNew = reg.Register("service-a", "http://localhost:8081", []string{"/prefix.a."})
	assert.True(t, isNew)

	instances := reg.GetInstances()
	assert.Len(t, instances, 2)
	assert.Equal(t, []string{"/prefix.a."}, instances[0].Prefixes)
}

func TestRegistry_Deregister(t *testing.T) {
	t.Parallel()

	reg := NewRegistry(15*time.Second, nil)
	reg.Register("service-a", "http://localhost:8080", nil)

	removed := reg.Deregister("service-a", "http://localhost:8080")
	assert.True(t, removed)

	removedAgain := reg.Deregister("service-a", "http://localhost:8080")
	assert.False(t, removedAgain)

	assert.Empty(t, reg.GetInstances())
}

func TestRegistry_EvictionWatcher(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	var evicted []string

	onEvict := func(svc, url string) {
		mu.Lock()
		defer mu.Unlock()
		evicted = append(evicted, svc+"::"+url)
	}

	reg := NewRegistry(50*time.Millisecond, onEvict)
	reg.Register("service-a", "http://localhost:8080", nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	reg.StartWatcher(ctx, 20*time.Millisecond)
	defer reg.Stop()

	time.Sleep(120 * time.Millisecond)

	mu.Lock()
	assert.Contains(t, evicted, "service-a::http://localhost:8080")
	mu.Unlock()

	assert.Empty(t, reg.GetInstances())
}

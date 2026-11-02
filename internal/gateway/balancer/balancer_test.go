package balancer

import (
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadBalancer_DynamicAddAndRemove(t *testing.T) {
	t.Parallel()

	upstreams := []string{
		"http://127.0.0.1:8080",
		"http://127.0.0.1:8081",
	}

	lb, err := New(upstreams)
	require.NoError(t, err)
	assert.Equal(t, 2, lb.ServerCount())

	// Round robin should cycle across servers
	s1 := lb.Next()
	s2 := lb.Next()
	s3 := lb.Next()

	assert.NotEqual(t, s1.String(), s2.String())
	assert.Equal(t, s1.String(), s3.String())

	// Dynamically add a third server
	err = lb.AddServer("http://127.0.0.1:8082")
	require.NoError(t, err)
	assert.Equal(t, 3, lb.ServerCount())

	// Adding duplicate should not change count
	err = lb.AddServer("http://127.0.0.1:8082")
	require.NoError(t, err)
	assert.Equal(t, 3, lb.ServerCount())

	// Dynamically remove a server
	lb.RemoveServer("http://127.0.0.1:8081")
	assert.Equal(t, 2, lb.ServerCount())

	// Ensure removed server is no longer selected
	for i := 0; i < 10; i++ {
		srv := lb.Next()
		assert.NotEqual(t, "http://127.0.0.1:8081", srv.String())
	}
}

func TestLoadBalancer_EmptyUpstreams(t *testing.T) {
	t.Parallel()

	lb, err := New([]string{})
	assert.ErrorIs(t, err, ErrNoUpstreamsAvailable)
	assert.Nil(t, lb)
}

func TestLoadBalancer_CircuitBreaker_BypassesFailedNode(t *testing.T) {
	t.Parallel()

	cfg := Config{
		FailureThreshold:    3,
		CooldownTimeout:     100 * time.Millisecond,
		HalfOpenSuccessGoal: 1,
	}

	upstreams := []string{
		"http://127.0.0.1:8080",
		"http://127.0.0.1:8081",
	}

	lb, err := NewWithConfig(upstreams, cfg)
	require.NoError(t, err)

	u1, _ := url.Parse("http://127.0.0.1:8080")
	u2, _ := url.Parse("http://127.0.0.1:8081")

	assert.Equal(t, StateClosed, lb.GetNodeState(u1))
	assert.Equal(t, StateClosed, lb.GetNodeState(u2))
	assert.Equal(t, 2, lb.HealthyCount())

	// 1. Report failures for u1 until threshold is reached (3 times)
	lb.ReportFailure(u1)
	assert.Equal(t, StateClosed, lb.GetNodeState(u1))
	lb.ReportFailure(u1)
	assert.Equal(t, StateClosed, lb.GetNodeState(u1))
	lb.ReportFailure(u1) // Reaches threshold 3!

	assert.Equal(t, StateOpen, lb.GetNodeState(u1), "u1 should be tripped to StateOpen")
	assert.Equal(t, StateClosed, lb.GetNodeState(u2))
	assert.Equal(t, 1, lb.HealthyCount())

	// 2. Next() calls should now bypass u1 and ONLY return u2
	for i := 0; i < 10; i++ {
		selected := lb.Next()
		require.NotNil(t, selected)
		assert.Equal(t, u2.String(), selected.String(), "u1 is open, so all requests must route to u2")
	}

	// 3. If u2 also fails 3 times, entire pool is open
	lb.ReportFailure(u2)
	lb.ReportFailure(u2)
	lb.ReportFailure(u2)
	assert.Equal(t, StateOpen, lb.GetNodeState(u2))
	assert.Equal(t, 0, lb.HealthyCount())

	// Next() must now fast-fail and return nil
	allFailed := lb.Next()
	assert.Nil(t, allFailed, "When all servers are tripped to StateOpen, Next() must return nil")
}

func TestLoadBalancer_CircuitBreaker_HalfOpenRecoveryAndReTrip(t *testing.T) {
	t.Parallel()

	cfg := Config{
		FailureThreshold:    2,
		CooldownTimeout:     50 * time.Millisecond,
		HalfOpenSuccessGoal: 1,
	}

	lb, err := NewWithConfig([]string{"http://127.0.0.1:8080"}, cfg)
	require.NoError(t, err)
	u, _ := url.Parse("http://127.0.0.1:8080")

	// Trip to StateOpen
	lb.ReportFailure(u)
	lb.ReportFailure(u)
	assert.Equal(t, StateOpen, lb.GetNodeState(u))

	// While cooling down, Next() returns nil
	assert.Nil(t, lb.Next())

	// Wait for cooldown to expire
	time.Sleep(60 * time.Millisecond)

	// Next() should now transition to StateHalfOpen and return the probe target
	probe := lb.Next()
	require.NotNil(t, probe)
	assert.Equal(t, u.String(), probe.String())
	assert.Equal(t, StateHalfOpen, lb.GetNodeState(u))

	// Case A: Probe succeeds -> transitions back to StateClosed
	lb.ReportSuccess(u)
	assert.Equal(t, StateClosed, lb.GetNodeState(u))

	// Trip it again to test failed probe
	lb.ReportFailure(u)
	lb.ReportFailure(u)
	assert.Equal(t, StateOpen, lb.GetNodeState(u))

	time.Sleep(60 * time.Millisecond)
	probe2 := lb.Next()
	require.NotNil(t, probe2)
	assert.Equal(t, StateHalfOpen, lb.GetNodeState(u))

	// Case B: Probe fails in Half-Open -> immediately re-trips back to StateOpen
	lb.ReportFailure(u)
	assert.Equal(t, StateOpen, lb.GetNodeState(u))
}

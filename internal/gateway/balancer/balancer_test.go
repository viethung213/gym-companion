package balancer

import (
	"testing"

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

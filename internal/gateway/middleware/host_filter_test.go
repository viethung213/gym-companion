package middleware

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHostFilter(t *testing.T) {
	t.Parallel()

	hf := NewHostFilter()
	hf.SetAllowedHosts([]string{"api.gymcompanion.com", "staging.gymcompanion.com:8443"})

	assert.True(t, hf.IsAllowed("api.gymcompanion.com"))
	assert.True(t, hf.IsAllowed("api.gymcompanion.com:8000"))
	assert.True(t, hf.IsAllowed("staging.gymcompanion.com:8443"))
	assert.False(t, hf.IsAllowed("123.45.67.89"))
	assert.False(t, hf.IsAllowed("evil.com"))
	assert.Equal(t, 2, hf.Count())

	// Dynamic AddHost
	hf.AddHost("newdomain.com")
	assert.True(t, hf.IsAllowed("newdomain.com"))
	assert.Equal(t, 3, hf.Count())

	// Dynamic RemoveHost
	assert.True(t, hf.RemoveHost("newdomain.com"))
	assert.False(t, hf.IsAllowed("newdomain.com"))
	assert.False(t, hf.RemoveHost("nonexistent.com"))

	// GetAllowedHosts
	hosts := hf.GetAllowedHosts()
	assert.Len(t, hosts, 2)

	// Clear
	hf.Clear()
	assert.Equal(t, 0, hf.Count())
	// Empty whitelist allows all (dev bootstrap mode)
	assert.True(t, hf.IsAllowed("anydomain.com"))
}

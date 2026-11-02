package discovery

import (
	"net"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetLocalIP(t *testing.T) {
	ip := GetLocalIP()
	assert.NotEmpty(t, ip)

	parsed := net.ParseIP(ip)
	assert.NotNil(t, parsed, "GetLocalIP must return a valid IP address")
	t.Logf("Detected local machine IP: %s", ip)
}

func TestResolveSelfAddress(t *testing.T) {
	addr := ResolveSelfAddress("8080")
	assert.True(t, strings.HasPrefix(addr, "http://"))
	assert.True(t, strings.HasSuffix(addr, ":8080"))

	ip := strings.TrimPrefix(addr, "http://")
	ip = strings.TrimSuffix(ip, ":8080")
	assert.NotNil(t, net.ParseIP(ip))
	t.Logf("Resolved self address: %s", addr)
}

func TestResolveSelfAddress_PortFromEnv(t *testing.T) {
	t.Setenv("APP_PORT", "9999")
	addr := ResolveSelfAddress("")
	assert.True(t, strings.HasSuffix(addr, ":9999"))
}

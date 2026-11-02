package middleware

import (
	"log"
	"net"
	"net/http"
	"strings"
	"sync"

	"github.com/viethung213/gym-companion/internal/gateway/gwerror"
)

// HostFilter validates incoming HTTP Host and HTTP/2 :authority against an allowed whitelist.
type HostFilter struct {
	mu           sync.RWMutex
	allowedHosts map[string]bool
}

// NewHostFilter initializes an empty host filter.
func NewHostFilter() *HostFilter {
	return &HostFilter{
		allowedHosts: make(map[string]bool),
	}
}

// SetAllowedHosts dynamically updates the whitelist of allowed domain names.
func (hf *HostFilter) SetAllowedHosts(hosts []string) {
	hf.mu.Lock()
	defer hf.mu.Unlock()

	newMap := make(map[string]bool, len(hosts))
	for _, h := range hosts {
		clean := strings.ToLower(strings.TrimSpace(h))
		if clean != "" {
			newMap[clean] = true
		}
	}
	hf.allowedHosts = newMap
}

// AddHost adds a single domain name or host:port into the allowed whitelist in a thread-safe manner.
func (hf *HostFilter) AddHost(host string) {
	clean := strings.ToLower(strings.TrimSpace(host))
	if clean == "" {
		return
	}

	hf.mu.Lock()
	defer hf.mu.Unlock()
	hf.allowedHosts[clean] = true
}

// RemoveHost removes a single domain name or host:port from the whitelist.
// Returns true if the host was found and removed, false otherwise.
func (hf *HostFilter) RemoveHost(host string) bool {
	clean := strings.ToLower(strings.TrimSpace(host))
	if clean == "" {
		return false
	}

	hf.mu.Lock()
	defer hf.mu.Unlock()
	if _, exists := hf.allowedHosts[clean]; exists {
		delete(hf.allowedHosts, clean)
		return true
	}
	return false
}

// GetAllowedHosts returns a snapshot slice of all permitted hosts in the whitelist.
func (hf *HostFilter) GetAllowedHosts() []string {
	hf.mu.RLock()
	defer hf.mu.RUnlock()

	hosts := make([]string, 0, len(hf.allowedHosts))
	for h := range hf.allowedHosts {
		hosts = append(hosts, h)
	}
	return hosts
}

// Count returns the number of active domain rules in the whitelist.
func (hf *HostFilter) Count() int {
	hf.mu.RLock()
	defer hf.mu.RUnlock()
	return len(hf.allowedHosts)
}

// Clear wipes all domain rules from the whitelist.
func (hf *HostFilter) Clear() {
	hf.mu.Lock()
	defer hf.mu.Unlock()
	hf.allowedHosts = make(map[string]bool)
}

// IsAllowed checks if an incoming host or authority is permitted.
func (hf *HostFilter) IsAllowed(incomingHost string) bool {
	hf.mu.RLock()
	defer hf.mu.RUnlock()

	// If no whitelist is configured (e.g. initial dev bootstrap), allow all
	if len(hf.allowedHosts) == 0 {
		return true
	}

	host := strings.ToLower(strings.TrimSpace(incomingHost))
	if host == "" {
		return false
	}

	// 1. Exact match (e.g. "api.gymcompanion.com:8000" or "api.gymcompanion.com")
	if hf.allowedHosts[host] {
		return true
	}

	// 2. Strip port if present
	h, _, err := net.SplitHostPort(host)
	if err == nil && hf.allowedHosts[h] {
		return true
	}

	return false
}

// Wrap returns an http.Handler middleware enforcing the host filter.
func (hf *HostFilter) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		incomingHost := r.Host
		if incomingHost == "" {
			incomingHost = r.URL.Host
		}
		if incomingHost == "" {
			incomingHost = r.Header.Get("X-Forwarded-Host")
		}

		if !hf.IsAllowed(incomingHost) {
			log.Printf("[Gateway Security] Rejected unauthorized Host/Authority: %q from %s", incomingHost, r.RemoteAddr)
			gwerror.Forbidden(w, r, "Direct IP access or unauthorized domain name rejected")
			return
		}

		next.ServeHTTP(w, r)
	})
}

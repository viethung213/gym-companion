package telemetry

import (
	"sort"
	"strings"
	"sync"
	"time"
)

// RequestJourney captures the full lifecycle trace of an incoming HTTP request.
type RequestJourney struct {
	RequestID        string    `json:"requestId"`
	Timestamp        time.Time `json:"timestamp"`
	ClientIP         string    `json:"clientIp"`
	Method           string    `json:"method"`
	Path             string    `json:"path"`
	SelectedUpstream string    `json:"selectedUpstream"`
	StatusCode       int       `json:"statusCode"`
	DurationMs       float64   `json:"durationMs"`
	Error            string    `json:"error,omitempty"`
}

// TraceFilter defines query criteria for filtering stored request traces.
type TraceFilter struct {
	RequestID  string
	Path       string
	Slowest    bool
	ErrorsOnly bool
	Limit      int
}

// TraceStore implements a thread-safe ring buffer for storing and querying request journeys.
type TraceStore struct {
	mu       sync.RWMutex
	buffer   []RequestJourney
	capacity int
	head     int
	size     int
}

// NewTraceStore creates a new ring buffer trace store with the specified capacity.
func NewTraceStore(capacity int) *TraceStore {
	if capacity <= 0 {
		capacity = 2000
	}
	return &TraceStore{
		buffer:   make([]RequestJourney, capacity),
		capacity: capacity,
	}
}

// Record appends a request journey trace into the ring buffer.
func (ts *TraceStore) Record(trace *RequestJourney) {
	if trace == nil {
		return
	}
	ts.mu.Lock()
	defer ts.mu.Unlock()

	ts.buffer[ts.head] = *trace
	ts.head = (ts.head + 1) % ts.capacity
	if ts.size < ts.capacity {
		ts.size++
	}
}

// Count returns the current number of traces held in the store.
func (ts *TraceStore) Count() int {
	ts.mu.RLock()
	defer ts.mu.RUnlock()
	return ts.size
}

// Clear removes all stored traces.
func (ts *TraceStore) Clear() {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.head = 0
	ts.size = 0
}

// Query retrieves stored traces matching the provided filter criteria.
func (ts *TraceStore) Query(filter TraceFilter) []RequestJourney {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	} else if limit > 200 {
		limit = 200
	}

	matched := make([]RequestJourney, 0, ts.size)
	for i := 0; i < ts.size; i++ {
		// Newest first: head - 1 - i
		idx := (ts.head - 1 - i + ts.capacity) % ts.capacity
		item := ts.buffer[idx]

		if filter.RequestID != "" && item.RequestID != filter.RequestID {
			continue
		}
		if filter.Path != "" && !strings.Contains(item.Path, filter.Path) {
			continue
		}
		if filter.ErrorsOnly && item.StatusCode < 400 && item.Error == "" {
			continue
		}
		matched = append(matched, item)
	}

	if filter.Slowest {
		sort.SliceStable(matched, func(i, j int) bool {
			return matched[i].DurationMs > matched[j].DurationMs
		})
	}

	if len(matched) > limit {
		matched = matched[:limit]
	}

	result := make([]RequestJourney, len(matched))
	copy(result, matched)
	return result
}

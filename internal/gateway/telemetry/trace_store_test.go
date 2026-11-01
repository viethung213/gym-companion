package telemetry

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestTraceStore_RingBufferAndQuery(t *testing.T) {
	capacity := 5
	store := NewTraceStore(capacity)

	if got, want := store.Count(), 0; got != want {
		t.Errorf("Initial count got %d, want %d", got, want)
	}

	// Insert 7 items into capacity 5 ring buffer
	for i := 1; i <= 7; i++ {
		store.Record(&RequestJourney{
			RequestID:        fmt.Sprintf("req-%d", i),
			Timestamp:        time.Now(),
			ClientIP:         "127.0.0.1",
			Method:           "POST",
			Path:             fmt.Sprintf("/api/v1/test/%d", i),
			SelectedUpstream: "http://upstream:8080",
			StatusCode:       200 + (i%3)*100, // 200, 300, 400, 200, 300, 400, 200
			DurationMs:       float64(i * 10),
		})
	}

	if got, want := store.Count(), 5; got != want {
		t.Errorf("Count after wrap got %d, want %d", got, want)
	}

	// Query default (limit 50) -> should have 5 items, newest first: req-7, req-6, req-5, req-4, req-3
	all := store.Query(TraceFilter{})
	if len(all) != 5 {
		t.Fatalf("Query len got %d, want 5", len(all))
	}
	if got, want := all[0].RequestID, "req-7"; got != want {
		t.Errorf("Newest item got %s, want %s", got, want)
	}
	if got, want := all[4].RequestID, "req-3"; got != want {
		t.Errorf("Oldest surviving item got %s, want %s", got, want)
	}

	// Query by RequestID
	req5 := store.Query(TraceFilter{RequestID: "req-5"})
	if len(req5) != 1 || req5[0].RequestID != "req-5" {
		t.Errorf("Query by RequestID failed")
	}

	// Query evicted item
	req1 := store.Query(TraceFilter{RequestID: "req-1"})
	if len(req1) != 0 {
		t.Errorf("Evicted item req-1 should return 0 results")
	}

	// Query by Path filter
	test6 := store.Query(TraceFilter{Path: "test/6"})
	if len(test6) != 1 || test6[0].RequestID != "req-6" {
		t.Errorf("Query by Path filter failed")
	}

	// Query Slowest
	slowest := store.Query(TraceFilter{Slowest: true, Limit: 2})
	if len(slowest) != 2 {
		t.Fatalf("Slowest len got %d, want 2", len(slowest))
	}
	if slowest[0].DurationMs < slowest[1].DurationMs {
		t.Errorf("Slowest sort incorrect: %v < %v", slowest[0].DurationMs, slowest[1].DurationMs)
	}

	// Query ErrorsOnly
	errs := store.Query(TraceFilter{ErrorsOnly: true})
	for _, errItem := range errs {
		if errItem.StatusCode < 400 && errItem.Error == "" {
			t.Errorf("ErrorsOnly returned non-error status: %d", errItem.StatusCode)
		}
	}

	// Clear
	store.Clear()
	if got, want := store.Count(), 0; got != want {
		t.Errorf("Count after Clear got %d, want %d", got, want)
	}
}

func TestTraceStore_Concurrency(t *testing.T) {
	store := NewTraceStore(50)
	var wg sync.WaitGroup

	// Concurrently record
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				store.Record(&RequestJourney{
					RequestID:  fmt.Sprintf("w%d-%d", id, j),
					Timestamp:  time.Now(),
					Path:       "/concurrency",
					StatusCode: 200,
					DurationMs: 5.0,
				})
			}
		}(i)
	}

	// Concurrently query
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				_ = store.Query(TraceFilter{Limit: 10})
				time.Sleep(time.Millisecond)
			}
		}()
	}

	wg.Wait()
	if store.Count() != 50 {
		t.Errorf("Expected store count 50, got %d", store.Count())
	}
}

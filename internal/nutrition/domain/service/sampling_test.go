package service

import (
	"testing"
	"time"
)

func TestSamplePaged_PaginationAndOffset(t *testing.T) {
	t.Parallel()

	catalog := makeTestCatalog()
	now := time.Date(2026, 11, 4, 10, 0, 0, 0, time.UTC)

	rng1 := NewDeterministicRNG("test-user", now)
	page1 := SamplePaged(catalog, 6, 1, rng1)
	if len(page1) != 6 {
		t.Fatalf("expected page1 to have 6 items, got %d", len(page1))
	}

	rng2 := NewDeterministicRNG("test-user", now)
	page2 := SamplePaged(catalog, 6, 2, rng2)
	if len(page2) != 6 {
		t.Fatalf("expected page2 to have 6 items, got %d", len(page2))
	}

	p1IDs := make(map[string]bool)
	for _, it := range page1 {
		p1IDs[it.ID()] = true
	}
	overlap := 0
	for _, it := range page2 {
		if p1IDs[it.ID()] {
			overlap++
		}
	}
	if overlap > 0 {
		t.Errorf("expected no overlap between consecutive page 1 and page 2, got %d overlapping items", overlap)
	}
}

func TestSamplePaged_EdgeCases(t *testing.T) {
	t.Parallel()

	catalog := makeTestCatalog()
	now := time.Now()
	rng := NewDeterministicRNG("edge-user", now)

	// Empty catalog returns nil
	if res := SamplePaged(nil, 5, 1, rng); res != nil {
		t.Errorf("expected nil for empty catalog, got %v", res)
	}

	// targetCount <= 0 defaults to 6
	resDefault := SamplePaged(catalog, 0, 1, rng)
	if len(resDefault) != 6 {
		t.Errorf("expected default 6 items, got %d", len(resDefault))
	}

	// page <= 0 defaults to 1
	resPage := SamplePaged(catalog, 4, 0, rng)
	if len(resPage) != 4 {
		t.Errorf("expected 4 items for page 0 default, got %d", len(resPage))
	}
}

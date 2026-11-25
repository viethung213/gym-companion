package query

import (
	"slices"
	"testing"

	"github.com/viethung213/gym-companion/internal/coaching/domain/roadmap"
)

// The default list is the athlete's history; a status narrows it.
func TestListedStatuses(t *testing.T) {
	if got := listedStatuses(""); !slices.Equal(got, []roadmap.Status{roadmap.StatusActive, roadmap.StatusCompleted}) {
		t.Errorf("default = %v, want ACTIVE and COMPLETED", got)
	}
	if got := listedStatuses(roadmap.StatusCompleted); !slices.Equal(got, []roadmap.Status{roadmap.StatusCompleted}) {
		t.Errorf("COMPLETED = %v, want only COMPLETED", got)
	}
}

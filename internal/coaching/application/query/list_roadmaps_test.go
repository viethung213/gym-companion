package query

import (
	"slices"
	"testing"

	"github.com/viethung213/gym-companion/internal/coaching/domain/roadmap"
)

// The default list is the athlete's history; the block planned ahead shows up
// only when asked for by status.
func TestListedStatuses(t *testing.T) {
	if got := listedStatuses(""); !slices.Equal(got, []roadmap.Status{roadmap.StatusActive, roadmap.StatusCompleted}) {
		t.Errorf("default = %v, want ACTIVE and COMPLETED", got)
	}
	if got := listedStatuses(roadmap.StatusPlanned); !slices.Equal(got, []roadmap.Status{roadmap.StatusPlanned}) {
		t.Errorf("PLANNED = %v, want only PLANNED", got)
	}
}

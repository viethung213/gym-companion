package query_test

import (
	"context"
	"testing"

	"github.com/viethung213/gym-companion/internal/exercise/application/port"
	"github.com/viethung213/gym-companion/internal/exercise/application/query"
	"github.com/viethung213/gym-companion/internal/exercise/domain"
	"github.com/viethung213/gym-companion/internal/shared/middleware"
)

// filterSpy records the filters the repository receives.
type filterSpy struct {
	mockRandomRepo
	got *port.SearchFilters
}

func (s *filterSpy) SearchActive(_ context.Context, f *port.SearchFilters) ([]*domain.Exercise, int32, error) {
	s.got = f
	return nil, 0, nil
}

func TestSearchExercises_StatusFiltersAreAdminOnly(t *testing.T) {
	asked := &port.SearchFilters{Status: domain.StatusDraft, IncludeAllStatuses: true}
	cases := []struct {
		name      string
		ctx       context.Context
		wantDraft bool
	}{
		{"guest", context.Background(), false},
		{"user", middleware.WithActor(context.Background(), "u-1", "User"), false},
		{"admin", middleware.WithActor(context.Background(), "a-1", "Admin"), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			spy := &filterSpy{}
			if _, err := query.NewSearchExercisesHandler(spy).Handle(tc.ctx, query.SearchExercisesQuery{Filters: asked}); err != nil {
				t.Fatal(err)
			}
			gotDraft := spy.got.Status == domain.StatusDraft && spy.got.IncludeAllStatuses
			if gotDraft != tc.wantDraft {
				t.Fatalf("status filters passed = %v, want %v (got %+v)", gotDraft, tc.wantDraft, *spy.got)
			}
		})
	}
	if asked.Status != domain.StatusDraft || !asked.IncludeAllStatuses {
		t.Fatal("the caller's filters must not be mutated")
	}
}

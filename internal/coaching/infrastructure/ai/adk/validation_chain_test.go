package adk

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/viethung213/gym-companion/internal/coaching/domain/roadmap"
)

// TestValidationChain_HallucinatedIDNeverReachesDomain walks the whole path a
// generated plan takes — retry, validation, domain mapping — and asserts an
// invented exercise ID cannot survive it.
func TestValidationChain_HallucinatedIDNeverReachesDomain(t *testing.T) {
	// "barbell-bench-press" is the classic plausible-expansion of the real
	// catalog id "bench-press".
	const real, fake = "bench-press", "barbell-bench-press"

	t.Run("rejected then corrected", func(t *testing.T) {
		c := mapperFor(t, real)
		att := &recordingAttempt{plans: []*GeneratedPlan{
			planOf(sessionOf("2026-08-03", fake)),
			planOf(sessionOf("2026-08-03", real)),
		}}

		res, err := runWithRetries(context.Background(), c.validator, att.fn, nil)
		if err != nil {
			t.Fatalf("runWithRetries returned error: %v", err)
		}
		if att.calls != 2 {
			t.Errorf("got %d attempts, want 2", att.calls)
		}
		if !strings.Contains(strings.Join(att.seenIssues[1], "\n"), fake) {
			t.Errorf("attempt 2 was not told which id failed: %v", att.seenIssues[1])
		}

		rm, err := c.mapToDomainRoadmap(context.Background(), res.Plan, res.Names, "user-1", getMapNow())
		if err != nil {
			t.Fatalf("mapToDomainRoadmap returned error: %v", err)
		}
		assertNoExerciseID(t, rm, fake)
	})

	t.Run("dropped when never corrected", func(t *testing.T) {
		c := mapperFor(t, real)
		att := alwaysReturns(planOf(sessionOf("2026-08-03", real, fake)))

		res, err := runWithRetries(context.Background(), c.validator, att.fn, nil)
		if err != nil {
			t.Fatalf("runWithRetries returned error: %v", err)
		}
		if !res.Degraded {
			t.Error("got Degraded = false, want true after salvage")
		}

		rm, err := c.mapToDomainRoadmap(context.Background(), res.Plan, res.Names, "user-1", getMapNow())
		if err != nil {
			t.Fatalf("mapToDomainRoadmap returned error: %v", err)
		}
		assertNoExerciseID(t, rm, fake)
	})

	t.Run("fails when nothing valid remains", func(t *testing.T) {
		c := mapperFor(t, real)
		att := alwaysReturns(planOf(sessionOf("2026-08-03", fake)))

		if _, err := runWithRetries(context.Background(), c.validator, att.fn, nil); !errors.Is(err, ErrPlanGenerationFailed) {
			t.Errorf("got error %v, want ErrPlanGenerationFailed", err)
		}
	})
}

// TestValidationChain_EveryExerciseCarriesACatalogName is what the application
// layer's guard relies on: a name proves the ID was looked up, not invented.
func TestValidationChain_EveryExerciseCarriesACatalogName(t *testing.T) {
	c := mapperFor(t, "bench-press", "squat")
	plan := planOf(sessionOf("2026-08-03", "bench-press", "squat"))

	res, err := runWithRetries(context.Background(), c.validator, alwaysReturns(plan).fn, nil)
	if err != nil {
		t.Fatalf("runWithRetries returned error: %v", err)
	}

	rm, err := c.mapToDomainRoadmap(context.Background(), res.Plan, res.Names, "user-1", getMapNow())
	if err != nil {
		t.Fatalf("mapToDomainRoadmap returned error: %v", err)
	}

	for _, w := range rm.Weeks() {
		for _, d := range w.Days() {
			for _, s := range d.Sessions() {
				for _, ex := range s.Info().Prescription.MainExercises {
					if ex.ExerciseName == "" {
						t.Errorf("exercise %s reached the domain with no catalog name", ex.ExerciseID)
					}
				}
			}
		}
	}
}

// assertNoExerciseID fails if unwanted appears in any slot of any session.
func assertNoExerciseID(t *testing.T, rm *roadmap.Roadmap, unwanted string) {
	t.Helper()

	seen := 0
	for _, w := range rm.Weeks() {
		for _, d := range w.Days() {
			for _, s := range d.Sessions() {
				p := s.Info().Prescription
				for _, group := range [][]roadmap.PrescribedExercise{
					p.WarmUps, p.MainExercises, p.CoolDowns,
				} {
					for _, ex := range group {
						seen++
						if ex.ExerciseID == unwanted {
							t.Errorf("hallucinated exercise %q reached the domain", unwanted)
						}
					}
				}
			}
		}
	}

	if seen == 0 {
		t.Fatal("no exercises inspected; the assertion would pass vacuously")
	}
}

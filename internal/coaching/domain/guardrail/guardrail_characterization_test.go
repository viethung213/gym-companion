package guardrail

import (
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/coaching/application/port"
	"github.com/viethung213/gym-companion/internal/coaching/domain/service"
	"github.com/viethung213/gym-companion/internal/coaching/domain/taxonomy"
)

// TestCharacterization_F2_GuardrailUsesUserInjuries pins finding F2 of
// docs/05_exercise_catalog_refactor_spec.md section 3: coaching/module.go used
// to build the shared engine with no injuries, so BR-AC-09 never blocked. The
// engine is still built once at module init (as in module.go), and the user's
// active injuries now reach it through Check.
func TestCharacterization_F2_GuardrailUsesUserInjuries(t *testing.T) {
	// Every session of the baseline roadmap prescribes ex-bench (CHEST primary).
	r := buildValidRoadmap(t)

	// The user's profile carries an active chest injury and a recovered one,
	// as UserProfileReader would report them.
	recovered := time.Date(2026, 8, 2, 0, 0, 0, 0, time.UTC)
	profile := port.Profile{
		UserID: "user-1",
		ActiveInjuries: []port.InjuryStatus{
			{Region: taxonomy.JointRegionChest, Severity: taxonomy.InjurySeverityModerate},
			{Region: taxonomy.JointRegionKnee, Severity: taxonomy.InjurySeveritySevere, RecoveredAt: &recovered},
		},
	}

	// Production construction (coaching/module.go): one engine, no user known yet.
	guard := NewEngine(service.NewOverloadValidator(), benchCatalog(), nil)

	athlete := profile.Athlete()
	got := guard.Check(r, athlete.Injuries, time.Time{})

	assertViolation(t, got, "BR-AC-09")
}

package policy

import "github.com/viethung213/gym-companion/internal/workout_execution/domain/aggregate"

// criticalErrorCodes is the definitive registry of error codes that are
// treated as critical regardless of the Severity field value.
// Add new safety-critical codes here without touching application handlers.
var criticalErrorCodes = map[string]struct{}{
	"ERR_BAR_TRAPPED":   {},
	"ERR_FALL_DETECTED": {},
}

// IsCritical reports whether a session error should be treated as a
// safety-critical emergency. An error is critical when either:
//   - its Severity is "CRITICAL", or
//   - its ErrorCode is explicitly registered as critical.
func IsCritical(e aggregate.SessionError) bool {
	if e.Severity == "CRITICAL" {
		return true
	}
	_, ok := criticalErrorCodes[e.ErrorCode]
	return ok
}

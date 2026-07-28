package policy

import "github.com/viethung213/gym-companion/internal/workout_execution/domain/aggregate"

// isCriticalErrorCode checks if code is registered as a critical posture error.
func isCriticalErrorCode(code string) bool {
	return code == "ERR_BAR_TRAPPED" || code == "ERR_FALL_DETECTED"
}

// IsCritical reports whether a session error should be treated as a
// safety-critical emergency. An error is critical when either:
//   - its Severity is "CRITICAL", or
//   - its ErrorCode is explicitly registered as critical.
func IsCritical(e *aggregate.SessionError) bool {
	if e == nil {
		return false
	}
	if e.Severity == "CRITICAL" {
		return true
	}
	return isCriticalErrorCode(e.ErrorCode)
}

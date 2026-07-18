package transport

import (
	"errors"
	"log"

	"github.com/viethung213/gym-companion/internal/coaching/application/command"
	"github.com/viethung213/gym-companion/internal/coaching/domain"
	"github.com/viethung213/gym-companion/internal/shared/middleware"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var errProfileReaderUnavailable = errors.New("profile snapshot integration is not available yet")

type requestValidationError struct {
	message string
}

func (e *requestValidationError) Error() string {
	return e.message
}

func invalidRequest(message string) error {
	return &requestValidationError{message: message}
}

func rpcError(err error) error {
	var validationError *requestValidationError
	switch {
	case errors.Is(err, middleware.ErrUnauthorized):
		return status.Error(codes.Unauthenticated, err.Error())
	case errors.Is(err, middleware.ErrForbidden):
		return status.Error(codes.PermissionDenied, err.Error())
	case errors.Is(err, domain.ErrNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, command.ErrActiveRoadmapExists):
		return status.Error(codes.AlreadyExists, err.Error())
	case errors.Is(err, command.ErrRoadmapCycleComplete),
		errors.Is(err, command.ErrRestDay),
		errors.Is(err, command.ErrNoMatchingExercise),
		errors.Is(err, command.ErrInjurySafetyBlock),
		errors.Is(err, command.ErrPreviousScheduleMismatch):
		return status.Error(codes.FailedPrecondition, err.Error())
	case errors.Is(err, errProfileReaderUnavailable):
		return status.Error(codes.FailedPrecondition, err.Error())
	case isDomainValidationError(err), errors.As(err, &validationError):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, command.ErrPreviousScheduleRequired):
		return status.Error(codes.InvalidArgument, err.Error())
	default:
		log.Printf("Coaching RPC internal error: %v", err)
		return status.Error(codes.Internal, "internal coaching error")
	}
}

func isDomainValidationError(err error) bool {
	return errors.Is(err, domain.ErrPlanningGoalRequired) ||
		errors.Is(err, domain.ErrExperienceLevelRequired) ||
		errors.Is(err, domain.ErrTrainingDaysOutOfRange) ||
		errors.Is(err, domain.ErrSessionDurationOutOfRange) ||
		errors.Is(err, domain.ErrEquipmentRequired) ||
		errors.Is(err, domain.ErrTimezoneRequired) ||
		errors.Is(err, domain.ErrRoadmapStartDateRequired) ||
		errors.Is(err, domain.ErrPreferredWeekdaysInsufficient)
}

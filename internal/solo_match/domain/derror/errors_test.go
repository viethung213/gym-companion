package derror_test

import (
	"testing"

	"github.com/viethung213/gym-companion/internal/solo_match/domain/derror"
)

func TestDomainErrors(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name string
		err  error
	}{
		{name: "ErrMatchNotActive", err: derror.ErrMatchNotActive},
		{name: "ErrParticipantNotFound", err: derror.ErrParticipantNotFound},
		{name: "ErrMatchNotTimedOutYet", err: derror.ErrMatchNotTimedOutYet},
		{name: "ErrInvalidParticipantCount", err: derror.ErrInvalidParticipantCount},
		{name: "ErrSoloMatchNotFound", err: derror.ErrSoloMatchNotFound},
		{name: "ErrActiveMatchNotFound", err: derror.ErrActiveMatchNotFound},
		{name: "ErrEmptyMatchID", err: derror.ErrEmptyMatchID},
		{name: "ErrEmptyTicketID", err: derror.ErrEmptyTicketID},
		{name: "ErrUserAlreadyInQueue", err: derror.ErrUserAlreadyInQueue},
		{name: "ErrEmptyParticipantID", err: derror.ErrEmptyParticipantID},
		{name: "ErrParticipantFinished", err: derror.ErrParticipantFinished},
		{name: "ErrEmptyUserID", err: derror.ErrEmptyUserID},
		{name: "ErrNegativeScore", err: derror.ErrNegativeScore},
		{name: "ErrInvalidMatchStatus", err: derror.ErrInvalidMatchStatus},
		{name: "ErrInvalidMatchResult", err: derror.ErrInvalidMatchResult},
		{name: "ErrInvalidChallengeType", err: derror.ErrInvalidChallengeType},
		{name: "ErrParticipantsNotCompleted", err: derror.ErrParticipantsNotCompleted},
		{name: "ErrNilParticipant", err: derror.ErrNilParticipant},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if tc.err == nil {
				t.Errorf("%s should not be nil", tc.name)
			}
			if tc.err.Error() == "" {
				t.Errorf("%s message should not be empty", tc.name)
			}
		})
	}
}

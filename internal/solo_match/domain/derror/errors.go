package derror

import "errors"

// Domain Business Errors for Solo Match Bounded Context.
var (
	// Solo Match Aggregate Errors
	ErrMatchNotActive           = errors.New("match is not active")
	ErrParticipantNotFound      = errors.New("participant not found in match")
	ErrMatchNotTimedOutYet      = errors.New("match has not timed out yet")
	ErrInvalidParticipantCount  = errors.New("solo match must have exactly two participants")
	ErrSoloMatchNotFound        = errors.New("solo match not found")
	ErrActiveMatchNotFound      = errors.New("active match not found")
	ErrActiveMatchAlreadyExists = errors.New("user already has an active match in progress")
	ErrEmptyMatchID             = errors.New("match id cannot be empty")

	// Matchmaking Ticket Errors
	ErrEmptyTicketID = errors.New("ticket id cannot be empty")

	// Participant & User Snapshot Errors
	ErrEmptyParticipantID  = errors.New("participant id cannot be empty")
	ErrParticipantFinished = errors.New("participant already finished")
	ErrEmptyUserID         = errors.New("user id cannot be empty")
	ErrBotCannotForfeit    = errors.New("bot cannot forfeit match")
	ErrNoBotAvailable      = errors.New("no active bot available")

	// Value Object Errors
	ErrNegativeScore        = errors.New("score cannot be negative")
	ErrInvalidMatchStatus   = errors.New("invalid match status")
	ErrInvalidMatchResult   = errors.New("invalid match result")
	ErrInvalidChallengeType = errors.New("invalid challenge type")

	// Domain Service (Resolver) Errors
	ErrParticipantsNotCompleted = errors.New("cannot resolve match when participants are not completed")
	ErrNilParticipant           = errors.New("participant cannot be nil")
)

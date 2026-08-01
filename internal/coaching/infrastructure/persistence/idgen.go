package persistence

import (
	"github.com/google/uuid"
	"github.com/viethung213/gym-companion/internal/coaching/application/port"
)

// UUIDGenerator produces random UUIDv4 identifiers for Coaching aggregates.
type UUIDGenerator struct{}

// NewID returns a new UUIDv4 string.
func (UUIDGenerator) NewID() string { return uuid.NewString() }

// compile-time verification
var _ port.IDGenerator = UUIDGenerator{}

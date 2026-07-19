package persistence

import (
	"time"

	"github.com/google/uuid"
	"github.com/viethung213/gym-companion/internal/coaching/application/port"
)

type SystemClock struct{}

var _ port.Clock = SystemClock{}

func (SystemClock) Now() time.Time {
	return time.Now()
}

type RandomIDGenerator struct{}

var _ port.IDGenerator = RandomIDGenerator{}

func (RandomIDGenerator) NewID() (string, error) {
	return uuid.New().String(), nil
}

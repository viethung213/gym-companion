//go:build unit

package aggregate_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/viethung213/gym-companion/internal/profile/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/profile/domain/derror"
	"github.com/viethung213/gym-companion/internal/profile/domain/vo"
)

// slots returns n distinct weekly slots, two a day from Monday.
func slots(n int) []string {
	days := []string{"mon", "tue", "wed", "thu", "fri", "sat", "sun"}
	out := make([]string, 0, n)
	for i := range n {
		out = append(out, fmt.Sprintf("%s:%02d:00-%02d:00", days[i%7], 6+12*(i/7), 7+12*(i/7)))
	}
	return out
}

func newSlotProfile(t *testing.T, times []string) (*aggregate.UserProfile, error) {
	t.Helper()
	bio, err := vo.NewBiologicalMetrics(70, 172, 25, "MALE")
	require.NoError(t, err)
	return aggregate.NewUserProfile("user-1", bio, "BEGINNER", nil, times, nil, nil, "", 0, 0, nil)
}

func TestWorkoutSlots_AtMostTenPerWeek(t *testing.T) {
	_, err := newSlotProfile(t, slots(8)) // 6 training + 2 recovery
	require.NoError(t, err)
	_, err = newSlotProfile(t, slots(aggregate.MaxWorkoutSlotsPerWeek))
	require.NoError(t, err)

	_, err = newSlotProfile(t, slots(aggregate.MaxWorkoutSlotsPerWeek+1))
	require.ErrorIs(t, err, derror.ErrTooManyWorkoutSlots)

	// Duplicates count once.
	dup := append(slots(aggregate.MaxWorkoutSlotsPerWeek), " MON:06:00-07:00")
	_, err = newSlotProfile(t, dup)
	assert.NoError(t, err)
}

func TestWorkoutSlots_UpdateRejectsOnlyANewOversizedSelection(t *testing.T) {
	p, err := newSlotProfile(t, slots(3))
	require.NoError(t, err)
	bio := p.BiologicalMetrics()

	err = p.UpdateProfile(bio, "", nil, slots(aggregate.MaxWorkoutSlotsPerWeek+1), nil, nil, "", 0, 0)
	require.ErrorIs(t, err, derror.ErrTooManyWorkoutSlots)
	assert.Equal(t, slots(3), p.PreferredWorkoutTimes(), "a rejected update must not change the slots")

	// Passing the stored slots through unchanged is always accepted.
	require.NoError(t, p.UpdateProfile(bio, "", nil, p.PreferredWorkoutTimes(), nil, nil, "", 0, 0))
}

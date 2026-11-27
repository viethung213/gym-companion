package adapters

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// A slot given by its start only ("tue:18:00") is kept: planning schedules by
// the start, and dropping it put the athlete on default days. A broken start
// or end is still unreadable.
func TestParseWorkoutTimesList_StartOnlySlot(t *testing.T) {
	slots, unreadable := parseWorkoutTimesList([]string{"tue:18:00", "thu:18:00-19:30", "fri:25:00", "sat:18:00-nope"})
	assert.Len(t, slots, 2)
	assert.Equal(t, "18:00", slots[0].StartTime)
	assert.Empty(t, slots[0].EndTime)
	assert.Equal(t, []string{"fri:25:00", "sat:18:00-nope"}, unreadable)
}

package adapters

import (
	"slices"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/coaching/application/port"
)

func TestParseWorkoutTimesList(t *testing.T) {
	slots, unreadable := parseWorkoutTimesList([]string{
		"mon:17:30-19:00",          // what the app sends
		"WED 06:00-07:30",          // also accepted by the profile service
		`{"fri": ["18:00-19:00"]}`, // JSON map
		`[{"day_of_week":6,"start_time":"08:00","end_time":"09:30"}]`, // JSON list
		"someday:18:00-19:00", // unknown day
		"tue:soon",            // not a clock range
		"",                    // ignored, not reported
	})

	want := []port.Slot{
		{DayOfWeek: time.Monday, StartTime: "17:30", EndTime: "19:00"},
		{DayOfWeek: time.Wednesday, StartTime: "06:00", EndTime: "07:30"},
		{DayOfWeek: time.Friday, StartTime: "18:00", EndTime: "19:00"},
		{DayOfWeek: time.Saturday, StartTime: "08:00", EndTime: "09:30"},
	}
	if !slices.Equal(slots, want) {
		t.Errorf("slots = %+v\nwant %+v", slots, want)
	}
	if wantBad := []string{"someday:18:00-19:00", "tue:soon"}; !slices.Equal(unreadable, wantBad) {
		t.Errorf("unreadable = %q, want %q", unreadable, wantBad)
	}
}

package event

import "testing"

func TestEventNames(t *testing.T) {
	tests := []struct {
		name string
		evt  Event
		want string
	}{
		{"RoadmapInitiated", &RoadmapInitiated{}, "contracts.core.coaching.v1.event.RoadmapInitiated"},
		{"RoadmapAdjusted", &RoadmapAdjusted{}, "contracts.core.coaching.v1.event.RoadmapAdjusted"},
		{"SessionPlanExecuted", &SessionPlanExecuted{}, "contracts.core.coaching.v1.event.SessionPlanExecuted"},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.evt.EventName(); got != tt.want {
				t.Errorf("EventName() = %q, want %q", got, tt.want)
			}
		})
	}
}

package domain_test

import (
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/coaching/domain"
)

func TestWorkoutRoadmap_Initiate(t *testing.T) {
	t.Parallel()

	now := time.Now()
	startDate := time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name    string
		id      string
		userID  string
		wantErr bool
	}{
		{
			name:    "valid initiation creates 4-week active roadmap",
			id:      "roadmap-123",
			userID:  "user-456",
			wantErr: false,
		},
		{
			name:    "missing id returns error",
			id:      "",
			userID:  "user-456",
			wantErr: true,
		},
		{
			name:    "missing user_id returns error",
			id:      "roadmap-123",
			userID:  "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			roadmap, err := domain.Initiate(tt.id, tt.userID, startDate, now)

			if (err != nil) != tt.wantErr {
				t.Fatalf("got err = %v, wantErr = %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}

			if got, want := roadmap.ID(), tt.id; got != want {
				t.Errorf("ID got = %s, want = %s", got, want)
			}
			if got, want := roadmap.UserID(), tt.userID; got != want {
				t.Errorf("UserID got = %s, want = %s", got, want)
			}
			if got, want := roadmap.Status(), domain.RoadmapStatusActive; got != want {
				t.Errorf("Status got = %s, want = %s", got, want)
			}

			// Verify 4-week duration: 28 days total (startDate + 27 days)
			expectedEndDate := startDate.AddDate(0, 0, 27)
			if got, want := roadmap.EndDate(), expectedEndDate; !got.Equal(want) {
				t.Errorf("EndDate got = %s, want = %s", got, want)
			}
		})
	}
}

func TestWorkoutRoadmap_LifecycleTransitions(t *testing.T) {
	t.Parallel()

	now := time.Now()
	roadmap, err := domain.Initiate("r-1", "u-1", now, now)
	if err != nil {
		t.Fatalf("initiate roadmap failed: %v", err)
	}

	// Active -> Paused
	if err := roadmap.Pause(now); err != nil {
		t.Fatalf("pause roadmap failed: %v", err)
	}
	if got, want := roadmap.Status(), domain.RoadmapStatusPaused; got != want {
		t.Errorf("status after pause got = %s, want = %s", got, want)
	}

	// Paused -> Resumed (Active)
	if err := roadmap.Resume(now); err != nil {
		t.Fatalf("resume roadmap failed: %v", err)
	}
	if got, want := roadmap.Status(), domain.RoadmapStatusActive; got != want {
		t.Errorf("status after resume got = %s, want = %s", got, want)
	}

	// Active -> Completed
	if err := roadmap.Complete(now); err != nil {
		t.Fatalf("complete roadmap failed: %v", err)
	}
	if got, want := roadmap.Status(), domain.RoadmapStatusCompleted; got != want {
		t.Errorf("status after complete got = %s, want = %s", got, want)
	}

	// Completed -> Pause (should fail)
	if err := roadmap.Pause(now); err == nil {
		t.Errorf("expected error pausing completed roadmap, got nil")
	}
}

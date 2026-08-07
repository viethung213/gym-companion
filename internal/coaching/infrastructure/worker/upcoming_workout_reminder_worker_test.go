package worker

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/coaching/domain/event"
	"github.com/viethung213/gym-companion/internal/coaching/domain/roadmap"
)

type mockRoadmapRepo struct {
	sessions []*roadmap.SessionPlanInfo
	err      error
}

func (m *mockRoadmapRepo) Save(ctx context.Context, r *roadmap.Roadmap) error { return nil }
func (m *mockRoadmapRepo) FindByID(ctx context.Context, id string) (*roadmap.Roadmap, error) {
	return nil, nil
}
func (m *mockRoadmapRepo) FindActiveByUser(ctx context.Context, userID string) (*roadmap.Roadmap, error) {
	return nil, nil
}
func (m *mockRoadmapRepo) ListByUser(ctx context.Context, userID string, status roadmap.Status, limit, offset int) ([]*roadmap.Roadmap, error) {
	return nil, nil
}
func (m *mockRoadmapRepo) FindSessionByID(ctx context.Context, sessionPlanID string) (*roadmap.Roadmap, error) {
	return nil, nil
}
func (m *mockRoadmapRepo) FindPendingSessionsByDate(ctx context.Context, targetDate time.Time) ([]*roadmap.SessionPlanInfo, error) {
	return m.sessions, m.err
}

type mockOutboxWriter struct {
	mu     sync.Mutex
	events []event.Event
}

func (m *mockOutboxWriter) Enqueue(ctx context.Context, partitionKey string, events ...event.Event) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, events...)
	return nil
}

func TestUpcomingWorkoutReminderWorker_RunCheck(t *testing.T) {
	now := time.Date(2026, 8, 7, 10, 0, 0, 0, time.Local)
	// Slot time set 60 minutes in the future
	futureTime := now.Add(60 * time.Minute)
	slotTimeStr := futureTime.Format("15:04")

	sessions := []*roadmap.SessionPlanInfo{
		{
			SessionPlanID: "sess_001",
			UserID:        "usr_100",
			ScheduledDate: now,
			SlotTime:      slotTimeStr,
			Status:        roadmap.SessionPlanStatusPending,
			Prescription: roadmap.WorkoutPrescription{
				MainExercises: []roadmap.PrescribedExercise{
					{ExerciseName: "Bench Press"},
				},
			},
		},
	}

	repo := &mockRoadmapRepo{sessions: sessions}
	writer := &mockOutboxWriter{}

	worker := NewUpcomingWorkoutReminderWorker(repo, writer, 100*time.Millisecond)
	worker.runCheckAt(context.Background(), now)

	writer.mu.Lock()
	count := len(writer.events)
	writer.mu.Unlock()

	if got, want := count, 1; got != want {
		t.Fatalf("events count got %d, want %d", got, want)
	}

	reminder, ok := writer.events[0].(*event.UpcomingWorkoutReminder)
	if !ok {
		t.Fatalf("event type got %T, want *event.UpcomingWorkoutReminder", writer.events[0])
	}

	if got, want := reminder.UserID, "usr_100"; got != want {
		t.Errorf("UserID got %s, want %s", got, want)
	}
	if got, want := reminder.WorkoutName, "Bench Press"; got != want {
		t.Errorf("WorkoutName got %s, want %s", got, want)
	}
}

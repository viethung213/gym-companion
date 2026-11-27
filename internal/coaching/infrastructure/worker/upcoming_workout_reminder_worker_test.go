package worker

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
	_ "time/tzdata"

	"github.com/viethung213/gym-companion/internal/coaching/application/port"
	"github.com/viethung213/gym-companion/internal/coaching/domain/event"
	"github.com/viethung213/gym-companion/internal/coaching/domain/roadmap"
)

type mockRoadmapRepo struct {
	sessions []port.ScheduledSession
	err      error

	from, to time.Time
}

func (m *mockRoadmapRepo) Save(ctx context.Context, r *roadmap.Roadmap) error { return nil }
func (m *mockRoadmapRepo) FindByID(ctx context.Context, id string) (*roadmap.Roadmap, error) {
	return nil, nil
}
func (m *mockRoadmapRepo) FindActiveByUser(ctx context.Context, userID string) (*roadmap.Roadmap, error) {
	return nil, nil
}
func (m *mockRoadmapRepo) ListByUser(ctx context.Context, userID string, statuses []roadmap.Status, limit, offset int) ([]*roadmap.Roadmap, error) {
	return nil, nil
}
func (m *mockRoadmapRepo) FindSessionByID(ctx context.Context, sessionPlanID string) (*roadmap.Roadmap, error) {
	return nil, nil
}
func (m *mockRoadmapRepo) FindPendingSessionsStarting(_ context.Context, from, to time.Time) ([]port.ScheduledSession, error) {
	if m.from.IsZero() {
		m.from, m.to = from, to // first call
	}
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
	ict, err := time.LoadLocation("Asia/Ho_Chi_Minh")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 7, 10, 0, 0, 0, time.UTC) // 17:00 ICT
	startsAt := now.Add(60 * time.Minute)

	repo := &mockRoadmapRepo{sessions: []port.ScheduledSession{{
		Info: roadmap.SessionPlanInfo{
			SessionPlanID: "sess_001",
			UserID:        "usr_100",
			ScheduledDate: time.Date(2026, 8, 7, 0, 0, 0, 0, time.UTC),
			SlotTime:      "18:00-19:00",
			Status:        roadmap.SessionPlanStatusPending,
			Prescription: roadmap.WorkoutPrescription{
				MainExercises: []roadmap.PrescribedExercise{{ExerciseName: "Bench Press"}},
			},
		},
		StartsAt: startsAt,
		Location: ict,
	}}}
	writer := &mockOutboxWriter{}

	worker := NewUpcomingWorkoutReminderWorker(repo, writer, 100*time.Millisecond)
	worker.runCheckAt(context.Background(), now)
	worker.runCheckAt(context.Background(), now.Add(5*time.Minute)) // same session again: no duplicate

	if !repo.from.Equal(now.Add(45*time.Minute)) || !repo.to.Equal(now.Add(75*time.Minute)) {
		t.Errorf("queried window [%s, %s], want [now+45m, now+75m]", repo.from, repo.to)
	}

	writer.mu.Lock()
	defer writer.mu.Unlock()
	if got, want := len(writer.events), 1; got != want {
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
	if got, want := reminder.ScheduledTime, "18:00"; got != want {
		t.Errorf("ScheduledTime got %s, want %s (the athlete's wall clock)", got, want)
	}
}

// flakyOutbox fails its first Enqueue.
type flakyOutbox struct {
	mockOutboxWriter
	failed bool
}

func (f *flakyOutbox) Enqueue(ctx context.Context, key string, events ...event.Event) error {
	if !f.failed {
		f.failed = true
		return errors.New("db hiccup")
	}
	return f.mockOutboxWriter.Enqueue(ctx, key, events...)
}

func TestUpcomingWorkoutReminderWorker_RetriesAfterEnqueueFailure(t *testing.T) {
	now := time.Date(2026, 8, 7, 10, 0, 0, 0, time.UTC)
	repo := &mockRoadmapRepo{sessions: []port.ScheduledSession{{
		Info: roadmap.SessionPlanInfo{
			SessionPlanID: "sess_001", UserID: "usr_100", SlotTime: "11:00-12:00",
			ScheduledDate: time.Date(2026, 8, 7, 0, 0, 0, 0, time.UTC), Status: roadmap.SessionPlanStatusPending,
		},
		StartsAt: now.Add(time.Hour),
		Location: time.UTC,
	}}}
	writer := &flakyOutbox{}
	worker := NewUpcomingWorkoutReminderWorker(repo, writer, time.Minute)

	worker.runCheckAt(context.Background(), now)                    // enqueue fails
	worker.runCheckAt(context.Background(), now.Add(5*time.Minute)) // retried

	if got := len(writer.events); got != 1 {
		t.Fatalf("got %d reminders, want 1 after the retry", got)
	}
}

package worker

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/viethung213/gym-companion/internal/coaching/application/port"
	"github.com/viethung213/gym-companion/internal/coaching/domain/event"
	"github.com/viethung213/gym-companion/internal/coaching/domain/roadmap"
)

const (
	defaultWorkoutReminderWindow    = 45 * time.Minute
	defaultWorkoutReminderWindowMax = 75 * time.Minute
	remindBeforeWorkoutMinutes      = 60
)

// UpcomingWorkoutReminderWorker scans pending workout sessions scheduled for today
// and enqueues UpcomingWorkoutReminder CloudEvents into coaching outbox ~60 minutes prior.
type UpcomingWorkoutReminderWorker struct {
	roadmapRepo   port.RoadmapRepository
	outboxWriter  port.OutboxWriter
	interval      time.Duration
	stopChan      chan struct{}
	mu            sync.Mutex
	sentReminders map[string]time.Time
}

// NewUpcomingWorkoutReminderWorker constructs a new UpcomingWorkoutReminderWorker.
func NewUpcomingWorkoutReminderWorker(
	roadmapRepo port.RoadmapRepository,
	outboxWriter port.OutboxWriter,
	interval time.Duration,
) *UpcomingWorkoutReminderWorker {
	if interval <= 0 {
		interval = 5 * time.Minute
	}
	return &UpcomingWorkoutReminderWorker{
		roadmapRepo:   roadmapRepo,
		outboxWriter:  outboxWriter,
		interval:      interval,
		stopChan:      make(chan struct{}),
		sentReminders: make(map[string]time.Time),
	}
}

// Start launches the periodic ticker.
func (w *UpcomingWorkoutReminderWorker) Start(ctx context.Context) error {
	slog.InfoContext(ctx, "UpcomingWorkoutReminderWorker started, checking upcoming workout sessions", slog.Duration("interval", w.interval))
	ticker := time.NewTicker(w.interval)

	go func() {
		for {
			select {
			case <-ticker.C:
				w.runCheck(ctx)
			case <-w.stopChan:
				ticker.Stop()
				return
			case <-ctx.Done():
				ticker.Stop()
				return
			}
		}
	}()

	return nil
}

// runCheck inspects pending sessions for today and enqueues reminder events.
func (w *UpcomingWorkoutReminderWorker) runCheck(ctx context.Context) {
	w.runCheckAt(ctx, time.Now())
}

func (w *UpcomingWorkoutReminderWorker) runCheckAt(ctx context.Context, now time.Time) {
	w.cleanupOldReminders(now)

	// Sessions starting 45-75 minutes from now; the repository resolves each
	// slot's wall clock in its roadmap's zone.
	sessions, err := w.roadmapRepo.FindPendingSessionsStarting(ctx,
		now.Add(defaultWorkoutReminderWindow), now.Add(defaultWorkoutReminderWindowMax))
	if err != nil {
		slog.ErrorContext(ctx, "Error fetching pending sessions in UpcomingWorkoutReminderWorker", slog.String("error", err.Error()))
		return
	}

	for i := range sessions {
		session := &sessions[i].Info
		if session.Status != roadmap.SessionPlanStatusPending {
			continue
		}
		startsAt := sessions[i].StartsAt
		reminderKey := fmt.Sprintf("%s:%s", session.SessionPlanID, startsAt.UTC().Format("2006-01-02_15:04"))

		w.mu.Lock()
		_, alreadySent := w.sentReminders[reminderKey]
		if !alreadySent {
			w.sentReminders[reminderKey] = now
		}
		w.mu.Unlock()

		if alreadySent {
			continue
		}

		workoutName := getWorkoutNameFromPrescription(session.Prescription)
		loc := sessions[i].Location
		if loc == nil {
			loc = time.UTC
		}
		timeStr := startsAt.In(loc).Format("15:04") // the athlete's wall clock

		reminderEvent := &event.UpcomingWorkoutReminder{
			UserID:              session.UserID,
			WorkoutName:         workoutName,
			ScheduledTime:       timeStr,
			RemindBeforeMinutes: remindBeforeWorkoutMinutes,
			Title:               "Nhắc nhở buổi tập ⏰",
			Body:                fmt.Sprintf("Buổi tập '%s' của bạn sẽ bắt đầu lúc %s (trước 1 tiếng). Chuẩn bị sẵn sàng nhé!", workoutName, timeStr),
		}

		if err := w.outboxWriter.Enqueue(ctx, session.UserID, reminderEvent); err != nil {
			// Release the slot so the next tick in the window retries.
			w.mu.Lock()
			delete(w.sentReminders, reminderKey)
			w.mu.Unlock()
			slog.ErrorContext(ctx, "Error enqueuing reminder for session",
				slog.String("session_id", session.SessionPlanID),
				slog.String("user_id", session.UserID),
				slog.String("error", err.Error()),
			)
		} else {
			slog.InfoContext(ctx, "Enqueued workout reminder for user",
				slog.String("user_id", session.UserID),
				slog.String("session_id", session.SessionPlanID),
				slog.String("time", timeStr),
			)
		}
	}
}

func getWorkoutNameFromPrescription(p roadmap.WorkoutPrescription) string {
	if len(p.MainExercises) > 0 && p.MainExercises[0].ExerciseName != "" {
		return p.MainExercises[0].ExerciseName
	}
	return "Buổi tập đã lên lịch"
}

func (w *UpcomingWorkoutReminderWorker) cleanupOldReminders(now time.Time) {
	w.mu.Lock()
	defer w.mu.Unlock()

	for key, sentAt := range w.sentReminders {
		if now.Sub(sentAt) > 24*time.Hour {
			delete(w.sentReminders, key)
		}
	}
}

// Stop terminates the worker ticker loop.
func (w *UpcomingWorkoutReminderWorker) Stop() {
	close(w.stopChan)
}

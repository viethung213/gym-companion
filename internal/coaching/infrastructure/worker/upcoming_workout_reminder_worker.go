package worker

import (
	"context"
	"fmt"
	"log"
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
	log.Printf("[UpcomingWorkoutReminderWorker] Started — checking upcoming workout sessions every %v", w.interval)
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

	sessions, err := w.roadmapRepo.FindPendingSessionsByDate(ctx, now)
	if err != nil {
		log.Printf("[UpcomingWorkoutReminderWorker] Error fetching pending sessions: %v", err)
		return
	}

	for _, session := range sessions {
		if session == nil || session.Status != roadmap.SessionPlanStatusPending {
			continue
		}

		workoutTime := parseWorkoutTime(session.ScheduledDate, session.SlotTime)
		timeDiff := workoutTime.Sub(now)

		// Check if session falls into the 60m reminder window (45m to 75m before workout)
		if timeDiff >= defaultWorkoutReminderWindow && timeDiff <= defaultWorkoutReminderWindowMax {
			reminderKey := fmt.Sprintf("%s:%s", session.SessionPlanID, workoutTime.Format("2006-01-02_15:04"))

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
			timeStr := workoutTime.Format("15:04")

			reminderEvent := &event.UpcomingWorkoutReminder{
				UserID:              session.UserID,
				WorkoutName:         workoutName,
				ScheduledTime:       timeStr,
				RemindBeforeMinutes: remindBeforeWorkoutMinutes,
				Title:               "Nhắc nhở buổi tập ⏰",
				Body:                fmt.Sprintf("Buổi tập '%s' của bạn sẽ bắt đầu lúc %s (trước 1 tiếng). Chuẩn bị sẵn sàng nhé!", workoutName, timeStr),
			}

			if err := w.outboxWriter.Enqueue(ctx, session.UserID, reminderEvent); err != nil {
				log.Printf("[UpcomingWorkoutReminderWorker] Error enqueuing reminder for session %s (user %s): %v", session.SessionPlanID, session.UserID, err)
			} else {
				log.Printf("[UpcomingWorkoutReminderWorker] Enqueued workout reminder for user %s, session %s at %s", session.UserID, session.SessionPlanID, timeStr)
			}
		}
	}
}

func parseWorkoutTime(scheduledDate time.Time, slotTime string) time.Time {
	year, month, day := scheduledDate.Date()
	hour, minute := 9, 0 // Default 09:00 if slotTime is empty

	if slotTime != "" {
		_, _ = fmt.Sscanf(slotTime, "%d:%d", &hour, &minute)
	}

	wt := time.Date(year, month, day, hour, minute, 0, 0, scheduledDate.Location())
	if wt.Before(scheduledDate) && scheduledDate.Sub(wt) > 12*time.Hour {
		wt = wt.AddDate(0, 0, 1)
	}
	return wt
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

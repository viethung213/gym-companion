//go:build integration

package persistence_test

import (
	"context"
	"testing"
	"time"
	_ "time/tzdata"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/viethung213/gym-companion/internal/coaching/application/port"
	"github.com/viethung213/gym-companion/internal/coaching/domain/roadmap"
	"github.com/viethung213/gym-companion/internal/coaching/infrastructure/persistence"
)

// newPersistedRoadmap builds a 4-week Asia/Ho_Chi_Minh roadmap starting Mon 2026-11-23
// with a completed Monday session and pending Thursday + next-Monday sessions.
func newPersistedRoadmap(t *testing.T, userID string) *roadmap.Roadmap {
	t.Helper()
	now := time.Date(2026, 11, 23, 0, 0, 0, 0, time.UTC)
	start := now
	rmID := uuid.NewString()
	rm, err := roadmap.NewRoadmap(&roadmap.Info{
		RoadmapID: rmID, UserID: userID, Timezone: "Asia/Ho_Chi_Minh",
		StartDate: start, EndDate: start.AddDate(0, 0, 28),
	}, nil, now)
	require.NoError(t, err)

	weeks := make([]*roadmap.WeekPlan, 0, roadmap.WeeksPerRoadmap)
	for n := range roadmap.WeeksPerRoadmap {
		ws := start.AddDate(0, 0, n*7)
		w, wErr := roadmap.NewWeekPlan(&roadmap.WeekPlanInfo{
			WeekPlanID: uuid.NewString(), RoadmapID: rmID, UserID: userID, WeekNumber: int32(n + 1),
			Phase: roadmap.PhaseAccumulation, TargetRPE: 7, StartDate: ws, EndDate: ws.AddDate(0, 0, 7),
		})
		require.NoError(t, wErr)
		weeks = append(weeks, w)
	}

	addDay := func(w *roadmap.WeekPlan, date time.Time, slot string, status roadmap.SessionPlanStatus) {
		d, dErr := roadmap.NewDayPlan(&roadmap.DayPlanInfo{
			DayPlanID: uuid.NewString(), WeekPlanID: w.ID(), RoadmapID: rmID, UserID: userID, ScheduledDate: date,
		})
		require.NoError(t, dErr)
		s, sErr := roadmap.RehydrateSessionPlan(&roadmap.SessionPlanInfo{
			SessionPlanID: uuid.NewString(), DayPlanID: d.ID(), WeekPlanID: w.ID(), RoadmapID: rmID, UserID: userID,
			ScheduledDate: date, SlotTime: slot, EstimatedDurationMinutes: 60, Status: status,
			Source: roadmap.SessionPlanSourceScheduled, GeneratedAt: now,
		})
		require.NoError(t, sErr)
		require.NoError(t, d.AddSession(s))
		require.NoError(t, w.AddDay(d))
	}
	addDay(weeks[0], start, "17:30-19:00", roadmap.SessionPlanStatusCompleted)
	addDay(weeks[0], start.AddDate(0, 0, 3), "17:30-19:00", roadmap.SessionPlanStatusPending)
	addDay(weeks[1], start.AddDate(0, 0, 7), "17:30-19:00", roadmap.SessionPlanStatusPending)

	rm, err = roadmap.RehydrateRoadmap(func() *roadmap.Info { i := rm.Info(); return &i }(), weeks)
	require.NoError(t, err)
	return rm
}

func TestRoadmapRepository_TimezoneAndReschedule_Integration(t *testing.T) {
	db := getCoachingTestDB(t)
	repo := persistence.NewRoadmapRepository(db)
	ctx := context.Background()
	userID := uuid.NewString()

	rm := newPersistedRoadmap(t, userID)
	require.NoError(t, repo.Save(ctx, rm))
	t.Cleanup(func() { db.Exec("DELETE FROM coaching.roadmaps WHERE roadmap_id = ?", rm.ID()) })

	loaded, err := repo.FindByID(ctx, rm.ID())
	require.NoError(t, err)
	assert.Equal(t, "Asia/Ho_Chi_Minh", loaded.Info().Timezone)

	// Tue 2026-11-24 09:00 ICT: move the athlete to Tue/Thu 06:00 slots. The
	// Thursday day plan is emptied and refilled on the same date.
	now := time.Date(2026, 11, 24, 2, 0, 0, 0, time.UTC)
	drafts := []*roadmap.SessionPlanInfo{
		{ScheduledDate: time.Date(2026, 11, 26, 0, 0, 0, 0, time.UTC), SlotTime: "06:00-07:00", EstimatedDurationMinutes: 60},
		{ScheduledDate: time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC), SlotTime: "06:00-07:00", EstimatedDurationMinutes: 60},
	}
	require.NoError(t, loaded.ReschedulePending(now, drafts, uuid.NewString))
	require.NoError(t, repo.Save(ctx, loaded))

	reloaded, err := repo.FindByID(ctx, rm.ID())
	require.NoError(t, err)
	var slots []string
	completed := 0
	for _, w := range reloaded.Weeks() {
		for _, d := range w.Days() {
			for _, s := range d.Sessions() {
				info := s.Info()
				if info.Status == roadmap.SessionPlanStatusCompleted {
					completed++
					continue
				}
				slots = append(slots, info.ScheduledDate.Format(time.DateOnly)+" "+info.SlotTime)
			}
		}
	}
	assert.Equal(t, 1, completed, "the completed session must survive the reschedule")
	assert.ElementsMatch(t, []string{"2026-11-26 06:00-07:00", "2026-12-01 06:00-07:00"}, slots)

	var dayRows int64
	require.NoError(t, db.Table("coaching.day_plans").Where("roadmap_id = ?", rm.ID()).Count(&dayRows).Error)
	assert.Equal(t, int64(3), dayRows, "emptied day plans must be deleted")
}

func TestRoadmapRepository_FindPendingSessionsStarting_Integration(t *testing.T) {
	db := getCoachingTestDB(t)
	repo := persistence.NewRoadmapRepository(db)
	ctx := context.Background()

	rm := newPersistedRoadmap(t, uuid.NewString())
	require.NoError(t, repo.Save(ctx, rm))
	t.Cleanup(func() { db.Exec("DELETE FROM coaching.roadmaps WHERE roadmap_id = ?", rm.ID()) })

	// Thu 2026-11-26 17:30 ICT = 10:30 UTC.
	at := time.Date(2026, 11, 26, 10, 30, 0, 0, time.UTC)
	all, err := repo.FindPendingSessionsStarting(ctx, at.Add(-15*time.Minute), at.Add(15*time.Minute))
	require.NoError(t, err)
	got := ownSessions(all, rm.ID())
	require.Len(t, got, 1)
	assert.True(t, got[0].StartsAt.Equal(at), "StartsAt = %s, want %s", got[0].StartsAt, at)
	assert.Equal(t, "Asia/Ho_Chi_Minh", got[0].Location.String())

	// 17:30 UTC on the same day is 00:30 ICT the next day: nothing starts then.
	later, err := repo.FindPendingSessionsStarting(ctx, at.Add(7*time.Hour-15*time.Minute), at.Add(7*time.Hour+15*time.Minute))
	require.NoError(t, err)
	assert.Empty(t, ownSessions(later, rm.ID()))
}

func TestRoadmapRepository_SecondActiveRoadmapIsDomainError_Integration(t *testing.T) {
	db := getCoachingTestDB(t)
	repo := persistence.NewRoadmapRepository(db)
	ctx := context.Background()
	userID := uuid.NewString()

	first := newPersistedRoadmap(t, userID)
	require.NoError(t, repo.Save(ctx, first))
	t.Cleanup(func() { db.Exec("DELETE FROM coaching.roadmaps WHERE user_id = ?", userID) })

	err := repo.Save(ctx, newPersistedRoadmap(t, userID))
	require.ErrorIs(t, err, roadmap.ErrActiveRoadmapExists)
}

// ownSessions keeps the sessions of one roadmap: the database may hold others.
func ownSessions(all []port.ScheduledSession, roadmapID string) []port.ScheduledSession {
	var out []port.ScheduledSession
	for _, s := range all {
		if s.Info.RoadmapID == roadmapID {
			out = append(out, s)
		}
	}
	return out
}

// A profile change the schedule could not absorb survives save and load, is
// listed for the lifecycle worker, and is gone once cleared.
func TestRoadmapRepository_RegenerationDue_Integration(t *testing.T) {
	db := getCoachingTestDB(t)
	repo := persistence.NewRoadmapRepository(db)
	ctx := context.Background()
	userID := uuid.NewString()

	rm := newPersistedRoadmap(t, userID)
	changedAt := time.Date(2026, 11, 24, 3, 0, 0, 0, time.UTC)
	rm.MarkRegenerationDue(changedAt, "injury_reported")
	require.NoError(t, repo.Save(ctx, rm))
	t.Cleanup(func() { db.Exec("DELETE FROM coaching.roadmaps WHERE roadmap_id = ?", rm.ID()) })

	loaded, err := repo.FindByID(ctx, rm.ID())
	require.NoError(t, err)
	due, err := loaded.PendingRegeneration()
	require.NoError(t, err)
	assert.True(t, due.ChangedAt.Equal(changedAt))
	assert.Equal(t, "injury_reported", due.Reason)

	users, err := repo.UsersWithRegenerationDue(ctx)
	require.NoError(t, err)
	assert.Contains(t, users, userID)

	loaded.ClearRegenerationDue()
	require.NoError(t, repo.Save(ctx, loaded))
	users, err = repo.UsersWithRegenerationDue(ctx)
	require.NoError(t, err)
	assert.NotContains(t, users, userID)
}

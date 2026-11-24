package roadmap

import (
	"errors"
	"strings"
	"testing"
	"time"
	_ "time/tzdata"
)

// scheduleFixture is a 4-week roadmap in Asia/Ho_Chi_Minh starting Mon 2026-11-23,
// with weeks built the way mapToDomainRoadmap builds them ([start, start+7)).
func scheduleFixture(t *testing.T) *Roadmap {
	t.Helper()
	start := time.Date(2026, 11, 23, 0, 0, 0, 0, time.UTC)
	info := Info{
		RoadmapID: "rm-1", UserID: "user-1", Status: StatusActive, Timezone: "Asia/Ho_Chi_Minh",
		StartDate: start, EndDate: start.AddDate(0, 0, 28),
	}
	weeks := make([]*WeekPlan, 0, WeeksPerRoadmap)
	for n := range WeeksPerRoadmap {
		ws := start.AddDate(0, 0, n*7)
		w, err := NewWeekPlan(&WeekPlanInfo{
			WeekPlanID: "w" + string(rune('1'+n)), RoadmapID: "rm-1", UserID: "user-1",
			WeekNumber: int32(n + 1), Phase: PhaseAccumulation, TargetRPE: 7,
			StartDate: ws, EndDate: ws.AddDate(0, 0, 7),
		})
		if err != nil {
			t.Fatal(err)
		}
		weeks = append(weeks, w)
	}
	rm, err := RehydrateRoadmap(&info, weeks)
	if err != nil {
		t.Fatal(err)
	}
	return rm
}

// addSession puts a session on date at slot into week w (0-based).
func addSession(t *testing.T, rm *Roadmap, w int, id string, date time.Time, slot string, status SessionPlanStatus) {
	t.Helper()
	week := rm.weeks[w]
	day, err := week.dayOn(date, func() string { return "d-" + id })
	if err != nil {
		t.Fatal(err)
	}
	s, err := RehydrateSessionPlan(&SessionPlanInfo{
		SessionPlanID: id, DayPlanID: day.ID(), WeekPlanID: week.ID(), RoadmapID: "rm-1", UserID: "user-1",
		ScheduledDate: date, SlotTime: slot, Status: status, Source: SessionPlanSourceScheduled,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := day.AddSession(s); err != nil {
		t.Fatal(err)
	}
}

func day(d int) time.Time { return time.Date(2026, 11, d, 0, 0, 0, 0, time.UTC) }

func sessionIDs(ss []*SessionPlan) []string {
	out := make([]string, 0, len(ss))
	for _, s := range ss {
		out = append(out, s.ID())
	}
	return out
}

func TestSessionPlan_StartsAtUsesWallClockInZone(t *testing.T) {
	rm := scheduleFixture(t)
	addSession(t, rm, 0, "s1", day(24), "17:30-19:00", SessionPlanStatusPending)
	s, _ := rm.FindSession("s1")

	got := s.StartsAt(rm.Location())
	want := time.Date(2026, 11, 24, 10, 30, 0, 0, time.UTC) // 17:30 ICT = 10:30 UTC
	if !got.Equal(want) {
		t.Fatalf("StartsAt = %s, want %s", got.UTC(), want)
	}
}

func TestRoadmap_LocationFallsBackToUTC(t *testing.T) {
	rm := scheduleFixture(t)
	rm.info.Timezone = "Mars/Base"
	if rm.Location() != time.UTC {
		t.Fatalf("Location = %v, want UTC for an unknown zone", rm.Location())
	}
	rm.SetTimezone("")
	if rm.Info().Timezone != "Mars/Base" {
		t.Fatal("SetTimezone(\"\") must keep the stored zone")
	}
}

func TestRoadmap_UpcomingPendingSessions(t *testing.T) {
	rm := scheduleFixture(t)
	addSession(t, rm, 0, "past-today", day(24), "06:00-07:00", SessionPlanStatusPending)
	addSession(t, rm, 0, "later-today", day(24), "17:30-19:00", SessionPlanStatusPending)
	addSession(t, rm, 0, "done-tomorrow", day(25), "17:30-19:00", SessionPlanStatusCompleted)
	addSession(t, rm, 1, "next-week", day(30), "06:00-07:00", SessionPlanStatusPending)

	// 2026-11-24 09:00 ICT: the 06:00 slot has started, the 17:30 one has not.
	now := time.Date(2026, 11, 24, 2, 0, 0, 0, time.UTC)
	got := sessionIDs(rm.UpcomingPendingSessions(now))
	want := []string{"later-today", "next-week"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("UpcomingPendingSessions = %v, want %v", got, want)
	}
}

func TestRoadmap_ReschedulePending(t *testing.T) {
	now := time.Date(2026, 11, 24, 2, 0, 0, 0, time.UTC) // Tue 09:00 ICT
	ids := 0
	newID := func() string { ids++; return "new-" + string(rune('a'+ids)) }

	t.Run("replaces upcoming pending, keeps finished and past sessions", func(t *testing.T) {
		rm := scheduleFixture(t)
		addSession(t, rm, 0, "done-mon", day(23), "17:30-19:00", SessionPlanStatusCompleted)
		addSession(t, rm, 0, "missed-tue", day(24), "06:00-07:00", SessionPlanStatusPending)
		addSession(t, rm, 0, "old-thu", day(26), "17:30-19:00", SessionPlanStatusPending)
		addSession(t, rm, 1, "old-mon", day(30), "17:30-19:00", SessionPlanStatusPending)

		drafts := []*SessionPlanInfo{
			{ScheduledDate: day(24), SlotTime: "18:00-19:00"}, // same day as a kept session
			{ScheduledDate: day(27), SlotTime: "06:00-07:00"}, // new day
			{ScheduledDate: day(30), SlotTime: "06:00-07:00"}, // reuses week 2's day
		}
		if err := rm.ReschedulePending(now, drafts, newID); err != nil {
			t.Fatalf("ReschedulePending: %v", err)
		}

		for _, gone := range []string{"old-thu", "old-mon"} {
			if _, ok := rm.FindSession(gone); ok {
				t.Errorf("%s should be removed", gone)
			}
		}
		for _, kept := range []string{"done-mon", "missed-tue"} {
			if _, ok := rm.FindSession(kept); !ok {
				t.Errorf("%s should be kept", kept)
			}
		}
		upcoming := rm.UpcomingPendingSessions(now)
		if len(upcoming) != 3 {
			t.Fatalf("got %d upcoming sessions, want 3", len(upcoming))
		}
		for _, s := range upcoming {
			info := s.Info()
			if info.Status != SessionPlanStatusPending || info.RoadmapID != "rm-1" || info.UserID != "user-1" {
				t.Errorf("session %s has wrong ownership/status: %+v", s.ID(), info)
			}
		}
		for _, d := range rm.weeks[0].Days() {
			if d.ScheduledDate().Equal(day(26)) {
				t.Error("emptied Thursday day plan should be dropped")
			}
		}
		removedSessions, removedDays := rm.RemovedChildren()
		if strings.Join(removedSessions, ",") != "old-thu,old-mon" {
			t.Errorf("removed sessions = %v, want [old-thu old-mon]", removedSessions)
		}
		if strings.Join(removedDays, ",") != "d-old-thu,d-old-mon" {
			t.Errorf("removed days = %v, want [d-old-thu d-old-mon]", removedDays)
		}
		if got := rm.weeks[0].TotalSessions(); got != 4 {
			t.Errorf("week 1 holds %d sessions, want 4 (2 kept + 2 new)", got)
		}
	})

	t.Run("rejects a draft outside the roadmap", func(t *testing.T) {
		rm := scheduleFixture(t)
		err := rm.ReschedulePending(now, []*SessionPlanInfo{{ScheduledDate: day(23).AddDate(0, 0, 28), SlotTime: "06:00"}}, newID)
		if !errors.Is(err, ErrInvalidRoadmap) {
			t.Fatalf("got %v, want ErrInvalidRoadmap", err)
		}
	})

	t.Run("rejects a week over the cap", func(t *testing.T) {
		rm := scheduleFixture(t)
		drafts := make([]*SessionPlanInfo, 0, MaxSessionsPerWeek+1)
		for i := range MaxSessionsPerWeek + 1 {
			drafts = append(drafts, &SessionPlanInfo{ScheduledDate: day(24 + i%5), SlotTime: "20:0" + string(rune('0'+i))})
		}
		if err := rm.ReschedulePending(now, drafts, newID); !errors.Is(err, ErrInvalidRoadmap) {
			t.Fatalf("got %v, want ErrInvalidRoadmap", err)
		}
	})
}

func TestRoadmap_ReschedulePending_SkipsStartedAndOccupiedSlots(t *testing.T) {
	now := time.Date(2026, 11, 24, 2, 0, 0, 0, time.UTC) // Tue 09:00 ICT
	rm := scheduleFixture(t)
	addSession(t, rm, 0, "done-wed", day(25), "17:30-19:00", SessionPlanStatusCompleted)
	addSession(t, rm, 0, "old-thu", day(26), "17:30-19:00", SessionPlanStatusPending)

	n := 0
	drafts := []*SessionPlanInfo{
		{ScheduledDate: day(24), SlotTime: "06:00-07:00"}, // started at 06:00 ICT
		{ScheduledDate: day(25), SlotTime: "17:30-19:00"}, // the completed session holds it
		{ScheduledDate: day(27), SlotTime: "17:30-19:00"},
		{ScheduledDate: day(27), SlotTime: "17:30-19:00"}, // duplicate draft
	}
	if err := rm.ReschedulePending(now, drafts, func() string { n++; return "id-" + string(rune('a'+n)) }); err != nil {
		t.Fatalf("ReschedulePending: %v", err)
	}

	var got []string
	for _, w := range rm.weeks {
		for _, d := range w.days {
			for _, s := range d.sessions {
				got = append(got, s.info.ScheduledDate.Format(time.DateOnly)+" "+s.info.SlotTime)
			}
		}
	}
	want := "2026-11-25 17:30-19:00,2026-11-27 17:30-19:00"
	if strings.Join(got, ",") != want {
		t.Fatalf("sessions = %v, want %s", got, want)
	}
}

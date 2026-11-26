package roadmap

import (
	"testing"
	"time"
)

// deloadFixture is scheduleFixture with week 3 PEAK and week 4 DELOAD. The PEAK
// week holds one session with peakSets main sets; each DELOAD session holds one
// main line per entry of its sets slice.
func deloadFixture(t *testing.T, peakSets int32, deload [][]int32, status []SessionPlanStatus) *Roadmap {
	t.Helper()
	rm := scheduleFixture(t)
	rm.weeks[2].info.Phase = PhasePeak
	rm.weeks[3].info.Phase = PhaseDeload
	add := func(w int, id string, date time.Time, st SessionPlanStatus, sets []int32) {
		addSession(t, rm, w, id, date, "17:30-19:00", st)
		s, _ := rm.FindSession(id)
		for i, n := range sets {
			s.info.Prescription.MainExercises = append(s.info.Prescription.MainExercises,
				PrescribedExercise{ExerciseID: id + "-ex" + string(rune('a'+i)), TargetSets: n, TargetReps: 8, TargetRPE: 5.5})
		}
	}
	add(2, "peak", day(30).AddDate(0, 0, 9), SessionPlanStatusPending, []int32{peakSets})
	for i, sets := range deload {
		add(3, "dl"+string(rune('1'+i)), time.Date(2026, 12, 14+2*i, 0, 0, 0, 0, time.UTC), status[i], sets)
	}
	return rm
}

func setsOf(t *testing.T, rm *Roadmap, id string) []int32 {
	t.Helper()
	s, _ := rm.FindSession(id)
	mains := s.Info().Prescription.MainExercises
	out := make([]int32, 0, len(mains))
	for _, ex := range mains {
		out = append(out, ex.TargetSets)
	}
	return out
}

func equalSets(a, b []int32) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// beforeTheBlock is a "now" before every session of the fixture.
func beforeTheBlock() time.Time { return time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC) }

// PEAK 10 sets allows 7 in DELOAD; 4+4 = 8 loses one set from the largest line.
func TestFitDeloadVolume_CutsToTheAllowance(t *testing.T) {
	rm := deloadFixture(t, 10, [][]int32{{4}, {4}}, []SessionPlanStatus{SessionPlanStatusPending, SessionPlanStatusPending})

	changed, err := rm.FitDeloadVolume(beforeTheBlock())
	if err != nil || !changed {
		t.Fatalf("changed %v err %v, want a cut", changed, err)
	}
	if got := rm.WorkingSets(PhaseDeload); got != 7 {
		t.Errorf("DELOAD sets = %d, want 7", got)
	}
}

// Small lines used to stay above the allowance after proportional scaling:
// 1,1,1,3 + 1,3 = 10 scaled by 0.7 and floored at 1 is 1,1,1,2 + 1,2 = 8 > 7.
// The cut keeps going on the larger lines until it fits.
func TestFitDeloadVolume_ReachesTheAllowanceWithSmallLines(t *testing.T) {
	rm := deloadFixture(t, 10, [][]int32{{1, 1, 1, 3}, {1, 3}}, []SessionPlanStatus{SessionPlanStatusPending, SessionPlanStatusPending})

	if _, err := rm.FitDeloadVolume(beforeTheBlock()); err != nil {
		t.Fatal(err)
	}
	if got := rm.WorkingSets(PhaseDeload); got != 7 {
		t.Errorf("DELOAD sets = %d, want 7", got)
	}
}

// A finished session is never rewritten; the cut lands on the upcoming one,
// whose length follows its new prescription.
func TestFitDeloadVolume_LeavesFinishedSessions(t *testing.T) {
	rm := deloadFixture(t, 10, [][]int32{{4}, {4}}, []SessionPlanStatus{SessionPlanStatusCompleted, SessionPlanStatusPending})

	if _, err := rm.FitDeloadVolume(beforeTheBlock()); err != nil {
		t.Fatal(err)
	}
	if got := setsOf(t, rm, "dl1"); !equalSets(got, []int32{4}) {
		t.Errorf("completed session sets = %v, want untouched [4]", got)
	}
	if got := setsOf(t, rm, "dl2"); !equalSets(got, []int32{3}) {
		t.Errorf("upcoming session sets = %v, want [3]", got)
	}
	s, _ := rm.FindSession("dl2")
	if want := s.Info().Prescription.EstimatedMinutes(); s.Info().EstimatedDurationMinutes != want {
		t.Errorf("length %d, want %d from the cut prescription", s.Info().EstimatedDurationMinutes, want)
	}
}

// When finished sessions alone exceed the allowance, the upcoming ones go down
// to one set per line and no further.
func TestFitDeloadVolume_FloorsAtOneSetPerLine(t *testing.T) {
	rm := deloadFixture(t, 10, [][]int32{{8}, {3, 3}}, []SessionPlanStatus{SessionPlanStatusCompleted, SessionPlanStatusPending})

	if _, err := rm.FitDeloadVolume(beforeTheBlock()); err != nil {
		t.Fatal(err)
	}
	if got := setsOf(t, rm, "dl2"); !equalSets(got, []int32{1, 1}) {
		t.Errorf("upcoming session sets = %v, want [1 1]", got)
	}
}

// Within the allowance nothing changes.
func TestFitDeloadVolume_NoCutWithinTheAllowance(t *testing.T) {
	rm := deloadFixture(t, 10, [][]int32{{3}, {4}}, []SessionPlanStatus{SessionPlanStatusPending, SessionPlanStatusPending})

	changed, err := rm.FitDeloadVolume(beforeTheBlock())
	if err != nil || changed {
		t.Errorf("changed %v err %v, want nothing", changed, err)
	}
}

// A fatigue deload keeps 70% of each session's sets: 10 becomes 7, cut from
// the largest lines. The DELOAD week is already unloaded and stays as it is.
func TestUnloadUpcoming_CutsNonDeloadWeeksOnly(t *testing.T) {
	rm := deloadFixture(t, 0, [][]int32{{2, 2}}, []SessionPlanStatus{SessionPlanStatusPending})
	s, _ := rm.FindSession("peak")
	s.info.Prescription.MainExercises = []PrescribedExercise{
		{ExerciseID: "a", TargetSets: 4, TargetReps: 8, TargetRPE: 8},
		{ExerciseID: "b", TargetSets: 3, TargetReps: 8, TargetRPE: 8},
		{ExerciseID: "c", TargetSets: 3, TargetReps: 8, TargetRPE: 8},
	}

	cut, err := rm.UnloadUpcoming(map[string]int{"peak": 10, "dl1": 4}, 0.7, beforeTheBlock())
	if err != nil {
		t.Fatalf("UnloadUpcoming: %v", err)
	}
	if len(cut) != 1 || cut[0] != "peak" {
		t.Errorf("cut %v, want only the PEAK session", cut)
	}
	if got := setsOf(t, rm, "peak"); !equalSets(got, []int32{2, 2, 3}) {
		t.Errorf("PEAK sets = %v, want [2 2 3]", got)
	}
	if got := setsOf(t, rm, "dl1"); !equalSets(got, []int32{2, 2}) {
		t.Errorf("DELOAD sets = %v, want them untouched", got)
	}
}

// A model that already cut deeper than asked, and a session the deload did
// not see before, are left as they are.
func TestUnloadUpcoming_LeavesDeepEnoughAndUnknownSessions(t *testing.T) {
	rm := deloadFixture(t, 3, nil, nil)

	cut, err := rm.UnloadUpcoming(map[string]int{"peak": 6}, 0.7, beforeTheBlock())
	if err != nil || len(cut) != 0 {
		t.Fatalf("cut %v err %v, want nothing (3 <= 4)", cut, err)
	}
	if cut, _ = rm.UnloadUpcoming(map[string]int{}, 0.7, beforeTheBlock()); len(cut) != 0 {
		t.Errorf("cut %v, want nothing for an unknown session", cut)
	}
}

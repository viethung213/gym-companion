package roadmap

import (
	"errors"
	"testing"
	"time"
)

func TestRoadmap_RegenerationDue(t *testing.T) {
	rm := &Roadmap{}
	if _, err := rm.PendingRegeneration(); !errors.Is(err, ErrNoRegenerationDue) {
		t.Fatalf("fresh roadmap: err = %v, want ErrNoRegenerationDue", err)
	}

	first := time.Date(2026, 11, 27, 10, 0, 0, 0, time.UTC)
	rm.MarkRegenerationDue(first.Add(time.Hour), "injury_reported")
	rm.MarkRegenerationDue(first, "profile_updated") // an older change is covered
	due, err := rm.PendingRegeneration()
	if err != nil || !due.ChangedAt.Equal(first.Add(time.Hour)) || due.Reason != "injury_reported" {
		t.Fatalf("due = %+v err %v, want the latest change kept", due, err)
	}

	if due.RegenerationExpired(due.ChangedAt.Add(RegenerationGiveUpAfter)) {
		t.Error("expired at exactly the give-up age")
	}
	if !due.RegenerationExpired(due.ChangedAt.Add(RegenerationGiveUpAfter + time.Minute)) {
		t.Error("not expired past the give-up age")
	}

	rm.ClearRegenerationDue()
	if _, err := rm.PendingRegeneration(); !errors.Is(err, ErrNoRegenerationDue) {
		t.Errorf("after clear: err = %v, want ErrNoRegenerationDue", err)
	}
}

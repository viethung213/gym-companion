package redis

import (
	"math"
	"testing"
	"time"
)

func TestScoreCodec_RoundTrip(t *testing.T) {
	times := []struct {
		in   time.Time
		want time.Time
	}{
		{scoreEpoch(), scoreEpoch()},
		{
			time.Date(2026, 11, 27, 10, 0, 0, 999_000_000, time.UTC),
			time.Date(2026, 11, 27, 10, 0, 0, 0, time.UTC),
		},
		{
			time.Date(2026, 11, 27, 17, 0, 5, 0, time.FixedZone("ICT", 7*3600)),
			time.Date(2026, 11, 27, 10, 0, 5, 0, time.UTC),
		},
	}
	for _, xp := range []int64{0, 1, 490_050, scoreMaxXp} {
		for _, tc := range times {
			gotXp, gotTime := decodeScore(encodeScore(xp, tc.in))
			if gotXp != xp || !gotTime.Equal(tc.want) || gotTime.Location() != time.UTC {
				t.Errorf("round trip (%d, %v) = (%d, %v), want (%d, %v UTC)", xp, tc.in, gotXp, gotTime, xp, tc.want)
			}
		}
	}
}

func TestScoreCodec_HigherXpRanksFirst(t *testing.T) {
	earlier := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	later := earlier.Add(300 * 24 * time.Hour)
	if !(encodeScore(101, later) < encodeScore(100, earlier)) {
		t.Fatal("higher xp must produce a lower (better) score regardless of time")
	}
}

func TestScoreCodec_EarlierTimeWinsOnTie(t *testing.T) {
	earlier := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	if !(encodeScore(500, earlier) < encodeScore(500, earlier.Add(time.Second))) {
		t.Fatal("on equal xp the earlier update must produce a lower (better) score")
	}
	if encodeScore(500, earlier) != encodeScore(500, earlier.Add(999*time.Millisecond)) {
		t.Fatal("sub-second differences must not change the score")
	}
}

func TestScoreCodec_Clamp(t *testing.T) {
	_, at := decodeScore(encodeScore(10, scoreEpoch().Add(-48*time.Hour)))
	if !at.Equal(scoreEpoch()) {
		t.Errorf("time before epoch decoded to %v, want %v", at, scoreEpoch())
	}

	farFuture := scoreEpoch().Add(time.Duration(scoreMaxSeconds+10) * time.Second)
	_, at = decodeScore(encodeScore(10, farFuture))
	if want := scoreEpoch().Add(time.Duration(scoreMaxSeconds) * time.Second); !at.Equal(want) {
		t.Errorf("far future decoded to %v, want %v", at, want)
	}

	if xp, _ := decodeScore(encodeScore(scoreMaxXp+5, scoreEpoch())); xp != scoreMaxXp {
		t.Errorf("xp above max decoded to %d, want %d", xp, scoreMaxXp)
	}
	if xp, _ := decodeScore(encodeScore(-3, scoreEpoch())); xp != 0 {
		t.Errorf("negative xp decoded to %d, want 0", xp)
	}
}

func TestScoreCodec_ExactInteger(t *testing.T) {
	score := encodeScore(scoreMaxXp, scoreEpoch())
	composite := -score
	if composite != math.Trunc(composite) {
		t.Fatalf("score %v is not an integer", score)
	}
	if composite > float64(int64(1)<<53) {
		t.Fatalf("composite %v exceeds 2^53", composite)
	}
	worst := encodeScore(0, scoreEpoch().Add(time.Duration(scoreMaxSeconds)*time.Second))
	if worst != 0 || score >= worst {
		t.Fatalf("scores must lie in [-(2^53), 0]: best %v, worst %v", score, worst)
	}
}

func TestScoreCodec_PreLaunchHistoryKeepsTimeOrder(t *testing.T) {
	earlier := time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC)
	later := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)
	if !(encodeScore(300, earlier) < encodeScore(300, later)) {
		t.Fatal("updates from 2024 must still tie-break by time (epoch too late)")
	}
}

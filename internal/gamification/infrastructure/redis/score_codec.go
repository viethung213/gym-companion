package redis

import (
	"log/slog"
	"time"
)

// Score layout (see leaderboards ADR-0003 §3.2). A float64 represents integers exactly up to
// 2^53, so the composite packs 22 bits of XP above 31 bits of inverted time-since-epoch:
//
//	composite = xp<<31 | (scoreMaxSeconds - secondsSinceEpoch)
//	score     = -composite
//
// Higher XP, then earlier update, yields a larger composite and therefore a lower score, so
// ascending ZRANGE/ZRANK give leaderboard order and equal scores fall back to user_id ASC.
const (
	scoreEpochUnix  int64 = 1577836800 // 2020-01-01T00:00:00Z; 2^31 s reaches 2088
	scoreTimeBits         = 31
	scoreMaxSeconds int64 = 1<<scoreTimeBits - 1
	scoreMaxXp      int64 = 1<<22 - 1
)

func scoreEpoch() time.Time {
	return time.Unix(scoreEpochUnix, 0).UTC()
}

// encodeScore converts a user's XP and last update time into a ZSET score.
func encodeScore(xp int64, updatedAt time.Time) float64 {
	if xp > scoreMaxXp {
		slog.Warn("Leaderboard XP exceeds score capacity, clamping",
			slog.String("module", "gamification"),
			slog.Int64("xp", xp),
			slog.Int64("max_xp", scoreMaxXp))
	}
	clampedXp := clampInt64(xp, 0, scoreMaxXp)
	seconds := clampInt64(updatedAt.Unix()-scoreEpochUnix, 0, scoreMaxSeconds)

	composite := clampedXp<<scoreTimeBits | (scoreMaxSeconds - seconds)
	return -float64(composite)
}

// decodeScore recovers XP and the (second-precision, UTC) update time from a ZSET score.
func decodeScore(score float64) (xp int64, updatedAt time.Time) {
	composite := int64(-score)
	xp = composite >> scoreTimeBits
	seconds := scoreMaxSeconds - composite&scoreMaxSeconds
	return xp, time.Unix(scoreEpochUnix+seconds, 0).UTC()
}

func clampInt64(v, lo, hi int64) int64 {
	return max(lo, min(v, hi))
}

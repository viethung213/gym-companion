package consumer

import (
	"context"
	"errors"
	"testing"

	"github.com/segmentio/kafka-go"
)

// A refresh signal that keeps failing (Redis down) must be dropped: not dead-lettered, which would
// flood the system DLQ during an outage, and not left uncommitted, which would stall the consumer.
// The periodic reconcile re-derives the score from PostgreSQL.
func TestLeaderboardLoopConfig_DropsExhaustedSignals(t *testing.T) {
	cfg := leaderboardLoopConfig()

	if cfg.DLQWriter != nil {
		t.Error("DLQWriter set, want failed signals kept out of the system DLQ")
	}
	if cfg.OnDLQ == nil {
		t.Fatal("OnDLQ = nil, want exhausted signals acknowledged so the offset is committed")
	}
	msg := &kafka.Message{Topic: "gamification.events", Offset: 7}
	if err := cfg.OnDLQ(context.Background(), msg, errors.New("redis down")); err != nil {
		t.Errorf("OnDLQ = %v, want nil (message handled by dropping it)", err)
	}
	if cfg.MaxRetries != consumerMaxRetries || cfg.BaseBackoff != consumerBaseBackoff {
		t.Errorf("retries = %d/%v, want the module defaults %d/%v",
			cfg.MaxRetries, cfg.BaseBackoff, consumerMaxRetries, consumerBaseBackoff)
	}
}

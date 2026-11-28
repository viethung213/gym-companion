package gamification

import (
	"context"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/google/uuid"
	goredis "github.com/redis/go-redis/v9"
	"github.com/segmentio/kafka-go"
	"github.com/viethung213/gym-companion/internal/gamification/application/command"
	"github.com/viethung213/gym-companion/internal/gamification/domain/repository"
	"github.com/viethung213/gym-companion/internal/gamification/infrastructure/persistence"
	lbredis "github.com/viethung213/gym-companion/internal/gamification/infrastructure/redis"
	"github.com/viethung213/gym-companion/internal/gamification/infrastructure/worker"
	"github.com/viethung213/gym-companion/internal/gamification/transport/consumer"
)

const (
	leaderboardConsumerGroup         = "gamification-leaderboard-group"
	gamificationEventsTopic          = "gamification.events"
	leaderboardReconcileIntervalEnv  = "LEADERBOARD_RECONCILE_INTERVAL"
	defaultLeaderboardReconcileEvery = 15 * time.Minute
	leaderboardRebuildBatchSize      = 1000
	leaderboardRedisFailureCooldown  = 30 * time.Second
	leaderboardRebuildRunTimeout     = 10 * time.Minute
	leaderboardProfileCacheTTL       = 24 * time.Hour
)

// leaderboardReadModel groups the leaderboard read path and, when Redis and Kafka are both
// available, the components that keep the Redis read model in sync (leaderboards ADR-0003).
type leaderboardReadModel struct {
	repository repository.LeaderboardRepository
	// snapshots is the snapshot repository every writer and reader must share: the read-through
	// profile cache in Redis mode (so renames reach the cache), PostgreSQL otherwise.
	snapshots repository.UserSnapshotRepository
	projector *command.ProjectLeaderboardScoreHandler // nil disables post-commit projection
	consumer  *consumer.LeaderboardXpEarnedConsumer
	worker    *worker.LeaderboardReconcileWorker
}

// newLeaderboardReadModel serves straight from PostgreSQL unless redisClient is set and
// openReader returns a reader. openReader is only called once Redis is available, because opening
// a group reader joins the consumer group and an idle member would hold partitions unconsumed.
func newLeaderboardReadModel(
	gormRepo *persistence.GormLeaderboardRepository,
	snapshots repository.UserSnapshotRepository,
	redisClient *goredis.Client,
	openReader func() *kafka.Reader,
	reconcileInterval time.Duration,
) *leaderboardReadModel {
	postgresOnly := &leaderboardReadModel{repository: gormRepo, snapshots: snapshots}
	if redisClient == nil {
		return postgresOnly
	}
	reader := openReader()
	if reader == nil {
		return postgresOnly
	}

	projection := lbredis.NewLeaderboardProjection(redisClient, uuid.NewString())
	projector := command.NewProjectLeaderboardScoreHandler(projection)
	refresher := command.NewRefreshLeaderboardScoreHandler(gormRepo, projection)
	rebuilder := command.NewRebuildLeaderboardHandler(
		gormRepo,
		projection,
		leaderboardRebuildBatchSize,
	)
	reconcileWorker := worker.NewLeaderboardReconcileWorker(
		rebuilder, reconcileInterval, leaderboardRebuildRunTimeout)
	profiles := lbredis.NewCachedUserSnapshotRepository(
		redisClient, snapshots, leaderboardProfileCacheTTL)
	redisRepo := lbredis.NewLeaderboardRepository(redisClient, projection, profiles, gormRepo)

	slog.Info("Leaderboard read model: redis",
		slog.String("module", "gamification"),
		slog.String("consumer_group", leaderboardConsumerGroup),
		slog.Duration("reconcile_interval", reconcileInterval))

	return &leaderboardReadModel{
		repository: lbredis.NewFallbackLeaderboardRepository(
			redisRepo,
			gormRepo,
			reconcileWorker.RequestRebuild,
			leaderboardRedisFailureCooldown,
		),
		snapshots: profiles,
		projector: projector,
		consumer:  consumer.NewLeaderboardXpEarnedConsumer(reader, refresher),
		worker:    reconcileWorker,
	}
}

// start launches the sync components (no-op when the read model is PostgreSQL only).
func (m *leaderboardReadModel) start(ctx context.Context, wg *sync.WaitGroup) {
	if m.consumer != nil {
		wg.Go(func() { m.consumer.Start(ctx) })
	}
	if m.worker != nil {
		wg.Go(func() { m.worker.Start(ctx) })
	}
}

// leaderboardReconcileInterval reads LEADERBOARD_RECONCILE_INTERVAL (Go duration), defaulting to
// 15m.
func leaderboardReconcileInterval() time.Duration {
	raw := os.Getenv(leaderboardReconcileIntervalEnv)
	if raw == "" {
		return defaultLeaderboardReconcileEvery
	}
	interval, err := time.ParseDuration(raw)
	if err != nil || interval <= 0 {
		slog.Warn("Invalid leaderboard reconcile interval, using default",
			slog.String("module", "gamification"),
			slog.String("value", raw),
			slog.Duration("default", defaultLeaderboardReconcileEvery))
		return defaultLeaderboardReconcileEvery
	}
	return interval
}

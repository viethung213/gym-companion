//go:build integration

package test_test

import (
	"context"
	"errors"
	"os"
	"testing"

	goredis "github.com/redis/go-redis/v9"
	"github.com/viethung213/gym-companion/internal/gamification/domain/repository"
	"github.com/viethung213/gym-companion/internal/gamification/infrastructure/persistence"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// parityTestRedisDB keeps the test away from the application's leaderboard key (DB 0).
const parityTestRedisDB = 15

var errRollbackParity = errors.New("rollback parity test transaction")

// TestLeaderboardParity_Postgres compares the Redis read model with real PostgreSQL ordering
// (uuid and timestamptz comparison). It runs inside a rolled-back transaction, so existing rows take
// part in the comparison but nothing is persisted.
//
//	TEST_DATABASE_URL=postgres://... [TEST_REDIS_ADDR=localhost:6379] go test -tags integration ./internal/gamification/test/ -run Parity
func TestLeaderboardParity_Postgres(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	db, err := gorm.Open(postgres.Open(databaseURL), &gorm.Config{
		Logger:                 logger.Default.LogMode(logger.Silent),
		SkipDefaultTransaction: true,
	})
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}

	seeds := parityDataset()
	txManager := persistence.NewSQLTransactionManager(db)
	err = txManager.WithTransaction(context.Background(), func(txCtx context.Context) error {
		seedParityData(txCtx, t, db, seeds)

		var redisRepo repository.LeaderboardRepository
		if addr := os.Getenv("TEST_REDIS_ADDR"); addr != "" {
			client := goredis.NewClient(&goredis.Options{Addr: addr, DB: parityTestRedisDB})
			t.Cleanup(func() {
				_ = client.FlushDB(context.Background()).Err()
				_ = client.Close()
			})
			redisRepo = buildRedisLeaderboardWithClient(txCtx, t, db, client)
		} else {
			redisRepo = buildRedisLeaderboard(txCtx, t, db)
		}

		assertLeaderboardParity(txCtx, t, persistence.NewGormLeaderboardRepository(db), redisRepo, seedIDs(seeds))
		return errRollbackParity
	})
	if !errors.Is(err, errRollbackParity) {
		t.Fatalf("parity transaction: %v", err)
	}
}

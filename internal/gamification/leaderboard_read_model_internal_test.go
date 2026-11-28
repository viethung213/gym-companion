package gamification

import (
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"
	"github.com/segmentio/kafka-go"
	"github.com/viethung213/gym-companion/internal/gamification/infrastructure/persistence"
	lbredis "github.com/viethung213/gym-companion/internal/gamification/infrastructure/redis"
	"gorm.io/gorm"
)

func testLeaderboardDeps(t *testing.T) (*persistence.GormLeaderboardRepository, *persistence.GormUserSnapshotRepository) {
	t.Helper()
	db := &gorm.DB{Config: &gorm.Config{}}
	return persistence.NewGormLeaderboardRepository(db), persistence.NewGormUserSnapshotRepository(db)
}

func readerFactory(reader *kafka.Reader, opened *int) func() *kafka.Reader {
	return func() *kafka.Reader {
		*opened++
		return reader
	}
}

func TestLeaderboardReadModel_DisabledWithoutRedisOrKafka(t *testing.T) {
	gormRepo, snapshots := testLeaderboardDeps(t)
	reader := kafka.NewReader(kafka.ReaderConfig{Brokers: []string{"localhost:1"}, GroupID: "g", Topic: "t"})
	t.Cleanup(func() { _ = reader.Close() })
	client := goredis.NewClient(&goredis.Options{Addr: miniredis.RunT(t).Addr()})
	t.Cleanup(func() { _ = client.Close() })

	tests := []struct {
		name       string
		client     *goredis.Client
		reader     *kafka.Reader
		wantOpened int
	}{
		// Opening a group reader joins the consumer group, so it must not happen without Redis.
		{name: "no redis", reader: reader, wantOpened: 0},
		{name: "no kafka", client: client, wantOpened: 1},
		{name: "none", wantOpened: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opened := 0
			m := newLeaderboardReadModel(
				gormRepo, snapshots, tt.client, readerFactory(tt.reader, &opened), time.Minute)
			if opened != tt.wantOpened {
				t.Errorf("readers opened = %d, want %d", opened, tt.wantOpened)
			}
			if m.repository != gormRepo {
				t.Errorf("repository = %T, want the Gorm repository", m.repository)
			}
			if m.snapshots != snapshots {
				t.Errorf("snapshots = %T, want the PostgreSQL snapshot repository", m.snapshots)
			}
			if m.projector != nil || m.consumer != nil || m.worker != nil {
				t.Errorf("redis components wired while disabled: %+v", m)
			}
		})
	}
}

func TestLeaderboardReadModel_EnabledWithRedisAndKafka(t *testing.T) {
	gormRepo, snapshots := testLeaderboardDeps(t)
	reader := kafka.NewReader(kafka.ReaderConfig{Brokers: []string{"localhost:1"}, GroupID: "g", Topic: "t"})
	t.Cleanup(func() { _ = reader.Close() })
	client := goredis.NewClient(&goredis.Options{Addr: miniredis.RunT(t).Addr()})
	t.Cleanup(func() { _ = client.Close() })

	opened := 0
	m := newLeaderboardReadModel(
		gormRepo, snapshots, client, readerFactory(reader, &opened), time.Minute)
	if _, ok := m.repository.(*lbredis.FallbackLeaderboardRepository); !ok {
		t.Errorf("repository = %T, want *FallbackLeaderboardRepository", m.repository)
	}
	if _, ok := m.snapshots.(*lbredis.CachedUserSnapshotRepository); !ok {
		t.Errorf("snapshots = %T, want *CachedUserSnapshotRepository", m.snapshots)
	}
	if m.projector == nil || m.consumer == nil || m.worker == nil {
		t.Errorf("redis components missing: %+v", m)
	}
}

func TestLeaderboardReconcileInterval(t *testing.T) {
	tests := []struct {
		give string
		want time.Duration
	}{
		{give: "", want: 15 * time.Minute},
		{give: "30s", want: 30 * time.Second},
		{give: "2m", want: 2 * time.Minute},
		{give: "garbage", want: 15 * time.Minute},
		{give: "-5m", want: 15 * time.Minute},
		{give: "0", want: 15 * time.Minute},
	}
	for _, tt := range tests {
		t.Run("value="+tt.give, func(t *testing.T) {
			t.Setenv("LEADERBOARD_RECONCILE_INTERVAL", tt.give)
			if got := leaderboardReconcileInterval(); got != tt.want {
				t.Errorf("leaderboardReconcileInterval(%q) = %v, want %v", tt.give, got, tt.want)
			}
		})
	}
}

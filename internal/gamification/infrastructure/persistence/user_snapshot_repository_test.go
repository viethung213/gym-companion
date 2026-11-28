package persistence_test

import (
	"context"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/gamification/domain/entity"
	"github.com/viethung213/gym-companion/internal/gamification/test/testutil"
)

func TestGormUserSnapshotRepository_SaveAndGet(t *testing.T) {
	_, repos := testutil.NewTestDB(t)
	ctx := context.Background()
	now := time.Now().UTC()

	if err := repos.UserSnapshotRepo.SaveSnapshot(ctx, entity.NewUserSnapshot("u-1", "First", "a.png", now)); err != nil {
		t.Fatalf("SaveSnapshot: %v", err)
	}
	if err := repos.UserSnapshotRepo.SaveSnapshot(ctx, entity.NewUserSnapshot("u-1", "Renamed", "b.png", now)); err != nil {
		t.Fatalf("SaveSnapshot upsert: %v", err)
	}

	got, err := repos.UserSnapshotRepo.GetSnapshot(ctx, "u-1")
	if err != nil || got == nil || got.DisplayName != "Renamed" || got.AvatarURL != "b.png" {
		t.Fatalf("GetSnapshot = %+v, %v; want Renamed/b.png", got, err)
	}

	missing, err := repos.UserSnapshotRepo.GetSnapshot(ctx, "nobody")
	if err != nil || missing != nil {
		t.Errorf("GetSnapshot(nobody) = %+v, %v; want nil, nil", missing, err)
	}
}

func TestGormUserSnapshotRepository_SaveNil(t *testing.T) {
	_, repos := testutil.NewTestDB(t)
	if err := repos.UserSnapshotRepo.SaveSnapshot(context.Background(), nil); err == nil {
		t.Fatal("SaveSnapshot(nil) error = nil, want error")
	}
}

func TestGormUserSnapshotRepository_GetSnapshots(t *testing.T) {
	_, repos := testutil.NewTestDB(t)
	ctx := context.Background()
	now := time.Now().UTC()

	for _, id := range []string{"u-1", "u-2", "u-3"} {
		if err := repos.UserSnapshotRepo.SaveSnapshot(ctx, entity.NewUserSnapshot(id, "Name "+id, "", now)); err != nil {
			t.Fatalf("SaveSnapshot(%s): %v", id, err)
		}
	}

	got, err := repos.UserSnapshotRepo.GetSnapshots(ctx, []string{"u-1", "u-3", "unknown"})
	if err != nil {
		t.Fatalf("GetSnapshots: %v", err)
	}
	if len(got) != 2 || got["u-1"] == nil || got["u-3"] == nil || got["u-1"].DisplayName != "Name u-1" {
		t.Errorf("GetSnapshots = %+v, want u-1 and u-3 only", got)
	}
	if _, ok := got["unknown"]; ok {
		t.Error("GetSnapshots contains unknown id")
	}

	empty, err := repos.UserSnapshotRepo.GetSnapshots(ctx, nil)
	if err != nil || empty == nil || len(empty) != 0 {
		t.Errorf("GetSnapshots(nil) = %v, %v; want empty non-nil map", empty, err)
	}
}

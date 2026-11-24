package seeder_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/glebarez/sqlite"
	exercisemsg "github.com/viethung213/gym-companion/internal/gen/go/contracts/supporting/exercise/v1/message"
	"github.com/viethung213/gym-companion/internal/shared/middleware"
	"github.com/viethung213/gym-companion/internal/shared/seeder"
	"gorm.io/gorm"
)

const (
	upstreamPath           = "data/upstream/free-exercise-db/exercises.json"
	upstreamExerciseCount  = 876
	upstreamWithoutImages  = 3
	aiMotionSlugCount      = 12
	aiMotionExerciseCount  = 16
	nullEquipmentExercises = 77
)

type mockExerciseRegistrar struct {
	mu        sync.Mutex
	calls     []*exercisemsg.CreateExerciseRequest
	adminPass bool
	errToRet  error
	db        *sql.DB // when set, created slugs are persisted like the real service
}

func (m *mockExerciseRegistrar) CreateExercise(ctx context.Context, req *exercisemsg.CreateExerciseRequest) (*exercisemsg.CreateExerciseResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if actor, err := middleware.RequireAdmin(ctx); err == nil && actor.IsAdmin() {
		m.adminPass = true
	}
	if m.errToRet != nil {
		return nil, m.errToRet
	}
	if m.db != nil {
		if _, err := m.db.ExecContext(ctx, `INSERT INTO exercise.exercises (slug) VALUES (?)`, req.GetSlug()); err != nil {
			return nil, err
		}
	}

	m.calls = append(m.calls, req)
	return &exercisemsg.CreateExerciseResponse{}, nil
}

type upstreamRecord struct {
	ID             string   `json:"id"`
	Equipment      *string  `json:"equipment"`
	Category       string   `json:"category"`
	Level          string   `json:"level"`
	PrimaryMuscles []string `json:"primaryMuscles"`
}

func loadUpstream(t *testing.T) []upstreamRecord {
	t.Helper()

	raw, err := os.ReadFile(upstreamPath)
	if err != nil {
		t.Fatalf("read upstream: %v", err)
	}
	var records []upstreamRecord
	if err := json.Unmarshal(raw, &records); err != nil {
		t.Fatalf("unmarshal upstream: %v", err)
	}
	return records
}

// sourceCode mirrors the documented code convention ("e-z curl bar" -> "EZ_CURL_BAR").
func sourceCode(v string) string {
	return strings.ToUpper(strings.ReplaceAll(strings.ReplaceAll(v, "-", ""), " ", "_"))
}

func TestSeedExercises_NilService(t *testing.T) {
	if err := seeder.SeedExercises(context.Background(), nil, nil); err == nil {
		t.Fatal("expected error when exerciseService is nil, got nil")
	}
}

func TestSeedExercises_ImportsWholeCatalog(t *testing.T) {
	mock := &mockExerciseRegistrar{}
	if err := seeder.SeedExercises(context.Background(), nil, mock); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !mock.adminPass {
		t.Error("expected admin authorization in context")
	}
	if got := len(mock.calls); got != upstreamExerciseCount {
		t.Fatalf("got %d exercises, want %d", got, upstreamExerciseCount)
	}

	drafts := 0
	for _, req := range mock.calls {
		switch req.GetStatus() {
		case exercisemsg.ExerciseStatus_EXERCISE_STATUS_DRAFT:
			drafts++
			if len(req.GetImages()) != 0 {
				t.Errorf("%s: DRAFT but has images", req.GetSlug())
			}
		case exercisemsg.ExerciseStatus_EXERCISE_STATUS_ACTIVE:
		default:
			t.Errorf("%s: unexpected status %v", req.GetSlug(), req.GetStatus())
		}
	}
	if drafts != upstreamWithoutImages {
		t.Errorf("got %d DRAFT exercises, want %d", drafts, upstreamWithoutImages)
	}
}

func TestSeedExercises_DistributionMatchesSource(t *testing.T) {
	mock := &mockExerciseRegistrar{}
	if err := seeder.SeedExercises(context.Background(), nil, mock); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := map[string]int{}
	nullEquipment := 0
	for _, r := range loadUpstream(t) {
		equipment := "body only"
		if r.Equipment == nil {
			nullEquipment++
		} else {
			equipment = *r.Equipment
		}
		want["equipment:"+sourceCode(equipment)]++
		want["category:"+sourceCode(r.Category)]++
		want["level:"+sourceCode(r.Level)]++
		want["primary:"+sourceCode(r.PrimaryMuscles[0])]++
	}
	if nullEquipment != nullEquipmentExercises {
		t.Fatalf("upstream null equipment = %d, want %d", nullEquipment, nullEquipmentExercises)
	}

	got := map[string]int{}
	for _, req := range mock.calls {
		got["equipment:"+strings.TrimPrefix(req.GetEquipment().String(), "EQUIPMENT_")]++
		got["category:"+strings.TrimPrefix(req.GetCategory().String(), "EXERCISE_CATEGORY_")]++
		got["level:"+strings.TrimPrefix(req.GetLevel().String(), "EXERCISE_LEVEL_")]++
		got["primary:"+strings.TrimPrefix(req.GetPrimaryMuscle().String(), "MUSCLE_GROUP_")]++
	}

	if len(got) != len(want) {
		t.Errorf("got %d distinct buckets, want %d", len(got), len(want))
	}
	for key, n := range want {
		if got[key] != n {
			t.Errorf("%s: got %d, want %d", key, got[key], n)
		}
	}
}

func TestSeedExercises_AIMotionSlugs(t *testing.T) {
	mock := &mockExerciseRegistrar{}
	if err := seeder.SeedExercises(context.Background(), nil, mock); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	bySlug := map[string][]string{}
	for _, req := range mock.calls {
		if req.GetMotionSlug() == "" {
			continue
		}
		if req.GetStatus() != exercisemsg.ExerciseStatus_EXERCISE_STATUS_ACTIVE {
			t.Errorf("%s: AI exercise is not ACTIVE", req.GetSlug())
		}
		bySlug[req.GetMotionSlug()] = append(bySlug[req.GetMotionSlug()], req.GetSlug())
	}

	if len(bySlug) != aiMotionSlugCount {
		t.Errorf("got %d motion slugs, want %d: %v", len(bySlug), aiMotionSlugCount, bySlug)
	}
	total := 0
	for _, exercises := range bySlug {
		total += len(exercises)
	}
	if total != aiMotionExerciseCount {
		t.Errorf("got %d AI exercises, want %d", total, aiMotionExerciseCount)
	}
	if got := bySlug["deadlift"]; len(got) != 1 || got[0] != "Barbell_Deadlift" {
		t.Errorf("deadlift covers %v, want only Barbell_Deadlift (Q2)", got)
	}
}

func TestSeedExercises_IdempotentBySlug(t *testing.T) {
	db := setupSlugDB(t)
	mock := &mockExerciseRegistrar{db: db}
	ctx := context.Background()

	if err := seeder.SeedExercises(ctx, db, mock); err != nil {
		t.Fatalf("first seed: %v", err)
	}
	first := len(mock.calls)

	if err := seeder.SeedExercises(ctx, db, mock); err != nil {
		t.Fatalf("second seed: %v", err)
	}

	if first != upstreamExerciseCount {
		t.Errorf("first seed created %d, want %d", first, upstreamExerciseCount)
	}
	if got := len(mock.calls) - first; got != 0 {
		t.Errorf("second seed created %d exercises, want 0", got)
	}
}

func TestSeedExercises_CreateFailuresAreReturned(t *testing.T) {
	errCreate := errors.New("boom")
	mock := &mockExerciseRegistrar{errToRet: errCreate}

	err := seeder.SeedExercises(context.Background(), nil, mock)
	if !errors.Is(err, errCreate) {
		t.Errorf("got error %v, want it to wrap %v", err, errCreate)
	}
}

func TestSeedExercises_ContextCancellation(t *testing.T) {
	mock := &mockExerciseRegistrar{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := seeder.SeedExercises(ctx, nil, mock); !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled error, got %v", err)
	}
}

func setupSlugDB(t *testing.T) *sql.DB {
	t.Helper()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	gormDB, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{SkipDefaultTransaction: true})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	db, err := gormDB.DB()
	if err != nil {
		t.Fatalf("sqlite handle: %v", err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })

	if _, err := db.Exec(`
		ATTACH DATABASE ':memory:' AS exercise;
		CREATE TABLE exercise.exercises (slug TEXT PRIMARY KEY);
	`); err != nil {
		t.Fatalf("create schema: %v", err)
	}

	return db
}

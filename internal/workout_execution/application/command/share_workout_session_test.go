package command_test

import (
	"context"
	"errors"
	"testing"

	"github.com/viethung213/gym-companion/internal/workout_execution/application/apperror"
	"github.com/viethung213/gym-companion/internal/workout_execution/application/command"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/derror"
	"github.com/viethung213/gym-companion/internal/workout_execution/domain/event"
)

func TestShareWorkoutSessionHandler_Handle(t *testing.T) {
	ctx := context.Background()

	t.Run("empty session ID returns ErrInvalidInput", func(t *testing.T) {
		h := command.NewShareWorkoutSessionHandler(nil, nil, nil, nil)
		_, err := h.Handle(ctx, command.ShareWorkoutSessionCommand{SessionID: ""})
		if !errors.Is(err, apperror.ErrInvalidInput) {
			t.Errorf("got err = %v, want ErrInvalidInput", err)
		}
	})

	t.Run("session not found returns error", func(t *testing.T) {
		sessionRepo := newMockSessionRepo(t)
		h := command.NewShareWorkoutSessionHandler(sessionRepo, nil, nil, nil)
		_, err := h.Handle(ctx, command.ShareWorkoutSessionCommand{SessionID: "s-missing"})
		if !errors.Is(err, derror.ErrWorkoutSessionNotFound) {
			t.Errorf("got err = %v, want ErrWorkoutSessionNotFound", err)
		}
	})

	t.Run("forbidden user returns ErrForbidden", func(t *testing.T) {
		sess, _ := aggregate.NewWorkoutSession("s-1", "user-owner", "p-1")
		_ = sess.Complete(false, false)
		sessionRepo := newMockSessionRepo(t)
		sessionRepo.session = sess

		h := command.NewShareWorkoutSessionHandler(sessionRepo, nil, nil, nil)
		_, err := h.Handle(ctx, command.ShareWorkoutSessionCommand{
			SessionID: "s-1",
			UserID:    "user-other",
		})
		if !errors.Is(err, derror.ErrForbidden) {
			t.Errorf("got err = %v, want ErrForbidden", err)
		}
	})

	t.Run("auto-resolves WorkoutTitle from motionRepo", func(t *testing.T) {
		sess, _ := aggregate.NewWorkoutSession("s-3", "u-1", "p-1")
		_ = sess.LogSet(aggregate.WorkoutSetLog{
			ID:         "set-1",
			SetNumber:  1,
			ExerciseID: "ex-bench-101",
			ActualReps: 10,
			Weight:     80.0,
		})
		_ = sess.Complete(false, false)
		sess.PopEvents()

		sessionRepo := newMockSessionRepo(t)
		sessionRepo.session = sess
		outbox := newMockOutboxWriter(t)
		tx := newMockTxManager(t)

		motionRepo := newMockMotionSpecRepo(t)
		motionSpec := aggregate.NewDraftMotionSpecification("ex-bench-101", "Barbell Bench Press", "det.onnx", "skel.onnx")
		_ = motionRepo.Save(ctx, motionSpec)

		h := command.NewShareWorkoutSessionHandler(sessionRepo, motionRepo, outbox, tx)
		res, err := h.Handle(ctx, command.ShareWorkoutSessionCommand{
			SessionID: "s-3",
			UserID:    "u-1",
			Caption:   "Hard workout!",
		})
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if !res.IsShared {
			t.Error("want IsShared = true")
		}
		if len(outbox.events) != 1 {
			t.Fatalf("want 1 event written, got %d", len(outbox.events))
		}
		ev, ok := outbox.events[0].(*event.WorkoutSessionShared)
		if !ok {
			t.Fatalf("expected *event.WorkoutSessionShared, got %T", outbox.events[0])
		}
		if ev.WorkoutTitle != "Barbell Bench Press" {
			t.Errorf("got WorkoutTitle = %q, want 'Barbell Bench Press'", ev.WorkoutTitle)
		}
	})

	t.Run("falls back to ExerciseID when not in motionRepo", func(t *testing.T) {
		sess, _ := aggregate.NewWorkoutSession("s-4", "u-1", "p-1")
		_ = sess.LogSet(aggregate.WorkoutSetLog{
			ID:         "set-1",
			SetNumber:  1,
			ExerciseID: "ex-custom-999",
			ActualReps: 5,
			Weight:     100.0,
		})
		_ = sess.Complete(false, false)
		sess.PopEvents()

		sessionRepo := newMockSessionRepo(t)
		sessionRepo.session = sess
		outbox := newMockOutboxWriter(t)
		tx := newMockTxManager(t)
		motionRepo := newMockMotionSpecRepo(t)

		h := command.NewShareWorkoutSessionHandler(sessionRepo, motionRepo, outbox, tx)
		_, err := h.Handle(ctx, command.ShareWorkoutSessionCommand{
			SessionID: "s-4",
			UserID:    "u-1",
		})
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		ev, ok := outbox.events[0].(*event.WorkoutSessionShared)
		if !ok {
			t.Fatalf("expected *event.WorkoutSessionShared, got %T", outbox.events[0])
		}
		if ev.WorkoutTitle != "ex-custom-999" {
			t.Errorf("got WorkoutTitle = %q, want 'ex-custom-999'", ev.WorkoutTitle)
		}
		if ev.ExerciseCount != 1 {
			t.Errorf("got ExerciseCount = %d, want 1", ev.ExerciseCount)
		}
	})

	t.Run("multiple exercises formats title as [first] + [X] bài khác and sets ExerciseCount", func(t *testing.T) {
		sess, _ := aggregate.NewWorkoutSession("s-multi", "u-1", "p-1")
		_ = sess.LogSet(aggregate.WorkoutSetLog{
			ID:         "set-1",
			SetNumber:  1,
			ExerciseID: "ex-bench-101",
			ActualReps: 10,
			Weight:     80.0,
		})
		_ = sess.LogSet(aggregate.WorkoutSetLog{
			ID:         "set-2",
			SetNumber:  2,
			ExerciseID: "ex-squat-202",
			ActualReps: 8,
			Weight:     100.0,
		})
		_ = sess.LogSet(aggregate.WorkoutSetLog{
			ID:         "set-3",
			SetNumber:  3,
			ExerciseID: "ex-deadlift-303",
			ActualReps: 5,
			Weight:     120.0,
		})
		_ = sess.Complete(false, false)
		sess.PopEvents()

		sessionRepo := newMockSessionRepo(t)
		sessionRepo.session = sess
		outbox := newMockOutboxWriter(t)
		tx := newMockTxManager(t)

		motionRepo := newMockMotionSpecRepo(t)
		motionSpec := aggregate.NewDraftMotionSpecification("ex-bench-101", "Barbell Bench Press", "det.onnx", "skel.onnx")
		_ = motionRepo.Save(ctx, motionSpec)

		h := command.NewShareWorkoutSessionHandler(sessionRepo, motionRepo, outbox, tx)
		res, err := h.Handle(ctx, command.ShareWorkoutSessionCommand{
			SessionID: "s-multi",
			UserID:    "u-1",
		})
		if err != nil {
			t.Fatalf("unexpected err: %v", err)
		}
		if !res.IsShared {
			t.Error("want IsShared = true")
		}
		if len(outbox.events) != 1 {
			t.Fatalf("want 1 event written, got %d", len(outbox.events))
		}
		ev, ok := outbox.events[0].(*event.WorkoutSessionShared)
		if !ok {
			t.Fatalf("expected *event.WorkoutSessionShared, got %T", outbox.events[0])
		}
		if ev.WorkoutTitle != "Barbell Bench Press + 2 bài khác" {
			t.Errorf("got WorkoutTitle = %q, want 'Barbell Bench Press + 2 bài khác'", ev.WorkoutTitle)
		}
		if ev.ExerciseCount != 3 {
			t.Errorf("got ExerciseCount = %d, want 3", ev.ExerciseCount)
		}
	})
}

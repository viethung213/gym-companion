package aggregate_test

import (
	"errors"
	"math"
	"testing"
	"time"

	"github.com/viethung213/gym-companion/internal/gamification/domain/aggregate"
	"github.com/viethung213/gym-companion/internal/gamification/domain/event"
	"github.com/viethung213/gym-companion/internal/gamification/domain/vo"
)

func TestNewUserWallet(t *testing.T) {
	if _, err := aggregate.NewUserWallet(""); !errors.Is(err, aggregate.ErrEmptyUserID) {
		t.Fatalf("NewUserWallet('') err = %v, want %v", err, aggregate.ErrEmptyUserID)
	}

	w, err := aggregate.NewUserWallet("user-1")
	if err != nil {
		t.Fatalf("NewUserWallet('user-1') unexpected err: %v", err)
	}
	if w.UserID() != "user-1" || w.Balance() != 0 {
		t.Fatalf("unexpected wallet state: id=%s balance=%d", w.UserID(), w.Balance())
	}
}

func TestUserWallet_AwardAndDailyCap(t *testing.T) {
	now := time.Date(2026, 10, 23, 10, 0, 0, 0, time.UTC)
	w, _ := aggregate.NewUserWallet("user-1")

	// Reject non-positive amount or Spend reason in Award
	if _, _, err := w.Award("e-1", 0, vo.ReasonEarnWorkout, nil, "k-1", "2026-10-23", now); !errors.Is(err, aggregate.ErrInvalidCoinAmount) {
		t.Fatalf("expected ErrInvalidCoinAmount, got %v", err)
	}
	if _, _, err := w.Award("e-1", 10, vo.ReasonSpendRedeem, nil, "k-1", "2026-10-23", now); !errors.Is(err, aggregate.ErrInvalidTransactionReason) {
		t.Fatalf("expected ErrInvalidTransactionReason, got %v", err)
	}

	// First workout reward on 2026-10-23 succeeds
	entry, ev, err := w.Award("e-1", vo.WorkoutCoinReward, vo.ReasonEarnWorkout, nil, "WORKOUT:user-1:2026-10-23", "2026-10-23", now)
	if err != nil {
		t.Fatalf("Award unexpected err: %v", err)
	}
	if w.Balance() != 10 || entry.BalanceAfter != 10 {
		t.Fatalf("expected balance 10, got %d", w.Balance())
	}
	if _, ok := ev.(*event.CoinsEarned); !ok {
		t.Fatalf("expected *event.CoinsEarned, got %T", ev)
	}

	// Second workout reward on same date hits O(1) Daily Cap
	if _, _, err := w.Award("e-2", vo.WorkoutCoinReward, vo.ReasonEarnWorkout, nil, "WORKOUT:user-1:2026-10-23-b", "2026-10-23", now); !errors.Is(err, aggregate.ErrDailyCapReached) {
		t.Fatalf("expected ErrDailyCapReached, got %v", err)
	}

	// Overflow protection
	richWallet := aggregate.ReconstituteUserWallet("user-1", math.MaxInt64-5, nil, nil, nil, now)
	if _, _, err := richWallet.Award("e-3", 10, vo.ReasonEarnBadge, nil, "BADGE:user-1:FIRST", "2026-10-23", now); !errors.Is(err, aggregate.ErrBalanceOverflow) {
		t.Fatalf("expected ErrBalanceOverflow, got %v", err)
	}
}

func TestUserWallet_SpendAndAdminAdjust(t *testing.T) {
	now := time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)
	w := aggregate.ReconstituteUserWallet("user-1", 50, nil, nil, nil, now)

	// Insufficient funds rejected
	if _, _, err := w.Spend("e-1", 60, vo.ReasonSpendRedeem, nil, "spend-1", nil, now); !errors.Is(err, aggregate.ErrInsufficientFunds) {
		t.Fatalf("expected ErrInsufficientFunds, got %v", err)
	}

	// Valid spend succeeds
	entry, ev, err := w.Spend("e-2", 30, vo.ReasonSpendRedeem, nil, "spend-2", nil, now)
	if err != nil {
		t.Fatalf("Spend unexpected err: %v", err)
	}
	if w.Balance() != 20 || entry.Amount != -30 || entry.BalanceAfter != 20 {
		t.Fatalf("unexpected state after spend: balance=%d entry=%+v", w.Balance(), entry)
	}
	if _, ok := ev.(*event.CoinsSpent); !ok {
		t.Fatalf("expected *event.CoinsSpent, got %T", ev)
	}

	// Admin negative adjustment exceeding balance rejected
	if _, _, err := w.AdminAdjust("e-3", -25, vo.ReasonAdminAdjust, "admin-1", "adj-1", "penalty", now); !errors.Is(err, aggregate.ErrInsufficientFunds) {
		t.Fatalf("expected ErrInsufficientFunds on admin negative adjust, got %v", err)
	}

	// Admin math.MinInt64 rejected
	if _, _, err := w.AdminAdjust("e-4", math.MinInt64, vo.ReasonAdminAdjust, "admin-1", "adj-2", "overflow", now); !errors.Is(err, aggregate.ErrInvalidCoinAmount) {
		t.Fatalf("expected ErrInvalidCoinAmount on math.MinInt64, got %v", err)
	}

	// Valid admin adjustment succeeds
	adjEntry, _, err := w.AdminAdjust("e-5", -15, vo.ReasonAdminAdjust, "admin-1", "adj-3", "reconcile", now)
	if err != nil {
		t.Fatalf("AdminAdjust unexpected err: %v", err)
	}
	if w.Balance() != 5 || adjEntry.BalanceAfter != 5 {
		t.Fatalf("expected balance 5 after admin adjust, got %d", w.Balance())
	}
}

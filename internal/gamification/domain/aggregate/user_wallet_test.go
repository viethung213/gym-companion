package aggregate_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/viethung213/gym-companion/internal/gamification/domain/aggregate"
)

func TestUserWallet_NewUserWallet(t *testing.T) {
	t.Run("creates valid wallet with 0 balance", func(t *testing.T) {
		w, err := aggregate.NewUserWallet("usr-test-1")
		require.NoError(t, err)
		assert.Equal(t, "usr-test-1", w.UserID())
		assert.Equal(t, int64(0), w.Balance())
	})

	t.Run("rejects empty user ID", func(t *testing.T) {
		_, err := aggregate.NewUserWallet("")
		assert.ErrorIs(t, err, aggregate.ErrEmptyUserID)
	})
}

func TestUserWallet_AdjustBalance(t *testing.T) {
	now := time.Now().UTC()

	t.Run("adds coins successfully", func(t *testing.T) {
		w, err := aggregate.NewUserWallet("usr-test-1")
		require.NoError(t, err)

		err = w.AdjustBalance(100, now)
		require.NoError(t, err)

		assert.Equal(t, int64(100), w.Balance())
	})

	t.Run("subtracts coins with sufficient balance", func(t *testing.T) {
		w := aggregate.ReconstituteUserWallet("usr-test-1", 150, now, now)

		err := w.AdjustBalance(-50, now)
		require.NoError(t, err)

		assert.Equal(t, int64(100), w.Balance())
	})

	t.Run("rejects subtraction exceeding available balance", func(t *testing.T) {
		w := aggregate.ReconstituteUserWallet("usr-test-1", 30, now, now)

		err := w.AdjustBalance(-50, now)
		assert.ErrorIs(t, err, aggregate.ErrInsufficientBalance)
		assert.Equal(t, int64(30), w.Balance(), "balance must remain unchanged after failed adjustment")
	})

	t.Run("rejects zero amount", func(t *testing.T) {
		w, _ := aggregate.NewUserWallet("usr-test-1")
		err := w.AdjustBalance(0, now)
		assert.ErrorIs(t, err, aggregate.ErrZeroCoinAmount)
	})

	t.Run("rejects amount exceeding upper safety limit", func(t *testing.T) {
		w, _ := aggregate.NewUserWallet("usr-test-1")
		err := w.AdjustBalance(2_000_000, now)
		assert.ErrorIs(t, err, aggregate.ErrCoinAmountOutOfRange)
	})
}

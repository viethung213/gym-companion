//go:build unit

package vo_test

import (
	"strings"
	"testing"
	_ "time/tzdata"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/viethung213/gym-companion/internal/profile/domain/derror"
	"github.com/viethung213/gym-companion/internal/profile/domain/vo"
)

func TestParseTimezone(t *testing.T) {
	t.Run("valid IANA names", func(t *testing.T) {
		for _, name := range []string{"Asia/Ho_Chi_Minh", "America/New_York", "UTC", " Europe/London "} {
			tz, err := vo.ParseTimezone(name)
			require.NoError(t, err, name)
			assert.Equal(t, strings.TrimSpace(name), tz.String())
		}
	})

	t.Run("empty defaults to UTC", func(t *testing.T) {
		tz, err := vo.ParseTimezone("  ")
		require.NoError(t, err)
		assert.Equal(t, vo.DefaultTimezone, tz)
	})

	t.Run("invalid names are rejected", func(t *testing.T) {
		for _, name := range []string{"Mars/Base", "Local", "GMT+7 Hanoi", strings.Repeat("A", 65)} {
			_, err := vo.ParseTimezone(name)
			require.ErrorIs(t, err, derror.ErrInvalidTimezone, name)
		}
	})

	t.Run("zero value renders as UTC", func(t *testing.T) {
		assert.Equal(t, "UTC", vo.Timezone("").String())
	})
}

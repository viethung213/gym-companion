package fcm_test

import (
	"context"
	"testing"

	"github.com/viethung213/gym-companion/internal/notification/infrastructure/config"
	"github.com/viethung213/gym-companion/internal/notification/infrastructure/fcm"
)

func TestFCMClientMock(t *testing.T) {
	t.Parallel()

	cfg := config.Config{
		FCMServerKey: "", // empty key triggers Mock push mode
	}

	client := fcm.NewClient(cfg)

	t.Run("empty tokens", func(t *testing.T) {
		t.Parallel()

		resp, err := client.SendPush(context.Background(), nil, "Title", "Body", nil)
		if err != nil {
			t.Fatalf("got unexpected error: %v", err)
		}
		if got, want := resp.SuccessCount, 0; got != want {
			t.Errorf("got SuccessCount %d, want %d", got, want)
		}
	})

	t.Run("uninitialized messaging client send push", func(t *testing.T) {
		t.Parallel()

		tokens := []string{"token-1", "token-2"}
		resp, err := client.SendPush(context.Background(), tokens, "Title", "Body", map[string]string{"foo": "bar"})
		if err != nil {
			t.Fatalf("got unexpected error: %v", err)
		}

		if got, want := resp.SuccessCount, 0; got != want {
			t.Errorf("got SuccessCount %d, want %d", got, want)
		}
		if got, want := resp.FailureCount, 2; got != want {
			t.Errorf("got FailureCount %d, want %d", got, want)
		}
		if resp.InvalidTokens != nil {
			t.Errorf("got InvalidTokens %v, want nil", resp.InvalidTokens)
		}
	})
}

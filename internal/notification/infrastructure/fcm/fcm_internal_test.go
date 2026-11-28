package fcm

import (
	"context"
	"encoding/json"
	"testing"

	firebase "firebase.google.com/go/v4"
	"github.com/viethung213/gym-companion/internal/notification/infrastructure/config"
	"google.golang.org/api/option"
)

func TestBuildServiceAccountJSON_QuotedKey(t *testing.T) {
	t.Parallel()

	rawKey := "\"-----BEGIN PRIVATE KEY-----\\nMIIEvAIBADANBgkqhkiG9w0BAQEFAASCBKYwggSiAgEAAoIBAQC3VILFRBPB4C2+\\nmi5S7MChzsyfVjAmtAtPPyrONUBS+i3M6QaUXYcNQ0zTs8xu4Bp50Kjf8EssOqMj\\nJj0TEH5fo3UQzGw6DPSLS0qxNMQP2od4UJSMGjzz/ZumvXelP7VD4s/7VjM27+tE\\nckUr6UPmY5DqosV+iaUN45QqgezjX3cigs1L2S9pjpdMV5u5kKXqopFGg0ypgsj+\\nxPzHgonj2wGkerdu1GuJS0rREcdMY5P/3XZOY20WXhAhufSpCnAYgGrpaAS05vi2\\naBxdc7hmm8i88M30S3KTI2LM2Egbhpg4hBtbhE4DvId5ArLFbwnOxDWtfc1jlITX\\nEa9GXT4jAgMBAAECggEAKBFg6WNZHBj2dn91G2uiKFstzq9EI9la1W2dWec6kUgb\\nhXrylK+iSixI2MYlqp/xOWl410TJ8Tc5m3+KfqWusRFlDxBWoRNeySjiALvjFAHz\\ndMDLzpkXdE4b3EPxp9y2LcmfIjPzSpECLvn4k0HOtMZTEG0V0/OabuAr7GRsFT9T\\ngjwAMlL9rLGuXJL2VlpgFaJucswEYvz8jlvKT8ddE9e5u1KsskRqSHHnDkChcWzh\\n0RUyb8jydF8ram8Uua08Z1Frs+8NO1ONCtJfD3sSFeLK6AxiVH8mg7X/3TLNYS4d\\nSkJMFCiWtg808uVurF0TujQfvHs7ZVqX//bZaQ2uQQKBgQDut4wDrKdpOMkNUSgt\\nNQmeS9rd/ASao6xiLPJb5tnf5JhucTl/I8yseAPbe8xoCOVzBmpuAjTjI7Z/Our4\\nIQME9NkuElme3toJYaApY3vvxFyoYLcMAvnfq5AUt4c1Ld8VO9Um4hsncUYoMfG8\\n1yu//9KV3/tUUW9/22S56TOzwQKBgQDEmmgw9w/1gVgvCADP8fnqsWvJ0w1VmAnm\\nrxJuV1k2okfdLvnV/DWkgtkUaWkECk2XUb2I1quygRteAdUIlPgyzbBH5+faMJRr\\n43OeZ8RDsDLMRZyGQkFxied5q9sPFgUe4A8GFrAQI6KUiVcnkSHtX+KoKlLDjc5i\\nUD49ORNa4wKBgBppXmltqdwfd0cs6ogN7TGBbQbQgmXlpAX8uRfn88Us0pytlFH3\\n0QZoBO08xkHgchqG5DGlttpGskNjbmnjpSTgPkM1rorAV1lgXssVgbnmfOX4RWhj\\nz+eqYLeaErCUkbEUvKGftKdphfyeggOFvwZwNIl2jaUnmXGJAZz8K0dBAoGATh91\\nK4/hPZMqMXYirH7g6kuaI1bEQ7Dorkv/LO10u+r8V33lNQzFXBvKmvvQCRT3BBwt\\nDoUPMR1wfIiJ2KEphzaIx+n1DZSkRcLaHKXrLWKErPs0WX1WeAFug8CwiWwcmMo7\\n7hVx3NfsMM3qXgSj0Kl7bYtz29Urn5F3Sei2MjUCgYAIzB4YnNVqFMKpqjROXqSQ\\nUtPBFKhJO7X2r4++gU7WQGq3WSGeh+m9+4DaAugutci7WRefKJoG+G/GsYVQLxby\\neGbdmZb7Gaxyasa6nv0jvezbNx9rJxtBCirMHI4idgeFA0t2Z6PJfJr+LX/HpL+3\\nzy9p8RPFgJmdlmCRxdZrxQ==\\n-----END PRIVATE KEY-----\\n\""

	cfg := &config.Config{
		FCMClientEmail: "firebase-adminsdk-fbsvc@gym-companion-502204.iam.gserviceaccount.com",
		FCMPrivateKey:  rawKey,
		FCMProjectID:   "gym-companion-502204",
	}

	data, err := buildServiceAccountJSON(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var sa serviceAccountJSON
	if err := json.Unmarshal(data, &sa); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if sa.PrivateKey[0] == '"' || sa.PrivateKey[len(sa.PrivateKey)-1] == '"' {
		t.Errorf("private key still has quotes: %s", sa.PrivateKey)
	}

	app, err := firebase.NewApp(context.Background(), nil, option.WithCredentialsJSON(data))
	if err != nil {
		t.Fatalf("failed to init firebase app with parsed credentials: %v", err)
	}

	msgClient, err := app.Messaging(context.Background())
	if err != nil {
		t.Fatalf("failed to init messaging client: %v", err)
	}
	if msgClient == nil {
		t.Fatal("expected non-nil messagingClient")
	}
}

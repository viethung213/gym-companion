package vo_test

import (
	"testing"

	"github.com/viethung213/gym-companion/internal/social/domain/vo"
)

func TestReactionType(t *testing.T) {
	tests := []struct {
		name    string
		give    string
		want    vo.ReactionType
		wantErr bool
	}{
		{name: "like", give: "LIKE", want: vo.ReactionTypeLike, wantErr: false},
		{name: "fire", give: "FIRE", want: vo.ReactionTypeFire, wantErr: false},
		{name: "muscle", give: "MUSCLE", want: vo.ReactionTypeMuscle, wantErr: false},
		{name: "clap", give: "CLAP", want: vo.ReactionTypeClap, wantErr: false},
		{name: "invalid", give: "INVALID", want: "", wantErr: true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			got, err := vo.NewReactionType(tt.give)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr = %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
			if !tt.wantErr && got.String() != string(tt.want) {
				t.Fatalf("got string %q, want %q", got.String(), string(tt.want))
			}
		})
	}
}

func TestReactionType_ValueAndScan(t *testing.T) {
	r := vo.ReactionTypeLike

	val, err := r.Value()
	if err != nil {
		t.Fatalf("unexpected error getting value: %v", err)
	}
	if val != "LIKE" {
		t.Fatalf("got %v, want %q", val, "LIKE")
	}

	var scanned vo.ReactionType
	if err := scanned.Scan(nil); err != nil {
		t.Fatalf("failed scanning nil: %v", err)
	}
	if scanned != "" {
		t.Fatalf("expected empty for nil scan, got %v", scanned)
	}

	if err := scanned.Scan("FIRE"); err != nil {
		t.Fatalf("failed scanning string: %v", err)
	}
	if scanned != vo.ReactionTypeFire {
		t.Fatalf("got %v, want FIRE", scanned)
	}

	if err := scanned.Scan([]byte("CLAP")); err != nil {
		t.Fatalf("failed scanning bytes: %v", err)
	}
	if scanned != vo.ReactionTypeClap {
		t.Fatalf("got %v, want CLAP", scanned)
	}

	if err := scanned.Scan(12345); err == nil {
		t.Fatalf("expected error scanning int, got nil")
	}
}

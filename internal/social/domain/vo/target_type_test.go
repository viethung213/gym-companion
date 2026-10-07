package vo_test

import (
	"testing"

	"github.com/viethung213/gym-companion/internal/social/domain/vo"
)

func TestTargetType(t *testing.T) {
	tests := []struct {
		name    string
		give    string
		want    vo.TargetType
		wantErr bool
	}{
		{name: "post target", give: "POST", want: vo.TargetTypePost, wantErr: false},
		{name: "activity target", give: "ACTIVITY", want: vo.TargetTypeActivity, wantErr: false},
		{name: "unknown target", give: "COMMENT", want: "", wantErr: true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			got, err := vo.NewTargetType(tt.give)
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

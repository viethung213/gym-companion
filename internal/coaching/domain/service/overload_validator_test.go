package service

import (
	"errors"
	"testing"
)

func TestOverloadValidator_Cap(t *testing.T) {
	v := NewOverloadValidator()
	tests := []struct {
		name      string
		suggested float64
		baseline  float64
		want      float64
	}{
		{"within band", 100, 100, 100},
		{"upper cap: 200 vs baseline 100 → 130", 200, 100, 130},
		{"lower cap: 40 vs baseline 100 → 70", 40, 100, 70},
		{"exactly at upper", 130, 100, 130},
		{"exactly at lower", 70, 100, 70},
		{"heavier baseline: 250 vs 200 → 250 (within +25%)", 250, 200, 250},
		{"cap at +30%: 300 vs 200 → 260", 300, 200, 260},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := v.Cap(tt.suggested, tt.baseline)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("Cap(%v, %v) = %v, want %v", tt.suggested, tt.baseline, got, tt.want)
			}
		})
	}
}

func TestOverloadValidator_Cap_NoBaseline(t *testing.T) {
	v := NewOverloadValidator()
	got, err := v.Cap(80, 0)
	if !errors.Is(err, ErrNoBaseline) {
		t.Errorf("expected ErrNoBaseline, got %v", err)
	}
	if got != 80 {
		t.Errorf("expected suggested returned as-is, got %v", got)
	}
}

func TestOverloadValidator_Within(t *testing.T) {
	v := NewOverloadValidator()
	if !v.Within(100, 100) {
		t.Errorf("100/100 should be within band")
	}
	if v.Within(200, 100) {
		t.Errorf("200/100 should NOT be within band")
	}
	if !v.Within(80, 0) {
		t.Errorf("no baseline should return true (permissive)")
	}
}

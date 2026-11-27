package service

import "testing"

func TestOverloadValidator_Ceiling(t *testing.T) {
	v := NewOverloadValidator()
	if got := v.Ceiling(100); got != 130 {
		t.Errorf("Ceiling(100) = %v, want 130", got)
	}
}

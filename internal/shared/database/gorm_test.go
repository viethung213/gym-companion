package database

import (
	"testing"
)

func TestOpenGorm_NilConnection(t *testing.T) {
	_, err := OpenGorm(nil, nil)
	if err == nil {
		t.Fatal("expected error when passing nil connection, got nil")
	}
}

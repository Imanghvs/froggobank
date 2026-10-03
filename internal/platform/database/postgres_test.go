package database

import "testing"

func TestNewPoolReturnsErrorForInvalidURL(t *testing.T) {
	_, err := NewPool(t.Context(), "invalid-url")
	if err == nil {
		t.Fatal("expected error")
	}
}

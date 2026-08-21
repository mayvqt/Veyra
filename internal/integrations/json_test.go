package integrations

import (
	"strings"
	"testing"
)

func TestDecodeJSON(t *testing.T) {
	t.Run("decodes one value", func(t *testing.T) {
		var got map[string]int
		if err := DecodeJSON(strings.NewReader(`{"count": 2}`), &got, 64); err != nil {
			t.Fatal(err)
		}
		if got["count"] != 2 {
			t.Fatalf("count = %d, want 2", got["count"])
		}
	})

	t.Run("rejects multiple values", func(t *testing.T) {
		var got map[string]int
		if err := DecodeJSON(strings.NewReader(`{"count": 2} {"count": 3}`), &got, 64); err == nil {
			t.Fatal("expected trailing JSON value to be rejected")
		}
	})

	t.Run("rejects oversized body", func(t *testing.T) {
		var got string
		if err := DecodeJSON(strings.NewReader(`"12345"`), &got, 6); err == nil {
			t.Fatal("expected oversized response to be rejected")
		}
	})
}

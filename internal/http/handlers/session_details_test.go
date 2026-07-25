package handlers

import (
	"testing"
	"time"
)

func TestFormatSessionTime(t *testing.T) {
	if got := formatSessionTime(time.Time{}); got != "-" {
		t.Fatalf("expected dash for zero time, got %s", got)
	}
	ts := time.Date(2026, 5, 14, 12, 30, 0, 0, time.FixedZone("NZ", 12*3600))
	if got := formatSessionTime(ts); got != "2026-05-14T00:30:00Z" {
		t.Fatalf("unexpected UTC format: %s", got)
	}
}

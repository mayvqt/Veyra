package testutil

import (
	"net/http"
	"testing"
	"time"
)

type RoundTripFunc func(*http.Request) (*http.Response, error)

func (f RoundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func WaitForSignals(t testing.TB, started <-chan string, timeout time.Duration, want ...string) {
	t.Helper()
	seen := make(map[string]bool, len(want))
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	for len(seen) < len(want) {
		select {
		case name := <-started:
			seen[name] = true
		case <-timer.C:
			t.Fatalf("expected concurrent requests for %v, saw %v", want, seen)
		}
	}
}

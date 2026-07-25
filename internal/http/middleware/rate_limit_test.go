package middleware

import (
	"testing"
	"time"
)

func TestLoginRateLimiter(t *testing.T) {
	rl := NewLoginRateLimiter(2, time.Minute)
	if !rl.Allow("1.2.3.4", "u") {
		t.Fatal("first attempt should pass")
	}
	if !rl.Allow("1.2.3.4", "u") {
		t.Fatal("second attempt should pass")
	}
	if rl.Allow("1.2.3.4", "u") {
		t.Fatal("third should be blocked")
	}
}

func TestLoginRateLimiterNormalizesUsername(t *testing.T) {
	rl := NewLoginRateLimiter(2, time.Minute)
	if !rl.Allow("1.2.3.4", "User") {
		t.Fatal("first attempt should pass")
	}
	if !rl.Allow("1.2.3.4", " user ") {
		t.Fatal("second normalized attempt should pass")
	}
	if rl.Allow("1.2.3.4", "USER") {
		t.Fatal("third normalized attempt should be blocked")
	}
}

func TestLoginRateLimiterPrunesExpiredEntries(t *testing.T) {
	rl := NewLoginRateLimiter(5, time.Minute)
	rl.cleanupEvery = 1
	rl.byIP["9.9.9.9"] = bucket{count: 3, reset: time.Now().Add(-time.Minute)}
	rl.byUser["old"] = bucket{count: 3, reset: time.Now().Add(-time.Minute)}

	if !rl.Allow("1.1.1.1", "new") {
		t.Fatal("expected allow for fresh key")
	}
	if _, ok := rl.byIP["9.9.9.9"]; ok {
		t.Fatal("expected expired ip bucket to be pruned")
	}
	if _, ok := rl.byUser["old"]; ok {
		t.Fatal("expected expired username bucket to be pruned")
	}
}

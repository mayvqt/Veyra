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

func TestLoginRateLimiterBoundsHighCardinalityState(t *testing.T) {
	rl := NewLoginRateLimiter(5, time.Minute)
	rl.maxEntries = 2
	if !rl.Allow("1.1.1.1", "one") || !rl.Allow("2.2.2.2", "two") {
		t.Fatal("expected entries within the cap to be allowed")
	}
	if rl.Allow("3.3.3.3", "three") {
		t.Fatal("expected a new key beyond the cap to be rejected")
	}
	if len(rl.byIP) != 2 || len(rl.byUser) != 2 {
		t.Fatalf("limiter state grew beyond cap: ips=%d users=%d", len(rl.byIP), len(rl.byUser))
	}
}

func TestLoginRateLimiterBlockedUsernameDoesNotConsumeIPAddressAllowance(t *testing.T) {
	rl := NewLoginRateLimiter(2, time.Minute)
	if !rl.Allow("1.1.1.1", "blocked") || !rl.Allow("2.2.2.2", "blocked") {
		t.Fatal("expected initial username attempts to pass")
	}
	if rl.Allow("3.3.3.3", "blocked") {
		t.Fatal("expected username limit to block the attempt")
	}
	if !rl.Allow("3.3.3.3", "other") || !rl.Allow("3.3.3.3", "other") {
		t.Fatal("blocked username attempt consumed IP allowance")
	}
}

func TestLoginRateLimiterRejectsInvalidConfiguration(t *testing.T) {
	for _, rl := range []*LoginRateLimiter{
		NewLoginRateLimiter(0, time.Minute),
		NewLoginRateLimiter(2, 0),
	} {
		if rl.Allow("1.1.1.1", "user") {
			t.Fatal("invalid limiter configuration should fail closed")
		}
	}
}

package middleware

import (
	"strings"
	"sync"
	"time"
)

const maxLoginRateLimiterEntries = 10_000

type bucket struct {
	count int
	reset time.Time
}

type LoginRateLimiter struct {
	mu           sync.Mutex
	window       time.Duration
	max          int
	byIP         map[string]bucket
	byUser       map[string]bucket
	calls        uint64
	cleanupEvery uint64
	maxEntries   int
}

func NewLoginRateLimiter(max int, window time.Duration) *LoginRateLimiter {
	return &LoginRateLimiter{
		max:          max,
		window:       window,
		byIP:         map[string]bucket{},
		byUser:       map[string]bucket{},
		cleanupEvery: 256,
		maxEntries:   maxLoginRateLimiterEntries,
	}
}

func (l *LoginRateLimiter) Allow(ip, username string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.max <= 0 || l.window <= 0 || l.maxEntries <= 0 {
		return false
	}
	now := time.Now()
	l.calls++
	if l.cleanupEvery > 0 && l.calls%l.cleanupEvery == 0 {
		pruneExpiredBuckets(l.byIP, now)
		pruneExpiredBuckets(l.byUser, now)
	}
	username = strings.ToLower(strings.TrimSpace(username))
	if username == "" {
		username = "_"
	}
	if _, exists := l.byIP[ip]; !exists && len(l.byIP) >= l.maxEntries {
		return false
	}
	if _, exists := l.byUser[username]; !exists && len(l.byUser) >= l.maxEntries {
		return false
	}
	// Check both dimensions before recording either one. A blocked username must
	// not consume the shared IP allowance and lock out unrelated users.
	if !keyAllowed(l.byIP, ip, now, l.max) || !keyAllowed(l.byUser, username, now, l.max) {
		return false
	}
	recordKey(l.byIP, ip, now, l.window)
	recordKey(l.byUser, username, now, l.window)
	return true
}

func keyAllowed(m map[string]bucket, key string, now time.Time, max int) bool {
	b, ok := m[key]
	if !ok || !now.Before(b.reset) {
		return true
	}
	return b.count < max

}

func recordKey(m map[string]bucket, key string, now time.Time, window time.Duration) {
	b, ok := m[key]
	if !ok || !now.Before(b.reset) {
		m[key] = bucket{count: 1, reset: now.Add(window)}
		return
	}
	b.count++
	m[key] = b
}

func pruneExpiredBuckets(m map[string]bucket, now time.Time) {
	for k, b := range m {
		if !now.Before(b.reset) {
			delete(m, k)
		}
	}
}

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
	if !allowKey(l.byIP, ip, now, l.window, l.max) {
		return false
	}
	if !allowKey(l.byUser, username, now, l.window, l.max) {
		return false
	}
	return true
}

func allowKey(m map[string]bucket, key string, now time.Time, window time.Duration, max int) bool {
	b, ok := m[key]
	if !ok || now.After(b.reset) {
		m[key] = bucket{count: 1, reset: now.Add(window)}
		return true
	}
	if b.count >= max {
		return false
	}
	b.count++
	m[key] = b
	return true
}

func pruneExpiredBuckets(m map[string]bucket, now time.Time) {
	for k, b := range m {
		if now.After(b.reset) {
			delete(m, k)
		}
	}
}

// Package ratelimit provides a small, dependency-free keyed token-bucket limiter.
//
// It exists for the heartbeat ping endpoint: unauthenticated and O(1), so a leaked
// ping URL must not be usable to hammer the database. A genuine heartbeat pings
// every 1–60 minutes; anything faster is abuse, and this caps it per token.
package ratelimit

import (
	"sync"
	"time"
)

type bucket struct {
	tokens float64
	last   time.Time
}

// KeyedLimiter is a token-bucket rate limiter keyed by an arbitrary string. Safe
// for concurrent use.
type KeyedLimiter struct {
	mu      sync.Mutex
	buckets map[string]*bucket
	rate    float64 // tokens added per second
	burst   float64 // bucket capacity
	max     int     // soft cap on tracked keys before an eviction sweep
	now     func() time.Time
}

// New builds a limiter allowing `ratePerSec` sustained requests per key with a
// `burst` allowance. `maxKeys` bounds memory; when exceeded, idle (full) buckets
// are swept.
func New(ratePerSec float64, burst int, maxKeys int) *KeyedLimiter {
	if maxKeys <= 0 {
		maxKeys = 10000
	}
	return &KeyedLimiter{
		buckets: make(map[string]*bucket),
		rate:    ratePerSec,
		burst:   float64(burst),
		max:     maxKeys,
		now:     time.Now,
	}
}

// Allow reports whether a request for key may proceed, consuming one token if so.
func (l *KeyedLimiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	b, ok := l.buckets[key]
	if !ok {
		if len(l.buckets) >= l.max {
			l.evict(now)
		}
		// A brand-new key starts full, then immediately spends one token below.
		b = &bucket{tokens: l.burst, last: now}
		l.buckets[key] = b
	} else {
		// Refill by elapsed time, capped at burst.
		b.tokens += now.Sub(b.last).Seconds() * l.rate
		if b.tokens > l.burst {
			b.tokens = l.burst
		}
		b.last = now
	}

	if b.tokens >= 1 {
		b.tokens--
		return true
	}
	return false
}

// evict drops buckets that have fully refilled. A full bucket is behaviourally
// identical to a never-seen key, so removing it changes nothing except memory.
// Called under lock.
func (l *KeyedLimiter) evict(now time.Time) {
	for k, b := range l.buckets {
		refilled := b.tokens + now.Sub(b.last).Seconds()*l.rate
		if refilled >= l.burst {
			delete(l.buckets, k)
		}
	}
}

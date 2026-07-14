package ratelimit

import (
	"testing"
	"time"
)

// withClock lets the tests advance time deterministically instead of sleeping.
func (l *KeyedLimiter) withClock(now func() time.Time) *KeyedLimiter {
	l.now = now
	return l
}

func TestKeyedLimiter_BurstThenRefill(t *testing.T) {
	cur := time.Unix(0, 0)
	l := New(1, 3, 100).withClock(func() time.Time { return cur }) // 1/s, burst 3

	// Burst of 3 allowed immediately.
	for i := 0; i < 3; i++ {
		if !l.Allow("k") {
			t.Fatalf("request %d should be allowed within burst", i+1)
		}
	}
	// 4th is refused — bucket empty.
	if l.Allow("k") {
		t.Fatal("4th request should be refused (burst exhausted)")
	}
	// After 1s, exactly one token refills.
	cur = cur.Add(time.Second)
	if !l.Allow("k") {
		t.Fatal("request after 1s refill should be allowed")
	}
	if l.Allow("k") {
		t.Fatal("only one token should have refilled")
	}
}

func TestKeyedLimiter_KeysAreIndependent(t *testing.T) {
	cur := time.Unix(0, 0)
	l := New(1, 1, 100).withClock(func() time.Time { return cur })

	if !l.Allow("a") || !l.Allow("b") {
		t.Fatal("distinct keys must not share a bucket")
	}
	if l.Allow("a") {
		t.Fatal("key a should now be limited")
	}
}

func TestKeyedLimiter_EvictsIdleBuckets(t *testing.T) {
	cur := time.Unix(0, 0)
	// max=2 so a third distinct key triggers an eviction sweep.
	l := New(1, 1, 2).withClock(func() time.Time { return cur })

	l.Allow("a")
	l.Allow("b")
	// Let both refill fully so they are eligible for eviction.
	cur = cur.Add(10 * time.Second)
	l.Allow("c") // triggers evict; a and b are full -> dropped

	if got := len(l.buckets); got > 2 {
		t.Errorf("bucket map grew unbounded: %d entries", got)
	}
}

package middleware

import (
	"math"
	"testing"
	"time"

	"golang.org/x/time/rate"
)

func TestEvictAfter(t *testing.T) {
	tests := []struct {
		name  string
		r     rate.Limit
		burst int
		want  time.Duration
	}{
		{"forgot-password: burst 3, 1 per 10 min", rate.Every(10 * time.Minute), 3, 30 * time.Minute},
		{"verify-email: burst 3, 1 per min", rate.Every(time.Minute), 3, 3 * time.Minute},
		{"login: burst 3, 5 per min", rate.Limit(5.0 / 60.0), 3, 3 * time.Minute},
		{"unlimited rate keeps the minimum", rate.Inf, 3, 3 * time.Minute},
		{"a bucket that never refills is never evicted", 0, 3, time.Duration(math.MaxInt64)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := evictAfter(tt.r, tt.burst); got != tt.want {
				t.Errorf("evictAfter() = %v, want %v", got, tt.want)
			}
		})
	}
}

// A visitor must stay in the store until its bucket would have refilled
// completely; dropping it earlier hands back tokens the configured rate has
// not yet earned.
func TestEvictIdle_KeepsBucketUntilFullyRefilled(t *testing.T) {
	store := newRateLimiterStore(rate.Every(10*time.Minute), 3)
	const ip = "203.0.113.9"

	limiter := store.getLimiter(ip)
	for i := 0; i < 3; i++ {
		if !limiter.Allow() {
			t.Fatalf("token %d should be allowed within the burst", i+1)
		}
	}
	if limiter.Allow() {
		t.Fatal("4th token should be denied")
	}

	seen := store.visitors[ip].lastSeen

	store.evictIdle(seen.Add(4 * time.Minute))
	if _, ok := store.visitors[ip]; !ok {
		t.Fatal("exhausted bucket was evicted after 4 min idle, long before one token refills (10 min)")
	}
	if store.getLimiter(ip).Allow() {
		t.Error("exhausted bucket should still deny after 4 min idle")
	}

	// getLimiter above refreshed lastSeen; measure from the new value.
	seen = store.visitors[ip].lastSeen

	store.evictIdle(seen.Add(29 * time.Minute))
	if _, ok := store.visitors[ip]; !ok {
		t.Error("bucket evicted at 29 min idle, before it could have fully refilled (30 min)")
	}

	store.evictIdle(seen.Add(31 * time.Minute))
	if _, ok := store.visitors[ip]; ok {
		t.Error("fully refilled idle bucket should be evicted so the map stays bounded")
	}
}

func TestEvictIdle_DropsShortRefillBucketsAfterMinimumAge(t *testing.T) {
	store := newRateLimiterStore(rate.Every(time.Minute), 3)
	const ip = "198.51.100.4"

	store.getLimiter(ip)
	seen := store.visitors[ip].lastSeen

	store.evictIdle(seen.Add(2 * time.Minute))
	if _, ok := store.visitors[ip]; !ok {
		t.Error("evicted before the 3 min minimum age")
	}
	store.evictIdle(seen.Add(4 * time.Minute))
	if _, ok := store.visitors[ip]; ok {
		t.Error("idle bucket should be evicted after the 3 min minimum age")
	}
}

package middleware

import (
	"fmt"
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

// testCap is deliberately tiny so the tests can fill the store by hand.
const testCap = 3

func newCappedStore() *rateLimiterStore {
	store := newRateLimiterStore(rate.Every(10*time.Minute), 3)
	store.maxVisitors = testCap
	return store
}

func TestStore_DefaultVisitorCap(t *testing.T) {
	if defaultMaxVisitors != 50_000 {
		t.Errorf("defaultMaxVisitors = %d, want 50000", defaultMaxVisitors)
	}
	store := newRateLimiterStore(rate.Every(time.Minute), 3)
	if store.maxVisitors != defaultMaxVisitors {
		t.Errorf("new store maxVisitors = %d, want %d", store.maxVisitors, defaultMaxVisitors)
	}
}

// Keys an attacker controls (rotating X-Forwarded-For) must not grow the map
// without bound.
func TestStore_NeverExceedsVisitorCap(t *testing.T) {
	store := newCappedStore()

	for i := 0; i < 50; i++ {
		store.getLimiter(fmt.Sprintf("10.0.0.%d", i))
		if n := len(store.visitors); n > testCap {
			t.Fatalf("after %d distinct keys the store holds %d visitors, cap is %d", i+1, n, testCap)
		}
	}
}

func TestStore_FullStoreDropsExpiredEntriesFirst(t *testing.T) {
	store := newCappedStore()
	for _, ip := range []string{"a", "b", "c"} {
		store.getLimiter(ip)
	}
	expired := time.Now().Add(-2 * store.evictAfter)
	store.visitors["b"].lastSeen = expired
	store.visitors["c"].lastSeen = expired

	store.getLimiter("d")

	for _, ip := range []string{"a", "d"} {
		if _, ok := store.visitors[ip]; !ok {
			t.Errorf("%q should have been kept", ip)
		}
	}
	for _, ip := range []string{"b", "c"} {
		if _, ok := store.visitors[ip]; ok {
			t.Errorf("expired %q should have been dropped before any live entry", ip)
		}
	}
}

func TestStore_FullStoreWithNoExpiredEntriesEvictsExactlyOne(t *testing.T) {
	store := newCappedStore()
	for _, ip := range []string{"a", "b", "c"} {
		store.getLimiter(ip)
	}

	store.getLimiter("d")

	if n := len(store.visitors); n != 3 {
		t.Fatalf("store holds %d visitors, want 3", n)
	}
	if _, ok := store.visitors["d"]; !ok {
		t.Error("the new visitor must be kept")
	}
	kept := 0
	for _, ip := range []string{"a", "b", "c"} {
		if _, ok := store.visitors[ip]; ok {
			kept++
		}
	}
	if kept != 2 {
		t.Errorf("%d of the 3 existing visitors survived, want exactly 2", kept)
	}
}

func TestStore_KnownVisitorIsNeverEvictedToMakeRoomForItself(t *testing.T) {
	store := newCappedStore()
	var limiters []*rate.Limiter
	for _, ip := range []string{"a", "b", "c"} {
		limiters = append(limiters, store.getLimiter(ip))
	}

	if got := store.getLimiter("b"); got != limiters[1] {
		t.Error("an existing visitor must get its own limiter back")
	}
	if n := len(store.visitors); n != 3 {
		t.Errorf("store holds %d visitors, want 3", n)
	}
}

// Scanning a full store is O(n); under a flood it must not happen once per
// request.
func TestStore_FullStoreSweepsAtMostOncePerInterval(t *testing.T) {
	store := newCappedStore()
	for _, ip := range []string{"a", "b", "c"} {
		store.getLimiter(ip)
	}
	expired := time.Now().Add(-2 * store.evictAfter)
	store.visitors["b"].lastSeen = expired
	store.visitors["c"].lastSeen = expired
	store.lastSweep = time.Now() // a sweep just ran

	store.getLimiter("d")

	if n := len(store.visitors); n != 3 {
		t.Fatalf("store holds %d visitors, want 3", n)
	}
	if _, ok := store.visitors["d"]; !ok {
		t.Error("the new visitor must be kept")
	}
	_, bKept := store.visitors["b"]
	_, cKept := store.visitors["c"]
	if !bKept && !cKept {
		t.Error("both expired entries were swept although a sweep ran within the interval")
	}
}

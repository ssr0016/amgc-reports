package middleware

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/alexedwards/scs/v2"
	"github.com/labstack/echo/v4"
	"golang.org/x/time/rate"

	"github.com/ssr0016/pastoral-reports/backend/internal/apperror"
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

// Forwarded values that are not IP addresses must not each get a bucket of
// their own: they share the connection's, so rotating junk buys nothing.
func TestRateLimit_JunkForwardedValuesShareTheConnectionBucket(t *testing.T) {
	e := echo.New()
	e.HTTPErrorHandler = apperror.ErrorHandler
	e.GET("/x", func(c echo.Context) error { return c.NoContent(http.StatusOK) },
		RateLimit(rate.Every(10*time.Minute), 3))

	var codes []int
	for i := 0; i < 5; i++ {
		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/x", nil)
		req.Header.Set(echo.HeaderXForwardedFor, fmt.Sprintf("junk-%d", i))
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		codes = append(codes, rec.Code)
	}

	want := []int{200, 200, 200, 429, 429}
	for i := range want {
		if codes[i] != want[i] {
			t.Fatalf("statuses = %v, want %v", codes, want)
		}
	}
}

// statusFor sends one request carrying the given X-Forwarded-For value.
func statusFor(e *echo.Echo, forwarded string) int {
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/x", nil)
	req.Header.Set(echo.HeaderXForwardedFor, forwarded)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec.Code
}

func newBurstOf3Echo() *echo.Echo {
	e := echo.New()
	e.HTTPErrorHandler = apperror.ErrorHandler
	e.GET("/x", func(c echo.Context) error { return c.NoContent(http.StatusOK) },
		RateLimit(rate.Every(10*time.Minute), 3))
	return e
}

// One IPv6 client owns a /64. Rotating the low 64 bits must not hand it a new
// bucket per address.
func TestRateLimit_RotatingWithinAnIPv6PrefixSharesOneBucket(t *testing.T) {
	e := newBurstOf3Echo()

	var codes []int
	for i := 0; i < 5; i++ {
		codes = append(codes, statusFor(e, fmt.Sprintf("2001:db8:1:2::%x", i+1)))
	}

	want := []int{200, 200, 200, 429, 429}
	for i := range want {
		if codes[i] != want[i] {
			t.Fatalf("statuses = %v, want %v", codes, want)
		}
	}
}

// Grouping must not merge unrelated clients.
func TestRateLimit_DifferentPrefixesAndIPv4NeighboursAreIndependent(t *testing.T) {
	e := newBurstOf3Echo()

	for i := 0; i < 3; i++ {
		statusFor(e, "2001:db8:1:2::1")
		statusFor(e, "203.0.113.7")
	}
	if got := statusFor(e, "2001:db8:1:2::1"); got != http.StatusTooManyRequests {
		t.Fatalf("exhausted /64: status = %d, want 429", got)
	}
	if got := statusFor(e, "203.0.113.7"); got != http.StatusTooManyRequests {
		t.Fatalf("exhausted IPv4: status = %d, want 429", got)
	}

	if got := statusFor(e, "2001:db8:1:3::1"); got != http.StatusOK {
		t.Errorf("next /64: status = %d, want 200", got)
	}
	if got := statusFor(e, "203.0.113.8"); got != http.StatusOK {
		t.Errorf("neighbouring IPv4: status = %d, want 200", got)
	}
}

func newLimitedEcho(r rate.Limit, burst int) *echo.Echo {
	e := echo.New()
	e.HTTPErrorHandler = apperror.ErrorHandler
	e.GET("/x", func(c echo.Context) error { return c.NoContent(http.StatusOK) }, RateLimit(r, burst))
	return e
}

// responseFor sends one request as a fixed client.
func responseFor(e *echo.Echo) *httptest.ResponseRecorder {
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/x", nil)
	req.Header.Set(echo.HeaderXForwardedFor, "198.51.100.1")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

// exhaust spends the whole burst and returns the response that is then denied.
func exhaust(t *testing.T, e *echo.Echo, burst int) *httptest.ResponseRecorder {
	t.Helper()
	for i := 1; i <= burst; i++ {
		if got := responseFor(e).Code; got != http.StatusOK {
			t.Fatalf("request %d: status = %d, want 200", i, got)
		}
	}
	rec := responseFor(e)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("after the burst: status = %d, want 429", rec.Code)
	}
	return rec
}

func retryAfter(t *testing.T, rec *httptest.ResponseRecorder) int {
	t.Helper()
	v := rec.Header().Get("Retry-After")
	if v == "" {
		t.Fatal("429 has no Retry-After header")
	}
	secs, err := strconv.Atoi(v)
	if err != nil {
		t.Fatalf("Retry-After = %q, want delay-seconds (an integer): %v", v, err)
	}
	return secs
}

// A client that is told when to come back does not have to guess. The value is
// the time until the bucket holds a token again.
func TestRateLimit_429CarriesRetryAfterSeconds(t *testing.T) {
	rec := exhaust(t, newLimitedEcho(rate.Every(10*time.Minute), 3), 3)

	// One token refills every 600s; a few milliseconds have passed.
	if got := retryAfter(t, rec); got < 599 || got > 600 {
		t.Errorf("Retry-After = %d, want 599..600", got)
	}
}

func TestRateLimit_RetryAfterFollowsTheLimiterRate(t *testing.T) {
	tests := []struct {
		name     string
		r        rate.Limit
		burst    int
		min, max int
	}{
		{"1 per minute", rate.Every(time.Minute), 1, 59, 60},
		{"5 per minute", rate.Limit(5.0 / 60.0), 3, 11, 12},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := retryAfter(t, exhaust(t, newLimitedEcho(tt.r, tt.burst), tt.burst))
			if got < tt.min || got > tt.max {
				t.Errorf("Retry-After = %d, want %d..%d", got, tt.min, tt.max)
			}
		})
	}
}

// A sub-second wait still has to say "1": 0 would invite an immediate retry.
func TestRateLimit_RetryAfterIsAtLeastOneSecond(t *testing.T) {
	rec := exhaust(t, newLimitedEcho(rate.Limit(10), 1), 1) // a token every 100ms

	if got := retryAfter(t, rec); got != 1 {
		t.Errorf("Retry-After = %d, want 1", got)
	}
}

// Reading the wait must not spend tokens: hammering a denied client may not
// push its retry time further out.
func TestRateLimit_RetryAfterDoesNotGrowWithRepeatedDenials(t *testing.T) {
	e := newLimitedEcho(rate.Every(10*time.Minute), 3)
	exhaust(t, e, 3)

	var last int
	for i := 0; i < 5; i++ {
		last = retryAfter(t, responseFor(e))
	}
	if last > 600 {
		t.Errorf("Retry-After after 5 more denials = %d, want at most 600", last)
	}
}

func TestRateLimit_AllowedResponseHasNoRetryAfter(t *testing.T) {
	rec := responseFor(newLimitedEcho(rate.Every(time.Minute), 1))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if v := rec.Header().Get("Retry-After"); v != "" {
		t.Errorf("Retry-After = %q on an allowed response, want none", v)
	}
}

// A bucket that never refills has no honest retry time, so none is sent.
func TestRateLimit_RetryAfterIsSkippedWhenRateIsZero(t *testing.T) {
	rec := exhaust(t, newLimitedEcho(0, 1), 1)

	if v := rec.Header().Get("Retry-After"); v != "" {
		t.Errorf("Retry-After = %q for a limiter that never refills, want none", v)
	}
}

// RateLimitByUser keys on the session user, and falls back to the IP without one.
func TestRateLimitByUser_Key(t *testing.T) {
	sm := scs.New()
	mw := RateLimitByUser(sm, rate.Limit(0.0001), 1) // burst 1: a second hit on one key is 429

	// hit sends a request from one IP as the given user (0 = no session user).
	hit := func(user int64) error {
		ctx, err := sm.Load(context.Background(), "")
		if err != nil {
			t.Fatal(err)
		}
		if user != 0 {
			sm.Put(ctx, UserIDKey, user)
		}
		req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/", nil)
		req.RemoteAddr = "203.0.113.9:1234"
		c := echo.New().NewContext(req, httptest.NewRecorder())
		return mw(func(echo.Context) error { return nil })(c)
	}

	if err := hit(1); err != nil {
		t.Fatalf("user 1 first hit: %v", err)
	}
	if err := hit(1); err == nil {
		t.Error("user 1 second hit allowed, want rate limited")
	}
	if err := hit(2); err != nil {
		t.Errorf("user 2 from the same IP blocked by user 1: %v", err)
	}
	// no session user: keyed by IP, separate from any user key
	if err := hit(0); err != nil {
		t.Errorf("anonymous first hit: %v", err)
	}
	if err := hit(0); err == nil {
		t.Error("anonymous second hit allowed, want rate limited by IP")
	}
}

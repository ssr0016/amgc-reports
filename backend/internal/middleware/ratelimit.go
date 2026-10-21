package middleware

import (
	"math"
	"strconv"
	"sync"
	"time"

	"github.com/labstack/echo/v4"
	"golang.org/x/time/rate"

	"github.com/ssr0016/pastoral-reports/backend/internal/apperror"
	"github.com/ssr0016/pastoral-reports/backend/internal/request"
)

type visitor struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

const (
	// defaultMaxVisitors bounds the number of tracked clients per limiter. At
	// about 184 bytes each that is roughly 9 MiB, however many distinct keys a
	// client can invent.
	defaultMaxVisitors = 50_000
	// sweepInterval bounds how often a full store is scanned for idle entries,
	// so a flood of new keys cannot turn every request into an O(n) scan.
	sweepInterval = time.Second
	// cleanupInterval is how often idle visitors are looked for.
	cleanupInterval = 3 * time.Minute
	// minEvictAfter is the shortest idle time before a visitor may be dropped.
	minEvictAfter = 3 * time.Minute
)

type rateLimiterStore struct {
	mu          sync.Mutex
	visitors    map[string]*visitor
	rate        rate.Limit
	burst       int
	evictAfter  time.Duration
	maxVisitors int       // cap on tracked clients; 0 or less means unlimited
	lastSweep   time.Time // last time a full store was scanned for idle entries
}

func newRateLimiterStore(r rate.Limit, burst int) *rateLimiterStore {
	store := &rateLimiterStore{
		visitors:    make(map[string]*visitor),
		rate:        r,
		burst:       burst,
		evictAfter:  evictAfter(r, burst),
		maxVisitors: defaultMaxVisitors,
	}
	go func() {
		for {
			time.Sleep(cleanupInterval)
			store.evictIdle(time.Now())
		}
	}()
	return store
}

// evictAfter returns how long a visitor must have been idle before its bucket
// may be dropped. Dropping a bucket hands its client a full burst again, so
// this must not be shorter than the time the bucket needs to refill
// completely (burst / rate), otherwise the configured rate is not enforced:
// with a 1-per-10-minute rate, a 3-minute eviction would allow a fresh burst
// every few minutes. A bucket that never refills is never dropped.
func evictAfter(r rate.Limit, burst int) time.Duration {
	if r <= 0 {
		return time.Duration(math.MaxInt64)
	}
	d := minEvictAfter
	if r != rate.Inf {
		// Rounded to the millisecond: float division leaves nanosecond noise
		// (3 / (1/600) is 29m59.999999999s), which means nothing for a limit
		// measured in minutes.
		ms := math.Round(float64(burst) / float64(r) * 1000)
		if ms < float64(math.MaxInt64/int64(time.Millisecond)) {
			if refill := time.Duration(ms) * time.Millisecond; refill > d {
				d = refill
			}
		}
	}
	return d
}

// evictIdle drops visitors that have been idle for longer than evictAfter.
func (s *rateLimiterStore) evictIdle(now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.evictIdleLocked(now)
}

func (s *rateLimiterStore) evictIdleLocked(now time.Time) {
	for ip, v := range s.visitors {
		if now.Sub(v.lastSeen) > s.evictAfter {
			delete(s.visitors, ip)
		}
	}
}

// makeRoom frees a slot for a new visitor when the store is full. Idle
// entries go first. Sweep at most once per second; otherwise a full store
// makes every new-key request O(n), which is a CPU DoS. If the store is still
// full, one entry is dropped. Go randomises map iteration order, so the
// dropped entry is effectively random and the cost is O(1).
//
// Random eviction may drop a legitimate client's bucket under attack. The
// effect is a fresh bucket, not a lockout; TrustedProxyExtractor removes the
// header-rotation vector.
//
// The caller holds s.mu.
func (s *rateLimiterStore) makeRoom(now time.Time) {
	if now.Sub(s.lastSweep) >= sweepInterval {
		s.lastSweep = now
		s.evictIdleLocked(now)
	}
	if len(s.visitors) < s.maxVisitors {
		return
	}
	for ip := range s.visitors {
		delete(s.visitors, ip)
		return
	}
}

func (s *rateLimiterStore) getLimiter(ip string) *rate.Limiter {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	if v, exists := s.visitors[ip]; exists {
		v.lastSeen = now
		return v.limiter
	}

	if s.maxVisitors > 0 && len(s.visitors) >= s.maxVisitors {
		s.makeRoom(now)
	}
	limiter := rate.NewLimiter(s.rate, s.burst)
	s.visitors[ip] = &visitor{limiter: limiter, lastSeen: now}
	return limiter
}

// RateLimit returns a per-client-IP token-bucket limiter: burst requests are
// allowed at once and tokens refill at r per second. Every call creates its
// own bucket store, so using one RateLimit(...) value on several routes shares
// a bucket between them, while separate calls are independent. The client is
// identified by rateLimitKey (IPv6 clients by their /64). The key is only as
// trustworthy as the Echo IPExtractor; see TrustedProxyExtractor.
func RateLimit(r rate.Limit, burst int) echo.MiddlewareFunc {
	return rateLimitBy(r, burst, rateLimitKey)
}

// RateLimitByUser is RateLimit keyed on the session user, so people sharing an
// IP (one office NAT) get separate buckets and the key does not depend on
// proxy trust. A request without a session user falls back to the IP key; put
// RequireAuth before it and that fallback is never used.
func RateLimitByUser(r rate.Limit, burst int) echo.MiddlewareFunc {
	return rateLimitBy(r, burst, func(c echo.Context) string {
		if u, ok := request.UserFrom(c.Request().Context()); ok {
			return "user:" + strconv.FormatInt(u.ID, 10)
		}
		return rateLimitKey(c)
	})
}

func rateLimitBy(r rate.Limit, burst int, key func(echo.Context) string) echo.MiddlewareFunc {
	store := newRateLimiterStore(r, burst)
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			limiter := store.getLimiter(key(c))
			if !limiter.Allow() {
				if secs := retryAfterSeconds(limiter, r); secs > 0 {
					c.Response().Header().Set(echo.HeaderRetryAfter, strconv.Itoa(secs))
				}
				return apperror.RateLimit("rate limit exceeded, try again later")
			}
			return next(c)
		}
	}
}

// retryAfterSeconds returns the whole seconds until the limiter holds a token
// again, at least 1, or 0 when it never refills. It only reads the bucket:
// reserving a token here would push a denied client's wait further out.
func retryAfterSeconds(l *rate.Limiter, r rate.Limit) int {
	if r <= 0 {
		return 0
	}
	secs := math.Ceil((1 - l.Tokens()) / float64(r))
	if secs < 1 {
		return 1
	}
	return int(secs)
}

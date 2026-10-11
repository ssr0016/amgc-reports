package middleware

import (
	"math"
	"sync"
	"time"

	"github.com/labstack/echo/v4"
	"golang.org/x/time/rate"

	"github.com/ssr0016/pastoral-reports/backend/internal/apperror"
)

type visitor struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

const (
	// cleanupInterval is how often idle visitors are looked for.
	cleanupInterval = 3 * time.Minute
	// minEvictAfter is the shortest idle time before a visitor may be dropped.
	minEvictAfter = 3 * time.Minute
)

type rateLimiterStore struct {
	mu         sync.Mutex
	visitors   map[string]*visitor
	rate       rate.Limit
	burst      int
	evictAfter time.Duration
}

func newRateLimiterStore(r rate.Limit, burst int) *rateLimiterStore {
	store := &rateLimiterStore{
		visitors:   make(map[string]*visitor),
		rate:       r,
		burst:      burst,
		evictAfter: evictAfter(r, burst),
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

	for ip, v := range s.visitors {
		if now.Sub(v.lastSeen) > s.evictAfter {
			delete(s.visitors, ip)
		}
	}
}

func (s *rateLimiterStore) getLimiter(ip string) *rate.Limiter {
	s.mu.Lock()
	defer s.mu.Unlock()

	v, exists := s.visitors[ip]
	if !exists {
		limiter := rate.NewLimiter(s.rate, s.burst)
		s.visitors[ip] = &visitor{limiter: limiter, lastSeen: time.Now()}
		return limiter
	}
	v.lastSeen = time.Now()
	return v.limiter
}

// RateLimit returns a per-client-IP token-bucket limiter: burst requests are
// allowed at once and tokens refill at r per second. Every call creates its
// own bucket store, so using one RateLimit(...) value on several routes shares
// a bucket between them, while separate calls are independent. The IP comes
// from c.RealIP(); see H-4 for the trusted-proxy caveat.
func RateLimit(r rate.Limit, burst int) echo.MiddlewareFunc {
	store := newRateLimiterStore(r, burst)
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			ip := c.RealIP()
			limiter := store.getLimiter(ip)
			if !limiter.Allow() {
				return apperror.RateLimit("rate limit exceeded, try again later")
			}
			return next(c)
		}
	}
}

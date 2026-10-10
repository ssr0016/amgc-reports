package middleware

import (
	"sync"
	"time"

	"github.com/labstack/echo/v4"
	"golang.org/x/time/rate"

	"github.com/ssr0016/template/internal/apperror"
)

type visitor struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

type rateLimiterStore struct {
	mu       sync.Mutex
	visitors map[string]*visitor
	rate     rate.Limit
	burst    int
}

func newRateLimiterStore(r rate.Limit, burst int) *rateLimiterStore {
	store := &rateLimiterStore{
		visitors: make(map[string]*visitor),
		rate:     r,
		burst:    burst,
	}
	go func() {
		for {
			time.Sleep(3 * time.Minute)
			store.mu.Lock()
			for ip, v := range store.visitors {
				if time.Since(v.lastSeen) > 3*time.Minute {
					delete(store.visitors, ip)
				}
			}
			store.mu.Unlock()
		}
	}()
	return store
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

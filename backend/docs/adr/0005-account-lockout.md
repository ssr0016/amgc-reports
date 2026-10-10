# ADR 0005: Account Lockout (5 attempts, 15 min)

**Status:** Accepted
**Date:** 2026-09-17
**Deciders:** [your name]

---

## Context

Kailangan ng protection laban sa brute-force login attempts.

## Decision

5 failed logins -> 15 min lockout.

## Consequences

### Positive
- Simple -- madaling maintindihan
- Effective laban sa brute-force
- Standard sa industry

### Negative
- Pwede i-DOS ng attacker (i-lock ang legitimate user)
- Kailangan ng unlock mechanism

### Neutral
- May rate limiting din (5/min) bilang additional layer

## Alternatives Considered

### Exponential backoff
- **Why rejected:** Mas komplikado, mas mahirap i-explain

### CAPTCHA
- **Why rejected:** Dagdag na dependency, UX impact

## References

- internal/service/auth.go
- internal/middleware/ratelimit.go

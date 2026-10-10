# ADR 0003: CSRF Double-Submit Cookie Pattern

**Status:** Accepted
**Date:** 2026-09-17
**Deciders:** [your name]

---

## Context

Kailangan ng CSRF protection para sa session-based auth.

## Decision

Double-submit cookie pattern -- CSRF token sa cookie + header, i-compare.

## Consequences

### Positive
- Stateless -- walang server-side storage ng CSRF token
- Simple -- 1 cookie + 1 header
- Compatible sa session-based auth

### Negative
- Kailangan ng JavaScript para i-set ang header
- Vulnerable sa XSS (pero may HttpOnly session cookie naman)

### Neutral
- SameSite=Lax bilang additional layer

## Alternatives Considered

### Synchronizer token pattern
- **Why rejected:** Kailangan ng server-side storage per session

### SameSite=Strict only
- **Why rejected:** Hindi compatible sa lahat ng browser/flow

## References

- internal/middleware/csrf.go

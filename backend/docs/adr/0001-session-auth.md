# ADR 0001: Session-Based Auth (not JWT)

**Status:** Accepted
**Date:** 2026-09-17
**Deciders:** [your name]

---

## Context

Kailangan ng auth mechanism para sa API. Dalawang main options: session-based o JWT.

## Decision

Session-based auth gamit ang `alexedwards/scs` + Postgres store.

## Consequences

### Positive
- Revocation instant -- delete session
- CSRF protection natural (double-submit cookie)
- Server-side control -- pwede i-invalidate anytime
- Hindi kailangan ng token refresh logic

### Negative
- Kailangan ng session store (Postgres)
- Hindi stateless -- mas mahirap i-scale horizontally
- May DB round-trip per request

### Neutral
- Cookie-based -- kailangan ng SameSite + HttpOnly

## Alternatives Considered

### JWT
- **Why rejected:** Revocation mahirap, CSRF harder, kailangan ng refresh token rotation

### Cookie-only (no server-side store)
- **Why rejected:** Walang server-side revocation

## References

- internal/session/
- internal/middleware/auth.go

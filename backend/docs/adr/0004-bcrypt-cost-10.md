# ADR 0004: Bcrypt Cost 10

**Status:** Accepted
**Date:** 2026-09-17
**Deciders:** [your name]

---

## Context

Kailangan ng password hashing. Bcrypt cost ay 4-31, default 10.

## Decision

Bcrypt cost 10.

## Consequences

### Positive
- ~100ms per hash -- balance ng security at UX
- Standard sa industry
- Pwede i-increase later kung needed

### Negative
- Mas mababa kaysa sa cost 12 (mas secure)
- Mas mataas kaysa sa cost 8 (mas mabilis)

### Neutral
- May `golang.org/x/crypto/bcrypt` na

## Alternatives Considered

### Cost 12
- **Why rejected:** ~400ms -- masyadong mabagal para sa UX

### Argon2id
- **Why rejected:** Mas komplikado, bcrypt ay sapat na

## References

- internal/service/auth.go

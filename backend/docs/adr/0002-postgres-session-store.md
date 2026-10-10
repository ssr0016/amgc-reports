# ADR 0002: Postgres Session Store (not in-memory)

**Status:** Accepted
**Date:** 2026-09-17
**Deciders:** [your name]

---

## Context

Ang session store ay pwedeng in-memory (map) o Postgres.

## Decision

Postgres session store gamit ang `scs/postgresstore`.

## Consequences

### Positive
- Persistent across restarts
- Pwede i-share sa multiple instances
- Pwede i-query para sa audit
- May built-in cleanup

### Negative
- DB round-trip per request
- Kailangan ng index sa sessions table
- May cleanup job

### Neutral
- Same DB as main data

## Alternatives Considered

### In-memory (map)
- **Why rejected:** Hindi persistent, hindi scalable

### Redis
- **Why rejected:** Dagdag na dependency, may Postgres na

## References

- internal/session/
- internal/database/migrations/ (sessions table)

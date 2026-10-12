---
description: Deep Go code review for current changes
---
Review the current Go changes using:
1. `go-code-review` skill from beagle-go — idiomatic patterns, error handling, concurrency
2. `go-testing-code-review` skill — table-driven tests, mocking, coverage
3. Security scan for SQL injection, auth issues
4. Check `context.Context` propagation
5. Verify error wrapping with `errors.Is` / `errors.As`

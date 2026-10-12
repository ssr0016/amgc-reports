---
description: Load TDD context for Go backend work
---
You are working on a Go backend project. Follow this workflow strictly:

1. **PLAN**: Before any code, break the feature into testable increments
2. **RED**: Write a failing test first (table-driven, using Go's testing package)
3. **GREEN**: Write minimal code to make the test pass
4. **REFACTOR**: Clean up while keeping tests green

Always use `context.Context` for database operations. Always wrap errors.
Run `go test -v ./...` after every GREEN step.

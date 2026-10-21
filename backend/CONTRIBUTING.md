# Contributing Guide

Thanks for your interest in contributing to this project!

## Development Setup

1. Fork and clone the repo
2. Follow SETUP.md for installation
3. Create a feature branch

   git checkout -b feature/your-feature

## Code Standards

### Go Style

- Follow standard Go conventions
- Run gofmt before commit
- Run goimports for imports
- Use meaningful variable names
- Add comments for exported functions

### Project Structure

Code is grouped by domain under `internal/<domain>/`:

- `<domain>.go` - the `Service` and `Store` interfaces and the domain errors
- `model.go` - entities and request/response DTOs
- `<domain>impl/` - the unexported `service` and `store` behind `NewService` / `NewStore`, plus in-memory mocks
- `rest/` - Echo handlers: parse, validate, map errors, respond

Wiring is manual, in `cmd/api/app/server.go`; routes stay in `internal/router/router.go`.

### Architecture Rules

See docs/ARCHITECTURE.md for detailed rules:

- Handler to Store: for simple CRUD (`accesscontrol` has no service)
- Handler to Service to Store: for complex logic
- Services and stores take `context.Context` and plain values, never an Echo context
- Every layer must add value

## Testing

### Unit Tests

    make test

Or:

    go test ./internal/... -v

### Integration Tests

Requires Docker:

    go test -tags=integration ./internal/... -v

### Coverage

    go test ./internal/... -cover

Aim for 70%+ coverage on new code.

## Before Committing

1. Run formatter

   make fmt

2. Run linter

   make lint

3. Run tests

   make test

4. Run integration tests (if applicable)

   go test -tags=integration ./internal/...

5. Build

   make build

All checks must pass.

## Commit Messages

Follow Conventional Commits:

    feat: add user profile endpoint
    fix: resolve session cookie issue
    docs: update README
    test: add unit tests for auth
    refactor: extract validation logic
    chore: update dependencies
    ci: add GitHub Actions workflow

Format:

    type(scope): short description

    Optional body with more details.

    Optional footer with issue reference: Closes #123

### Types

- feat - New feature
- fix - Bug fix
- docs - Documentation
- style - Formatting
- refactor - Code refactoring
- test - Tests
- chore - Maintenance
- ci - CI/CD changes
- perf - Performance

## Pull Requests

1. Update README if needed
2. Add tests for new features
3. Ensure all CI checks pass
4. Request review
5. Address feedback

### PR Checklist

- [ ] Code follows style guide
- [ ] Tests added and passing
- [ ] Documentation updated
- [ ] No lint errors
- [ ] CI passes
- [ ] Commits follow conventional format

## Reporting Bugs

Open an issue with:

- Description of bug
- Steps to reproduce
- Expected behavior
- Actual behavior
- Environment (OS, Go version, Docker version)
- Screenshots (if applicable)

## Suggesting Features

Open an issue with:

- Use case description
- Proposed solution
- Alternative approaches
- Impact on existing code

## Code of Conduct

- Be respectful
- Constructive feedback only
- Focus on code, not people
- Help others learn

## Questions

Open a discussion or issue.

## License

By contributing, you agree that your contributions will be licensed under the MIT License.

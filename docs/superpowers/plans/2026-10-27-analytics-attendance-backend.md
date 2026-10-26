# Analytics Attendance Backend Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a permission-gated annual attendance analytics endpoint with an approved-only default and an all-submitted operational view.

**Architecture:** Extend the existing `stats` domain rather than creating a parallel analytics package. The store aggregates sparse `report_metrics` rows by report month/week and submitted church name; the service validates the metric/status filters; the existing stats handler and router provide the HTTP boundary and `tracker:read` authorization.

**Tech Stack:** Go, Echo, pgx/Squirrel, existing stats/report constants, focused unit tests.

**Spec:** User request in the conversation; frontend is intentionally out of scope for this pass.

## Global Constraints

- Default status is `approved`; `all` includes `pending`, `needs_revision`, and `approved`.
- Metric is one of the existing 21 `report.MetricKeys`.
- Year is required and must be a positive integer.
- Soft-deleted reports are excluded.
- No frontend files, Recharts dependency, migrations, or generated API types are changed in this pass.

## Review Focus

- Missing or invalid year: reject before the store is called.
- Unknown metric: reject before the store is called.
- Invalid status: reject before the store is called.
- Sparse weeks and months: return zero-filled 12×5 trend points.
- Church names with no value: group under a stable `Unassigned` label.

### Task 1: Analytics contract and aggregation

**Files:**
- Modify: `backend/internal/stats/stats.go`, `model.go`, `statsimpl/service.go`, `statsimpl/store.go`
- Test: `backend/internal/stats/statsimpl/service_test.go`, `backend/internal/stats/rest/stats_handler_test.go`

**Interfaces:**
- Produces `stats.Service.AnalyticsAttendance(ctx, stats.AttendanceQuery) (stats.AttendanceAnalytics, error)`.
- Produces `GET /admin/analytics/attendance?year=&metric=&status=` through the existing stats handler.

- [ ] Write failing service and handler tests for the default status, filter forwarding, response shape, and validation.
- [ ] Run the focused tests and confirm they fail because the contract is absent.
- [ ] Implement the DTOs, validation, one SQL aggregation query, and handler.
- [ ] Run focused backend tests and `gofmt`.

### Task 2: Route wiring and verification

**Files:**
- Modify: `backend/internal/router/router.go`, `backend/internal/router/stats_routes_test.go`

- [ ] Add the route beside `/admin/stats` and `/admin/tracker` using `tracker:read` and the read limiter.
- [ ] Extend route authorization coverage to the analytics endpoint.
- [ ] Run the focused stats, handler, and router tests.

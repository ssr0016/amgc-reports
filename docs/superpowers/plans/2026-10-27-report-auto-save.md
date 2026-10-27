# Report Auto-save Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace manual report draft saving and decision cards with local recovery, debounced server draft saving, and one submit action.

**Architecture:** `ReportForm` owns serialized form recovery, change persistence, debounce timing, status text, and submit labeling. New-report supplies a create-draft callback that returns an ID; edit-report supplies an update-draft callback. Existing report status navigation remains page-level and uses the current report query.

**Tech Stack:** Next.js 16, React 19, TanStack Query, Vitest, Testing Library, existing report API.

**Spec:** User-approved auto-save request in the conversation.

## Global Constraints

- Do not add dependencies or backend endpoints.
- Keep validation and existing error handling at the form/API boundary.
- Do not commit secrets or unrelated work.
- Run the explicitly requested full frontend/backend verification commands.

## Review Focus

- A new report has no server ID until its first autosave; test create-then-update routing.
- A refresh must recover only the matching user/reporting period; test discard removes the recovery state.
- Editing after a pending debounce must reset the timer and avoid stale payloads.
- Submit must not race an autosave; test the single primary action and final submission path.
- Read-only reports must never write local or server state.

### Task 1: ReportForm autosave behavior

**Files:**
- Modify: `frontend/src/components/report-form.tsx`
- Test: `frontend/src/components/report-form.test.tsx`

- [ ] Add failing tests for local recovery/discard, debounced autosave, autosave timestamp, and only one submit button.
- [ ] Run the focused form tests and confirm the new tests fail for the missing behavior.
- [ ] Add the smallest form callbacks/props needed for local storage and debounced save.
- [ ] Persist serialized form state on change, recover it on mount, discard it on request, and show the required banner/status text.
- [ ] Replace the draft submit control with the single submit control while preserving validation and cancel behavior.
- [ ] Run the focused form tests and confirm they pass.

### Task 2: New-report lifecycle

**Files:**
- Modify: `frontend/src/app/(app)/reports/new/new-report.tsx`
- Test: `frontend/src/app/(app)/reports/new/new-report.test.tsx`

- [ ] Add failing tests that remove decision cards and verify a new form exposes one submit action.
- [ ] Run the focused new-report tests and confirm the expected failures.
- [ ] Render the form directly when no report exists; remove draft deletion UI.
- [ ] Wire first autosave to `createReport(input, true)`, route to edit, and wire submit to create/submit as currently supported.
- [ ] Route existing draft/revision/submitted reports to their current edit/view destinations without rendering decision cards.
- [ ] Run the focused new-report tests.

### Task 3: Edit-report lifecycle and sidebar coverage

**Files:**
- Modify: `frontend/src/app/(app)/reports/[id]/edit/edit-report.tsx`
- Modify: `frontend/src/app/(app)/sidebar.tsx` only if implementation requires it
- Tests: `frontend/src/app/(app)/reports/[id]/edit/edit-report.test.tsx`, `frontend/src/app/(app)/sidebar.test.tsx`

- [ ] Add failing coverage for update autosave and `Resubmit Report`.
- [ ] Wire draft update callback and preserve 404 invalidation behavior.
- [ ] Ensure needs-revision submission updates and submits the same report.
- [ ] Verify all four sidebar labels, including no current report.

### Task 4: Verification and delivery

- [ ] Run `npm test`, `npm run typecheck`, `npm run lint`, and `npm run build` in `frontend`.
- [ ] Run `make test`, `make lint`, and `make vet` in `backend`.
- [ ] Inspect and show the real diff before committing.
- [ ] Commit the focused change and push `feature/report-auto-save` to `main` only after all requested gates pass.

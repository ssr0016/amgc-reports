# Completion recovery assessment

Recorded 2026-10-25 against local `main` at `fdaffe2e9727c97f68620f59258d226d7592fb45`.

The owner instructed that Feature 19 is done and must not be touched again. For this assessment, a reachable landing commit plus a matching archive spec hash establishes completion. This is a historical assessment, not fresh runtime verification or a change to the installed recovery contract.

| Item | Result | Landing commit reachable from main | Evidence |
|---|---|---|---|
| 19: seed idempotency | Complete; settled by owner | `d087da0da7f82517cecbe96ecb72429f7f398d8c` | Archived spec byte length and SHA-256 match its annotation; archive bytes match the work branch and main. Work commit is directly reachable from main. |
| 37b: permission-gated admin panel | Complete | `f5a0f05862876268a3d5d2a010182be93788f864` | Archived spec byte length and SHA-256 match; archive bytes match branch and main. Landing tree equals retained branch `feature/permission-gated-admin-panel` at `7ab87e2fd7a1543efcd31a426a429628b3a506ad`. |
| Fix: Report Dashboard month and year columns | Complete | `9c8d30e144fbfd6d539f17679b10210bc5c88bf1` | Archived spec byte length and SHA-256 match; archive bytes match branch and main. Landing tree equals retained branch `fix/report-dashboard-with-month-and-year-columns` at `3ed2a181b8822e45478ef3590fa8cca2d8ce46bb`. |
| Fix: admin panel refresh, loading, toasts and retry | Complete | `78870e1a4f5d04ffa77f402fcc465ce28b942eff` | Archived spec byte length and SHA-256 match; archive bytes match branch and main. Landing tree equals retained branch `fix/admin-panel-refresh-loading-toasts-and-retry` at `a44049a2e728734e2d9f395d2d8ef9c3ebe761de`. |

## Strict-recovery mismatch

The installed contract's merged-phase rule requires the current default-branch tip's sole parent to equal the recorded base. Main has advanced, so that rule cannot certify these historical landings. Feature 19 additionally landed through direct work-commit ancestry rather than the expected squash shape. No history rewrite is needed or authorized. Feature 19's spec, archive, plan entry, and implementation were left untouched.

The other three landing commits each have the recorded base as their sole parent. Their work tips are not ancestors of main, as expected for squash merges; exact landing-tree equality supplies the binding instead.

## Recommendation

Add a narrow historical-landing rule for main advancing past the recorded base: find one unambiguous landing commit reachable from main, require its sole parent to equal the recorded base, its complete tree to equal the retained final work commit's tree, and its archive to match both the archived spec digest and the work-branch archive bytes. Preserve live-state consistency checks and stop on conflicting or ambiguous evidence. Later commits on main should not invalidate this historical proof.

Direct or fast-forward landings such as Feature 19 need a separately documented owner exception or an explicitly supported landing path; do not silently treat them as squash merges. A matching archive hash alone proves spec integrity, not that the implementation landed or still works. The rule should settle historical status without claiming fresh verification or authorizing cleanup, commits, merges, or pushes.

No tests, cleanup, commits, pushes, branch switches, Git object writes, or product changes were performed. This assessment does not revalidate runtime behavior or the historical gates. The recovery contract itself was not edited.

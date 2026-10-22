# Fix: Excel sheet names for emoji and apostrophes

**Type:** Fix
**Status:** verified
**Branch:** fix/excel-sheet-names-for-emoji-and-apostrophes
**Fixes:** F-05

## The problem

The Excel export (feature 22) fails with a 500 for some church names. In a bulk export, one such report fails the whole month, because every report shares one workbook. No audit row is written, which is correct.

The cause is `sheetNames` in `backend/internal/report/reportimpl/excel.go`. excelize v2.11.0 rejects a sheet name (`checkSheetName`) when:

- it is longer than 31 **UTF-16 units**. `truncateRunes` counts runes, so emoji (two UTF-16 units each) get past it. 20 × "🙏" fails.
- it starts or ends with `'`. The `'` trim runs **before** the cut, so the cut can leave a new trailing `'`. `"a"×30 + "'b"` fails.

`church_name` and `worker` are free text (up to 150 characters), so a real report can hit either case. `TestSheetNames` only covers ASCII names.

## The fix

In `sheetNames`, the one place sheet names are made:

- Count the limit in UTF-16 units: cut whole runes until the name fits (31 for the base, and 31 minus the suffix length for ` (n)` names). Never split a surrogate pair.
- Trim `'` from both ends **after** each cut, for the base name and before adding a ` (n)` suffix. If the name ends up empty, fall back to `Report <id>`.
- Keep everything else unchanged: the character replacement for `[]:*?/\`, case-insensitive dedupe with ` (2)`, ` (3)`…, sheet order, filenames, audit rows, and the layout.

No new dependency (`unicode/utf16` is in the standard library) and no other layer changes. The download filename does not use this function and is not affected.

Not in this fix: F-06 (`filename*` escaping) and F-07 (`saveFile` test) stay open as separate P3 items. Excel's reserved sheet name "History" (it opens with a repair prompt) is not part of F-05; leave it for a separate decision.

## Build steps

- [x] **1. Make sheet names valid for any text.** Change `sheetNames`/`truncateRunes` in `excel.go` as above. Extend `TestSheetNames` in `excel_test.go` with:
  - 20 × "🙏";
  - `"a"×30 + "'b"`;
  - a name made only of `'`;
  - an emoji name that collides and needs ` (2)`;
  - a name whose cut leaves a leading or trailing `'` before the suffix.

  For each, assert the exact names. The existing "all names open in a workbook" check must still pass, now covering the new cases. Add one `ExportMonth` service test where one of two reports has an emoji church name: the export succeeds with both sheets.

  **Done when:** before the fix, the new cases fail with the excelize length and single-quote errors. After it, `go test ./internal/report/...` and `make lint` pass.

## Verify

- Backend: `make test`, `make lint`, `make vet`. No frontend or schema change, so frontend checks aren't needed; run `make test-integration` only if `/implement` touches more than `excel.go` and its tests.
- By hand: as an admin, give a report a church name of 20 emoji, or 30 letters followed by `'b`. Download Bulk for that month. You get a workbook with one sheet per report instead of an error toast.


<!-- blueprint:completion {"schemaVersion":2,"specBytes":3114,"specSha256":"d30d22187655dcc55bb1fb6c0b50f4a9227d859c5793b27b4e5df38ccfdf9537","branch":"refs/heads/fix/excel-sheet-names-for-emoji-and-apostrophes","head":"e3a9cb861585dd152e0246b3ac630290e415324c","baseRef":"refs/heads/main","baseCommit":"cf0fcb618b14bfa005ac0954e8c5dda03c187d78","sourceTree":"052206b79218b994a96a69de1564bba26799df49","landing":"local-merge","absentOptional":[]} -->

## Findings

### excel-sheet-names-for-emoji-and-apostrophes/F-05 [P2] closed - Some church names make the Excel export fail with a 500, and one such report blocks the whole month

**File:** backend/internal/report/reportimpl/excel.go:219-226, backend/internal/report/reportimpl/excel.go:234-239
**Found:** 2026-10-21 by /audit independent (scope: current; lens: quality)
**Why it matters:** `sheetNames` cuts names to 31 runes, but excelize v2.11.0 `checkSheetName` counts UTF-16 units, and it also rejects a name that starts or ends with `'`. The leading/trailing `'` is trimmed before the cut, so the cut can leave a new trailing `'`. A probe against `workbookBytes` (run through a `go test -overlay`, no repo file added) showed that 20 × "🙏" fails with "the sheet name length exceeds the 31 characters limit", and `"a"×30 + "'b"` fails with "the first or last character of the sheet name can not be a single quote". `church_name` is free text up to 150 characters, so an admin gets a 500 for that report. The bulk export then fails for the whole month, because every report shares one workbook. No audit row is written, which is correct. `TestSheetNames` only covers ASCII names.
**Suggested fix:** Truncate by UTF-16 units (or loop on excelize's own length check), trim `'` after truncating, including after the ` (n)` suffix step, and add the two cases above to `TestSheetNames`.
**Resolution:** Repaired in 772f8f5 and re-reviewed on 2026-10-22 by /audit independent (target e3a9cb8). The ledger still said `open` (not marked `fixed`). It is closed because all three close conditions hold. `fitSheetName` (excel.go:238-247) counts `utf16.RuneLen` the same way excelize's `countUTF16String` does, cuts only at rune boundaries, and trims `'` after every cut, including before the ` (n)` suffix. Excelize's other checks (blank, `:\/?*[]`) are still covered by the replacer and the `Report <id>` fallback. With n >= 16 (the cap is 100 reports) a cut never leaves an empty name. With the pre-fix excel.go swapped in through `go test -overlay`, `TestSheetNames` and the new `ExportMonth` emoji subtest fail with the excelize length error. Against the fix they pass. Probes of a leading `'` plus emoji, `a` plus 40 `'`, 40 × `é`, combining marks, and 30 × `y` plus an emoji all build a valid workbook. The repair added no defect. F-08 is an older dedupe gap, not caused by this repair.

## Independent review

**Status:** passed
**Target commit:** e3a9cb861585dd152e0246b3ac630290e415324c
**Base commit:** cf0fcb618b14bfa005ac0954e8c5dda03c187d78
**Base ref:** main
**Spec hash:** d30d22187655dcc55bb1fb6c0b50f4a9227d859c5793b27b4e5df38ccfdf9537
**Prepared by:** claude
**Builder model:** claude-opus-5-5
**Requested reviewer:** claude
**Requested model:** runtime default (exact model not known until reviewer starts)
**Requested execution:** automatic
**Requested at:** 2026-10-22T00:41:13Z
**Workflow:** regular
**Check required:** no
**Reviewer adapter:** claude
**Reviewer model:** claude-opus-5-5
**Reviewer context:** fresh subagent
**Actual execution:** automatic
**Reviewed at:** 2026-10-22T00:44:06Z
**Scope:** current
**Lenses:** quality, security, performance, tests
**Verdict:** passed
**Check result:** not-required

### Handoff

Review the active spec and the complete `cf0fcb618b14bfa005ac0954e8c5dda03c187d78..e3a9cb861585dd152e0246b3ac630290e415324c` delta in a fresh
session or isolated subagent without the builder conversation. Run all Audit lenses from scratch.
Run Check when required above. Do not edit product code, accept findings, or
reuse the existing findings as the review scope.

### Commands

- `git rev-parse HEAD`, `git merge-base main HEAD`, `sha256sum blueprint/context/current-feature.md`, `git status --porcelain`: pass (all match the request; only review.md differed)
- `make test` (backend): pass
- `make lint` (backend): pass (0 issues)
- `make vet` (backend): pass
- `go test -overlay` with the pre-fix excel.go, running `TestSheetNames|TestReportService_ExportMonth`: fail as expected (excelize length error), so the new tests catch the bug
- `go test -overlay` with a scratchpad-only edge-case probe (no repo file added): pass, results logged

### Evidence

- Delta: excel.go (`truncateRunes` replaced by `fitSheetName`), excel_test.go (6 new `TestSheetNames` cases), report_test.go (one `ExportMonth` emoji subtest), current-feature.md (the spec).
- excelize v2.11.0 `checkSheetName` (sheet.go:1512-1526) rejects a blank name, more than 31 units by `countUTF16String` (lib.go:992, `utf16.RuneLen` per rune), a leading or trailing `'`, and `:\/?*[]`. `fitSheetName` counts the same unit, cuts only at rune boundaries, and trims `'` after each cut, as the spec says.
- The suffix path's minimum budget is 31 - len(" (100)") = 25 units, so a non-empty trimmed base can't be cut to empty, and the dedupe loop always ends.
- Probes: a leading `'` plus 16 emoji, `a` plus 40 `'`, 40 × `é`, `x` plus 40 combining marks, and 30 × `y` plus an emoji all build valid sheets.
- Security: names only become sheet names and cell text, so there is no new trust boundary, and the filename path is unchanged. Performance: linear in the name length, bounded by the 100-report cap.
- Out of scope per the spec: F-06, F-07 and the reserved "History" name.

### Findings

- F-05 [P2] closed: the original bug is repaired and re-reviewed.
- F-08 [P3] open (new): names equal under `EqualFold` but not under `ToLower` merge two reports into one sheet. This predates the delta.
- No P0 or P1 findings are open or fixed.

### Remaining risk

- `make test-integration` was not run. The spec requires it only if the change goes beyond excel.go and its tests, and the extra report_test.go subtest uses the in-memory fixture.
- The by-hand Bulk download in a browser was not run (Check not required), and Excel desktop was not used to open the generated workbook.
- The dashboard activity file `blueprint/.state/run.json` was not updated, because the caller limited writes to findings.md and review.md.

## Try it now

1. Sign in as an admin and open a report for a month you can export.
2. Set its church name to 20 × "🙏", or to 30 letters followed by `'b`, and save.
3. On the admin reports page, choose **Download Bulk** for that month.
4. Expected: an `.xlsx` downloads with one sheet per report. The emoji report's sheet is named 15 × "🙏", or the 30 letters without the `'`. No error toast appears.
5. Unaffected path: **Download** on a single report with an ordinary church name still gives one sheet named after the church.

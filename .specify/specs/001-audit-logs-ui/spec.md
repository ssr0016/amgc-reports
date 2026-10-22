# Spec: Audit Logs UI

**From build-plan:** item 21 (Phase B), modules 5 (Logs tab) and 6 (login and logout audit) of `backend/docs/08-Frontend-Plan.md` §10
**Decisions in force:** D-10 (audit written in the same transaction as the change, `details` never holds narrative text), D-A1 (users are never hard-deleted), D-A11 (a deleted user's login looks like an unknown email)
**Decisions changed by this spec (owner, 2026-10-22):** D-A4 now also logs **failed** logins (it said successful logins and logouts only). The IP address stays hidden. D-10's "No read API" becomes an admin-only read API.
**Status:** final

## Problem

Report and user changes, exports and password resets are already audited, but nobody can read the trail in the app. Sign-ins aren't audited at all, so an admin can't see who signed in or spot repeated failed attempts against an account.

## Solution

Audit sign-ins, sign-outs and failed sign-ins alongside the actions already audited. Add a read-only **Logs** tab to the admin panel that lists the trail for one month at a time, newest first, filtered by action, paged on the server, with each row's details shown on demand.

## Behaviors

### What is logged

1. **Only state-changing and security events are logged.** Reading or listing reports, users, stats or the logs themselves, previewing an Excel file, and refreshing a session token write nothing.
2. **The 17 audited actions:**

   | Group | Action (stored code) | Label in the UI | Status |
   |---|---|---|---|
   | Admin | `CREATE_USER` | Created user | exists |
   | Admin | `UPDATE_USER` | Updated user | exists |
   | Admin | `DELETE_USER` | Deleted user | exists |
   | Admin | `RESTORE_USER` | Restored user | exists |
   | Admin | `CHANGE_USER_STATUS` | Changed user status | exists |
   | Admin | `ADMIN_RESET_PASSWORD` | Reset password | exists |
   | Admin | `APPROVE_REPORT` | Approved report | exists |
   | Admin | `UNAPPROVE_REPORT` | Unapproved report | exists |
   | Admin | `DOWNLOAD_SINGLE_EXCEL` | Downloaded Excel | exists |
   | Admin | `DOWNLOAD_BULK_EXCEL` | Downloaded Excel (bulk) | exists |
   | User | `CREATE_REPORT` | Created report | exists |
   | User | `UPDATE_REPORT` | Updated report | exists |
   | User | `DELETE_REPORT` | Deleted report | exists |
   | User | `CHANGE_PASSWORD` | Changed own password | exists |
   | Security | `LOGIN` | Logged in | **new** |
   | Security | `LOGOUT` | Logged out | **new** |
   | Security | `LOGIN_FAILED` | Failed login | **new** |

3. **Logged in.** Written when a sign-in succeeds and a session starts. The actor and the target are the user who signed in.
4. **Logged out.** Written when a sign-out ends a live session. The actor and the target are that session's user. A sign-out with no live session (already expired or revoked) writes nothing.
5. **Failed login.** Written whenever a sign-in is refused:
   - **Known account:** the target is the account, and the details give the reason: `wrong_password`, `locked`, or the account's status (`inactive`, `disciplinary` or `deceased`).
   - **Unknown, malformed or soft-deleted email:** the row has no target ID or name (its type is still User) and the reason is `unknown_account`. The typed email is **not** stored, because people sometimes type a password into the email box, and a deleted account must look the same as an unknown one (D-A11).
   - In both cases the actor is empty, since nobody signed in. Rows are rate-limited only by the login limiter that already exists.
6. **The response doesn't change.** Auditing a login, logout or failed login never changes what the client sees: the same status, message and timing as today. For a successful login, the session and its audit row are written together; if the audit row can't be written, the login fails with a 500 and no session starts. For a failed login, an audit write failure is recorded in the server log and the client still gets the usual refusal.

### What `details` may hold

7. **Details are an allow-list per action, never free text from a request.** They never contain a password, password hash, confirmation password, token, refresh token, session or CSRF value, cookie, or report narrative text (narrative, names, challenges, prayer request). The allow-list matches what the code writes today:
   - **Report actions and Excel downloads:** month, year and count.
   - **Create user:** role ID. **Update user:** none.
   - **Status change:** from, to, and sessions revoked.
   - **Delete user:** reason and sessions revoked.
   - **Restore user:** the deletion being undone (when, by whom, and its reason).
   - **Own password change, password reset:** sessions ended.
   - **Failed login:** the reason code.
   - **Login and logout:** none.
8. **An automated test fails the build** if any audited action writes a details key outside its allow-list.

### Logs tab

9. **Where it lives.** `/admin` gets a third tab, **Logs**, after Reports and Users, at `/admin?tab=logs`. It follows the existing tab rules: the tab is in the URL, switching replaces the history entry, and arrow keys move between tabs.
10. **Who can see it.** Only admins. A non-admin never sees the tab, and the API answers 403 to anyone who isn't an admin.
11. **Table columns, in order:**
    - **Timestamp:** date and time in Asia/Manila, e.g. "Oct 22, 2026, 4:05 PM".
    - **Actor:** the name stored when the action happened, with the stored role as a small label. For a failed login with no actor, "—".
    - **Action:** a badge with the label from behavior 2. Colors are grouped consistently: create, change, delete and restore, approve, download, and security, with failed login in the warning color.
    - **Target:** the record type (Report or User) and the stored target name. "—" when there's no target.
    - **Details:** a "View details" button that expands the row (see behavior 12).
12. **View details.** The button expands an inline panel under the row; pressing it again collapses it. Several rows can be open at once. The panel shows:
    - the full timestamp with seconds and the time zone;
    - the actor's name and role;
    - the target type, name and ID;
    - each allow-listed detail as a labelled value: reason codes in words, a status change as "Active → Inactive", "Sessions ended: 2".
    - It shows "No further details." when there are none.
    - It never shows the IP address or a raw JSON blob.
    - The button says whether its row is expanded and names the row it belongs to, for screen readers.
13. **Filters.**
    - **Month** and **Year** dropdowns. They start on the **current** month in Asia/Manila, not the previous one: a log view has to show today's activity, which is why it differs from the Reports tab. The Year list runs from 2026 to the current year. One month is always selected, so there's no "all time" option.
    - An **Action** dropdown: "All actions" (the default) or any one of the 17 labels.
    - Changing any filter goes back to page 1.
14. **Order.** Newest first. Rows with the same timestamp keep a stable order, so paging never repeats or skips a row.
15. **Count and paging.**
    - "N total logs" for the current filters, from the server, announced when it changes.
    - **50 rows per page**, paged on the server, with Previous and Next and "Page X of Y" (only shown when there's more than one page).
    - The page never loads more than one page of rows.
16. **Read-only.** The tab and its API have no create, edit, delete, purge or export. The audit rows can't be changed through the app.
17. **The IP address is never shown** anywhere in the tab, the details panel, or the logs API response, as D-A4 requires. It's still stored.
18. **States:**
    - **Loading:** a loading state.
    - **API error:** the error message with a way to retry.
    - **Empty:** "No logs match these filters."
    - **Not allowed (403):** "You are not allowed to view audit logs."
19. **Accessibility.** The tab meets the F13 standard: a real table with column headers, labelled filters, named buttons, focus kept on "View details" after it toggles, no axe violations, and usable at 320 px width.

## Edge cases

| Case | Expected behavior |
|---|---|
| Actor later soft-deleted, renamed or given another role | The name and role stored on the row are shown unchanged. Nothing is looked up live. |
| Target report or user later deleted | The stored target name is shown. There's no link, so nothing breaks. |
| Failed login for an unknown or deleted email | Actor "—", Target "—", details "Reason: Unknown account". The email isn't shown, because it isn't stored. |
| Failed login on a locked account | Target is the account, reason "Locked". A locked account's failed attempts are logged even though the password isn't checked. |
| Burst of failed logins (an attack) | Each attempt the login limiter lets through is one row. Requests the limiter refuses (429) are not logged. |
| Sign-out with an expired or already-revoked session | No row. |
| The same user signs in on two devices | Two "Logged in" rows. |
| An action the UI doesn't know (a future action that reached the API before the UI was updated) | A neutral badge with the raw code, and the details panel lists the keys as-is. The page doesn't break. |
| Details empty or of an unexpected shape | "No further details." |
| A row written at 12:30 AM on the 1st, Manila time (still the previous day in UTC) | It belongs to the new Manila month in both the Timestamp column and the Month filter. |
| A month with no rows (the future, or before go-live) | The empty state. |
| New rows written while an admin is paging | Rows that are already loaded don't move. The count and pages update on the next fetch, and nothing updates live. |
| Filters change while the admin is on page 4 | The tab goes back to page 1. |
| The audit insert fails during a successful login | The login answers 500 and no session or refresh token is left behind. |
| The audit insert fails during a failed login | The client gets the usual 401 or 403. The failure is recorded in the server log. |
| Session expired mid-view | The usual refresh-then-replay flow. If that fails, the user goes to sign-in. |
| Rate limited (429) on the logs API | The error state with the retry hint. |
| An existing row's details contain a key outside the allow-list | The UI only shows labelled allow-listed keys for known actions. Existing rows are not rewritten. |

## Non-goals

- **Logging reads:** viewing, listing, the stats cards, Excel previews or opening the Logs tab.
- **Logging token refreshes or refresh-token reuse** (D10 keeps reuse in the server log only).
- **Showing the IP address** (D-A4).
- **Storing the typed email** of a failed login for an unknown account.
- **Exporting logs** (CSV or Excel).
- **Retention, purge or archiving** (D-10, deferred).
- **Editing or deleting** audit rows.
- **Live updates.**
- **Filtering by actor or target**, or a "History" link from a report or user.
- **Non-admins seeing their own activity.**
- **Backfilling logins** from before this feature shipped.
- **The Tracker tab.**

## Success criteria

- [ ] A successful sign-in writes exactly one "Logged in" row, and a sign-out of a live session writes exactly one "Logged out" row.
- [ ] Each refused sign-in writes one "Failed login" row with the right reason. An unknown or deleted email stores no email and no target.
- [ ] Login, logout and refusal responses are unchanged: the same status codes and messages, and the existing auth tests still pass.
- [ ] If the audit insert fails during a successful login, the login fails with no session left behind (integration test).
- [ ] The three new actions are added to `chk_audit_logs_action` by a migration shipped in the same deploy as the code that writes them. `TestModelConstants_MatchCheckConstraints` passes, and `audit.Actions` has 17 entries.
- [ ] A test checks every action's details against its allow-list and finds no password, token or narrative key.
- [ ] Viewing reports, users, stats, previews or the logs writes no row.
- [ ] An admin sees the Logs tab, and a non-admin sees no tab and gets a 403 from the API.
- [ ] Month/Year and Action filters narrow the list, and "N total logs" equals the server total.
- [ ] 50 rows per page, newest first, never repeating or skipping a row across pages.
- [ ] "View details" expands and collapses a row and shows labelled values, never raw JSON or the IP.
- [ ] No IP address appears in the logs API response or the UI.
- [ ] Loading, error, empty and not-allowed states render.
- [ ] axe finds no violations, and the tab works at 320 px.
- [ ] Swagger is regenerated and `npm run gen:api` has no drift in CI.
- [ ] `docs/DECISIONS.md` (amend D-A4 and D-10), `docs/ARCHITECTURE.md` (audit actions) and `docs/API.md` (new endpoint) are updated.

## Open questions

None. The gate is the admin role, matching the other `/admin` endpoints. The default month is the current month, explained in behavior 13. The date filter is Month/Year. Details expand inline, so the tab doesn't need a new side-panel component.

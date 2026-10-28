# AmgReports: System Gaps & Roadmap (Summary)

## 🚨 1. CRITICAL (Production Readiness)

- [ ] **Email Infrastructure:** Self-service password reset, email verification, and email delivery via Resend/SendGrid.
- [ ] **Email Notifications:** Automatically email users when their reports are approved or unapproved.
  - **Real-World Scenarios for Email Notifications:**
    1. **Bulk Approve:** Batch emails into a single summary (e.g., "5 of your reports were approved today") to prevent spam.
    2. **Changed Mind (Approve -> Unapprove):** Send a follow-up email if status is reverted, including admin's reason.
    3. **Dead Email (Bounces):** Catch bounced emails via webhook (Resend/SendGrid) and mark the user account as "Email Invalid".
    4. **Admin Remarks:** If an admin adds a note during approval, quote it in the email.
    5. **Offline/In-App:** Send real-time UI updates (polling/WebSockets) and show a Toast if the user is logged in.
    6. **Security (Data Privacy):** Do not include sensitive metrics in the email. Just state "Report for [Month] approved." Let them log in to see the data.
- [ ] **Automated Backups:** Verify Neon backups and test a full database restore.
- [ ] **Domain Setup (F14):** Buy a custom domain, configure it on Vercel and Render, and set up the Cloudflare proxy.

## ⚡ 2. HIGH PRIORITY (Admin Efficiency)

- [x] **Bulk Actions:** Select all -> Approve/Unapprove, so reports are not handled one by one (`e5712c6`).
- [ ] **Advanced Filtering & Search:** Search by church name or worker name, and date range filters.
- [ ] **Audit Log CSV Export:** Downloadable logs for security investigations.

## ✨ 3. NICE-TO-HAVE (UX & Analytics)

- [ ] **Mobile PWA:** Installable on phones, with offline support (Service Workers).
- [x] **Advanced Analytics Dashboard:** Partly done: attendance charts (Recharts), period comparison, church filter, submission compliance and CSV/Excel/PDF export shipped; multi-year growth charts remain (build plan #40).

## 🛠️ 4. TECHNICAL DEBT (Maintenance)

- [x] **Refactor UploadAvatar:** The `gocyclo` warning (>15 complexity) in `avatar_handler.go` is fixed (`saveAvatar` and `deleteAvatar` split out).
- [ ] **Resolve 12 Open P3 Findings:** Check the audit ledger (`blueprint/context/findings.md`) for low-priority issues.
- [x] **Update Blueprint Docs:** Refresh `project-plan.md` and the overview files (stale source hash).

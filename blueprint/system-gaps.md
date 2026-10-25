# AmgReports: System Gaps & Roadmap (Summary)

## 🚨 1. CRITICAL (Production Readiness)

- [ ] **Email Infrastructure:** Self-service password reset, email verification, at email delivery via Resend/SendGrid.
- [ ] **Email Notifications:** Auto-email users kapag na-approve/unapprove ang reports.
  - **Real-World Scenarios for Email Notifications:**
    1. **Bulk Approve:** Batch emails into a single summary (e.g., "5 of your reports were approved today") to prevent spam.
    2. **Changed Mind (Approve -> Unapprove):** Send a follow-up email if status is reverted, including admin's reason.
    3. **Dead Email (Bounces):** Catch bounced emails via webhook (Resend/SendGrid) and mark the user account as "Email Invalid".
    4. **Admin Remarks:** If an admin adds a note during approval, quote it in the email.
    5. **Offline/In-App:** Send real-time UI updates (polling/WebSockets) and show a Toast if the user is logged in.
    6. **Security (Data Privacy):** Do not include sensitive metrics in the email. Just state "Report for [Month] approved." Let them log in to see the data.
- [ ] **Automated Backups:** Verify Neon backups at mag-test ng full database restore.
- [ ] **Domain Setup (F14):** Bumili ng custom domain, i-configure sa Vercel at Render, at i-setup ang Cloudflare proxy.

## ⚡ 2. HIGH PRIORITY (Admin Efficiency)

- [ ] **Bulk Actions:** Select all -> Approve/Unapprove (para hindi isa-isa).
- [ ] **Advanced Filtering & Search:** Search by Church Name o Worker Name, at date range filters.
- [ ] **Audit Log CSV Export:** Downloadable logs para sa security investigation.

## ✨ 3. NICE-TO-HAVE (UX & Analytics)

- [ ] **Mobile PWA:** Installable sa cellphone at may offline support (Service Workers).
- [ ] **Advanced Analytics Dashboard:** Visual charts (Recharts) para sa attendance trends at growth.

## 🛠️ 4. TECHNICAL DEBT (Maintenance)

- [ ] **Refactor UploadAvatar:** Fix `gocyclo` warning (>15 complexity) sa `avatar_handler.go` (pending pa ito).
- [ ] **Resolve 12 Open P3 Findings:** Check audit ledger para sa low-priority issues.
- [ ] **Update Blueprint Docs:** Refresh `project-plan.md` at overview files (stale source hash).

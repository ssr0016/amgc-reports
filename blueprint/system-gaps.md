=== AmgReports: System Gaps & Future Features (Notes) ===
Date: 2026-10-24
Status: Post-Feature 33 (Profile Picture) Planning

=== 1. CRITICAL (Must-have for Production) ===
- [ ] **Email Infrastructure (Forgot Password):** 
  - Implement self-service password reset.
  - Setup email provider (e.g., Resend, SendGrid, or AWS SES).
  - Email verification for new accounts.
- [ ] **Automated Backups & Restore:**
  - Verify Neon backup retention.
  - Test a full database restore process.
- [ ] **Finish F14 (Domain Setup):**
  - Buy custom domain.
  - Configure DNS on Vercel & Render.
  - Setup Cloudflare proxy.

=== 2. HIGH PRIORITY (Admin Efficiency) ===
- [ ] **Bulk Actions:**
  - Allow admins to select multiple reports and "Bulk Approve" or "Bulk Unapprove".
  - Bulk user actions (e.g., mass status updates).
- [ ] **Advanced Filtering & Search:**
  - Add search by Church Name or Worker Name in Admin tabs.
  - Date range filters for reports and audit logs.
- [ ] **Audit Log CSV Export (Task 30):**
  - Download audit logs for compliance/security investigations.

=== 3. NICE-TO-HAVE (User Experience & Analytics) ===
- [ ] **Mobile PWA (Progressive Web App):**
  - Make the frontend installable on mobile devices.
  - Add offline support (Service Workers) for poor signal areas.
- [ ] **Advanced Analytics Dashboard:**
  - Visual charts (Recharts) for attendance trends.
  - Identify late-submitting churches.
  - Month-over-month growth for new believers.
- [ ] **Email Notifications:**
  - Notify users when their report is approved/rejected.
  - Notify admins when a new report is submitted.

=== 4. TECHNICAL DEBT / MAINTENANCE ===
- [ ] **Refactor `UploadAvatar` function:** Fix the `gocyclo` error (complexity 26 > 15) by extracting validation logic into smaller helper functions.
- [ ] **Update Blueprint Docs:** Refresh `project-plan.md` and overview files (source hash is stale).
- [ ] **Resolve 12 Open P3 Findings:** Check the audit ledger for remaining low-priority issues.

=== NEXT ACTION ===
Once Codex/Claude quota resets:
1. Fix the `UploadAvatar` linter issue.
2. Commit & Push Feature 33.
3. Start on Email Infrastructure.

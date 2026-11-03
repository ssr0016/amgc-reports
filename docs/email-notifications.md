# Email notifications

Report email notifications use a durable PostgreSQL outbox and a background worker.
The report transition and outbox enqueue happen in the same transaction. Email delivery
can therefore be retried without losing the report-status change.

## Part A: operation

### 1. What sends email

The worker sends two report-status messages:

- `approved`: sent when an admin approves a report.
- `needs_revision`: sent when an admin requests changes.

Recipients are captured when the report transition occurs. The current recipient set is
the report owner plus the acting admin, with duplicate email addresses removed. The
outbox stores the recipient email, report month and year, event type, and admin remark.

Email templates do not include report metrics or other sensitive report data.

The worker starts only when all three email variables are configured:

```text
RESEND_API_KEY
EMAIL_FROM
EMAIL_REPLY_TO
```

Production refuses to start with an incomplete email configuration. Outside production,
leaving all three variables empty disables delivery and logs `email delivery disabled
(dev mode)`.

### 2. Outbox states and retries

`report_notification_outbox` stores durable delivery work. Rows use these states:

| State | Meaning |
| --- | --- |
| `pending` | Ready for delivery. |
| `claimed` | Leased by a worker. |
| `failed` | Retryable delivery failure; `next_attempt_at` controls the next try. |
| `sent` | Provider accepted the message. |
| `skipped` | Delivery will not be retried. |

The worker polls every 10 seconds, leases rows for five minutes, and retries up to five
attempts. Retry delay starts at one minute, doubles per attempt, and caps at one hour.
Non-retryable provider errors and exhausted retries become `skipped`.

### 3. Inspect delivery

Run this query against the application database:

```sql
SELECT
    id,
    idempotency_key,
    event_type,
    report_id,
    recipient_email,
    state,
    attempts,
    next_attempt_at,
    sent_at,
    skipped_at,
    skip_reason,
    last_error,
    created_at,
    updated_at
FROM report_notification_outbox
ORDER BY id DESC
LIMIT 100;
```

Useful filters:

```sql
-- Failed or skipped rows needing review
SELECT *
FROM report_notification_outbox
WHERE state IN ('failed', 'skipped')
ORDER BY updated_at DESC;

-- Delivery history for one report
SELECT *
FROM report_notification_outbox
WHERE report_id = 123
ORDER BY id;
```

Do not expose `last_error`, recipient addresses, or report metadata in public API
responses or logs.

### 4. Manual replay

Use manual replay only after checking the row and correcting the delivery problem. Replay
failed or skipped rows by resetting the row for the worker:

```sql
UPDATE report_notification_outbox
SET state = 'pending',
    attempts = 0,
    next_attempt_at = NOW(),
    claimed_at = NULL,
    sent_at = NULL,
    skipped_at = NULL,
    skip_reason = NULL,
    last_error = NULL,
    updated_at = NOW()
WHERE id = 123
  AND state IN ('failed', 'skipped')
RETURNING id, idempotency_key, state, next_attempt_at;
```

The replay uses the same `idempotency_key`, so if the worker accidentally re-processes a row that was already sent, ON CONFLICT DO NOTHING in the outbox enqueue prevents duplicates at the report-service level. This manual replay is only for rows that failed or were skipped; it is not a mechanism to resend already-sent emails.

Do not reset a `sent` row. A new delivery requires a new report event and a new
idempotency key.

## Part B: rollback and verification

### 5. Rollback

Rollback means disabling delivery while keeping report operations available. Use the
development configuration path:

1. Set `APP_ENV=development`.
2. Remove `RESEND_API_KEY`, `EMAIL_FROM`, and `EMAIL_REPLY_TO` from the runtime environment.
3. Restart or redeploy the API.
4. Confirm the log contains `email delivery disabled (dev mode)`.
5. Confirm report approval and request-revision actions still succeed.
6. Leave existing outbox rows intact for later inspection or replay.

Do not run the notification migration down as part of this rollback. Dropping the
outbox would destroy delivery history and pending work. Re-enable delivery by restoring
the three email variables and restarting the API.

### 6. Smoke test

Run the smoke test after enabling delivery, after a deployment, and after rollback.

1. Create or select a test report and a test owner account.
2. Approve the report as an admin.
3. Confirm one `approved` row appears in `report_notification_outbox` for each distinct recipient.
4. Confirm the worker changes each row to `sent` and the provider receives the message.
5. Request changes on the same report with an admin remark.
6. Confirm one `needs_revision` row per distinct recipient and verify the remark appears in the message.
7. Verify the message contains no report metrics or sensitive report data.
8. Repeat the same transition request and confirm the existing idempotency key does not create a duplicate outbox row.
9. Temporarily disable or reject delivery, then confirm the row becomes `failed` or `skipped` according to the provider response.
10. Reset one failed or skipped row using the replay query and confirm one worker attempt follows.
11. Remove the email variables, restart with `APP_ENV=development`, and confirm report transitions still work while no worker sends email.

The smoke test does not prove provider inbox placement, bounce handling, or spam-folder
delivery. Check provider delivery events separately when those signals are available.

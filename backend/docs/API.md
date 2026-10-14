# API Documentation

Base URL: http://localhost:8080/api/v1

## Authentication Flow

### 1. Accounts

There is no self-registration. An admin creates every account with `POST /admin/users`
(see "Create User" under Admin Endpoints) and gives the credentials to the user.

### 2. Login

POST /auth/login

    {
      "email": "user@example.com",
      "password": "StrongPass123!"
    }

Response (200): the user. Sets app_session cookie.

### 3. Authenticated Requests

Include session cookie with each request:

    curl -b cookies.txt http://localhost:8080/api/v1/auth/me

`/auth/me` returns `user_id`, `name`, `email`, `role`, `permissions`, and the profile text the
admin set for the account: `area_assignment` and `church_name` (empty strings when unset).
The report form starts from these, and from `name` for the report's worker.

### 4. Logout

POST /auth/logout

Response (200):

    {"message": "logged out"}

## Users

### List Users (paginated, admin role only)

GET /users?page=1&limit=10&email=user

Response (200):

    {
      "data": [...],
      "meta": {
        "page": 1,
        "limit": 10,
        "total": 42,
        "total_pages": 5
      }
    }

### Get User (admin role only)

GET /users/:id

## Admin Endpoints

Requires role: admin.

### List Roles (paginated)

GET /admin/roles?page=1&limit=10

### Create Role

POST /admin/roles

    {
      "name": "manager",
      "description": "Manager role"
    }

### Assign Permission

POST /admin/roles/:id/permissions

    {
      "permission_id": 1
    }

### List Permissions (paginated)

GET /admin/permissions?page=1&limit=10&resource=users

### Change User Role

PUT /admin/users/:id/role

    {
      "role_id": 2
    }

### Create User

POST /admin/users (admin role and `users:write`; this is the only way to create an account)

    {
      "email": "pastor@example.com",
      "name": "Pia",
      "password": "StrongPass123!",
      "role_id": 2,
      "area_assignment": "Metro Manila",
      "church_name": "AMGC Church"
    }

`email`, `name` and `password` are required. `role_id` is optional and defaults to the `user`
role. The two profile fields, `area_assignment` and `church_name`, are optional (150 characters each) and prefill the report form; a report's worker defaults to the user's `name`.
Response (201): the user, without the password. 409 if the email is taken, 422 for invalid
fields or an unknown `role_id`. The account and its audit row (`CREATE_USER`) are written in
one transaction. No verification token is created: email verification is not enforced at login.

### Update User

PUT /admin/users/:id (admin role and `users:write`)

    {
      "name": "Pia",
      "area_assignment": "Metro Manila",
      "church_name": "AMGC Church"
    }

Replaces the name and the two profile fields; text left out is cleared. The password and the
role are not changed here (use Change User Role). Response (200): the user. 404 for an unknown
user. Audited as `UPDATE_USER`.

## Reports

All Reports endpoints need a session cookie, and POST/PUT/PATCH/DELETE also need the CSRF token (see CSRF Protection). Each route needs one permission. A role may also hold an `_any` permission, which widens the scope from the caller's own reports to all reports.

| Method | Path | Permission | `_any` widening |
|---|---|---|---|
| GET | /reports | `reports:read` | `reports:read_any` lists everyone's reports |
| POST | /reports | `reports:write` | |
| GET | /reports/:id | `reports:read` | `reports:read_any` |
| PUT | /reports/:id | `reports:write` | `reports:write_any` |
| DELETE | /reports/:id | `reports:delete` | `reports:delete_any` |
| PATCH | /reports/:id/complete | `reports:approve` | none: works on any report by ID |

Note: IDs may have gaps. A failed create consumes a sequence value, so do not assume contiguous IDs.

Default roles: `user` has read, write and delete (own reports only). `editor` and `admin` have all seven. Another person's report returns 404, not 403.

### Create Report

POST /reports

    {
      "month": "January",
      "year": 2026,
      "worker": "Ana",
      "area_assignment": "North",
      "church_name": "Grace",
      "names": "",
      "narrative_report": "...",
      "challenges": "",
      "prayer_request": "",
      "metrics": {
        "tithes_offering": {"week1": 120.00, "week3": 0.5},
        "outreach": {"week2": 0}
      }
    }

Only `month` is required. `year` defaults to the current year (2000 to 2100). Empty strings are stored as NULL (D-8) and left out of responses. Metric keys are the 21 snake_case keys (for example `worship_service`, `tithes_offering`, `family_day`; see 05-Backend-Schema.md section 2.4). Weeks are `week1` to `week5`; send only the weeks you have. A week that is absent means "not reported", which is different from `0`. Values are JSON numbers from 0 to 9999999999.99 with at most 2 decimals.

Response (201): a report.

    {
      "id": 1,
      "created_by": 2,
      "month": "January",
      "year": 2026,
      "worker": "Ana",
      "metrics": {
        "tithes_offering": {"week1": 120.00, "week3": 0.50},
        "outreach": {"week2": 0}
      },
      "completed": false,
      "created_at": "2026-01-01T00:00:00Z",
      "updated_at": "2026-01-01T00:00:00Z"
    }

Numbers keep the database scale, so `0.5` comes back as `0.50`. `completed_at` and `completed_by` appear once approved. Errors: 409 if you already have an active report for that month and year; 422 for an unknown month or metric key, a value with more than 2 decimals, or a negative value.

### List Reports

GET /reports?page=1&limit=20&month=January&year=2026&worker=ana&area=north&church=grace

Newest first. `month` matches the month name ignoring case; `worker`, `area` and `church` are partial, case-insensitive matches (`%` and `_` are treated literally). `limit` is 1 to 100. Without `reports:read_any` the list contains only your own reports.

Response (200):

    {
      "data": [ { "id": 1, "...": "..." } ],
      "meta": {"page": 1, "limit": 20, "total": 1, "total_pages": 1}
    }

### Get Report

GET /reports/:id

Response (200): a report. 404 if it does not exist, was deleted, or belongs to someone else (without `reports:read_any`).

### Update Report

PUT /reports/:id

Same body as create. This replaces the report: every text field you leave out is cleared, and the whole `metrics` set is replaced. Send every field. A missing `year` keeps the report's current year. Response (200): the updated report. Errors: 404 (as for get), 409 if the new month and year collide with another of your reports, 422.

### Delete Report

DELETE /reports/:id

Soft delete. Response: 204 with no body. 404 if missing, already deleted, or someone else's (without `reports:delete_any`). The month and year can be filed again afterwards.

### Approve or Unapprove Report

PATCH /reports/:id/complete

No body. Flips the completed flag: an open report becomes completed, a completed one is reopened. Response (200) has metadata only, never report text:

    {"id": 1, "completed": true, "completed_at": "2026-01-02T00:00:00Z", "completed_by": 3}

When reopened, `completed` is `false` and the other two fields are left out. Use GET /reports/:id to read the report. 404 if it is missing or deleted.

### Reports Rate Limit

POST, PUT, DELETE and PATCH on /reports share one bucket per signed-in user: 60 per minute with a burst of 10 by default (`REPORTS_WRITE_RATE_LIMIT`, `REPORTS_WRITE_BURST`). Over the limit returns 429 with a `Retry-After` header. GET requests have their own limits (below).

## Error Responses

### 400 Bad Request

    {"error": "BAD_REQUEST", "message": "invalid request body"}

### 401 Unauthorized

    {"error": "UNAUTHORIZED", "message": "authentication required"}

### 403 Forbidden

    {"error": "FORBIDDEN", "message": "insufficient permissions"}

### 404 Not Found

    {"error": "NOT_FOUND", "message": "resource not found"}

### 409 Conflict

    {"error": "CONFLICT", "message": "email already taken"}

### 422 Validation Error

    {
      "error": "VALIDATION_ERROR",
      "message": "Key: 'CreateUserRequest.Email' Error:Field validation..."
    }

### Global and read limits

Every `/api/v1` route has a per-IP limit: 120 per minute with a burst of 60. GET /reports and GET /reports/:id also share one bucket per signed-in user: 60 per minute with a burst of 20. The read limit runs after the permission check. `/health`, `/ready`, `/live` and `/metrics` are outside `/api/v1` and are never limited. The values are constants in `cmd/api/main.go`, not environment variables. Over the limit returns 429 with a `Retry-After` header.

### 429 Rate Limit

    {"error": "RATE_LIMIT_EXCEEDED", "message": "rate limit exceeded"}

## CSRF Protection

For POST/PUT/PATCH/DELETE requests, include CSRF token:

1. GET /api/v1/auth/csrf - returns `{"csrf_token": "<token>"}` and sets the HttpOnly `csrf_token` cookie
   (any GET also sets the cookie, but only this endpoint exposes the value to JavaScript)
2. Include header: X-CSRF-Token: <token>

Browser clients must call the endpoint with `credentials: "include"` and send the returned
token in the `X-CSRF-Token` header on every POST/PUT/DELETE.

Example:

    curl -c cookies.txt http://localhost:8080/health > /dev/null
    CSRF=$(grep csrf_token cookies.txt | awk '{print $NF}')

    curl -X POST http://localhost:8080/api/v1/auth/login \
      -H "Content-Type: application/json" \
      -H "X-CSRF-Token: $CSRF" \
      -b cookies.txt -c cookies.txt \
      -d '{"email": "<ADMIN_EMAIL>", "password": "<ADMIN_PASSWORD>"}'

## Complete Example

Full auth flow:

    # 1. Get CSRF token
    curl -c cookies.txt http://localhost:8080/health > /dev/null
    CSRF=$(grep csrf_token cookies.txt | awk '{print $NF}')

    # 2. An admin creates the account first (POST /admin/users, see "Create User");
    #    there is no self-registration. Then log in as that user:

    # 3. Login
    curl -X POST http://localhost:8080/api/v1/auth/login \
      -H "Content-Type: application/json" \
      -H "X-CSRF-Token: $CSRF" \
      -b cookies.txt -c cookies.txt \
      -d '{"email": "user@example.com", "password": "StrongPass123!"}'

    # 4. Get current user
    curl -b cookies.txt http://localhost:8080/api/v1/auth/me

    # 5. List users (paginated; admin session only, any other role gets 403)
    curl -b cookies.txt "http://localhost:8080/api/v1/users?page=1&limit=10"

---

## Email Verification

Registration automatically generates a verification token.

In dev: token is logged via slog:
  msg="verification token generated" user_id=X token=XYZ

### Verify Email

GET /api/v1/auth/verify-email?token=XXX

Response (200):
  {"message": "email verified successfully"}

Errors:
- 400 - invalid or expired token
- 400 - token already used

## Password Reset

### Request Reset

POST /api/v1/auth/forgot-password

Request:
  {"email": "user@example.com"}

Response (200):
  {"message": "if the email exists, a reset link has been sent"}

Note: Always returns 200 (security - don't reveal if email exists).

### Reset Password

POST /api/v1/auth/reset-password

Request:
  {
    "token": "reset-token-from-email",
    "new_password": "NewStrongPass123x"
  }

Response (200):
  {"message": "password reset successful"}

Errors:
- 400 - invalid or expired token
- 400 - token already used

## Account Lockout

After 5 failed login attempts, account is locked for 15 minutes.

### Login When Locked

POST /api/v1/auth/login

Response (403):
  {"error": "FORBIDDEN", "message": "account is locked, try again later"}

### Auto-Unlock

After 15 minutes, account auto-unlocks.
Successful login resets the counter.

### Check Lock Status

SQL:
  SELECT email, failed_login_attempts, locked_until 
  FROM users WHERE email = 'user@example.com';

## Complete Auth Flow

Full example with email verify + password reset:

1. An admin creates the account
   POST /api/v1/admin/users
   Body: {"email":"user@example.com","name":"User","password":"StrongPass123"}

2. Email verification is not enforced at login, and an admin-created account gets no token
   (email delivery arrives with E-1). The verify endpoint still accepts existing tokens.

3. Verify email
   GET /api/v1/auth/verify-email?token=XXX

4. Login
   POST /api/v1/auth/login
   Body: {"email":"user@example.com","password":"StrongPass123"}

5. Forgot password (if needed)
   POST /api/v1/auth/forgot-password
   Body: {"email":"user@example.com"}

6. Reset password (get token from logs/email)
   POST /api/v1/auth/reset-password
   Body: {"token":"XXX","new_password":"NewStrongPass123x"}

## Pagination

All list endpoints support pagination:

| Param | Default | Max |
|---|---|---|
| page | 1 | - |
| limit | 20 | 100 |

Examples:
  GET /api/v1/users?page=2&limit=10
  GET /api/v1/admin/roles?page=1&limit=5
  GET /api/v1/admin/permissions?resource=users&page=1&limit=10

Response:
  {
    "data": [...],
    "meta": {
      "page": 2,
      "limit": 10,
      "total": 42,
      "total_pages": 5
    }
  }

## RBAC (Admin Endpoints)

Requires role: admin.

### List Roles
  GET /api/v1/admin/roles?page=1&limit=10

### Create Role
  POST /api/v1/admin/roles
  Body: {"name":"manager","description":"Manager role"}

### Assign Permission to Role
  POST /api/v1/admin/roles/:id/permissions
  Body: {"permission_id": 1}

### Revoke Permission
  DELETE /api/v1/admin/roles/:id/permissions/:pid

### List Permissions
  GET /api/v1/admin/permissions?page=1&limit=10&resource=users

### Create Permission
  POST /api/v1/admin/permissions
  Body: {"name":"posts:read","resource":"posts","action":"read"}

### Change User Role
  PUT /api/v1/admin/users/:id/role
  Body: {"role_id": 2}

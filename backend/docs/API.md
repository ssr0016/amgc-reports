# API Documentation

Base URL: http://localhost:8080/api/v1

## Authentication Flow

### 1. Register

POST /auth/register

    {
      "email": "user@example.com",
      "name": "User Name",
      "password": "StrongPass123!"
    }

Response (201):

    {
      "id": 1,
      "email": "user@example.com",
      "name": "User Name",
      "role_id": 1,
      "created_at": "2026-01-01T00:00:00Z"
    }

### 2. Login

POST /auth/login

    {
      "email": "user@example.com",
      "password": "StrongPass123!"
    }

Response (200): Same as register. Sets app_session cookie.

### 3. Authenticated Requests

Include session cookie with each request:

    curl -b cookies.txt http://localhost:8080/api/v1/auth/me

### 4. Logout

POST /auth/logout

Response (200):

    {"message": "logged out"}

## Users

### List Users (paginated)

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

### Get User

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
      "message": "Key: 'RegisterRequest.Email' Error:Field validation..."
    }

### 429 Rate Limit

    {"error": "RATE_LIMIT_EXCEEDED", "message": "rate limit exceeded"}

## CSRF Protection

For POST/PUT/DELETE requests, include CSRF token:

1. GET /health - receives csrf_token cookie
2. Include header: X-CSRF-Token: <token>

Example:

    curl -c cookies.txt http://localhost:8080/health > /dev/null
    CSRF=$(grep csrf_token cookies.txt | awk '{print $NF}')

    curl -X POST http://localhost:8080/api/v1/auth/login \
      -H "Content-Type: application/json" \
      -H "X-CSRF-Token: $CSRF" \
      -b cookies.txt -c cookies.txt \
      -d '{"email": "admin@example.com", "password": "AdminPass123!"}'

## Complete Example

Full auth flow:

    # 1. Get CSRF token
    curl -c cookies.txt http://localhost:8080/health > /dev/null
    CSRF=$(grep csrf_token cookies.txt | awk '{print $NF}')

    # 2. Register
    curl -X POST http://localhost:8080/api/v1/auth/register \
      -H "Content-Type: application/json" \
      -H "X-CSRF-Token: $CSRF" \
      -b cookies.txt -c cookies.txt \
      -d '{"email": "user@example.com", "name": "User", "password": "StrongPass123!"}'

    # 3. Login
    curl -X POST http://localhost:8080/api/v1/auth/login \
      -H "Content-Type: application/json" \
      -H "X-CSRF-Token: $CSRF" \
      -b cookies.txt -c cookies.txt \
      -d '{"email": "user@example.com", "password": "StrongPass123!"}'

    # 4. Get current user
    curl -b cookies.txt http://localhost:8080/api/v1/auth/me

    # 5. List users (paginated)
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

1. Register
   POST /api/v1/auth/register
   Body: {"email":"user@example.com","name":"User","password":"StrongPass123"}

2. Check token in logs (dev) or email (prod)
   Look for: "verification token generated"

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

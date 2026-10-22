package router

import "github.com/labstack/echo/v4"

// noAuthn stands in for the Authenticate middleware: the tests put the request's
// user in the context themselves, before the route groups run.
func noAuthn(next echo.HandlerFunc) echo.HandlerFunc { return next }

// reviewerPerms is a custom role holding every admin-area permission (37b), plus
// users:write and roles:write, which must still not open any admin-only route.
var reviewerPerms = []string{
	"reports:read", "reports:read_any", "reports:approve", "reports:export",
	"tracker:read", "users:list", "audit:read", "users:write", "roles:write",
}

// plainUserPerms are the seeded user role's: every worker reads every report.
var plainUserPerms = []string{"users:read", "reports:read", "reports:read_any", "reports:write", "reports:delete"}

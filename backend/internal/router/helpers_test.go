package router

import "github.com/labstack/echo/v4"

// noAuthn stands in for the Authenticate middleware: the tests put the request's
// user in the context themselves, before the route groups run.
func noAuthn(next echo.HandlerFunc) echo.HandlerFunc { return next }

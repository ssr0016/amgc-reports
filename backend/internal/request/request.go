// Package request carries the signed-in caller through the request context, so
// handlers, middleware and services do not depend on how the caller was
// authenticated.
package request

import (
	"context"
	"time"
)

// User is the authenticated caller. The role and permissions are not carried;
// middleware.LoadCaller reads them live from the database once per request.
type User struct {
	ID int64
	// SessionID names the sign-in (refresh-token family) the request came from, and
	// IssuedAt is when its access token was signed. Both are empty in a test that
	// builds a User by hand.
	SessionID string
	IssuedAt  time.Time
}

type userKey struct{}

// WithUser returns ctx carrying u.
func WithUser(ctx context.Context, u User) context.Context {
	return context.WithValue(ctx, userKey{}, u)
}

// UserFrom returns the caller in ctx. ok is false when no one is signed in.
func UserFrom(ctx context.Context) (User, bool) {
	u, ok := ctx.Value(userKey{}).(User)
	return u, ok && u.ID != 0
}

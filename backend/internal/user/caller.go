package user

import "context"

// Caller is the signed-in user with their role and its permission names, read in
// one query by middleware.LoadCaller and kept in the request context, so the
// route gates and the services after it never load them again.
type Caller struct {
	*User
	Permissions map[string]bool
}

type callerKey struct{}

// WithCaller returns ctx carrying c.
func WithCaller(ctx context.Context, c *Caller) context.Context {
	return context.WithValue(ctx, callerKey{}, c)
}

// CallerFrom returns the caller LoadCaller put in ctx, if it is the user id.
// ok is false outside a request that went through LoadCaller, or for another user.
func CallerFrom(ctx context.Context, id int64) (*Caller, bool) {
	c, ok := ctx.Value(callerKey{}).(*Caller)
	return c, ok && c.ID == id
}

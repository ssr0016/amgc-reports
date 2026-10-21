package userimpl

import "context"

// testPassword satisfies the strongpass rule.
const testPassword = "X7kP9mQ2vL8nR4tY6wB3zC5dF1gH0jK"

// fakeTx runs fn directly. The in-memory mocks have no transaction to roll back.
type fakeTx struct{}

func (fakeTx) InTransaction(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}

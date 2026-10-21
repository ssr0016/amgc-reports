package repository

import "context"

// TxRunner runs fn in one transaction and passes it a ctx that carries it;
// repository calls made with that ctx join it. Services depend on it instead of
// the database, so tests can substitute a fake. *database.DB satisfies it.
type TxRunner interface {
	InTransaction(ctx context.Context, fn func(ctx context.Context) error) error
}

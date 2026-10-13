package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
)

// fakeTx records Commit and Rollback; every other pgx.Tx method panics via the
// nil embedded interface, which would flag an unexpected call.
type fakeTx struct {
	pgx.Tx
	commitErr error
	commits   int
	rollbacks int
}

func (f *fakeTx) Commit(context.Context) error { f.commits++; return f.commitErr }
func (f *fakeTx) Rollback(context.Context) error {
	f.rollbacks++
	return nil
}

type fakeBeginner struct {
	tx       *fakeTx
	beginErr error
}

func (b *fakeBeginner) Begin(context.Context) (pgx.Tx, error) {
	if b.beginErr != nil {
		return nil, b.beginErr
	}
	return b.tx, nil
}

func TestWithTx(t *testing.T) {
	errBoom := errors.New("boom")
	errBegin := errors.New("no connection")
	errCommit := errors.New("commit failed")

	tests := []struct {
		name          string
		beginErr      error
		commitErr     error
		fn            func(Querier) error
		wantErr       error
		wantCommits   int
		wantRollbacks int
	}{
		{
			name:          "nil return commits",
			fn:            func(Querier) error { return nil },
			wantCommits:   1,
			wantRollbacks: 1, // deferred; a no-op on a real committed tx
		},
		{
			name:          "error rolls back and is returned",
			fn:            func(Querier) error { return errBoom },
			wantErr:       errBoom,
			wantRollbacks: 1,
		},
		{
			name:          "commit failure is wrapped",
			commitErr:     errCommit,
			fn:            func(Querier) error { return nil },
			wantErr:       errCommit,
			wantCommits:   1,
			wantRollbacks: 1, // the deferred rollback is a no-op on a real tx
		},
		{
			name:     "begin failure never calls fn",
			beginErr: errBegin,
			fn:       func(Querier) error { t.Fatal("fn must not run"); return nil },
			wantErr:  errBegin,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tx := &fakeTx{commitErr: tt.commitErr}
			err := WithTx(context.Background(), &fakeBeginner{tx: tx, beginErr: tt.beginErr}, tt.fn)

			if tt.wantErr == nil {
				require.NoError(t, err)
			} else {
				require.ErrorIs(t, err, tt.wantErr)
			}
			require.Equal(t, tt.wantCommits, tx.commits)
			require.Equal(t, tt.wantRollbacks, tx.rollbacks)
		})
	}
}

func TestWithTx_PanicRollsBackAndRepanics(t *testing.T) {
	tx := &fakeTx{}

	require.PanicsWithValue(t, "kaboom", func() {
		_ = WithTx(context.Background(), &fakeBeginner{tx: tx}, func(Querier) error {
			panic("kaboom")
		})
	})
	require.Equal(t, 0, tx.commits)
	require.Equal(t, 1, tx.rollbacks)
}

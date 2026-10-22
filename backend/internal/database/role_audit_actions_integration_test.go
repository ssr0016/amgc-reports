//go:build integration

package database_test

import (
	"context"
	"database/sql"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/require"
)

// The role actions (D-A21) are allowed after Up and refused for new rows after Down;
// Up again brings them back.
func TestRoleAuditActions_UpAndDown(t *testing.T) {
	tdb := migratedDB(t)
	ctx := context.Background()
	insert := func() error {
		_, err := tdb.Pool.Exec(ctx,
			`INSERT INTO audit_logs (action, target_type) VALUES ('CREATE_ROLE', 'Role')`)
		return err
	}
	require.NoError(t, insert(), "Up allows CREATE_ROLE")

	db := sql.OpenDB(stdlib.GetPoolConnector(tdb.Pool))
	defer func() { _ = db.Close() }()
	goose.SetBaseFS(os.DirFS("migrations"))
	require.NoError(t, goose.SetDialect("postgres"))
	require.NoError(t, goose.DownTo(db, ".", 20261022120000), "Down keeps the row already written (NOT VALID)")
	require.Error(t, insert(), "Down refuses new role rows")

	tdb.MigrateWithGoose(t)
	require.NoError(t, insert(), "Up allows them again")
}

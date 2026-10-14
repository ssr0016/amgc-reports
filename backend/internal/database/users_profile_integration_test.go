//go:build integration

package database_test

import (
	"context"
	"database/sql"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A report's worker defaults to the user's name, so users has no worker column of
// its own; only the area and church are profile fields.
func TestUsersProfileColumns_WorkerRemovedAndRestorable(t *testing.T) {
	tdb := migratedDB(t)
	has := func(column string) bool {
		var found bool
		require.NoError(t, tdb.Pool.QueryRow(context.Background(),
			`SELECT EXISTS (SELECT 1 FROM information_schema.columns
			   WHERE table_schema = 'public' AND table_name = 'users' AND column_name = $1)`, column).Scan(&found))
		return found
	}

	assert.False(t, has("worker"), "users.worker is redundant with users.name")
	assert.True(t, has("area_assignment"))
	assert.True(t, has("church_name"))

	// Down brings back an empty, nullable column; Up removes it again.
	db := sql.OpenDB(stdlib.GetPoolConnector(tdb.Pool))
	defer func() { _ = db.Close() }()
	goose.SetBaseFS(os.DirFS("migrations"))
	require.NoError(t, goose.SetDialect("postgres"))
	// DownTo, not Down: Down undoes only the newest migration, which is not this one any more.
	require.NoError(t, goose.DownTo(db, ".", 20261014100100))
	assert.True(t, has("worker"), "down restores the column")

	var nullable string
	require.NoError(t, tdb.Pool.QueryRow(context.Background(),
		`SELECT is_nullable FROM information_schema.columns WHERE table_name = 'users' AND column_name = 'worker'`).Scan(&nullable))
	assert.Equal(t, "YES", nullable)

	tdb.MigrateWithGoose(t)
	assert.False(t, has("worker"), "up removes it again")
}

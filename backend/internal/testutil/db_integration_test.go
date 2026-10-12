//go:build integration

package testutil

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRemoveRole_RestoresAfterTest(t *testing.T) {
	tdb := SetupPostgres(t)
	tdb.RunMigrations(t)
	ctx := context.Background()

	const name, description = "user", "a description worth restoring"
	_, err := tdb.Pool.Exec(ctx, `UPDATE roles SET description = $2 WHERE name = $1`, name, description)
	require.NoError(t, err)

	roleDescription := func() (string, bool) {
		var got string
		err := tdb.Pool.QueryRow(ctx, `SELECT description FROM roles WHERE name = $1`, name).Scan(&got)
		return got, err == nil
	}

	// The role is removed inside this subtest and must be back once its
	// cleanup has run, i.e. before the next subtest starts.
	t.Run("role is gone while the test runs", func(t *testing.T) {
		tdb.RemoveRole(t, name)

		_, found := roleDescription()
		require.False(t, found, "RemoveRole must delete the role")
	})

	t.Run("role is restored with its description afterwards", func(t *testing.T) {
		got, found := roleDescription()
		require.True(t, found, "t.Cleanup must restore the role")
		require.Equal(t, description, got)
	})
}

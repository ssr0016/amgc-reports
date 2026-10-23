//go:build integration

package userimpl

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// GetCaller reads the user, role and the role's permission names in one query; a
// role without permissions gives an empty set, and a missing user nil, nil.
func TestUserRepo_GetCaller(t *testing.T) {
	repo, tdb := setupTestRepo(t)
	ctx := context.Background()

	u, err := repo.CreateWithPassword(ctx, "caller@example.com", "Caller", "hash")
	require.NoError(t, err)

	c, err := repo.GetCaller(ctx, u.ID)
	require.NoError(t, err)
	require.NotNil(t, c)
	require.NotNil(t, c.Role)
	assert.Equal(t, u.ID, c.ID)
	assert.NotNil(t, c.Permissions)

	_, err = tdb.Pool.Exec(ctx, `
		WITH p AS (
			INSERT INTO permissions (name, resource, action) VALUES ('caller:one', 'caller', 'one'), ('caller:two', 'caller', 'two')
			ON CONFLICT (name) DO UPDATE SET name = EXCLUDED.name RETURNING id)
		INSERT INTO role_permissions (role_id, permission_id) SELECT $1, id FROM p ON CONFLICT DO NOTHING`, u.RoleID)
	require.NoError(t, err)

	c, err = repo.GetCaller(ctx, u.ID)
	require.NoError(t, err)
	assert.True(t, c.Permissions["caller:one"])
	assert.True(t, c.Permissions["caller:two"])
	assert.False(t, c.Permissions["caller:three"])

	missing, err := repo.GetCaller(ctx, u.ID+1000)
	require.NoError(t, err)
	assert.Nil(t, missing)
}

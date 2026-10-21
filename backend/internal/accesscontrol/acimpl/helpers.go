package acimpl

import (
	"context"
	"fmt"

	sq "github.com/Masterminds/squirrel"

	"github.com/ssr0016/pastoral-reports/backend/internal/database"
)

// deleteByID deletes a row by ID from the given table.
// Returns an error wrapping database.ErrNotFound if no rows were affected, and one
// wrapping database.ErrForeignKeyViolation if other rows still reference the row.
func deleteByID(ctx context.Context, db *database.DB, table string, id int64, extra ...sq.Sqlizer) error {
	del := db.Builder.Delete(table).Where(sq.Eq{"id": id})
	for _, cond := range extra {
		del = del.Where(cond)
	}
	query, args, err := del.ToSql()
	if err != nil {
		return fmt.Errorf("build query: %w", err)
	}
	result, err := db.Pool.Exec(ctx, query, args...)
	if database.IsForeignKeyViolation(err) {
		return fmt.Errorf("delete %s: %w: %w", table, database.ErrForeignKeyViolation, err)
	}
	if err != nil {
		return fmt.Errorf("delete %s: %w", table, err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("%s: %w", table, database.ErrNotFound)
	}
	return nil
}

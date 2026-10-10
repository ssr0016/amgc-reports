package repository

import (
	"context"
	"fmt"

	sq "github.com/Masterminds/squirrel"

	"github.com/ssr0016/template/internal/database"
)

// deleteByID deletes a row by ID from the given table.
// Returns an error if no rows were affected.
func deleteByID(ctx context.Context, db *database.DB, table string, id int64) error {
	query, args, err := db.Builder.
		Delete(table).
		Where(sq.Eq{"id": id}).
		ToSql()
	if err != nil {
		return fmt.Errorf("build query: %w", err)
	}
	result, err := db.Pool.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("delete %s: %w", table, err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("%s not found", table)
	}
	return nil
}

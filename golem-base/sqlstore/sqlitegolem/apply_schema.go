package sqlitegolem

import (
	"context"
	"database/sql"
	"fmt"

	_ "embed"
)

//go:embed schema.sql
var schema string

func ApplySchema(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, schema)
	if err != nil {
		return fmt.Errorf("failed to apply schema: %w", err)
	}
	return nil
}

func ApplySchemaTx(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, schema)
	if err != nil {
		return fmt.Errorf("failed to apply schema: %w", err)
	}
	return nil
}

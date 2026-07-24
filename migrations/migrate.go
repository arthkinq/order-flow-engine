// Package migrate provides embedded SQL migrations that run automatically at startup.
package migrate

import (
	"database/sql"
	_ "embed"
	"fmt"
	"log"
)

//go:embed 000001_create_orders_table.up.sql
var migrationSQL string

// Up applies all embedded migrations to the database.
// Uses IF NOT EXISTS to make it safe for repeated calls (idempotent).
func Up(db *sql.DB) error {
	log.Println("Applying database migrations...")

	_, err := db.Exec(migrationSQL)
	if err != nil {
		return fmt.Errorf("failed to apply migrations: %w", err)
	}

	log.Println("Database migrations applied successfully.")
	return nil
}

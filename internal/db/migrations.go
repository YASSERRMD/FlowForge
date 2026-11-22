// Package db owns PostgreSQL access for FlowForge: migrations embedded in
// the binary and the shared connection helpers.
package db

import (
	"database/sql"
	"embed"
	"fmt"

	"github.com/pressly/goose/v3"
)

// Migrations holds every SQL migration, embedded at build time so the
// binaries are self-contained and forward-only deploys need no extra files.
//
//go:embed migrations/*.sql
var Migrations embed.FS

func init() {
	goose.SetBaseFS(Migrations)
	if err := goose.SetDialect("postgres"); err != nil {
		panic(fmt.Sprintf("goose dialect: %v", err))
	}
}

// MigrateUp applies every pending migration in version order.
func MigrateUp(sqlDB *sql.DB) error {
	if err := goose.Up(sqlDB, "migrations"); err != nil {
		return fmt.Errorf("migrate up: %w", err)
	}
	return nil
}

// MigrateDown rolls back the single most recent migration. Intended for
// local development and migration tests only — never called in deploys.
func MigrateDown(sqlDB *sql.DB) error {
	if err := goose.Down(sqlDB, "migrations"); err != nil {
		return fmt.Errorf("migrate down: %w", err)
	}
	return nil
}

// MigrateStatus lists applied and pending migrations.
func MigrateStatus(sqlDB *sql.DB) error {
	if err := goose.Status(sqlDB, "migrations"); err != nil {
		return fmt.Errorf("migrate status: %w", err)
	}
	return nil
}

// MigrateVersion reports the current schema version.
func MigrateVersion(sqlDB *sql.DB) (int64, error) {
	v, err := goose.GetDBVersion(sqlDB)
	if err != nil {
		return 0, fmt.Errorf("migrate version: %w", err)
	}
	return v, nil
}

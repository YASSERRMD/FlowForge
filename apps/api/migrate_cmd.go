package main

import (
	"database/sql"
	"flag"
	"fmt"
	"os"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/YASSERRMD/FlowForge/internal/config"
	"github.com/YASSERRMD/FlowForge/internal/db"
)

// runMigrate implements the `api migrate` subcommand: the only path that
// should ever mutate schema in deployed environments.
func runMigrate(cfg config.Config, args []string) int {
	fs := flag.NewFlagSet("migrate", flag.ContinueOnError)
	status := fs.Bool("status", false, "print migration status and exit")
	down := fs.Bool("down", false, "roll back one migration (development only)")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	sqlDB, err := sql.Open("pgx", cfg.DatabaseURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open database: %v\n", err)
		return 1
	}
	defer sqlDB.Close()

	switch {
	case *status:
		if err := db.MigrateStatus(sqlDB); err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			return 1
		}
	case *down:
		if cfg.Env == config.EnvProduction {
			fmt.Fprintln(os.Stderr, "refusing migrate -down in production")
			return 1
		}
		if err := db.MigrateDown(sqlDB); err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			return 1
		}
	default:
		if err := db.MigrateUp(sqlDB); err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			return 1
		}
		v, err := db.MigrateVersion(sqlDB)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			return 1
		}
		fmt.Printf("schema at version %d\n", v)
	}
	return 0
}

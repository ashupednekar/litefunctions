package cmd

import (
	"database/sql"
	"fmt"
	"log"

	"github.com/ashupednekar/litewebservices-portal/pkg"
	"github.com/ashupednekar/litewebservices-portal/pkg/state/connections"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/lib/pq"
	"github.com/pressly/goose/v3"
	"github.com/spf13/cobra"
)

var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Database migration commands",
	Long:  `Run database migrations using goose. Supports 'up' and 'down' subcommands.`,
}

var migrateUpCmd = &cobra.Command{
	Use:   "up",
	Short: "Run all pending migrations",
	Run: func(cmd *cobra.Command, args []string) {
		if err := runMigrations("up"); err != nil {
			log.Fatalf("Migration up failed: %v", err)
		}
		fmt.Println("✓ Migrations completed successfully")
	},
}

var migrateDownCmd = &cobra.Command{
	Use:   "down",
	Short: "Rollback the last migration",
	Run: func(cmd *cobra.Command, args []string) {
		if err := runMigrations("down"); err != nil {
			log.Fatalf("Migration down failed: %v", err)
		}
		fmt.Println("✓ Migration rolled back successfully")
	},
}

func runMigrations(direction string) error {
	connections.ConnectDB()
	if connections.DBPool == nil {
		return fmt.Errorf("failed to connect to database")
	}

	db, err := sql.Open("pgx", pkg.Cfg.DatabaseUrl)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	migrationsDir := "./migrations"

	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("failed to set dialect: %w", err)
	}

  _, err = db.Exec("create schema if not exists " + pq.QuoteIdentifier(pkg.Cfg.DatabaseSchema))
	if err != nil {
		return fmt.Errorf("failed to create schema: %w", err)
	}

	// Set search_path for THIS connection (goose uses the same connection)
	_, err = db.Exec("set search_path to " + pq.QuoteIdentifier(pkg.Cfg.DatabaseSchema))
	if err != nil {
		return fmt.Errorf("failed to set search_path: %w", err)
	}

	switch direction {
	case "up":
		if err := goose.Up(db, migrationsDir); err != nil {
			return fmt.Errorf("failed to run migrations: %w", err)
		}
	case "down":
		if err := goose.Down(db, migrationsDir); err != nil {
			return fmt.Errorf("failed to rollback migration: %w", err)
		}
	default:
		return fmt.Errorf("unknown migration direction: %s", direction)
	}

	return nil
}

func init() {
	rootCmd.AddCommand(migrateCmd)
	migrateCmd.AddCommand(migrateUpCmd)
	migrateCmd.AddCommand(migrateDownCmd)
}

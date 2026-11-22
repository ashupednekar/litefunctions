package cmd

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"os"

	agentmigrations "github.com/ashupednekar/litefunctions/agent/migrations"
	"github.com/ashupednekar/litefunctions/portal/internal/config"
	"github.com/ashupednekar/litefunctions/portal/internal/state/connections"
	"github.com/ashupednekar/litefunctions/portal/migrations"
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
			slog.Error("Migration up failed", "error", err)
			os.Exit(1)
		}
		slog.Info("Migrations completed successfully")
	},
}

var migrateDownCmd = &cobra.Command{
	Use:   "down",
	Short: "Rollback the last migration",
	Run: func(cmd *cobra.Command, args []string) {
		if err := runMigrations("down"); err != nil {
			slog.Error("Migration down failed", "error", err)
			os.Exit(1)
		}
		slog.Info("Migration rolled back successfully")
	},
}

func runMigrations(direction string) error {
	connections.ConnectDB()
	if connections.DBPool == nil {
		return fmt.Errorf("failed to connect to database")
	}

	db, err := sql.Open("pgx", config.Cfg.DatabaseUrl)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	_, err = db.Exec("create schema if not exists " + pq.QuoteIdentifier(config.Cfg.DatabaseSchema))
	if err != nil {
		return fmt.Errorf("failed to create schema: %w", err)
	}

	// Set search_path for THIS connection (goose uses the same connection)
	_, err = db.Exec("set search_path to " + pq.QuoteIdentifier(config.Cfg.DatabaseSchema))
	if err != nil {
		return fmt.Errorf("failed to set search_path: %w", err)
	}

	portalProvider, err := goose.NewProvider(goose.DialectPostgres, db, migrations.Files)
	if err != nil {
		return fmt.Errorf("create portal migration provider: %w", err)
	}
	agentProvider, err := goose.NewProvider(
		goose.DialectPostgres,
		db,
		agentmigrations.Files,
		goose.WithTableName("agent_goose_db_version"),
	)
	if err != nil {
		return fmt.Errorf("create agent migration provider: %w", err)
	}

	ctx := context.Background()
	switch direction {
	case "up":
		if _, err := portalProvider.Up(ctx); err != nil {
			return fmt.Errorf("failed to run portal migrations: %w", err)
		}
		if _, err := agentProvider.Up(ctx); err != nil {
			return fmt.Errorf("failed to run agent migrations: %w", err)
		}
	case "down":
		if _, err := agentProvider.Down(ctx); err != nil && !errors.Is(err, goose.ErrNoNextVersion) {
			return fmt.Errorf("failed to roll back agent migration: %w", err)
		}
		if _, err := portalProvider.Down(ctx); err != nil {
			return fmt.Errorf("failed to roll back portal migration: %w", err)
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

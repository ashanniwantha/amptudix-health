package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"os"

	_ "github.com/jackc/pgx/v5/stdlib" // database/sql driver for pgx

	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/lock"

	"github.com/ashanniwantha/amptudix-health/internal/database/migrations"
)

func main() {
	if err := run(); err != nil {
		slog.Error("migration failed", "error", err)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) < 2 {
		return fmt.Errorf("usage: %s <up|down|status|version>", os.Args[0])
	}
	cmd := os.Args[1]

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		return fmt.Errorf("DATABASE_URL is not set")
	}

	// goose.NewProvider requires a *sql.DB, not a pgxpool.Pool.
	// The pgx stdlib driver registers itself via the blank import above.
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer db.Close()

	// Session-level advisory lock: only one instance runs migrations
	// at a time when multiple replicas start concurrently.
	locker, err := lock.NewPostgresSessionLocker()
	if err != nil {
		return fmt.Errorf("create session locker: %w", err)
	}

	provider, err := goose.NewProvider(
		goose.DialectPostgres,
		db,
		migrations.FS,
		goose.WithSessionLocker(locker),
		goose.WithVerbose(true), // log each applied migration
	)
	if err != nil {
		return fmt.Errorf("create goose provider: %w", err)
	}

	ctx := context.Background()

	switch cmd {
	case "up":
		results, err := provider.Up(ctx)
		if err != nil {
			var partial *goose.PartialError
			if errors.As(err, &partial) {
				for _, r := range partial.Applied {
					slog.Info("applied", "version", r.Source.Version, "path", r.Source.Path)
				}
				slog.Error("failed on", "version", partial.Failed.Source.Version, "error", partial.Err)
			}
			return fmt.Errorf("up: %w", err)
		}
		for _, r := range results {
			slog.Info("applied", "version", r.Source.Version, "path", r.Source.Path, "duration", r.Duration)
		}
		return nil

	case "down":
		result, err := provider.Down(ctx)
		if err != nil {
			return fmt.Errorf("down: %w", err)
		}
		slog.Info("rolled back", "version", result.Source.Version, "path", result.Source.Path)
		return nil

	case "status":
		status, err := provider.Status(ctx)
		if err != nil {
			return fmt.Errorf("status: %w", err)
		}
		for _, s := range status {
			slog.Info("migration",
				"version", s.Source.Version,
				"path", s.Source.Path,
				"state", s.State,
				"applied_at", s.AppliedAt,
			)
		}
		return nil

	case "version":
		v, err := provider.GetDBVersion(ctx)
		if err != nil {
			return fmt.Errorf("version: %w", err)
		}
		fmt.Println(v)
		return nil

	default:
		return fmt.Errorf("unknown command %q: want up, down, status, or version", cmd)
	}
}

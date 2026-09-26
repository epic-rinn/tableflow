// Command migrate applies Goose SQL migrations to PostgreSQL. It is a release
// step run with the owner/migration role; the API never migrates on startup.
//
// Usage: migrate [-dir db/migrations] up|up-by-one|down|status|version
package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "migrate:", err)
		os.Exit(1)
	}
}

func run() error {
	dir := flag.String("dir", "db/migrations", "migration directory")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: migrate [-dir path] up|up-by-one|down|status|version")
		fmt.Fprintln(os.Stderr, "MIGRATION_DATABASE_URL must name the owner/migration role connection.")
	}
	flag.Parse()
	if flag.NArg() != 1 {
		flag.Usage()
		return errors.New("exactly one command required")
	}
	command := flag.Arg(0)
	fsys := os.DirFS(*dir)

	databaseURL := os.Getenv("MIGRATION_DATABASE_URL")
	if databaseURL == "" {
		return errors.New("MIGRATION_DATABASE_URL is required")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return errors.New("invalid MIGRATION_DATABASE_URL")
	}
	defer db.Close()
	p, err := goose.NewProvider(goose.DialectPostgres, db, fsys)
	if err != nil {
		return err
	}

	switch command {
	case "up":
		results, err := p.Up(ctx)
		printResults(results)
		return err
	case "up-by-one":
		result, err := p.UpByOne(ctx)
		printResults([]*goose.MigrationResult{result})
		return err
	case "down":
		result, err := p.Down(ctx)
		printResults([]*goose.MigrationResult{result})
		return err
	case "status":
		statuses, err := p.Status(ctx)
		if err != nil {
			return err
		}
		for _, s := range statuses {
			applied := "pending"
			if s.State == goose.StateApplied {
				applied = "applied " + s.AppliedAt.UTC().Format("2006-01-02T15:04:05Z")
			}
			fmt.Printf("%-14d %-40s %s\n", s.Source.Version, s.Source.Path, applied)
		}
		return nil
	case "version":
		v, err := p.GetDBVersion(ctx)
		if err != nil {
			return err
		}
		fmt.Println(v)
		return nil
	default:
		flag.Usage()
		return fmt.Errorf("unknown command %q", command)
	}
}

func printResults(results []*goose.MigrationResult) {
	for _, r := range results {
		if r != nil {
			fmt.Println(r.String())
		}
	}
}

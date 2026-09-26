// Package dbtest creates disposable, migrated PostgreSQL databases for tests.
//
// TEST_DATABASE_URL must point at a disposable server with permission to
// create databases. Tests skip when it is unset unless TABLEFLOW_REQUIRE_DB=1,
// which `make api-test-db` sets so a missing database fails instead of silently passing.
package dbtest

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	_ "github.com/jackc/pgx/v5/stdlib" // database/sql driver for Goose
	"github.com/pressly/goose/v3"
)

// MigrationsDir returns the absolute path of the Goose migrations.
func MigrationsDir() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "db", "migrations")
}

// AdminURL returns TEST_DATABASE_URL or skips/fails the test.
func AdminURL(t testing.TB) string {
	t.Helper()
	u := os.Getenv("TEST_DATABASE_URL")
	if u == "" {
		if os.Getenv("TABLEFLOW_REQUIRE_DB") == "1" {
			t.Fatal("TEST_DATABASE_URL is required when TABLEFLOW_REQUIRE_DB=1")
		}
		t.Skip("TEST_DATABASE_URL not set; skipping PostgreSQL test")
	}
	return u
}

// NewDatabase creates an empty uniquely named database, dropped on cleanup,
// and returns its connection URL.
func NewDatabase(t testing.TB) string {
	t.Helper()
	admin := AdminURL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	u, err := url.Parse(admin)
	if err != nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") {
		t.Fatalf("TEST_DATABASE_URL must be a postgres:// URL")
	}
	cfg, err := pgx.ParseConfig(admin)
	if err != nil {
		t.Fatalf("parse TEST_DATABASE_URL: invalid connection string")
	}
	conn, err := pgx.ConnectConfig(ctx, cfg)
	if err != nil {
		t.Fatalf("connect to test server: %v", err)
	}
	defer conn.Close(ctx)

	var b [6]byte
	_, _ = rand.Read(b[:])
	name := "tableflow_test_" + hex.EncodeToString(b[:])
	if _, err := conn.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{name}.Sanitize()); err != nil {
		t.Fatalf("create database: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		c, err := pgx.ConnectConfig(ctx, cfg)
		if err != nil {
			t.Errorf("cleanup connect: %v", err)
			return
		}
		defer c.Close(ctx)
		if _, err := c.Exec(ctx, "DROP DATABASE "+pgx.Identifier{name}.Sanitize()+" WITH (FORCE)"); err != nil {
			t.Errorf("drop database %s: %v", name, err)
		}
	})

	u.Path = "/" + name
	return u.String()
}

// Provider opens a Goose provider for the database URL.
func Provider(t testing.TB, databaseURL string) (*goose.Provider, *sql.DB) {
	t.Helper()
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	p, err := goose.NewProvider(goose.DialectPostgres, db, os.DirFS(MigrationsDir()))
	if err != nil {
		t.Fatalf("goose provider: %v", err)
	}
	return p, db
}

// NewMigratedDatabase creates a disposable database with all migrations applied.
func NewMigratedDatabase(t testing.TB) string {
	t.Helper()
	u := NewDatabase(t)
	p, _ := Provider(t, u)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	if _, err := p.Up(ctx); err != nil {
		t.Fatalf("migrate up: %v", err)
	}
	return u
}

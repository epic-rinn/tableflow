package dbtest

import (
	"context"
	"testing"
	"time"

	"github.com/pressly/goose/v3"
)

// FreshMigrationAndDatabaseSmoke: every migration applies to an empty
// PostgreSQL 18 database, reverses to zero, and re-applies cleanly.
func TestFreshMigrationAndDatabaseSmoke(t *testing.T) {
	u := NewDatabase(t)
	p, db := Provider(t, u)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	var serverVersion int
	if err := db.QueryRowContext(ctx, "SELECT current_setting('server_version_num')::integer").Scan(&serverVersion); err != nil {
		t.Fatal(err)
	}
	if serverVersion < 180000 {
		t.Fatalf("test server is PostgreSQL %d; the baseline requires 18+", serverVersion)
	}

	sources := p.ListSources()
	if len(sources) == 0 {
		t.Fatal("no migrations found")
	}
	latest := sources[len(sources)-1].Version

	if _, err := p.Up(ctx); err != nil {
		t.Fatalf("fresh up: %v", err)
	}
	assertVersion(t, ctx, p, latest)

	statuses, err := p.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range statuses {
		if s.State != goose.StateApplied {
			t.Fatalf("migration %d not applied", s.Source.Version)
		}
	}

	if _, err := p.DownTo(ctx, 0); err != nil {
		t.Fatalf("down to zero: %v", err)
	}
	assertVersion(t, ctx, p, 0)

	if _, err := p.Up(ctx); err != nil {
		t.Fatalf("re-apply after down: %v", err)
	}
	assertVersion(t, ctx, p, latest)
	t.Logf("PostgreSQL %d: applied, reversed and re-applied %d migration(s) to version %d", serverVersion, len(sources), latest)
}

func assertVersion(t *testing.T, ctx context.Context, p *goose.Provider, want int64) {
	t.Helper()
	got, err := p.GetDBVersion(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("database version %d, want %d", got, want)
	}
}

// TestUpgradeFromPreviousRelease: a database at the previous migration
// version upgrades to the latest one.
func TestUpgradeFromPreviousRelease(t *testing.T) {
	u := NewDatabase(t)
	p, _ := Provider(t, u)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	sources := p.ListSources()
	if len(sources) < 2 {
		t.Skip("only one migration; no upgrade path yet")
	}
	previous := sources[len(sources)-2].Version
	if _, err := p.UpTo(ctx, previous); err != nil {
		t.Fatalf("up to previous release %d: %v", previous, err)
	}
	if _, err := p.Up(ctx); err != nil {
		t.Fatalf("upgrade from %d: %v", previous, err)
	}
	assertVersion(t, ctx, p, sources[len(sources)-1].Version)
}

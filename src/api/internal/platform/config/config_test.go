package config

import (
	"strings"
	"testing"
	"time"
)

func env(values map[string]string) func(string) string {
	return func(k string) string { return values[k] }
}

func TestLoadDefaults(t *testing.T) {
	cfg, err := Load(env(map[string]string{"DATABASE_URL": "postgres://example.invalid/db"}))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.HTTPAddr != "127.0.0.1:8080" || cfg.DBMaxConns != 10 {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
	if cfg.ReadinessTimeout != time.Second || cfg.ShutdownTimeout != 10*time.Second {
		t.Fatalf("unexpected timeout defaults: %+v", cfg)
	}
}

func TestLoadRejectsInvalidValuesWithoutEchoingSecrets(t *testing.T) {
	_, err := Load(env(map[string]string{
		"DB_MAX_CONNS":      "0",
		"READINESS_TIMEOUT": "-1s",
	}))
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	for _, want := range []string{"DATABASE_URL is required", "DB_MAX_CONNS", "READINESS_TIMEOUT"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error %q missing %q", msg, want)
		}
	}
}

func TestLoadDoesNotEchoInvalidValue(t *testing.T) {
	_, err := Load(env(map[string]string{"DATABASE_URL": "x", "HTTP_WRITE_TIMEOUT": "secret-looking-value"}))
	if err == nil || strings.Contains(err.Error(), "secret-looking-value") {
		t.Fatalf("expected sanitized error, got %v", err)
	}
}

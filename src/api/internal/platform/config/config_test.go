package config

import (
	"strings"
	"testing"
	"time"
)

// testKey is base64 of 32 zero bytes (test-only).
const testKey = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="

func env(values map[string]string) func(string) string {
	return func(k string) string { return values[k] }
}

func TestLoadDefaults(t *testing.T) {
	cfg, err := Load(env(map[string]string{"DATABASE_URL": "postgres://example.invalid/db", "DATA_ENCRYPTION_KEY": testKey}))
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
	for _, want := range []string{"DATABASE_URL is required", "DB_MAX_CONNS", "READINESS_TIMEOUT", "DATA_ENCRYPTION_KEY is required"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error %q missing %q", msg, want)
		}
	}
}

func TestLoadDoesNotEchoInvalidValue(t *testing.T) {
	_, err := Load(env(map[string]string{"DATABASE_URL": "x", "DATA_ENCRYPTION_KEY": testKey, "HTTP_WRITE_TIMEOUT": "secret-looking-value"}))
	if err == nil || strings.Contains(err.Error(), "secret-looking-value") {
		t.Fatalf("expected sanitized error, got %v", err)
	}
}

func TestLoadIdentityDefaultsAndValidation(t *testing.T) {
	cfg, err := Load(env(map[string]string{"DATABASE_URL": "x", "DATA_ENCRYPTION_KEY": testKey}))
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.AdminOrigins) != 2 || cfg.AdminOrigins[0] != "http://localhost:3001" {
		t.Fatalf("admin origins %v", cfg.AdminOrigins)
	}
	if cfg.StaffSessionIdle != time.Hour || cfg.StaffSessionAbsolute != 12*time.Hour || len(cfg.TrustedProxies) != 0 {
		t.Fatalf("identity defaults %+v", cfg)
	}
	for name, value := range map[string]string{
		"ADMIN_ORIGINS":          "https://admin.example/path",
		"TRUSTED_PROXY_CIDRS":    "not-a-cidr",
		"STAFF_SESSION_ABSOLUTE": "48h",
		"STAFF_SESSION_IDLE":     "13h",
		"PWA_ORIGINS":            "not an origin",
		"DATA_ENCRYPTION_KEY":    "c2hvcnQ=",
	} {
		if _, err := Load(env(map[string]string{"DATABASE_URL": "x", "DATA_ENCRYPTION_KEY": testKey, name: value})); err == nil {
			t.Errorf("%s=%q accepted", name, value)
		}
	}
}

func TestMailConfigRequiresTLS(t *testing.T) {
	base := map[string]string{"DATABASE_URL": "x", "DATA_ENCRYPTION_KEY": testKey}
	with := func(kv ...string) map[string]string {
		m := map[string]string{}
		for k, v := range base {
			m[k] = v
		}
		for i := 0; i+1 < len(kv); i += 2 {
			m[kv[i]] = kv[i+1]
		}
		return m
	}
	cfg, err := Load(env(with()))
	if err != nil || cfg.SMTPAddr != "smtp.resend.com:587" || cfg.SMTPTLS != "starttls" || cfg.MailAdapter != "smtp" {
		t.Fatalf("defaults: %+v %v", cfg, err)
	}
	ok := []map[string]string{
		with("SMTP_USERNAME", "resend", "SMTP_PASSWORD", "dummy"),
		with("SMTP_ADDR", "smtp.resend.com:465", "SMTP_TLS", "implicit", "SMTP_USERNAME", "resend", "SMTP_PASSWORD", "dummy"),
		with("MAIL_ADAPTER", "file", "MAIL_OUTBOX_DIR", "/tmp/outbox"),
	}
	bad := []map[string]string{
		with("SMTP_TLS", "none"),
		with("SMTP_TLS", "none", "SMTP_ADDR", "127.0.0.1:1025"),
		with("SMTP_USERNAME", "resend"),
		with("MAIL_ADAPTER", "file"),
		with("MAIL_ADAPTER", "sendmail"),
		with("SMTP_ADDR", "no-port"),
	}
	for i, m := range ok {
		if _, err := Load(env(m)); err != nil {
			t.Errorf("ok[%d] rejected: %v", i, err)
		}
	}
	for i, m := range bad {
		_, err := Load(env(m))
		if err == nil {
			t.Errorf("bad[%d] accepted", i)
		} else if strings.Contains(err.Error(), "dummy") {
			t.Errorf("error echoes the password: %v", err)
		}
	}
}

func TestPoolStatsInterval(t *testing.T) {
	base := map[string]string{"DATABASE_URL": "x", "DATA_ENCRYPTION_KEY": testKey}
	with := func(k, v string) map[string]string {
		m := map[string]string{k: v}
		for bk, bv := range base {
			m[bk] = bv
		}
		return m
	}
	if cfg, err := Load(env(base)); err != nil || cfg.DBPoolStatsInterval != 0 {
		t.Fatalf("default: %v %v", cfg.DBPoolStatsInterval, err)
	}
	if cfg, err := Load(env(with("DB_POOL_STATS_INTERVAL", "10s"))); err != nil || cfg.DBPoolStatsInterval != 10*time.Second {
		t.Fatalf("10s: %v %v", cfg.DBPoolStatsInterval, err)
	}
	if _, err := Load(env(with("DB_POOL_STATS_INTERVAL", "2h"))); err == nil {
		t.Fatal("2h accepted")
	}
}

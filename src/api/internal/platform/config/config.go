// Package config loads API process configuration from the environment.
package config

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds process-level settings. Secrets such as DATABASE_URL are never
// logged or returned by String methods.
type Config struct {
	HTTPAddr    string
	DatabaseURL string
	DBMaxConns  int32
	// DBPoolStatsInterval logs connection-pool statistics (acquire waits,
	// saturation) at this interval; 0 disables. For load tests and ops.
	DBPoolStatsInterval time.Duration
	ReadinessTimeout    time.Duration
	ShutdownTimeout     time.Duration
	StatementTimeout    time.Duration
	LockTimeout         time.Duration
	IdleInTxTimeout     time.Duration
	ReadHeaderTimeout   time.Duration
	ReadTimeout         time.Duration
	WriteTimeout        time.Duration
	IdleTimeout         time.Duration

	// AdminOrigins are the exact browser origins allowed to send
	// cookie-authenticated staff mutations (CSRF defence).
	AdminOrigins []string
	// TrustedProxies may supply X-Forwarded-For. Default: none. List only a
	// reverse proxy that appends/overwrites the header with the real client
	// address; the Next.js dev proxy forwards client-supplied values
	// unchanged and must not be trusted.
	TrustedProxies       []netip.Prefix
	StaffSessionIdle     time.Duration
	StaffSessionAbsolute time.Duration

	// PWAOrigins are the exact customer origins allowed to send guest,
	// anonymous and member mutations.
	PWAOrigins []string
	// DataKey (32 bytes) seals secret-bearing stored data such as
	// idempotency replay responses. Required; never logged.
	DataKey []byte

	// Mail (ADR-0005): Resend over SMTP with TLS required, or the file
	// outbox for automated tests; sender; public PWA URL for emailed links.
	MailAdapter  string // smtp | file
	MailOutbox   string // directory for the file adapter
	SMTPAddr     string
	SMTPTLS      string // implicit | starttls
	SMTPUsername string
	SMTPPassword string // secret; never logged
	MailFrom     string
	PWAPublicURL string
}

// Load reads configuration using getenv (os.Getenv in production).
func Load(getenv func(string) string) (Config, error) {
	if getenv == nil {
		getenv = os.Getenv
	}
	cfg := Config{
		HTTPAddr:    valueOr(getenv("HTTP_ADDR"), "127.0.0.1:8080"),
		DatabaseURL: getenv("DATABASE_URL"),
	}
	var errs []error
	if cfg.DatabaseURL == "" {
		errs = append(errs, errors.New("DATABASE_URL is required"))
	}

	maxConns, err := intValue(getenv, "DB_MAX_CONNS", 10, 1, 100)
	errs = appendErr(errs, err)
	cfg.DBMaxConns = int32(maxConns)

	durations := []struct {
		dst      *time.Duration
		name     string
		fallback time.Duration
	}{
		{&cfg.ReadinessTimeout, "READINESS_TIMEOUT", time.Second},
		{&cfg.ShutdownTimeout, "SHUTDOWN_TIMEOUT", 10 * time.Second},
		{&cfg.StatementTimeout, "DB_STATEMENT_TIMEOUT", 5 * time.Second},
		{&cfg.LockTimeout, "DB_LOCK_TIMEOUT", 2 * time.Second},
		{&cfg.IdleInTxTimeout, "DB_IDLE_IN_TRANSACTION_TIMEOUT", 10 * time.Second},
		{&cfg.ReadHeaderTimeout, "HTTP_READ_HEADER_TIMEOUT", 5 * time.Second},
		{&cfg.ReadTimeout, "HTTP_READ_TIMEOUT", 10 * time.Second},
		{&cfg.WriteTimeout, "HTTP_WRITE_TIMEOUT", 15 * time.Second},
		{&cfg.IdleTimeout, "HTTP_IDLE_TIMEOUT", 60 * time.Second},
		{&cfg.StaffSessionIdle, "STAFF_SESSION_IDLE", 60 * time.Minute},
	}
	for _, d := range durations {
		*d.dst, err = durationValue(getenv, d.name, d.fallback)
		errs = appendErr(errs, err)
	}
	if getenv("DB_POOL_STATS_INTERVAL") != "" {
		cfg.DBPoolStatsInterval, err = boundedDuration(getenv, "DB_POOL_STATS_INTERVAL", time.Minute, time.Hour)
		errs = appendErr(errs, err)
	}
	cfg.StaffSessionAbsolute, err = boundedDuration(getenv, "STAFF_SESSION_ABSOLUTE", 12*time.Hour, 24*time.Hour)
	errs = appendErr(errs, err)
	if cfg.StaffSessionIdle > cfg.StaffSessionAbsolute {
		errs = append(errs, errors.New("STAFF_SESSION_IDLE must not exceed STAFF_SESSION_ABSOLUTE"))
	}

	cfg.AdminOrigins, err = origins(valueOr(getenv("ADMIN_ORIGINS"), "http://localhost:3001,http://127.0.0.1:3001"))
	errs = appendErr(errs, err)
	cfg.PWAOrigins, err = origins(valueOr(getenv("PWA_ORIGINS"), "http://localhost:3000,http://127.0.0.1:3000"))
	errs = appendErr(errs, err)
	cfg.MailAdapter = valueOr(getenv("MAIL_ADAPTER"), "smtp")
	cfg.MailOutbox = getenv("MAIL_OUTBOX_DIR")
	cfg.SMTPAddr = valueOr(getenv("SMTP_ADDR"), "smtp.resend.com:587")
	cfg.SMTPTLS = valueOr(getenv("SMTP_TLS"), "starttls")
	cfg.SMTPUsername = getenv("SMTP_USERNAME")
	cfg.SMTPPassword = getenv("SMTP_PASSWORD")
	errs = appendErr(errs, validateSMTP(cfg))
	cfg.MailFrom = valueOr(getenv("MAIL_FROM"), "TableFlow <no-reply@tableflow.local>")
	var publicURL []string
	publicURL, err = origins(valueOr(getenv("PWA_PUBLIC_URL"), "http://localhost:3000"))
	errs = appendErr(errs, err)
	if len(publicURL) == 1 {
		cfg.PWAPublicURL = publicURL[0]
	} else if err == nil {
		errs = append(errs, errors.New("PWA_PUBLIC_URL must be a single origin"))
	}
	cfg.DataKey, err = dataKey(getenv("DATA_ENCRYPTION_KEY"))
	errs = appendErr(errs, err)
	cfg.TrustedProxies, err = prefixes(getenv("TRUSTED_PROXY_CIDRS"))
	errs = appendErr(errs, err)

	if err := errors.Join(errs...); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func valueOr(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}

func appendErr(errs []error, err error) []error {
	if err != nil {
		return append(errs, err)
	}
	return errs
}

func intValue(getenv func(string) string, name string, fallback, lo, hi int) (int, error) {
	raw := getenv(name)
	if raw == "" {
		return fallback, nil
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v < lo || v > hi {
		return 0, fmt.Errorf("%s must be an integer between %d and %d", name, lo, hi)
	}
	return v, nil
}

func durationValue(getenv func(string) string, name string, fallback time.Duration) (time.Duration, error) {
	max := 5 * time.Minute
	if fallback > max {
		max = 24 * time.Hour
	}
	return boundedDuration(getenv, name, fallback, max)
}

func boundedDuration(getenv func(string) string, name string, fallback, max time.Duration) (time.Duration, error) {
	raw := getenv(name)
	if raw == "" {
		return fallback, nil
	}
	v, err := time.ParseDuration(raw)
	if err != nil || v <= 0 || v > max {
		return 0, fmt.Errorf("%s must be a positive duration of at most %s", name, max)
	}
	return v, nil
}

// origins parses a comma-separated list of exact scheme://host[:port] origins.
func origins(raw string) ([]string, error) {
	var out []string
	for _, part := range strings.Split(raw, ",") {
		o := strings.TrimSpace(part)
		u, err := url.Parse(o)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" ||
			u.Path != "" || u.RawQuery != "" || u.User != nil || u.Fragment != "" {
			return nil, errors.New("ADMIN_ORIGINS/PWA_ORIGINS must be comma-separated scheme://host[:port] origins")
		}
		out = append(out, o)
	}
	return out, nil
}

func prefixes(raw string) ([]netip.Prefix, error) {
	var out []netip.Prefix
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	for _, part := range strings.Split(raw, ",") {
		p, err := netip.ParsePrefix(strings.TrimSpace(part))
		if err != nil {
			return nil, errors.New("TRUSTED_PROXY_CIDRS must be comma-separated CIDR prefixes")
		}
		out = append(out, p.Masked())
	}
	return out, nil
}

func dataKey(raw string) ([]byte, error) {
	if raw == "" {
		return nil, errors.New("DATA_ENCRYPTION_KEY is required (base64 of 32 random bytes)")
	}
	k, err := base64.StdEncoding.DecodeString(raw)
	if err != nil || len(k) != 32 {
		return nil, errors.New("DATA_ENCRYPTION_KEY must be base64 of exactly 32 bytes")
	}
	return k, nil
}

// validateSMTP allows only TLS delivery (no plaintext mode) and complete
// credentials. Missing credentials are allowed so local work is not blocked;
// the API then reports mail as not configured and every send fails loudly.
func validateSMTP(cfg Config) error {
	switch cfg.MailAdapter {
	case "smtp":
	case "file":
		if cfg.MailOutbox == "" {
			return errors.New("MAIL_ADAPTER=file requires MAIL_OUTBOX_DIR (automated tests only)")
		}
		return nil
	default:
		return errors.New("MAIL_ADAPTER must be smtp or file")
	}
	if cfg.SMTPTLS != "implicit" && cfg.SMTPTLS != "starttls" {
		return errors.New("SMTP_TLS must be implicit or starttls; plaintext SMTP is not supported")
	}
	if _, _, err := net.SplitHostPort(cfg.SMTPAddr); err != nil {
		return errors.New("SMTP_ADDR must be host:port")
	}
	if (cfg.SMTPUsername == "") != (cfg.SMTPPassword == "") {
		return errors.New("set both SMTP_USERNAME and SMTP_PASSWORD, or neither")
	}
	return nil
}

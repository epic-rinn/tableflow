// Package config loads API process configuration from the environment.
package config

import (
	"errors"
	"fmt"
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
	HTTPAddr          string
	DatabaseURL       string
	DBMaxConns        int32
	ReadinessTimeout  time.Duration
	ShutdownTimeout   time.Duration
	StatementTimeout  time.Duration
	LockTimeout       time.Duration
	IdleInTxTimeout   time.Duration
	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration

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
	cfg.StaffSessionAbsolute, err = boundedDuration(getenv, "STAFF_SESSION_ABSOLUTE", 12*time.Hour, 24*time.Hour)
	errs = appendErr(errs, err)
	if cfg.StaffSessionIdle > cfg.StaffSessionAbsolute {
		errs = append(errs, errors.New("STAFF_SESSION_IDLE must not exceed STAFF_SESSION_ABSOLUTE"))
	}

	cfg.AdminOrigins, err = origins(valueOr(getenv("ADMIN_ORIGINS"), "http://localhost:3001,http://127.0.0.1:3001"))
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
			return nil, errors.New("ADMIN_ORIGINS must be comma-separated scheme://host[:port] origins")
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

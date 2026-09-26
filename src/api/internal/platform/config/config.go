// Package config loads API process configuration from the environment.
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
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
	}
	for _, d := range durations {
		*d.dst, err = durationValue(getenv, d.name, d.fallback)
		errs = appendErr(errs, err)
	}
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
	raw := getenv(name)
	if raw == "" {
		return fallback, nil
	}
	v, err := time.ParseDuration(raw)
	if err != nil || v <= 0 || v > 5*time.Minute {
		return 0, fmt.Errorf("%s must be a positive duration of at most 5m", name)
	}
	return v, nil
}

// Command api runs the TableFlow HTTP API. With -healthcheck it instead
// probes the local readiness endpoint and exits 0 (ready) or 1, for container
// health checks on images without curl.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/epic-rinn/tableflow/src/api/internal/platform/app"
	"github.com/epic-rinn/tableflow/src/api/internal/platform/config"
)

func main() {
	healthcheck := flag.Bool("healthcheck", false, "probe the local readiness endpoint and exit")
	flag.Parse()
	if *healthcheck {
		os.Exit(probe(os.Getenv("HTTP_ADDR")))
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(logger); err != nil {
		logger.Error("api exited", "error", err.Error())
		os.Exit(1)
	}
}

// probe calls /api/v1/health/ready on the listen address (0.0.0.0 → loopback).
func probe(addr string) int {
	if addr == "" {
		addr = "127.0.0.1:8080"
	}
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return 1
	}
	if host == "" || host == "0.0.0.0" || host == "::" || strings.HasPrefix(host, "[") {
		host = "127.0.0.1"
	}
	client := http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://%s/api/v1/health/ready", net.JoinHostPort(host, port)))
	if err != nil {
		return 1
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 1
	}
	return 0
}

func run(logger *slog.Logger) error {
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	ln, err := net.Listen("tcp", cfg.HTTPAddr)
	if err != nil {
		return err
	}
	return app.Run(ctx, cfg, logger, ln)
}

// Command ogoune-agent is the host monitoring agent. It collects
// system metrics and streams them to the Ogoune backend over the
// /api/v1/agent/stream WebSocket, authenticated by a per-host ag_live_
// credential. The agent is optional and fail-safe: it never crashes the host,
// retries on failure, and slows to a capped back-off on a revoked credential.
package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
)

// version is the agent version reported in each frame. Overridable at build time
// via -ldflags "-X main.version=…".
var version = "0.1.0"

func main() {
	cfg, err := Load(os.Args[1:], os.Getenv, os.ReadFile)
	if err != nil {
		// Fail fast, before any network attempt, with a clear message.
		slog.Error("ogoune-agent: invalid configuration", "error", err)
		os.Exit(2)
	}

	setupLogging(cfg.LogLevel)
	slog.Info("ogoune-agent starting", "version", version, "backend", cfg.BackendURL, "interval", cfg.Interval.String())

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	streamer := NewStreamer(cfg, newGopsutilCollector(version))
	if err := streamer.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		slog.Error("ogoune-agent stopped", "error", err)
		os.Exit(1)
	}
	slog.Info("ogoune-agent stopped cleanly")
}

func setupLogging(level string) {
	var lvl slog.Level
	switch level {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: lvl})))
}

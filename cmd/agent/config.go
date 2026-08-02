package main

import (
	"flag"
	"fmt"
	"io"
	"net"
	"net/url"
	"strings"
	"time"
)

const (
	defaultConfigPath = "/etc/ogoune/agent.cfg"
	defaultInterval   = 10 * time.Second
	defaultLogLevel   = "info"
	credentialPrefix  = "ag_live_"
)

// Config is the resolved agent configuration. Precedence: flags > env > file >
// defaults.
type Config struct {
	BackendURL         string
	Credential         string
	Interval           time.Duration
	LogLevel           string
	InsecureSkipVerify bool
}

// Load resolves configuration from (in increasing precedence) defaults, an
// env-style config file (/etc/ogoune/agent.cfg, KEY=value with OGOUNE_* keys —
// the same format used by systemd EnvironmentFile and docker --env-file),
// environment variables, and command-line flags. getenv and readFile are
// injected for testability. A missing config file is not an error when the
// required values are supplied by env/flags.
func Load(args []string, getenv func(string) string, readFile func(string) ([]byte, error)) (Config, error) {
	cfg := Config{Interval: defaultInterval, LogLevel: defaultLogLevel}

	// Resolve config file path (flag/env override the default) with a pre-parse.
	configPath := defaultConfigPath
	if v := getenv("OGOUNE_CONFIG"); v != "" {
		configPath = v
	}
	pre := flag.NewFlagSet("pre", flag.ContinueOnError)
	pre.SetOutput(io.Discard)
	preConfig := pre.String("config", "", "path to config file")
	_ = pre.Parse(args)
	if *preConfig != "" {
		configPath = *preConfig
	}

	// 1) File (optional) — env-style KEY=value (OGOUNE_*).
	if b, err := readFile(configPath); err == nil {
		fileEnv, perr := parseEnvFile(b)
		if perr != nil {
			return Config{}, fmt.Errorf("parse config %s: %w", configPath, perr)
		}
		if err := applyEnv(&cfg, func(k string) string { return fileEnv[k] }); err != nil {
			return Config{}, fmt.Errorf("config %s: %w", configPath, err)
		}
	}

	// 2) Environment (overrides the file).
	if err := applyEnv(&cfg, getenv); err != nil {
		return Config{}, err
	}

	// 3) Flags (highest precedence; only override when explicitly set).
	fs := flag.NewFlagSet("ogoune-agent", flag.ContinueOnError)
	backendURL := fs.String("backend-url", "", "backend WebSocket URL (wss://host/api/v1/agent/stream)")
	credential := fs.String("credential", "", "host agent credential (ag_live_…)")
	interval := fs.Duration("interval", 0, "sample interval (e.g. 10s)")
	logLevel := fs.String("log-level", "", "log level (debug|info|warn|error)")
	insecure := fs.Bool("insecure", false, "skip TLS verification (dev only)")
	fs.String("config", "", "path to config file (default "+defaultConfigPath+")")
	if err := fs.Parse(args); err != nil {
		return Config{}, err
	}
	fs.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "backend-url":
			cfg.BackendURL = *backendURL
		case "credential":
			cfg.Credential = *credential
		case "interval":
			cfg.Interval = *interval
		case "log-level":
			cfg.LogLevel = *logLevel
		case "insecure":
			cfg.InsecureSkipVerify = *insecure
		}
	})

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// applyEnv applies OGOUNE_* settings from get (non-empty values only), so the
// same logic serves both the config-file layer and the process environment.
func applyEnv(cfg *Config, get func(string) string) error {
	if v := get("OGOUNE_BACKEND_URL"); v != "" {
		cfg.BackendURL = v
	}
	if v := get("OGOUNE_CREDENTIAL"); v != "" {
		cfg.Credential = v
	}
	if v := get("OGOUNE_INTERVAL"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return fmt.Errorf("invalid OGOUNE_INTERVAL %q: %w", v, err)
		}
		cfg.Interval = d
	}
	if v := get("OGOUNE_LOG_LEVEL"); v != "" {
		cfg.LogLevel = v
	}
	if v := get("OGOUNE_INSECURE"); v == "true" || v == "1" {
		cfg.InsecureSkipVerify = true
	}
	return nil
}

// parseEnvFile parses an env-style KEY=value file: blank lines and #-comments are
// ignored; surrounding quotes on the value are stripped; a leading "export " is
// tolerated.
func parseEnvFile(b []byte) (map[string]string, error) {
	out := map[string]string{}
	for _, raw := range strings.Split(string(b), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			return nil, fmt.Errorf("invalid line %q (expected KEY=value)", raw)
		}
		out[strings.TrimSpace(k)] = strings.Trim(strings.TrimSpace(v), `"'`)
	}
	return out, nil
}

// isPlaintextToRemote reports whether backendURL uses plaintext ws:// to a
// non-loopback host — the case where credentials/metrics traverse the network in
// the clear and wss:// (TLS) should be used instead. Loopback (localhost dev)
// stays frictionless. A malformed URL classifies as false (Validate handles it).
func isPlaintextToRemote(backendURL string) bool {
	u, err := url.Parse(backendURL)
	if err != nil || u.Scheme != "ws" {
		return false
	}
	host := u.Hostname()
	if host == "localhost" {
		return false
	}
	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
		return false
	}
	return true
}

// Validate checks the resolved configuration.
func (c Config) Validate() error {
	if strings.TrimSpace(c.BackendURL) == "" {
		return fmt.Errorf("backend_url is required")
	}
	u, err := url.Parse(c.BackendURL)
	if err != nil || (u.Scheme != "ws" && u.Scheme != "wss") || u.Host == "" {
		return fmt.Errorf("backend_url must be a ws:// or wss:// URL, got %q", c.BackendURL)
	}
	if !strings.HasPrefix(c.Credential, credentialPrefix) {
		return fmt.Errorf("credential is required and must start with %q", credentialPrefix)
	}
	if c.Interval < time.Second {
		return fmt.Errorf("interval must be at least 1s, got %s", c.Interval)
	}
	return nil
}

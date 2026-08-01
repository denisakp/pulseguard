package main

import (
	"flag"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	defaultConfigPath = "/etc/ogoune-agent.yaml"
	defaultInterval   = 10 * time.Second
	defaultLogLevel   = "info"
	credentialPrefix  = "ag_live_"
)

// Config is the resolved agent configuration. Precedence: flags > env > file >
// defaults.
type Config struct {
	BackendURL         string        `yaml:"backend_url"`
	Credential         string        `yaml:"credential"`
	Interval           time.Duration `yaml:"interval"`
	LogLevel           string        `yaml:"log_level"`
	InsecureSkipVerify bool          `yaml:"insecure_skip_verify"`
}

// Load resolves configuration from (in increasing precedence) defaults, a YAML
// file, environment variables, and command-line flags. getenv and readFile are
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

	// 1) File (optional).
	if b, err := readFile(configPath); err == nil {
		if err := yaml.Unmarshal(b, &cfg); err != nil {
			return Config{}, fmt.Errorf("parse config %s: %w", configPath, err)
		}
	}

	// 2) Environment.
	if v := getenv("OGOUNE_BACKEND_URL"); v != "" {
		cfg.BackendURL = v
	}
	if v := getenv("OGOUNE_CREDENTIAL"); v != "" {
		cfg.Credential = v
	}
	if v := getenv("OGOUNE_INTERVAL"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return Config{}, fmt.Errorf("invalid OGOUNE_INTERVAL %q: %w", v, err)
		}
		cfg.Interval = d
	}
	if v := getenv("OGOUNE_LOG_LEVEL"); v != "" {
		cfg.LogLevel = v
	}
	if v := getenv("OGOUNE_INSECURE"); v == "true" || v == "1" {
		cfg.InsecureSkipVerify = true
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

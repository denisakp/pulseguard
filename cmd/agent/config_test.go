package main

import (
	"errors"
	"os"
	"testing"
	"time"
)

func noEnv(string) string { return "" }

func envFrom(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func noFile(string) ([]byte, error) { return nil, os.ErrNotExist }

func fileFrom(content string) func(string) ([]byte, error) {
	return func(string) ([]byte, error) { return []byte(content), nil }
}

func TestLoad_FlagsOverEnvOverFile(t *testing.T) {
	file := "backend_url: wss://file/api/v1/agent/stream\ncredential: ag_live_file\ninterval: 30s\n"
	env := map[string]string{
		"OGOUNE_CREDENTIAL": "ag_live_env",
		"OGOUNE_INTERVAL":   "20s",
	}
	args := []string{"--credential", "ag_live_flag", "--interval", "15s"}

	cfg, err := Load(args, envFrom(env), fileFrom(file))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	// backend_url only in file → file wins (no env/flag override).
	if cfg.BackendURL != "wss://file/api/v1/agent/stream" {
		t.Errorf("BackendURL = %q, want file value", cfg.BackendURL)
	}
	// credential: flag > env > file.
	if cfg.Credential != "ag_live_flag" {
		t.Errorf("Credential = %q, want ag_live_flag (flag wins)", cfg.Credential)
	}
	// interval: flag wins.
	if cfg.Interval != 15*time.Second {
		t.Errorf("Interval = %s, want 15s (flag wins)", cfg.Interval)
	}
}

func TestLoad_EnvOverFile(t *testing.T) {
	file := "backend_url: wss://file/api/v1/agent/stream\ncredential: ag_live_file\n"
	env := map[string]string{"OGOUNE_CREDENTIAL": "ag_live_env"}
	cfg, err := Load(nil, envFrom(env), fileFrom(file))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Credential != "ag_live_env" {
		t.Errorf("Credential = %q, want env value", cfg.Credential)
	}
}

func TestLoad_Defaults(t *testing.T) {
	cfg, err := Load(
		[]string{"--backend-url", "wss://h/api/v1/agent/stream", "--credential", "ag_live_x"},
		noEnv, noFile,
	)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Interval != defaultInterval {
		t.Errorf("Interval = %s, want default %s", cfg.Interval, defaultInterval)
	}
	if cfg.LogLevel != defaultLogLevel {
		t.Errorf("LogLevel = %q, want %q", cfg.LogLevel, defaultLogLevel)
	}
}

func TestLoad_MissingRequired(t *testing.T) {
	// No backend_url anywhere.
	if _, err := Load([]string{"--credential", "ag_live_x"}, noEnv, noFile); err == nil {
		t.Error("expected error when backend_url missing")
	}
	// No credential anywhere.
	if _, err := Load([]string{"--backend-url", "wss://h/api/v1/agent/stream"}, noEnv, noFile); err == nil {
		t.Error("expected error when credential missing")
	}
}

func TestValidate(t *testing.T) {
	base := Config{BackendURL: "wss://h/api/v1/agent/stream", Credential: "ag_live_x", Interval: 10 * time.Second}
	if err := base.Validate(); err != nil {
		t.Fatalf("valid config rejected: %v", err)
	}

	bad := base
	bad.BackendURL = "http://h" // not ws/wss
	if err := bad.Validate(); err == nil {
		t.Error("non-ws backend_url should be rejected")
	}

	bad = base
	bad.Credential = "pk_live_x" // wrong prefix
	if err := bad.Validate(); err == nil {
		t.Error("non-ag_live credential should be rejected")
	}

	bad = base
	bad.Interval = 500 * time.Millisecond
	if err := bad.Validate(); err == nil {
		t.Error("sub-second interval should be rejected")
	}
}

func TestLoad_InvalidEnvInterval(t *testing.T) {
	env := map[string]string{"OGOUNE_INTERVAL": "not-a-duration"}
	_, err := Load(
		[]string{"--backend-url", "wss://h/api/v1/agent/stream", "--credential", "ag_live_x"},
		envFrom(env), noFile,
	)
	if err == nil {
		t.Error("invalid OGOUNE_INTERVAL should error")
	}
	_ = errors.Unwrap(err)
}

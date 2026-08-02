package bootstrap

import (
	"log/slog"
	"os"

	"github.com/denisakp/ogoune/internal/config"
	icmppkg "github.com/denisakp/ogoune/internal/icmp"
	"github.com/denisakp/ogoune/pkg/crypto"
	"github.com/denisakp/ogoune/pkg/logger"
	"github.com/denisakp/ogoune/pkg/safenet"
)

// InitConfig loads configuration, initializes the logger, and validates the crypto key.
func InitConfig(app *App) {
	cfg := config.MustInit()
	app.Cfg = &cfg

	// OpenAPI contract is generated from Go annotations and embedded at build time
	// (api/openapi, spec 074) — no runtime SwaggerInfo mutation needed.

	// Initialize structured logger
	l := logger.New(cfg.LogFormat, cfg.LogLevel)
	slog.SetDefault(l)

	slog.Info("starting Ogoune application")
	LogStartupEdition()

	// Fail fast if APP_SECRET_KEY is missing or malformed
	crypto.SetGlobalProvider(&crypto.EnvKeyProvider{})
	if err := crypto.ValidateKey(); err != nil {
		slog.Error("crypto key validation failed", "error", err)
		os.Exit(1)
	}
	slog.Info("encryption key validated")

	// Detect ICMP capability at startup
	icmpCapability := icmppkg.Detect()
	logICMPCapabilityState(cfg.EnableICMP, icmpCapability)

	// SSRF policy: allow private/LAN targets only when explicitly opted in.
	safenet.SetAllowPrivate(cfg.AllowPrivateTargets)
	if cfg.AllowPrivateTargets {
		slog.Warn("ALLOW_PRIVATE_TARGETS enabled — monitors may reach private/loopback addresses; keep this off on public/multi-tenant instances")
	}

	slog.Info("authentication configured", "email", cfg.AuthEmail)
}

# Ogoune host agent (`ogoune-agent`)

The optional host monitoring agent (spec 080). It runs on a monitored Linux
server, collects system metrics, and streams them to the Ogoune backend over the
`/api/v1/agent/stream` WebSocket, authenticated by a per-host `ag_live_…`
credential. The agent is **optional** — no core monitoring capability depends on
it — and **fail-safe**: it never crashes the host, retries on failure, and slows
to a capped back-off on a revoked credential.

## Install / build

Prebuilt artifacts are published per release (spec 082):

- **Container image**: `ghcr.io/denisakp/ogoune-agent:<version>` (+ `:latest`),
  multi-arch `linux/amd64,arm64`, versioned in lockstep with the API image.
- **Release binaries**: `ogoune-agent-linux-amd64` / `ogoune-agent-linux-arm64`
  (+ `SHA256SUMS`) attached to the GitHub Release.

```bash
# Container (reads host metrics with --pid=host):
docker run -d --name ogoune-agent --restart unless-stopped --pid=host --network=host \
  -e OGOUNE_BACKEND_URL=wss://your-ogoune/api/v1/agent/stream \
  -e OGOUNE_CREDENTIAL=ag_live_… ghcr.io/denisakp/ogoune-agent:latest
```

Build from source (contributors):

```bash
make build-agent                 # → dist/ogoune-agent (version stamped from git)
make build-agent-linux ARCH=amd64  # cross-compile a static linux binary (arm64 default)
```

## Register a host (operator)

```bash
curl -sS -X POST https://ogoune.example.com/api/v1/hosts \
  -H "Authorization: Bearer <operator-api-key>" \
  -H "Content-Type: application/json" -d '{"name":"web-1"}'
# → { "data": { "host": {...}, "credential": "ag_live_…" } }   (credential shown once)
```

## Configure

Config precedence: **flags > environment > file > defaults**.

Config file (default `/etc/ogoune/agent.cfg`, mode `0600`) is **env-style
`KEY=value`** — the same format works as the agent's config, as the systemd
`EnvironmentFile`, and as `docker --env-file`. See
[`packaging/agent/ogoune-agent.cfg.example`](../../packaging/agent/ogoune-agent.cfg.example):

```ini
OGOUNE_BACKEND_URL=wss://ogoune.example.com/api/v1/agent/stream   # required
OGOUNE_CREDENTIAL=ag_live_…                                       # required
#OGOUNE_INTERVAL=10s          # optional (default 10s)
#OGOUNE_LOG_LEVEL=info        # optional (debug|info|warn|error)
#OGOUNE_INSECURE=false        # dev/self-signed only
```

> Use `wss://` (TLS) in production. The agent warns when connecting over plaintext
> `ws://` to a non-local host (it still connects — local `ws://` stays easy).
> The previous `/etc/ogoune-agent.yaml` path/format is **deprecated**.

| Setting | Flag | Env |
|---|---|---|
| backend URL | `--backend-url` | `OGOUNE_BACKEND_URL` |
| credential | `--credential` | `OGOUNE_CREDENTIAL` |
| interval | `--interval` | `OGOUNE_INTERVAL` |
| log level | `--log-level` | `OGOUNE_LOG_LEVEL` |
| insecure TLS | `--insecure` | `OGOUNE_INSECURE` |
| config path | `--config` | `OGOUNE_CONFIG` |

## Run

Ad-hoc:

```bash
OGOUNE_BACKEND_URL=wss://ogoune.example.com/api/v1/agent/stream \
OGOUNE_CREDENTIAL=ag_live_… ./dist/ogoune-agent
```

As a service (systemd), using
[`packaging/agent/ogoune-agent.service`](../../packaging/agent/ogoune-agent.service):

The unit runs as a `DynamicUser` (unprivileged), which cannot read a root-owned
`0600` config file itself — so the env-style `/etc/ogoune/agent.cfg` doubles as the
systemd `EnvironmentFile`, which systemd reads privileged and injects into the
process:

```bash
sudo install -m755 dist/ogoune-agent /usr/local/bin/ogoune-agent

sudo mkdir -p /etc/ogoune
sudo tee /etc/ogoune/agent.cfg >/dev/null <<'EOF'
OGOUNE_BACKEND_URL=wss://your-ogoune/api/v1/agent/stream
OGOUNE_CREDENTIAL=ag_live_…
EOF
sudo chmod 600 /etc/ogoune/agent.cfg

sudo install -m644 packaging/agent/ogoune-agent.service /etc/systemd/system/
sudo systemctl daemon-reload && sudo systemctl enable --now ogoune-agent
journalctl -u ogoune-agent -f
```

## Behaviour

- **Cadence**: one metric frame per `interval` (~10s). The backend assigns the
  sample time — the agent never sends its own clock.
- **Reconnect**: exponential back-off (cap ~30s), retry forever; retries at
  startup if the backend is not yet reachable.
- **Revoked / invalid credential**: slow capped back-off (~5 min), retries
  forever and logs the reason; self-heals when the credential is re-validated. No
  tight loop, no exit.
- **Partial failure**: a metric that cannot be read is skipped for that frame;
  the rest of the sample is still sent.
- **Shutdown**: `SIGINT`/`SIGTERM` closes the socket cleanly and exits 0.
- **Invalid config**: exits non-zero with a clear message before any network
  attempt.

## Contract

The wire frame is defined once in [`pkg/agentwire`](../../pkg/agentwire) and
imported by both the agent and the backend ingestion handler, so they cannot
drift. It carries a `schema_version` (currently `1`); the backend treats a frame
without one as v1 for backward compatibility.

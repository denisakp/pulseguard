# Ogoune host agent (`ogoune-agent`)

The optional host monitoring agent (spec 080). It runs on a monitored Linux
server, collects system metrics, and streams them to the Ogoune backend over the
`/api/v1/agent/stream` WebSocket, authenticated by a per-host `ag_live_…`
credential. The agent is **optional** — no core monitoring capability depends on
it — and **fail-safe**: it never crashes the host, retries on failure, and slows
to a capped back-off on a revoked credential.

## Build

```bash
make build-agent          # → dist/ogoune-agent (version stamped from git)
# or: go build -o dist/ogoune-agent ./cmd/agent
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

Config file (default `/etc/ogoune-agent.yaml`, mode `0600`) — see
[`packaging/agent/ogoune-agent.example.yaml`](../../packaging/agent/ogoune-agent.example.yaml):

```yaml
backend_url: wss://ogoune.example.com/api/v1/agent/stream   # required
credential: ag_live_…                                       # required
interval: 10s          # optional (default 10s)
log_level: info        # optional (debug|info|warn|error)
insecure_skip_verify: false   # dev/self-signed only
```

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

```bash
sudo install -m755 dist/ogoune-agent /usr/local/bin/ogoune-agent
sudo install -m600 packaging/agent/ogoune-agent.example.yaml /etc/ogoune-agent.yaml   # then edit
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

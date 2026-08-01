# Host agent

The **host agent** (`ogoune-agent`) is an optional Go binary you install on a
monitored Linux server. It collects system metrics — OS, CPU, memory, per-mount
disk, and network — and streams them to Ogoune every ~10 seconds, so a host's
health shows up alongside your uptime monitors.

The agent is **optional**: nothing in Ogoune's monitoring depends on it. It is
also **fail-safe** — it never destabilises the host, retries through outages, and
slows down (rather than hammering) when a credential is revoked.

## 1. Register the host

From Ogoune (operator API key), register the host to get a one-time credential:

```bash
curl -sS -X POST https://your-ogoune/api/v1/hosts \
  -H "Authorization: Bearer <operator-api-key>" \
  -H "Content-Type: application/json" -d '{"name":"web-1"}'
# → { "data": { "host": {...}, "credential": "ag_live_…" } }
```

Copy the `credential` — it is shown **once**.

## 2. Install and run

```bash
make build-agent   # → dist/ogoune-agent

sudo install -m755 dist/ogoune-agent /usr/local/bin/ogoune-agent
sudo install -m600 packaging/agent/ogoune-agent.example.yaml /etc/ogoune-agent.yaml
# edit /etc/ogoune-agent.yaml: set backend_url + credential
sudo install -m644 packaging/agent/ogoune-agent.service /etc/systemd/system/
sudo systemctl daemon-reload && sudo systemctl enable --now ogoune-agent
journalctl -u ogoune-agent -f
```

Within ~15 seconds the host reports live metrics and shows as **online**.

## Configuration

Precedence: **flags > environment > file > defaults**.

| Setting | File key | Env | Flag | Default |
|---|---|---|---|---|
| Backend URL | `backend_url` | `OGOUNE_BACKEND_URL` | `--backend-url` | — (required) |
| Credential | `credential` | `OGOUNE_CREDENTIAL` | `--credential` | — (required) |
| Interval | `interval` | `OGOUNE_INTERVAL` | `--interval` | `10s` |
| Log level | `log_level` | `OGOUNE_LOG_LEVEL` | `--log-level` | `info` |
| Skip TLS verify | `insecure_skip_verify` | `OGOUNE_INSECURE` | `--insecure` | `false` |

## Behaviour

- **Reconnect**: exponential back-off (cap ~30s), retries forever; also retries at
  startup if the backend is not yet reachable.
- **Revoked credential**: slows to a capped (~5 min) back-off, retries forever and
  logs the reason; resumes automatically once you re-issue and update the
  credential.
- **Rotation**: issue a new credential in Ogoune, update the config, restart the
  service.
- **Shutdown**: `systemctl stop` closes the connection cleanly.

## Scope

Linux only for now. Windows/macOS agents, a Hosts UI, and kernel-level (eBPF)
capture are planned separately.

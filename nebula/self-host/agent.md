# Host agent

The **host agent** (`ogoune-agent`) is an optional Go binary you install on a
monitored Linux server. It collects system metrics — OS, CPU, memory, per-mount
disk, and network — and streams them to Ogoune every ~10 seconds, so a host's
health shows up alongside your uptime monitors.

The agent is **optional**: nothing in Ogoune's monitoring depends on it. It is
also **fail-safe** — it never destabilises the host, retries through outages, and
slows down (rather than hammering) when a credential is revoked.

## 1. Register the host

In Ogoune, open **Hosts → Register host**, enter a name, and copy the
`ag_live_…` credential — it is shown **once**. (You can also register via the API:
`POST /api/v1/hosts` with `{"name":"web-1"}` using an operator API key.)

## 2. Install the agent

Two ways to run it — a container, or a native binary managed by systemd. Both use
the host's credential from step 1.

> **Use TLS (`wss://`) in production.** The agent streams the credential and
> metrics over the connection; a plaintext `ws://` to a non-local host exposes
> them. The agent logs a warning when it connects over `ws://` to a remote host —
> it still connects (so local `ws://` testing stays easy), but production should
> always use `wss://`.

### Option A — Docker

```bash
docker run -d --name ogoune-agent --restart unless-stopped \
  --pid=host --network=host \
  -e OGOUNE_BACKEND_URL=wss://your-ogoune/api/v1/agent/stream \
  -e OGOUNE_CREDENTIAL=ag_live_… \
  ghcr.io/denisakp/ogoune-agent:latest
```

`--pid=host --network=host` let the containerised agent read the host's real
metrics and reach Ogoune.

### Option B — Native binary + systemd service

For hosts where you'd rather run the Go binary directly (no Docker), install it as
a systemd service so it starts on boot and restarts on failure.

**1. Get the binary.** Download the prebuilt binary for your architecture from the
[Ogoune releases](https://github.com/denisakp/ogoune/releases) and verify it
against the published `SHA256SUMS`:

```bash
# pick your arch: amd64 or arm64
curl -fsSLO https://github.com/denisakp/ogoune/releases/download/<version>/ogoune-agent-linux-arm64
curl -fsSLO https://github.com/denisakp/ogoune/releases/download/<version>/SHA256SUMS
sha256sum -c SHA256SUMS --ignore-missing
sudo install -m755 ogoune-agent-linux-arm64 /usr/local/bin/ogoune-agent
```

(Building from source is still possible — `make build-agent` → `dist/ogoune-agent`
— but most operators just download the release binary.)

**2. Write the config** at `/etc/ogoune/agent.cfg` (mode `0600` — it holds the
secret). This is an **env-style `KEY=value`** file: the same file serves as the
agent's config *and* as the systemd `EnvironmentFile` (and `docker --env-file`).
The service runs as an unprivileged `DynamicUser`, which cannot read a root-owned
config file itself; systemd reads this file privileged and injects `OGOUNE_*` into
the process:

```bash
sudo mkdir -p /etc/ogoune
sudo tee /etc/ogoune/agent.cfg >/dev/null <<'EOF'
OGOUNE_BACKEND_URL=wss://your-ogoune/api/v1/agent/stream
OGOUNE_CREDENTIAL=ag_live_…
EOF
sudo chmod 600 /etc/ogoune/agent.cfg
```

A commented template ships at `packaging/agent/ogoune-agent.cfg.example`.

**3. Install the systemd unit** (shipped in `packaging/agent/ogoune-agent.service`,
or create it):

```ini
# /etc/systemd/system/ogoune-agent.service
[Unit]
Description=Ogoune host monitoring agent
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
EnvironmentFile=-/etc/ogoune/agent.cfg
ExecStart=/usr/local/bin/ogoune-agent
Restart=on-failure
RestartSec=5
DynamicUser=yes
NoNewPrivileges=yes
ProtectSystem=strict
ProtectHome=yes
PrivateTmp=yes

[Install]
WantedBy=multi-user.target
```

> The previous default path `/etc/ogoune-agent.yaml` is **deprecated**. The agent
> now reads `/etc/ogoune/agent.cfg` by default; point `--config` / `OGOUNE_CONFIG`
> elsewhere if you need to.

**4. Enable and start it:**

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now ogoune-agent
journalctl -u ogoune-agent -f          # should log "agent: connected"
```

Within ~15 seconds the host reports live metrics and shows as **online**.

Manage the service the usual way: `systemctl status ogoune-agent`,
`systemctl restart ogoune-agent` (e.g. after rotating the credential),
`systemctl stop ogoune-agent` (closes the connection cleanly).

---

Open the **Hosts** section in Ogoune to see your fleet: each host's live CPU /
memory / disk, the services running on it, and per-host metric graphs. Link a
monitor to its host from the monitor page to see the machine's health alongside
its checks.

## Docker Compose

The prod compose ships an **opt-in** agent service (it needs a per-host
credential, so a plain `up` never starts it):

```bash
# register a host to get its ag_live_ credential, then:
OGOUNE_BACKEND_URL=wss://your-ogoune/api/v1/agent/stream \
OGOUNE_CREDENTIAL=ag_live_… \
  docker compose --profile agent up -d
```

The dev compose (`docker-compose.dev.yml`) is zero-touch: an `agent-init` service
registers a `local` host and hands the credential to the agent automatically — no
manual step.

## Configuration

Precedence: **flags > environment > file > defaults**. The config file
(`/etc/ogoune/agent.cfg`) is env-style `KEY=value` using the same `OGOUNE_*` keys.

| Setting | Env / file key | Flag | Default |
|---|---|---|---|
| Backend URL | `OGOUNE_BACKEND_URL` | `--backend-url` | — (required) |
| Credential | `OGOUNE_CREDENTIAL` | `--credential` | — (required) |
| Interval | `OGOUNE_INTERVAL` | `--interval` | `10s` |
| Log level | `OGOUNE_LOG_LEVEL` | `--log-level` | `info` |
| Skip TLS verify | `OGOUNE_INSECURE` | `--insecure` | `false` |

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

Linux only for now. Windows/macOS agents and kernel-level (eBPF) capture are
planned separately.

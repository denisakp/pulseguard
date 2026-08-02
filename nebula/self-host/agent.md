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

**1. Get the binary.** Download `ogoune-agent` for your platform (`linux/amd64` or
`linux/arm64`) from the Ogoune releases, or build it from source
(`make build-agent` → `dist/ogoune-agent`). Then install it:

```bash
sudo install -m755 ogoune-agent /usr/local/bin/ogoune-agent
```

**2. Write the config** at `/etc/ogoune-agent.yaml` (mode `0600` — it holds the
secret):

```bash
sudo tee /etc/ogoune-agent.yaml >/dev/null <<'EOF'
backend_url: wss://your-ogoune/api/v1/agent/stream
credential: ag_live_…
EOF
sudo chmod 600 /etc/ogoune-agent.yaml
```

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
ExecStart=/usr/local/bin/ogoune-agent      # reads /etc/ogoune-agent.yaml by default
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

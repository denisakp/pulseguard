# Bulk import & export

Manage your monitors as code. Ogoune can **export** every monitor to a single
YAML file and **import** a YAML manifest to create many monitors at once — handy
for backups, migrating between instances, or version-controlling your setup.

Both operations require a **read-write API key** (a read-only key gets `403`).

## Export

`GET /api/v1/monitors/export` returns a YAML manifest of **all** monitors in the
instance (there is no filtering — it is a full dump). The response downloads as
`ogoune-monitors.yaml`. Only declaration fields are emitted; IDs, timestamps,
status, and metrics are omitted, so the file round-trips cleanly back through
import.

## Import

`POST /api/v1/monitors/import` accepts a YAML manifest, either as the raw request
body (`Content-Type: text/yaml`) or as a multipart file field named `manifest`.

A manifest is a `version` (must be `1`), optional `defaults`, and a list of
`resources`:

```yaml
version: 1
defaults:
  interval: 60        # applied to any monitor that omits the field
  timeout: 10
resources:
  - name: Marketing site
    type: http
    target: https://example.com
    tags: [public, web]
    component: Website
    notification_channels: [Ops email]
  - name: Nightly backup job
    type: heartbeat
    heartbeat_interval: 3600
    heartbeat_grace: 300
```

Every row needs a `name` and `type` (`http`, `tcp`, `dns`, `icmp`, `keyword`,
`protocol`, or `heartbeat`). All types except `heartbeat` require a `target`.
Type-specific fields: `keyword` needs `keyword`; `protocol` needs `protocol_type`
and `protocol_port`; `heartbeat` needs `heartbeat_interval` and
`heartbeat_grace`. `tags` and `component` are created automatically if they don't
exist, but **notification channels must already exist** — a reference to an
unknown channel fails the row.

::: warning Create-only, all-or-nothing
Import **only creates** monitors — it never updates an existing one. A monitor
whose `name` already exists is **skipped** by default; pass
`?duplicatePolicy=error` to fail instead. Otherwise, if **any** row is invalid,
the **whole batch is rejected and nothing is written** — the response is a `422`
carrying a per-row report (`index`, `valid`, `action`, `errors`) so you can fix
and retry.
:::

::: tip Validate first
Add `?dryRun=true` to validate a manifest and get the full row-by-row report
without writing anything. A manifest is capped at 500 monitors.
:::

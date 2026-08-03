# Observability

Ogoune exposes a **Prometheus metrics endpoint** so you can scrape your monitoring
data into Prometheus and chart it in Grafana. It publishes both operational
metrics (check latency, throughput) and business metrics (which resources are up,
how many incidents are open, rolling uptime ratios) — the same numbers the
dashboard shows, in a form your own stack can alert on.

The endpoint is **off by default**. Enable it with two environment variables:

| Variable | Default | Purpose |
|---|---|---|
| `ENABLE_METRICS` | `false` | Turn the `/metrics` endpoint on. |
| `METRICS_TOKEN` | *(empty)* | Bearer token required to scrape. |

When `ENABLE_METRICS=true` and `METRICS_TOKEN` is set, Ogoune serves
`/metrics` (unversioned, at the server root) and requires
`Authorization: Bearer <token>`.

::: warning
If you enable metrics without setting `METRICS_TOKEN`, the endpoint is served
**unauthenticated** — Ogoune logs a warning at startup. Metrics include resource
names; always set a token, or restrict `/metrics` at the network/reverse-proxy
layer.
:::

## Exposed metrics

| Metric | Type | Labels | Meaning |
|---|---|---|---|
| `ogoune_resource_up` | gauge | `id`, `name`, `type` | 1 = up, 0 = down. |
| `ogoune_resource_status` | gauge | `id`, `name`, `type` | 0=unknown, 1=up, 2=down, 3=paused. |
| `ogoune_incidents_total` | gauge | `id`, `name`, `type` | All-time incidents for the resource. |
| `ogoune_incidents_active` | gauge | `id`, `name`, `type` | Currently open incidents. |
| `ogoune_uptime_ratio` | gauge | `id`, `name`, `type`, `window` | Uptime % (0–100); `window` is `24h`, `7d`, or `30d`. |
| `ogoune_check_duration_seconds` | histogram | `id`, `name`, `type` | Per-check latency. |
| `ogoune_checks_total` | counter | `id`, `name`, `type`, `status` | Check executions by outcome. |

Standard Go runtime and process collectors (`go_*`, `process_*`) are exported too.

## Scrape it

Check the endpoint with `curl`:

```bash
curl -H "Authorization: Bearer $METRICS_TOKEN" https://your-ogoune/metrics
```

Then point Prometheus at it:

```yaml
scrape_configs:
  - job_name: ogoune
    scheme: https
    authorization:
      type: Bearer
      credentials: "your-metrics-token"
    static_configs:
      - targets: ["your-ogoune:8080"]
```

## Grafana dashboard & alert rules

Ogoune can generate observability assets from your live configuration, served
under the authenticated API (any operator API key, read-only is fine):

- `GET /api/v1/integrations/grafana-dashboard` — a ready-to-import Grafana
  dashboard JSON.
- `GET /api/v1/integrations/alert-rules` — Prometheus alerting rules as YAML.
  Pass `?uptimeThreshold=99` to set the uptime SLO the rules fire below.

::: tip
Import the dashboard JSON directly in Grafana (**Dashboards → Import**), and drop
the rules file into your Prometheus `rule_files` — both stay in step with the
resources you actually monitor.
:::

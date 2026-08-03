# Status pages

A **public status page** exposes your uptime to the outside world — customers,
teammates, or a public audience — without giving anyone access to Ogoune itself.
It aggregates your monitors and components into a single at-a-glance verdict, a
per-monitor 90-day uptime bar, and a running incident history.

The data behind it is served by a set of **unauthenticated** JSON endpoints under
`/api/status`, so the page needs no API key and stays readable even while your
admin UI is locked down.

## What it shows

- An **overall verdict** — `operational`, `partial_degradation`, or `major_outage` — derived from your monitors' current states.
- **Components** grouping related monitors, each with its own aggregated state (`up`, `degraded`, `down`, `maintenance`, or `unknown`).
- A **90-day uptime ribbon** per monitor (`uptime_90d_ratio` plus a day-by-day `uptime_ribbon`).
- **Incidents** for the current month inline, with a full archive grouped by month.

## The public API

Fetch the live snapshot with a plain unauthenticated GET:

```bash
curl https://your-ogoune/api/status
```

```json
{
  "generated_at": "2026-08-03T09:00:00Z",
  "verdict": { "status": "operational", "label": "All systems operational", "color": "#22c55e" },
  "uptime_window": { "earliest_day": "2026-05-05", "latest_day": "2026-08-03" },
  "components": [
    {
      "id": "01J…",
      "name": "API",
      "aggregated_state": "up",
      "resources": [
        { "id": "01J…", "name": "Public API", "host": "api.example.com", "current_state": "up", "uptime_90d_ratio": 0.9993 }
      ]
    }
  ],
  "standalone_resources": [],
  "current_month_incidents": []
}
```

Companion endpoints: `/api/status/incidents` (archive, `from`/`to`/`component_id`
filters), `/api/status/incidents/{id}` (incident timeline),
`/api/status/uptime` (daily aggregates, `from` + `to` required), and
`/api/status/resource/{id}/windows` (24h / 7d / 30d / 90d rollups).

::: tip
Responses are short-cached (fresh for 60s, served stale up to 30s more), so a
status page under load never hammers your database.
:::

::: warning
Every monitor and component surfaced here is world-readable. Use component names
and monitor labels that you're comfortable exposing publicly — the page is meant
for customers, not internal detail.
:::

## Branding & custom domain

Status page settings (name, homepage link, logos, favicon, primary color, and an
optional custom domain) are managed in Ogoune under the status page settings. The
public bundle is served as static assets, so you can also point a dedicated
hostname like `status.yourdomain.com` at it.

Next: [Incidents](/guide/incidents).

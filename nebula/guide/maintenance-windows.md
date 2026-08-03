# Maintenance windows

A **maintenance window** tells Ogoune that planned downtime is expected, so it
doesn't page you for it. While a monitor is inside an active window, Ogoune still
runs its checks and records the results — but it **suppresses false positives**:
it won't advance failure counts, won't flip the monitor to `down`, and won't open
incidents or fire notifications. The check results are simply tagged as
maintenance activity.

## Why use them

Deploys, database migrations, and host reboots make healthy services look down.
Without a window, a five-minute restart can trigger an incident and wake up
whoever is on call. A maintenance window keeps that noise out of your incident
history.

## Scheduling a window

Every window has a `strategy`, a `title`, and the `resource_ids` it applies to.
There are two strategies:

- **One-time** (`one_time`) — a single fixed window. Provide `start_at` and
  `end_at` as RFC 3339 timestamps.
- **Recurring** (`cron`) — a repeating window. Provide a `cron_expr`, a
  `window_minutes` duration (how long each occurrence lasts), and optionally a
  `timezone`. You can bound the recurrence with `effective_from` /
  `effective_until`.

Create one via the API (a read-write API key is required for writes):

```bash
# One-time window during a planned deploy
curl -X POST https://your-ogoune/api/maintenances \
  -H "Authorization: Bearer $OGOUNE_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "title": "Nightly deploy",
    "strategy": "one_time",
    "start_at": "2026-08-04T02:00:00Z",
    "end_at":   "2026-08-04T02:30:00Z",
    "resource_ids": ["01J...","01K..."]
  }'
```

For a recurring window, swap in `"strategy": "cron"`, e.g.
`"cron_expr": "0 2 * * 0"` with `"window_minutes": 60` for a Sunday 02:00
one-hour slot.

::: tip
A window's `status` moves through `scheduled → active → finished`. Need to end an
active window early? Call `POST /api/maintenances/{id}/finish`.
:::

::: warning
Starting a window does **not** resolve incidents that are already open — Ogoune
only stops *creating* and advancing incidents while the window is active. Close
any in-progress incident yourself, or schedule the window before the work begins.
:::

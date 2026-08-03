# Dashboards

A **dashboard** is a saved custom view: pick which monitors it covers (its
*scope*), drop in a few widgets, choose a time range and refresh interval, and
Ogoune remembers it. Dashboards are config-only — Ogoune stores the layout and
scope; the live metric data is rendered when you open the view.

Anyone signed in can read every dashboard (reads are instance-wide), but only the
**owner** (the user who created it) can edit or delete one.

## Scope: which monitors a dashboard shows

Every dashboard has a `scope` with a `mode` and a matching `payload`. The mode
decides which field of the payload is used:

| Mode | Payload field | Selects monitors by… |
|---|---|---|
| `tag` | `tagIds` | one or more tags |
| `component` | `componentIds` | membership in a status-page component |
| `type` | `types` | monitor type (e.g. `http`, `tcp`, `dns`) |
| `manual` | `resourceIds` | an explicit hand-picked monitor list |

::: tip
`tag`, `component`, and `type` scopes are *dynamic* — a monitor that later gains
the tag (or type) shows up automatically. `manual` is a fixed list you curate by
ID.
:::

## Widgets

Each dashboard holds an ordered list of widgets. A widget's `widgetTypeId` must be
one of:

- `uptime-stat` — uptime percentage tile
- `incidents-list` — recent incidents
- `response-time` — response-time chart
- `resource-status-grid` — up/down grid across the scoped monitors

`position` (an integer ≥ 0) sets the order; `title` and `config` are optional
per-widget overrides.

## Create a dashboard

`POST /api/v1/dashboards` with a Bearer token. Here a dashboard scoped to two
tags:

```bash
curl -X POST https://your-ogoune/api/v1/dashboards \
  -H "Authorization: Bearer <api-key>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Production",
    "scope": { "mode": "tag", "payload": { "tagIds": ["tag_01H…", "tag_02H…"] } },
    "widgets": [
      { "id": "w1", "widgetTypeId": "uptime-stat", "position": 0 },
      { "id": "w2", "widgetTypeId": "incidents-list", "position": 1 }
    ],
    "defaultTimeRange": "24h",
    "refreshInterval": "1m",
    "visibility": "team"
  }'
```

To scope by **component** instead, swap the scope block for
`{"mode":"component","payload":{"componentIds":["cmp_…"]}}`; for a **manual** list,
`{"mode":"manual","payload":{"resourceIds":["mon_…","mon_…"]}}`. Only the payload
field that matches the mode is read.

Allowed enum values (validated server-side; a bad value returns `422`):

- `defaultTimeRange`: `24h`, `7d`, `30d`, `90d`
- `refreshInterval`: `off`, `30s`, `1m`, `5m`
- `visibility`: `private`, `team`, `public`

## Update, reorder, delete

- `PATCH /api/v1/dashboards/{id}` — partial update; send only the fields you want
  to change.
- `PUT /api/v1/dashboards/{id}/layout` — replace the whole ordered widget list
  (used when you drag widgets around):

  ```json
  { "widgets": [ { "id": "w2", "widgetTypeId": "incidents-list", "position": 0 },
                 { "id": "w1", "widgetTypeId": "uptime-stat", "position": 1 } ] }
  ```

- `DELETE /api/v1/dashboards/{id}` — removes it.

::: warning
Edit, layout, and delete are **owner-only**. Calling them on someone else's
dashboard returns `403 FORBIDDEN`.
:::

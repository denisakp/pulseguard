# Tags & components

Ogoune gives you two ways to organise monitors. They sound similar but do very
different jobs: a **tag** is a flat label for filtering, a **component** is a
logical group with its own rolled-up status and alert de-duplication.

## Tags — labels for filtering

A tag is just a name plus two optional cosmetics: a `color` and a
`description`. Attach the same tag to many monitors (`env:prod`, `team:payments`,
`region:eu`) and use it to slice your monitor list. Tags carry **no health or
alerting behaviour** — removing a tag never changes what gets checked or who
gets paged.

Manage them under `/api/v1/tags` (create, list, get, update, patch, delete).

```bash
curl -X POST https://your-ogoune/api/v1/tags \
  -H "Authorization: Bearer $OGOUNE_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"name": "prod", "color": "#e11d48", "description": "Production surface"}'
```

## Components — grouped monitors with rolled-up status

A component groups related monitors (say, everything behind "Checkout") into one
unit with a **derived status**:

- **down** — at least one member is `down` or `error`
- **degraded** — at least one member is `warn`, `pending`, or `unknown`
- **up** — everything else (paused members count as up)

Unlike a tag, a component **must contain at least one resource** (`resource_ids`
is required), and you can't delete one while resources are still attached — the
API returns `409 COMPONENT_HAS_RESOURCES`. Move or detach members first (the
bulk-assign / bulk-remove endpoints help).

### Grouping window — one alert instead of many

The `grouping_window_seconds` field is what makes a component more than a folder.
When a shared dependency fails, its monitors often fall over within seconds of
each other. With a grouping window set, Ogoune waits out a sliding timer before
notifying, then sends **a single component-level alert** listing every impacted
monitor — instead of one page per monitor.

Set it to `0` to disable (immediate per-status notification), or any value
**between 10 and 300 seconds**.

```bash
curl -X POST https://your-ogoune/api/v1/components \
  -H "Authorization: Bearer $OGOUNE_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Checkout",
    "description": "Payment path",
    "resource_ids": ["01H...", "01H...", "01H..."],
    "grouping_window_seconds": 60
  }'
```

::: tip
Point notification channels at the component itself. When a resource belongs to a
component, Ogoune resolves alerts through the component's channels — so the whole
group routes to one place.
:::

::: warning
The grouping window only affects **when and how alerts are batched**. It does not
delay the checks themselves — monitor status still updates in real time.
:::

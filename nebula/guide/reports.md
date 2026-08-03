# Monthly reports

Ogoune can email a **monthly uptime report** summarising the previous calendar
month across all your monitors. It's an at-a-glance recap for stakeholders who
don't live in the dashboard — one message, first of the month, no dashboard
login required.

## What's in the report

Each report covers one completed month and contains:

- **Overall uptime** percentage across all resources
- **Incident count** for the period
- **Total downtime** (in seconds)
- A **per-resource breakdown** — name, uptime %, and incident count for each monitor

Scope is currently **all resources** — the report always covers your whole fleet.

## How it works

The schedule is **fixed**: the report is generated for the *previous completed
month* and there is no cron expression to configure. A background job runs once at
startup and then daily; on each tick it checks whether the previous month already
has a report and, if not, generates and sends it. This means a missed run (e.g.
the instance was down on the 1st) **self-heals** on the next start or daily tick.
Generation is **idempotent per period** — a given month is only ever sent once.

Delivery goes through your **oldest configured SMTP notification channel**, sent to
the recipient address you set. So you must have at least one
[SMTP channel](/guide/notifications) configured for reports to be delivered.

::: warning
If SMTP delivery fails, the report is still recorded with status `failed` and the
job **never aborts** — the failure is logged, but future months keep generating.
A successfully sent report is recorded as `delivered`.
:::

## Enabling and disabling

In the reports settings, toggle **Enabled** and set a **recipient email**. A valid
recipient address is **required** whenever reports are enabled. Disabling the
toggle stops future reports; already-generated history is kept.

::: tip
Use the **preview** to see the current in-progress month's numbers on demand — it
is computed live and **not persisted**, so it never counts as the month's report.
:::

## Report history

Past reports are listed newest-first (default 6, up to 50). Each history entry
exposes `period`, `sentAt`, `status`, `uptimePct`, `incidentCount`,
`downtimeSeconds`, `recipientEmail`, and the `resourceBreakdown` array.

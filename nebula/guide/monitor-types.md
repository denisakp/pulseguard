# Monitor types

Ogoune ships several check strategies. Each implements a common `CheckStrategy`
contract, so new types can be added without touching the scheduler or worker pool.

| Type | Checks | Key fields |
|---|---|---|
| **HTTP** | Status code (a `HEAD` in the 200–399 range is UP), response time, redirects | `target` (full URL) |
| **TCP** | Port reachability | `target` (`host:port`) |
| **DNS** | Record resolution (`LookupHost`) | `target` (hostname) |
| **ICMP** | Ping / host reachability (single echo probe) | `target` (host) |
| **Keyword** | Presence/absence of a string in the response body | `keyword`, `keyword_mode` |
| **Heartbeat** | Push-based — you ping Ogoune on a schedule | `heartbeat_interval`, `heartbeat_grace` |
| **Protocol** | Application-layer handshakes (Redis, MongoDB, FTP, SSH, MySQL, PostgreSQL, RabbitMQ, Kafka) | `protocol_type`, `protocol_port` |

Every monitor also takes the common fields `name`, `type`, `interval` (10–3600 s),
`timeout` (1–60 s), and optional `confirmation_checks` / `confirmation_interval`
(how many consecutive failures confirm an incident before alerting).

## Keyword

A `GET` request reads up to 512 KB of the body, then applies a **case-sensitive**
substring match. `keyword_mode` is `contains` (UP when found) or `not_contains`
(UP when absent).

```json
{ "type": "keyword", "target": "https://example.com/health",
  "keyword": "OK", "keyword_mode": "contains" }
```

## Heartbeat (push)

Instead of Ogoune reaching out, a heartbeat monitor waits for **your** job (cron,
backup, worker) to check in. Ogoune generates a `heartbeat_slug`; ping it from your
job:

```
POST /api/v1/heartbeat/ping/{slug}
```

The endpoint is public — no auth header — so a `curl` at the end of a script is
enough. Configure `heartbeat_interval` (expected seconds between pings) and
`heartbeat_grace` (extra slack before it's marked down).

::: tip
A brand-new heartbeat stays in a *waiting* state until its first ping arrives, so
it never alerts before your job has run once.
:::

## SSL & domain expiry

SSL certificate and domain (WHOIS) expiry are **not** a separate monitor type —
they're enrichment on your active **HTTP** monitors. A daily job populates each
resource's SSL issuer / expiry date and domain registrar / expiry date, then
alerts as the deadline approaches.

Thresholds are set per monitor via `expiry_alert_thresholds` (a comma-separated
list of days-remaining, each 1–365). The default is `30,14,7,1`:

```json
{ "type": "http", "target": "https://example.com",
  "expiry_alert_thresholds": "30,14,7,1" }
```

Status is derived from days remaining: **expired** (≤ 0), **critical** (≤ 7),
**warning** (≤ 30), otherwise **ok**.

## Protocol with authentication

A protocol monitor picks its handler from `protocol_type` and connects on
`protocol_port` (or the default: Redis 6379, MongoDB 27017, FTP 21, SSH 22,
MySQL 3306, PostgreSQL 5432, RabbitMQ 5672, Kafka 9092). TLS is inferred from the
target — `rediss://`, `?tls=true`, or `?sslmode=require`.

For Redis, MySQL, and PostgreSQL you can go beyond port reachability and verify a
real login. Create the monitor, then attach a credential:

```
POST /api/v1/resources/{id}/credentials
{ "username": "monitor", "password": "s3cret" }
```

Redis uses `AUTH` (the ACL `AUTH <user> <pass>` form when a username is set);
MySQL and PostgreSQL open an authenticated connection and ping it. An auth failure
is reported distinctly from an unreachable port.

::: warning
Credentials are encrypted at rest (AES-256-GCM) and the password is never returned
by the API — reads show a mask. Without a credential, MySQL/PostgreSQL monitors
fall back to a plain TCP reachability check.
:::

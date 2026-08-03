# Enterprise Edition

The Enterprise Edition (EE) is the commercial license for features that only make sense in a
**multi-tenant, Cloud context** — not a "bigger database" tier.

::: tip Scaling Community Edition doesn't need EE
PostgreSQL + Redis/Asynq are part of the free, Apache 2.0 Community Edition — see
[Production self-hosting](/self-host/production). You don't need an EE license to run
Ogoune at production scale on your own infrastructure.
:::

EE and Community share one codebase. EE features are documented publicly here — running them
requires a valid license.

## What EE licenses

- **Team management** — roles (Owner / Admin / Member / Viewer), invitation flow, permission matrix
- **SSO / SAML** — Okta, Auth0, Azure AD, Google Workspace, generic SAML 2.0
- **Multi-tenancy** — organization isolation, required for Cloud and shared deployments
- **SOC 2 audit logs** — full action trail (user, action, resource, IP, outcome), filterable and exportable
- **Certified compliance** — FIPS 140-2 / HIPAA-validated Vault-backed key management, advanced GDPR tooling
- **Dedicated support** — SLA-backed response, security questionnaires, DPA

::: warning Most of this is still being built
As of this writing, the features above are on the
[roadmap](https://github.com/denisakp/ogoune/blob/main/roadmap.md#h3--enterprise-edition-q4-2026--q1-2027)
for Q4 2026 / Q1 2027. Today, `internal/ee/license/` only detects which edition is running
(via the `ENTERPRISE_LICENSE_KEY` prefix) — it doesn't yet gate any behavior. We'd rather say
that plainly than ship a page that overclaims.
:::

See [Licensing](/enterprise/licensing) for how edition detection works and how to obtain a
commercial license.

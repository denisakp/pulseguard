# API access & account security

Two things secure programmatic and human access to Ogoune: **API keys** for
scripts and integrations, and **two-factor authentication (2FA)** for your own
login. Both are managed from your account settings.

## API keys

An API key lets a script, CI job, or integration talk to the Ogoune API without
a browser session. Create one under **Account → API keys**: give it a name, pick
a scope, and optionally set an expiry date.

Every key carries one of two **scopes**:

- **`read`** — read-only. The key can `GET` monitors, incidents, hosts, and other
  resources, but any write (create, update, delete) is rejected with `403`.
- **`read_write`** — full read plus writes. Use this for automation that creates
  monitors, acknowledges incidents, or mutates configuration.

::: warning
The full key is shown **once**, at creation time. Copy it immediately — Ogoune
only ever stores a hash plus a short `pk_live_…` prefix, so it can never show you
the secret again. Lost it? Revoke the key and create a new one.
:::

Keys are long-lived until you revoke them (**Account → API keys → Revoke**) or
their optional expiry passes. Ogoune records each key's last-used time and IP so
you can spot a stale or leaked credential.

### Using a key

Pass the key on every request, either in an `X-API-Key` header or as a bearer
token — both are accepted:

```bash
# X-API-Key header
curl https://your-ogoune/api/v1/monitors \
  -H "X-API-Key: pk_live_xxxxxxxxxxxx"

# ...or as a bearer token
curl https://your-ogoune/api/v1/monitors \
  -H "Authorization: Bearer pk_live_xxxxxxxxxxxx"
```

::: tip
API keys can't manage other API keys — creating, listing, and revoking keys
requires a logged-in session. This keeps a leaked key from minting more keys.
:::

## Two-factor authentication (2FA)

2FA adds a time-based one-time password (TOTP) on top of your password. Enable it
under **Account → Security → Enable 2FA**:

1. Ogoune shows a QR code (an `otpauth://` URL). Scan it with an authenticator app
   (Google Authenticator, 1Password, Authy, …).
2. Enter the current 6-digit code to confirm. Ogoune then shows **10 one-time
   backup codes** — store them somewhere safe.

Once enabled, logging in becomes two steps: after your password, Ogoune returns a
short-lived 2FA challenge and asks for the current authenticator code before
issuing your session. Lost your device? Use one of the backup codes, or trigger
an **email-based 2FA reset** from the login screen.

::: warning
Backup codes are shown once and each works only once. Disabling 2FA (**Account →
Security → Disable 2FA**) also requires a valid code.
:::

# Self-hosting

terminal.army runs as a single `tarmy` binary plus one Postgres database. The
easiest way to host your own universe is Docker Compose.

## Docker Compose

```bash
git clone https://github.com/cobanov/terminal-army-go.git
cd terminal-army-go

cp .env.example .env
$EDITOR .env                      # at minimum set TARMY_JWT_SECRET

docker compose up -d --build      # postgres, one-shot migrate, tarmy
docker compose exec tarmy tarmy admin seed-universe
```

The stack has three services:

- **postgres** — the only persistent dependency (data in the `tarmy_pgdata` volume).
- **tarmy-migrate** — applies database migrations once, then exits.
- **tarmy** — the long-running HTTP server and queue scheduler.

The server listens on `:8080` inside the container. Put it behind a reverse
proxy (Caddy, nginx, Traefik) for TLS.

## Required configuration

| Variable | Purpose |
|----------|---------|
| `DATABASE_URL` | Postgres connection string (`postgres://…?sslmode=…`). |
| `TARMY_JWT_SECRET` | HMAC secret for session tokens (≥ 16 chars; use 32+). |
| `TARMY_PUBLIC_URL` | The external URL the client prints back for browser sign-in. |

Everything else has sensible defaults — universe shape
(`TARMY_DEFAULT_UNIVERSE_*`), JWT TTL, bcrypt cost, scheduler interval, log
level. See `.env.example` for the full set.

## Administration

```bash
docker compose exec tarmy tarmy admin seed-universe     # create the default universe (idempotent)
docker compose exec tarmy tarmy admin promote <user>    # grant admin role
docker compose exec tarmy tarmy admin demote  <user>    # revoke admin role
docker compose exec tarmy tarmy admin stats             # server-wide counters
docker compose exec tarmy tarmy admin list-users
```

Register a normal account on the site first, then `promote` it to unlock the
`/admin` panel.

## Upgrades

```bash
git pull
docker compose up -d --build      # migrate runs automatically before tarmy starts
```

## Security defaults

- bcrypt (cost 12) password hashing, HS256 session tokens with per-device
  session rows so logout truly invalidates a token.
- HttpOnly + SameSite cookies, `Secure` over TLS; double-submit CSRF on forms.
- Strict Content-Security-Policy, HSTS, `X-Frame-Options: DENY`.
- Per-IP rate limiting on auth endpoints.

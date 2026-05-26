# terminal-army-go

An OGame-style multiplayer strategy game you play from a terminal. Single Go
binary, single Postgres database, no JavaScript build pipeline.

This is a ground-up Go rewrite of the original Python prototype at
`cobanov/terminal.army`. It keeps the game mechanics (taken verbatim from the
[OGame Fandom Wiki](https://ogame.fandom.com/wiki/OGame_Wiki)) and adds a
proper server architecture, real authentication, a web surface for signup and
alliances, and an admin CLI.

## Why a rewrite

The Python prototype was a great way to learn the game loop, but it conflated
the TUI and the server, kept some state in-process, and asked for a Celery
worker plus Redis just to schedule queue events. The Go rewrite collapses all
of that into:

- one statically linked binary (`tarmy`)
- one Postgres database
- no third-party broker, no extra runtime, no JS toolchain

You ship the binary, point it at Postgres, and it serves HTTP, WebSocket, the
queue scheduler, the TUI, and the admin CLI from the same image.

## What ships in the binary

`tarmy` is a Cobra root command with four subcommand trees:

```
tarmy serve                 run HTTP/WebSocket API + queue scheduler
tarmy play [--server URL]   launch the Bubble Tea TUI client
tarmy migrate up|down|version
tarmy admin seed-universe   create the default universe (idempotent)
tarmy admin promote <user>  grant the admin role
tarmy admin demote <user>   restore the player role
tarmy admin stats           server-wide counters
tarmy admin list-users [--limit N] [--offset M]
```

Run `tarmy --help` for the full surface; every subcommand has its own
`--help`.

## Quick start: server operator (Docker)

```bash
git clone https://github.com/cobanov/terminal-army-go.git
cd terminal-army-go

cp .env.example .env
$EDITOR .env                              # at minimum set TARMY_JWT_SECRET

docker compose up -d --build              # postgres, migrate, tarmy
docker compose exec tarmy tarmy admin seed-universe
```

The server is now on `http://localhost:8080`. The migrate service runs once
and exits; the `tarmy` service keeps the API, WebSocket hub, and queue
scheduler online. See `docker-compose.yml` for the full env contract.

To make the first user an admin:

```bash
docker compose exec tarmy tarmy admin promote <username>
```

## Quick start: end user (just play)

If somebody else is running the server and you only want to play:

```bash
curl -fsSL https://raw.githubusercontent.com/cobanov/terminal-army-go/main/install.sh | bash

tarmy play --server https://your-friends-server.example
```

The install script picks the right binary for your OS and CPU (Linux or
macOS, amd64 or arm64), verifies its SHA256 if the release ships one, and
drops it into `/usr/local/bin` (or `~/.local/bin` if you are non-root).

## Quick start: local dev

```bash
docker compose up -d postgres
cp .env.example .env
make build
./tarmy migrate up
./tarmy serve &
./tarmy play
```

`make` targets you will likely use:

```
make build         # ./tarmy
make test          # go test -race
make test-cover    # writes coverage.out + coverage.html
make lint          # gofmt + go vet
make migrate-up    # apply migrations using local DATABASE_URL
make sqlc          # regenerate query packs (sqlc.yaml drives this)
make tidy          # go mod tidy
make clean
```

## Configuration

Every knob is an environment variable read once at startup by
`internal/config`. Defaults are baked in for local dev; production deploys
override what they need. See `.env.example` for the full set.

The required variables for production:

| Var                 | What it does                                  |
|---------------------|-----------------------------------------------|
| `DATABASE_URL`      | `postgres://...?sslmode=...` connection string |
| `TARMY_JWT_SECRET`  | HMAC secret for session tokens (32+ chars)    |
| `TARMY_PUBLIC_URL`  | External URL the TUI/web layer prints back    |

Everything else has a sensible default, including the seed-universe shape
(`TARMY_DEFAULT_UNIVERSE_*`) and rate-limit knobs.

## Architecture in one paragraph

`cmd/tarmy` wires Cobra commands. `internal/config` loads env into a typed
`Config`. `internal/store` holds the pgx pool, hand-written SQL queries, and
the embedded golang-migrate migrations. `internal/svc` is the service layer
(one file per domain: auth, planet, build, research, fleet, alliance, etc.).
`internal/httpapi` mounts the chi-based REST router and the WebSocket
endpoint. `internal/web` serves the HTML signup, login, alliance, and admin
pages (html/template + embed.FS). `internal/scheduler` polls the build and
fleet queues with `FOR UPDATE SKIP LOCKED`. `internal/tui` is the Bubble Tea
client. `internal/ws` is the WebSocket hub. `internal/game` is the pure-Go
formula library (no DB, no HTTP) sourced from the OGame Fandom Wiki and is
the only place where game math lives.

For a complete feature-by-feature map against the Python prototype, see
[`docs/COVERAGE.md`](docs/COVERAGE.md).

## Game mechanics: source of truth

Every formula, every coefficient, every tech tree prerequisite comes from the
[OGame Fandom Wiki](https://ogame.fandom.com/wiki/OGame_Wiki). When a value
in this codebase ever feels off, the wiki wins. The relevant pages are
cross-referenced in `internal/game/constants.go` and
`internal/game/formulas.go` so you can audit any number back to its source.

## Security defaults

- bcrypt at cost 12 for passwords
- HS256 JWTs, signed by `TARMY_JWT_SECRET`, with per-token device-session
  rows so logout actually invalidates the bearer
- HttpOnly + SameSite=Lax cookies, Secure when the request arrives over TLS
- Double-submit CSRF token on every POST form
- Strict Content-Security-Policy, HSTS, X-Frame-Options=DENY,
  X-Content-Type-Options=nosniff
- Per-IP token-bucket rate limit on `POST /api/v1/auth/*` and on web auth
  forms (10 burst, 1 per 5 s sustained)
- The web admin surface (`/admin`, `/admin/users`) is role-gated by the
  `requireAdmin` middleware; non-admin sessions get HTTP 403
- The web layer refuses to demote your own account (forces you to use
  `tarmy admin demote` so you cannot lock yourself out by a misclick)

## Runtime image

The Dockerfile builds a fully static binary with `CGO_ENABLED=0`, then copies
it into `gcr.io/distroless/static-debian12:nonroot`. Result:

- ~25 MB final image
- no shell, no libc, no package manager
- runs as uid 65532 by default
- the healthcheck runs `tarmy version` because there is no `sh` to wrap it in

The Docker layer cache keys on `go.mod`/`go.sum`, so iterative rebuilds skip
the module download. `--build-arg VERSION=v0.1.0 --build-arg COMMIT=$(git rev-parse HEAD)`
get baked into the binary via `-ldflags -X internal/version.*`.

## Repository layout

```
cmd/tarmy/                  Cobra entrypoint and subcommand wiring
internal/
  auth/                     JWT signer + chi auth middleware
  config/                   env -> typed Config loader
  game/                     pure-Go formulas, constants, tech tree
  httpapi/                  chi router + REST + WebSocket mount
  scheduler/                queue completion + fleet arrival worker
  store/                    pgx pool, queries, embedded migrations
  svc/                      service layer (one file per game domain)
  svc/admin/                admin CLI implementations
  tui/                      Bubble Tea client (login -> overview -> ...)
  util/                     small helpers (random codes, time)
  version/                  ldflags-injected version metadata
  web/                      signup, login, alliance, admin HTML surface
  ws/                       WebSocket hub
docs/COVERAGE.md            feature parity matrix vs Python prototype
docker-compose.yml          postgres + tarmy-migrate + tarmy
Dockerfile                  multi-stage build, distroless runtime
install.sh                  one-line binary installer
```

## Status

MVP-complete: registration, login, universe, planet, lazy resource update,
buildings + research + shipyard + defense queues, scheduler, galaxy view,
fleet movements, combat resolver, messages, reports, alliance, leaderboard,
stats, presence, TUI client, web signup/login/alliance/admin, full admin
CLI, Docker deploy, installer.

What is intentionally deferred:

- Moon mechanics (Phalanx, Jump Gate, debris-field moon-chance) are not yet
  modelled; planets only
- Espionage probe combat math beyond the basic raid resolver
- Merchant/trade routes
- Achievements, daily login rewards
- Browser front-end (the JSON API is stable and ready for one; the HTML
  surface is intentionally minimal)

See `docs/COVERAGE.md` for the line-by-line list.

## License

MIT.

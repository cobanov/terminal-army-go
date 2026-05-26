# Feature coverage vs the Python prototype

This document tracks where the Go rewrite stands against the original Python
prototype at `cobanov/terminal.army`. The columns mean:

- **Done** -- ported, wired into HTTP/TUI/scheduler, and reachable end-to-end
- **Partial** -- core logic exists but a piece of UX or admin glue is missing
- **Deferred** -- not in MVP; tracked for a later phase
- **Skipped** -- intentionally not ported (the Python version was wrong, or
  the feature is not part of the rewrite's scope)

Everything marked **Done** has been smoke-tested against a local
docker-compose stack. Anything that needs a more rigorous test plan than that
is called out in the "Known gaps" section at the end.

## Game mechanics (the math)

| Mechanic                              | Status  | Where it lives                                      | Notes                                                                 |
|---------------------------------------|---------|-----------------------------------------------------|-----------------------------------------------------------------------|
| Position-based temperature ranges     | Done    | `internal/game/constants.go` (PositionTemperature)   | Slots 1-15, T_max - T_min = 40, drawn at colonisation                 |
| Position-based metal/crystal bonus    | Done    | `internal/game/constants.go` (PositionResourceBonus) | Mirrors the Fandom table verbatim                                     |
| Position-based field range            | Done    | `internal/game/constants.go` (PositionFields)        | Random per planet inside the range                                    |
| Mine production formulas (M/C/D)      | Done    | `internal/game/production.go`                        | `30/20/10 * L * 1.1^L * speed * (...)` straight from the wiki         |
| Base passive production               | Done    | `internal/game/production.go`                        | 30 m/h, 15 c/h, 0 d/h scaled by economy speed                         |
| Energy production (solar plant)       | Done    | `internal/game/production.go`                        | `floor(20 * L * 1.1^L)`                                               |
| Energy production (solar satellite)   | Done    | `internal/game/production.go`                        | `floor((avg_temp + 160) / 6)` capped at 65                            |
| Energy production (fusion reactor)    | Done    | `internal/game/production.go`                        | `floor(30 * L * (1.05 + energy_tech*0.01)^L)` + deut consumption      |
| Mine energy consumption + factor      | Done    | `internal/game/production.go`                        | `production_factor = min(1, produced/consumed)`                       |
| Building cost formula                 | Done    | `internal/game/formulas.go`                          | `cost(L+1) = base * factor^L` with the Fandom base/factor table        |
| Research cost formula                 | Done    | `internal/game/formulas.go`                          | `cost(L+1) = base * 2^L` (1.75 for Astrophysics, 3.0 for Graviton)    |
| Build time formula                    | Done    | `internal/game/formulas.go`                          | `(M+C) / (2500 * (1+robotics) * speed * 2^nanite)` hours              |
| Research time formula                 | Done    | `internal/game/formulas.go`                          | `(M+C) / (1000 * speed * (1 + research_lab))` hours                   |
| Tech-tree prerequisite checking       | Done    | `internal/game/tech_tree.go`                         | One source of truth for "can the player queue this?"                  |
| Fleet flight time, deut, speed math   | Done    | `internal/game/fleet.go`                             | Classic OGame `sqrt` formula with engine-tech speed multipliers       |
| Basic combat resolver                 | Done    | `internal/game/fleet.go` + `internal/svc/fleet.go`   | Attack vs defender stacks, rapid-fire matrix, deut return for raids   |
| Espionage probe combat                | Partial | `internal/svc/fleet.go`                              | Probes can be sent; counter-espionage probability table not modelled  |
| Moon creation chance after combat     | Deferred| -                                                   | Tracked for the moons milestone                                       |
| Phalanx / jump gate                   | Deferred| -                                                   | Requires moons first                                                  |
| Plasma technology bonus               | Done    | `internal/game/production.go`                        | `(1 + plasma * 0.01)` metal, `0.0066` crystal, `0.0033` deut          |

All numeric constants used by the formulas above carry a Fandom Wiki URL in
the comment block immediately above them. If a number ever drifts from the
wiki, that comment is the authoritative diff target.

## Server and runtime

| Capability                            | Status  | Where it lives                                      | Notes                                                                 |
|---------------------------------------|---------|-----------------------------------------------------|-----------------------------------------------------------------------|
| Single-binary distribution            | Done    | `cmd/tarmy`                                         | One Cobra root, four subcommand trees                                 |
| Postgres pool (pgx v5)                | Done    | `internal/store`                                    | Hand-written queries; sqlc.yaml present for future generation         |
| Embedded migrations (golang-migrate)  | Done    | `internal/store/migrations`                         | Shipped via `go:embed`; `tarmy migrate up\|down\|version`             |
| HTTP server (chi v5)                  | Done    | `internal/httpapi/server.go`                        | Graceful shutdown, request logger, recoverer, request ID              |
| Structured logging (slog)             | Done    | `internal/httpapi/server.go`                        | JSON in prod (default), text optional via env                         |
| WebSocket hub                         | Done    | `internal/ws/ws.go`                                 | Per-user fan-out, integrated with svc.EventSink                       |
| Queue scheduler                       | Done    | `internal/scheduler/scheduler.go`                   | `FOR UPDATE SKIP LOCKED` for completion; configurable tick interval   |
| Fleet arrival worker                  | Done    | `internal/scheduler/fleet.go`                       | Same locking pattern; broadcasts arrival events                       |
| Lazy resource update                  | Done    | `internal/svc/resources.go`                         | Called on every read/write path; `resources_last_updated_at` advances |
| Presence tracking                     | Done    | `internal/svc/presence.go`                          | In-memory, fed by HTTP middleware + WebSocket connect/disconnect      |
| Rate limiting (per IP, token bucket)  | Done    | `internal/httpapi/ratelimit.go` + `internal/web/ratelimit.go` | Same shape on both surfaces (10 burst, 1/5s sustained)                |

## Authentication and accounts

| Capability                            | Status  | Where it lives                                      | Notes                                                                 |
|---------------------------------------|---------|-----------------------------------------------------|-----------------------------------------------------------------------|
| Username/email/password registration  | Done    | `internal/svc/auth.go`                              | bcrypt cost 12 by default                                             |
| Login with constant-time failure      | Done    | `internal/svc/auth.go`                              | Dummy bcrypt hash compared on missing user to flatten timing          |
| JWT issuance + verification           | Done    | `internal/auth/jwt.go`                              | HS256 signed by `TARMY_JWT_SECRET`; configurable TTL                  |
| Device sessions (revocable)           | Done    | `internal/svc/auth.go` + `device_sessions` table    | Logout deletes the row; absent row -> 401 even if JWT still parses    |
| `last_seen_at` bump on session use    | Done    | `internal/svc/auth.go::ResolveSession`              | Drives the admin `list-users` "last seen" column                      |
| Role column (player/admin)            | Done    | `internal/store/users.go` + migrations              | Default "player"; admin role required for the admin web surface       |
| Password reset / email verification   | Deferred| -                                                   | Needs an email transport; out of MVP scope                            |

## Game services (per domain)

| Domain                                | Status  | File                                                | Notes                                                                 |
|---------------------------------------|---------|-----------------------------------------------------|-----------------------------------------------------------------------|
| Universe + join + speed knobs         | Done    | `internal/svc/universe.go`                          | Speed is env-seeded; `tarmy admin seed-universe` is idempotent         |
| Planet + colonisation + slot pick     | Done    | `internal/svc/planet.go` + `internal/game/colonization.go` | Random `(galaxy, system, position)` insert with unique-constraint retry |
| Buildings: queue, list, cancel        | Done    | `internal/svc/build.go`                             | `SELECT FOR UPDATE` on planet row before queue insert                 |
| Research: queue, list, cancel         | Done    | `internal/svc/research.go`                          | Player-scoped; tech-tree prerequisite enforced                        |
| Shipyard: queue, list                 | Done    | `internal/svc/shipyard.go`                          | Shares the building queue scheduler                                   |
| Defense: queue, list                  | Done    | `internal/svc/shipyard.go`                          | Same as shipyard, separate item type                                  |
| Fleet dispatch + recall               | Done    | `internal/svc/fleet.go`                             | All fleet mission types resolved by `internal/scheduler/fleet.go`     |
| Combat reports                        | Done    | `internal/svc/reports.go`                           | Stored as a message of report type, retrievable via dedicated endpoint |
| Messages (in-game inbox)              | Done    | `internal/svc/messages.go`                          | List, read, delete                                                    |
| Alliance create/join/leave            | Done    | `internal/svc/alliance.go`                          | One-alliance-per-user; founder cannot leave with members remaining    |
| Leaderboard                           | Done    | `internal/svc/leaderboard.go`                       | Materialised from planet/research/fleet sums                          |
| Server stats                          | Done    | `internal/svc/stats.go`                             | Powers `tarmy admin stats` and the `/admin` dashboard                 |

## HTTP API surface

| Endpoint                              | Status  | Notes                                                                 |
|---------------------------------------|---------|-----------------------------------------------------------------------|
| `POST /api/v1/auth/register`          | Done    | Rate-limited per IP                                                   |
| `POST /api/v1/auth/login`             | Done    | Rate-limited per IP                                                   |
| `POST /api/v1/auth/logout`            | Done    | Revokes the device session                                            |
| `GET  /api/v1/me`                     | Done    | Returns the current user + active planet snapshot                     |
| `GET  /api/v1/universes`              | Done    | All universes the user can join                                       |
| `POST /api/v1/universes/{id}/join`    | Done    | Creates the user's first planet in that universe                      |
| `GET  /api/v1/planets`                | Done    | All planets owned by the caller                                       |
| `GET  /api/v1/planets/{id}`           | Done    | One planet with refreshed resources                                   |
| `GET  /api/v1/planets/{id}/production`| Done    | Per-hour and per-second projections                                   |
| `POST /api/v1/planets/{id}/buildings` | Done    | Queue a building upgrade                                              |
| `POST /api/v1/planets/{id}/shipyard`  | Done    | Queue ships                                                           |
| `POST /api/v1/planets/{id}/defense`   | Done    | Queue defense                                                         |
| `GET  /api/v1/planets/{id}/queues`    | Done    | Aggregated view of all queues for a planet                            |
| `GET  /api/v1/research`               | Done    | Player-wide research levels                                           |
| `POST /api/v1/research`               | Done    | Queue a research                                                      |
| `GET  /api/v1/galaxy/{g}/{s}`         | Done    | 15-slot system view; honours other players' visibility rules          |
| `POST /api/v1/fleet`                  | Done    | Dispatch                                                              |
| `GET  /api/v1/fleet`                  | Done    | In-flight + returning fleets                                          |
| `POST /api/v1/fleet/{id}/recall`      | Done    | Recall (clamps to the elapsed half-flight rule)                       |
| `GET/DELETE /api/v1/messages...`      | Done    | Inbox CRUD                                                            |
| `GET  /api/v1/reports...`             | Done    | Combat / espionage reports                                            |
| `GET/POST /api/v1/alliance...`        | Done    | Lobby + create + detail + join + leave                                |
| `GET  /api/v1/leaderboard`            | Done    | Top-N + caller rank                                                   |
| `GET  /api/v1/stats`                  | Done    | Server-wide counters                                                  |
| `GET  /api/v1/ws`                     | Done    | WebSocket upgrade, JWT in the `Sec-WebSocket-Protocol` header         |

## Web (HTML) surface

| Page                                  | Status  | Notes                                                                 |
|---------------------------------------|---------|-----------------------------------------------------------------------|
| `GET  /`                              | Done    | Landing page with install snippet + login/signup CTAs                 |
| `GET/POST /signup`                    | Done    | Rate-limited; CSRF                                                    |
| `GET/POST /login`                     | Done    | Rate-limited; CSRF; `?next=` aware                                    |
| `POST /logout`                        | Done    | Drops the cookie + revokes the device session                         |
| `GET  /alliance`                      | Done    | Lobby + create form (members only)                                    |
| `GET  /alliance/{id}`                 | Done    | Detail with join/leave actions                                        |
| `POST /alliance/{id}/join`            | Done    | CSRF + requireLogin                                                   |
| `POST /alliance/{id}/leave`           | Done    | CSRF + requireLogin                                                   |
| `GET  /admin`                         | Done    | Admin-only dashboard with server counters + universe table            |
| `GET  /admin/users`                   | Done    | Paginated user list with role pills                                   |
| `POST /admin/users`                   | Done    | Promote / demote; self-demotion blocked                               |

The HTML surface is deliberately tiny. Heavy interactive features live in the
TUI or are reachable through the REST API.

## TUI screens

| Screen                                | Status  | Notes                                                                 |
|---------------------------------------|---------|-----------------------------------------------------------------------|
| Login / register                      | Done    | `internal/tui/screen_login.go`, `screen_register.go`                  |
| Universe pick                         | Done    | `internal/tui/screen_universe.go`                                     |
| Overview                              | Done    | Resources, production rates, queues                                   |
| Buildings                             | Done    | Catalog + queue + cancel                                              |
| Research                              | Done    | Catalog + queue                                                       |
| Shipyard                              | Done    | Queue + per-planet inventory                                          |
| Defense                               | Done    | Same shape as shipyard                                                |
| Galaxy view                           | Done    | 15-slot system table with player names and alliance tags              |
| Fleet                                 | Done    | Active fleets + dispatch screen                                       |
| Fleet dispatch                        | Done    | Step-by-step target/ship/mission/speed flow                           |
| Messages                              | Done    | Inbox list + detail + delete                                          |
| Leaderboard                           | Done    | Top-N + scroll                                                        |
| Stats                                 | Done    | Mirrors the admin dashboard                                           |

## Admin surface

| Capability                            | Status  | Where it lives                                                        |
|---------------------------------------|---------|-----------------------------------------------------------------------|
| `tarmy admin seed-universe`           | Done    | `internal/svc/admin/admin.go`                                         |
| `tarmy admin promote <user>`          | Done    | `internal/svc/admin/admin.go`                                         |
| `tarmy admin demote <user>`           | Done    | `internal/svc/admin/admin.go`                                         |
| `tarmy admin stats`                   | Done    | `internal/svc/admin/admin.go`                                         |
| `tarmy admin list-users`              | Done    | `internal/svc/admin/admin.go`                                         |
| `/admin` dashboard                    | Done    | `internal/web/admin_handlers.go` + `templates/admin_dashboard.html`   |
| `/admin/users` paginated              | Done    | `internal/web/admin_handlers.go` + `templates/admin_users.html`       |
| Role-gated middleware                 | Done    | `internal/web/session.go::requireAdmin`                               |
| Self-demotion guard (web)             | Done    | `internal/web/admin_handlers.go::handleAdminUserPost`                 |

## Deploy and packaging

| Capability                            | Status  | Where it lives                                                        |
|---------------------------------------|---------|-----------------------------------------------------------------------|
| Multi-stage Dockerfile                | Done    | `Dockerfile`                                                          |
| Distroless runtime image              | Done    | `Dockerfile`                                                          |
| docker-compose stack                  | Done    | `docker-compose.yml` (postgres + tarmy-migrate + tarmy)               |
| One-line install script               | Done    | `install.sh`                                                          |
| Healthcheck via `tarmy version`       | Done    | `docker-compose.yml`                                                  |
| Version baked in via `-ldflags`       | Done    | `internal/version/version.go` + Dockerfile `--build-arg`              |

## Known gaps

The MVP is feature-complete against the Python prototype, but a few items
were intentionally pushed beyond the rewrite milestone:

1. **Test coverage is light.** The pure-math packages (`internal/game/*`)
   have stable, fully described formulas and are the obvious next target for
   a property-test pass; the service layer leans on smoke testing through
   docker-compose for now. Adding `go test -race` coverage is the first
   post-MVP task.
2. **No moons.** That includes Phalanx, Jump Gate, and the moon-chance
   roll after combat. Tracked for the moons milestone.
3. **Espionage** is implemented far enough to dispatch probes and resolve a
   raid, but the counter-espionage probability table and the detailed
   spy-report tiers are not in yet.
4. **No browser front-end.** The REST API is stable and documented in
   `internal/httpapi/routes.go`; a separate frontend repo can sit on top
   without changes to this binary.
5. **No email transport**, so password reset and email verification are
   deferred until SMTP/SES wiring lands.
6. **In-memory rate limiter and presence tracker.** They are correct for a
   single-instance deploy. Multi-instance deploys will need to swap both for
   a shared backend (Redis or Postgres) once horizontal scaling becomes a
   real requirement.

Everything in this document refers to behaviour reachable on `main` as of
the initial release commit. When a milestone above moves from Partial or
Deferred to Done, update the row and link the PR.

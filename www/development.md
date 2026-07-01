# Development

## Build from source

```bash
git clone https://github.com/cobanov/terminal-army-go.git
cd terminal-army-go

docker compose up -d postgres
cp .env.example .env
make build
./tarmy migrate up
./tarmy serve &
./tarmy --remote http://localhost:8080
```

## Make targets

```
make build         # ./tarmy
make test          # go test -race ./...
make test-cover    # coverage.out + coverage.html
make lint          # gofmt + go vet
make migrate-up    # apply migrations against local DATABASE_URL
make tidy          # go mod tidy
```

## Architecture

| Package | Responsibility |
|---------|----------------|
| `cmd/tarmy` | Cobra entrypoint and subcommand wiring. |
| `internal/config` | Environment → typed `Config`. |
| `internal/store` | pgx pool, hand-written SQL, embedded migrations. |
| `internal/svc` | Service layer (one file per domain) + the read-model view endpoints. |
| `internal/httpapi` | chi REST router. |
| `internal/web` | Server-rendered signup/login/alliance/admin pages. |
| `internal/scheduler` | Build/fleet queue completion worker. |
| `internal/tui` | The terminal client (presentation only — no game math). |
| `internal/game` | Pure OGame formulas, constants, and the tech tree. |

**Separation of concerns.** All game math lives in `internal/game` and is
exposed to the client only through resolved read-model endpoints
(`GET /planets/{id}/buildings|facilities|research|shipyard|defense`). The
terminal client renders those responses and never computes a cost, build time,
or prerequisite itself.

## Game mechanics are the source of truth

Every value in `internal/game` is cross-referenced to the
[OGame Fandom Wiki](https://ogame.fandom.com/wiki/OGame_Wiki). When a number
feels off, the wiki wins — see `docs/audit-game-2026-07-02.md` for the most
recent verification pass.

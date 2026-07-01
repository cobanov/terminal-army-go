# TUI Redesign — Design Spec

Status: **draft, in design** · Owner: Mert · Last updated: 2026-07-02

This is the agreed design for a from-scratch rewrite of the terminal client.
Two goals drive it:

1. **Hard separation** — the game system and its math live entirely server-side.
   The TUI becomes a pure presentation layer that never imports `internal/game`
   and never computes a cost, a build time, or a prerequisite.
2. **Better, mouse-first UI** — a grouped left menu, a top resource bar, a
   center that switches between clickable tables *and* keeps a live command
   line, and a right rail of live status. Fully responsive.

---

## 1. Separation: the three leaks we are closing

Today the client reaches into `internal/game` in three places:

| Leak | Location | What it does today |
|------|----------|--------------------|
| Cost / build-time math | `repl.go:printBuildingGroup` | calls `game.BuildingLevelCost`, `game.BuildTimeSeconds` client-side |
| Tech tree / prereqs | `repl.go:techTree*`, `missingTechPrereqs` | walks `game.TechPrerequisites` client-side |
| Catalog keys | `catalog.go` | builds lists from `game.Building*` constants |

Root cause: the API returns raw state (`Planet.Buildings` is `map[string]int`),
so the client is forced to re-derive everything renderable.

### The fix: server read-model (view) endpoints

The server exposes fully-resolved, render-ready rows. The TUI just draws them.

```
GET /api/v1/planets/{id}/buildings   -> []BuildingView
GET /api/v1/planets/{id}/facilities  -> []BuildingView   (facility subset)
GET /api/v1/planets/{id}/research    -> ResearchView      (levels + prereqs resolved)
GET /api/v1/planets/{id}/shipyard    -> []UnitView
GET /api/v1/planets/{id}/defense     -> []UnitView
```

Proposed DTOs (added to `internal/svc/types.go`, computed in `internal/svc`
using `internal/game`):

```go
type BuildingView struct {
    Key          string  `json:"key"`
    Label        string  `json:"label"`
    Category     string  `json:"category"`      // resource | facility
    Level        int     `json:"level"`
    NextCost     Cost    `json:"next_cost"`      // metal/crystal/deut/energy
    BuildSeconds int     `json:"build_seconds"`
    Affordable   bool    `json:"affordable"`
    Locked       bool    `json:"locked"`
    LockedReason string  `json:"locked_reason,omitempty"` // e.g. "Robotics L2"
}

type UnitView struct {
    Key         string `json:"key"`
    Label       string `json:"label"`
    Owned       int    `json:"owned"`
    UnitCost    Cost   `json:"unit_cost"`
    BuildableNow int   `json:"buildable_now"`   // how many current resources allow
    Locked      bool   `json:"locked"`
    LockedReason string `json:"locked_reason,omitempty"`
}

type ResearchView struct {
    Nodes []ResearchNode `json:"nodes"`          // flat list with parent links
}
type ResearchNode struct {
    Key, Label   string  `json:"..."`
    Level        int     `json:"level"`
    NextCost     Cost    `json:"next_cost"`
    BuildSeconds int     `json:"build_seconds"`
    Parent       string  `json:"parent,omitempty"`
    Locked       bool    `json:"locked"`
    LockedReason string  `json:"locked_reason,omitempty"`
}

type Cost struct {
    Metal, Crystal, Deuterium float64 `json:"..."`
    Energy int                        `json:"energy,omitempty"`
}
```

After this, the new TUI package imports only `internal/tui/client` and the
`internal/svc` DTOs. Zero `internal/game`.

**Game-mechanics audit (parallel track):** while adding the read model, we
re-verify every formula/coefficient/prereq against the OGame Fandom Wiki and
fix drift. `docs/COVERAGE.md` is the running ledger.

---

## 2. Layout

### Wide (>= ~120 cols) — 3 columns

```
┌ tarmy ───────────────────────────────────────────────────────────────────────────────┐
│ Genesis · 1:42:8   M 1.24M  C 812K  D 143K   E +2,140      ● online 3   ⌚15:04:22       │  top bar
├───────────┬──────────────────────────────────────────────────────────┬─────────────────┤
│  EMPIRE   │  BUILDINGS · Resources                                     │ QUEUE           │
│ ▸ Overview│ ┌───────────────────┬────┬────────┬────────┬─────────────┐│ ⏱ Metal Mine→12 │
│   Buildings│ │ Building          │ Lvl│  Metal │Crystal │   Time      ││   02:14         │
│   Facilities│ │ Metal Mine       │ 11 │  24.5K │   6.1K │  00:32:10   ││ ⏱ Cruiser ×3    │
│   Research │ │ Solar Plant     ✓ │ 10 │  32.0K │  12.8K │  [ Build ]  ││   01:02         │
│   Shipyard │ │ Deut Synth   🔒L2 │  7 │   —    │   —    │  locked     ││                 │
│   Defense  │ └───────────────────┴────┴────────┴────────┴─────────────┘│ FLEETS          │
│  OPS      │  (row hover = highlight · click = queue upgrade)           │ ▲#12 atk→2:88:4 │
│   Fleet    │                                                            │ ▼#9 return 00:20│
│   Galaxy   │                                                            │                 │
│  SOCIAL   │ ── history ─────────────────────────────────────────────  │ INBOX (2)       │
│   Messages │ > /upgrade metal_mine → queued L12, done 15:06             │ * Combat report │
│   Reports  │ tarmy> _                                                   │ * Msg: Zorg     │
│   Alliance │                                                            │                 │
│   Ranking  │                                                            │                 │
└───────────┴──────────────────────────────────────────────────────────┴─────────────────┘
 F1 Overview  F2 Build  F3 Research  F4 Fleet   /help    (footer is clickable)
```

### Regions

- **Top bar (fixed):** planet name + coords, resources with hourly rates,
  energy balance, online count, clock. Never scrolls.
- **Left menu (grouped):**
  - **Empire** — Overview · Buildings · Facilities · Research · Shipyard · Defense
  - **Ops** — Fleet · Galaxy
  - **Social** — Messages · Reports · Alliance · Ranking
  - Clicking an item switches the center view (identical to typing the command).
- **Center (view + command):**
  - Top ~70%: the selected view's **table** (clickable rows, `[Build]` buttons).
  - Bottom: a thin **history** strip (past commands + results) and an
    **always-active command input** (`tarmy> `). The command line works no
    matter which view is open.
- **Right rail (live):** Queue, Fleets, Inbox. Auto-refreshes. Items are
  clickable and jump to the relevant view.
- **Footer:** contextual hotkeys + `/help`; clickable.

### Responsive breakpoints

| Width | Layout |
|-------|--------|
| >= 120 | 3 columns as above |
| 80–119 | 2 columns: menu + center; right rail collapses to a compact strip under the top bar (Queue/Fleet/Inbox counts, click to expand) |
| < 80 | 1 column: left menu becomes a horizontal tab bar (or `/menu` overlay); right-rail info folds into Overview |

---

## 3. View → command map (click === type)

Every menu item and every clickable action has a command equivalent, so the
keyboard and the mouse are always interchangeable.

| View | Command | Table columns |
|------|---------|---------------|
| Overview | `/planet` | dashboard: resources, production, energy, queue summary, quest hint |
| Buildings | `/resources` | Building · Lvl · Metal · Crystal · Deut · Time · [Build] |
| Facilities | `/facilities` | same shape, facility subset |
| Research | `/research`, `/tree` | Tech · Lvl · Cost · Time · prereq status (tree indent) |
| Shipyard | `/ships` | Ship · Owned · Unit cost · Buildable-now · [Build n] |
| Defense | `/defense` | Unit · Owned · Unit cost · [Build n] |
| Fleet | `/fleet` | #id · mission · state · target · ETA · [Recall] |
| Galaxy | `/galaxy g:s` | pos · planet · owner · alliance · [actions] |
| Messages | `/messages` | read · #id · subject · time |
| Reports | `/reports` | #id · kind · subject · time |
| Alliance | `/alliance` | tag · name · members |
| Ranking | `/leaderboard` | rank · player · score |

---

## 4. Mouse interaction map (full interaction)

- **Left menu item** → switch view.
- **Table row** → select; **row action / `[Build]`** → queue upgrade/build.
- **Scroll wheel** → scroll the focused table or the history log.
- **Right-rail item** → jump to Fleet / Queue / Inbox view.
- **Footer hotkey** → run that command.
- **Hover** → highlight row/button (visual affordance).

Bubble Tea already runs with `WithMouseCellMotion`; we add `tea.MouseMsg`
hit-testing per rendered region (each view reports clickable zones).

---

## 5. Consolidation

Today there are two clients: the cockpit console (`RunConsole`, default) and
the screen navigator (`tarmy play`). Both are replaced by **one** new client.

- `tarmy` (default, TTY) → new unified TUI.
- Non-TTY / piped stdin → keep a minimal headless line REPL for scripts and
  smoke tests (the current `RunREPL` fallback path).
- `tarmy play` → removed (or aliased to the new client during transition).

---

## 6. Proposed package structure (new TUI)

```
internal/tui/
  app.go          root model: layout, focus, routing, responsive breakpoints
  theme.go        palette + lipgloss styles (adaptive light/dark)
  topbar.go       resource/status bar
  menu.go         grouped left menu + mouse hit-testing
  rail.go         right rail (queue/fleets/inbox)
  console.go      history log + command input (always-active)
  command.go      command parser + dispatch -> client calls (no game import)
  views/          one file per view (buildings, research, shipyard, ...)
  client/         unchanged HTTP client (typed API shims)
```

---

## 7. Build sequence

1. **Server read-model** — add the view DTOs + endpoints in `internal/svc` and
   `internal/httpapi`; unit-test against `internal/game`. TUI-agnostic.
2. **Game audit** — re-verify formulas/prereqs vs OGame wiki alongside step 1.
3. **New TUI skeleton** — layout, top bar, grouped menu, responsive frame,
   theme; wire the command line and history.
4. **Views** — port each view to the read-model API (no local math).
5. **Mouse** — hit-testing, clickable rows/menu/rail, scroll.
6. **Cutover** — make the new client the default; drop `tarmy play`; keep the
   headless REPL fallback.

Open items to confirm before coding are tracked at the top of this doc.

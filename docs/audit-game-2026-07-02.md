# Game-mechanics audit — 2026-07-02

`internal/game` audited against the OGame Fandom Wiki. Every building cost,
mine/energy/storage/build-time formula, research cost, ship/defense stat, and
prerequisite tree was cross-checked. Result: the economy math was already
correct; the discrepancies were concentrated in fleet movement and a few
prerequisite/stat values.

## Fixed (this commit)

| # | Sev | Mechanic | File | Fix |
|---|-----|----------|------|-----|
| 1 | HIGH | Flight duration had an extra `/fleetSpeed` and scaled the `+10` constant by throttle, making flights ~instant | `internal/game/fleet.go` | `t = (10 + (3500/spFactor)·√(10·D/V)) / A` per wiki |
| 2 | MED | Fuel speed factor was `(0.5+sp)²` | `internal/game/fleet.go` | `(1+sp)²` per /wiki/Fuel_Consumption |
| 3 | MED | Hyperspace Technology missing Shielding 5 prereq | `internal/game/constants.go` | added Shielding 5 |
| 4 | MED | Hyperspace Drive missing Energy 5 + Shielding 5 prereqs | `internal/game/constants.go` | added both |
| 5 | MED | Bomber fuel was 1000 | `internal/game/constants.go` | 700 per /wiki/Bomber |

Regression tests: `internal/game/audit_fixes_test.go`.

## Verified correct (no change)

Building base costs & factors; mine production (metal/crystal/deut incl. plasma
and average-temperature deuterium); mine energy; solar/fusion/satellite energy;
fusion deut burn; storage capacity; all research costs; build-time and
research-time formulas; ship & defense stats (except Bomber fuel); all
ship/defense prerequisites; drive speed bonuses; distance; temperature and
metal/crystal position bonuses; crawler cap/energy.

## Deferred — need your decision (see Obsidian blockers)

| # | Sev | Item | Why deferred |
|---|-----|------|--------------|
| 6 | MED | `FieldsRangesByPosition` differs from the current redesigned-universe table | Values are universe-config/balance dependent and the "correct" table varies by source; changing them retunes new-planet sizes. Product/balance decision. |
| 7 | LOW | Espionage Probe shield/weapon = 1 (wiki says 0) | Looks intentional to avoid divide-by-zero in the combat resolver. Left as-is; confirm intent. |
| 8 | LOW | Astrophysics → colony-slot cap and colonizable-position widening not implemented | Feature gap, not a formula bug. `planets = 1 + ceil(astro/2)` and position widening at astro 4/6/8 aren't enforced anywhere. Needs a small feature + product sign-off. |
| 9 | LOW | Astrophysics cost not rounded to nearest 100 | Cosmetic; exact rounding rule (half-up?) is fuzzy. |

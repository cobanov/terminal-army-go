# Game mechanics

Every formula below is sourced from the
[OGame Fandom Wiki](https://ogame.fandom.com/wiki/OGame_Wiki) and implemented in
`internal/game`. `L` is the building/research level, `speed` is the universe's
economy/fleet multiplier, and costs grow as `base × factor^(L-1)`.

## Resource production (per hour)

| Resource | Formula |
|----------|---------|
| Metal | `30 · L · 1.1^L · speed · (1 + plasma·0.01) · (1 + posBonus)` |
| Crystal | `20 · L · 1.1^L · speed · (1 + plasma·0.0066) · (1 + posBonus)` |
| Deuterium | `10 · L · 1.1^L · (1.36 − 0.004·T_avg) · speed · (1 + plasma·0.0033)` |

Every planet also receives a small passive trickle (`30·speed` metal,
`15·speed` crystal) even with no mines. `T_avg` is the planet's average
temperature — cooler planets synthesise more deuterium.

**Position bonuses.** Metal mines get +17/23/35/23/17% on slots 6–10; crystal
mines get +40/30/20% on slots 1–3.

## Energy

Mines only run at full output while energy production covers consumption;
otherwise output scales by the production factor.

| Source | Formula |
|--------|---------|
| Solar Plant | `floor(20 · L · 1.1^L)` |
| Fusion Reactor | `floor(30 · L · (1.05 + 0.01·energyTech)^L)` |
| Solar Satellite | `floor((T_avg + 160) / 6)` per satellite |
| Mine consumption | `ceil(coeff · L · 1.1^L)`, coeff 10 (metal/crystal) or 20 (deut) |

Fusion reactors burn `ceil(10 · speed · L · 1.1^L)` deuterium per hour.

## Storage

```
capacity(L) = 5000 · floor(2.5 · e^(20·L/33))
```

with a base capacity of 10,000 at level 0. Production is lost once a pool hits
its cap, so storage upgrades matter.

## Build & research time

```
build_seconds   = (metal + crystal) / (2500 · (1 + robotics) · speed · 2^nanite) · 3600
research_seconds = (metal + crystal) / (1000 · speed · (1 + lab)) · 3600
```

Redesigned universes apply an extra early-level speed-up (`(7 − L) / 2`) through
level 5, so the opening buildings finish quickly. `lab` is the highest Research
Lab across all your planets.

## Fleet movement

**Distance** between coordinates:

| Case | Distance |
|------|----------|
| Different galaxies | `20000 · |Δgalaxy|` |
| Same galaxy, different systems | `2700 + 95 · |Δsystem|` |
| Same system, different positions | `1000 + 5 · |Δposition|` |
| Same coordinates | `5` |

**Flight duration** (V = slowest ship speed, A = universe fleet speed,
`spFactor` = throttle ÷ 100):

```
duration = (10 + (3500 / spFactor) · √(10 · distance / V)) / A
```

**Fuel** (per ship, summed): `baseFuel · count · (distance / 35000) · (1 + spFactor)²`.

**Drive bonuses** add to base ship speed: Combustion +10%/level, Impulse
+20%/level, Hyperspace +30%/level.

## Costs & prerequisites

Building and research costs grow geometrically from a per-item base and factor,
and each item gates behind building/research prerequisites (e.g. Fusion Reactor
needs Deuterium Synthesizer 5 + Energy Technology 3). The client resolves all of
this server-side and shows you cost, time, affordability, and lock reasons per
row — you never have to compute anything by hand.

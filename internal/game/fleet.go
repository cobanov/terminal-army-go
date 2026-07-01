package game

import "math"

// Fleet movement formulas and a simplified single-round combat resolver.
// Source: https://ogame.fandom.com/wiki/Fleet
// Source: https://ogame.fandom.com/wiki/Combat

// -------- Distance / duration / fuel ---------------------------------------

// Distance returns the inter-coordinate distance in OGame units.
//   - Different galaxies: 20000 * |gDiff|
//   - Same galaxy, different systems: 2700 + 95 * |sDiff|
//   - Same system, different positions: 1000 + 5 * |pDiff|
//   - Same coordinates: 5
func Distance(gFrom, sFrom, pFrom, gTo, sTo, pTo int) int {
	if gFrom != gTo {
		return 20000 * absInt(gFrom-gTo)
	}
	if sFrom != sTo {
		return 2700 + 95*absInt(sFrom-sTo)
	}
	if pFrom != pTo {
		return 1000 + 5*absInt(pFrom-pTo)
	}
	return 5
}

// ShipSpeed returns the speed of one hull adjusted for its drive tech.
// Combustion = +10% per level, Impulse = +20%, Hyperspace = +30%.
// Source: https://ogame.fandom.com/wiki/Combustion_Drive
func ShipSpeed(ship ShipType, techLevels map[TechType]int) int {
	stat, ok := ShipStats[ship]
	if !ok {
		return 0
	}
	drive, ok := ShipDrive[ship]
	if !ok {
		return stat.Speed
	}
	level := techLevels[drive]
	multiplier := 0.10
	switch drive {
	case TechImpulseDrive:
		multiplier = 0.20
	case TechHyperspaceDrive:
		multiplier = 0.30
	}
	return int(float64(stat.Speed) * (1 + multiplier*float64(level)))
}

// FlightDurationSeconds applies the OGame travel-time formula:
//
//	t = (10 + (3500 / speedFactor) * sqrt(10 * distance / fleetSpeed)) / universeSpeedFleet
//
// fleetSpeed is the slowest ship in the fleet; speedPercent is the player's
// chosen throttle (10-100, in 10% steps), applied as speedFactor = pct/100 so
// only the square-root term is scaled by the throttle. universeSpeedFleet is
// the universe's fleet-speed multiplier.
// Source: https://ogame.fandom.com/wiki/Distance
func FlightDurationSeconds(distanceUnits, fleetSpeed, universeSpeedFleet, speedPercent int) int {
	if fleetSpeed <= 0 {
		return 1
	}
	if universeSpeedFleet < 1 {
		universeSpeedFleet = 1
	}
	sp := clampInt(speedPercent, 10, 100)
	spFactor := float64(sp) / 100.0
	base := (10.0 + (3500.0/spFactor)*math.Sqrt(10.0*float64(distanceUnits)/float64(fleetSpeed))) /
		float64(universeSpeedFleet)
	secs := int(base)
	if secs < 1 {
		secs = 1
	}
	return secs
}

// FleetFuelConsumption is a simplified per-fleet fuel cost: each ship
// contributes count * baseFuel * (distance / 35000) * (1 + sp)^2 deuterium,
// matching the OGame consumption curve's speed factor without modelling
// per-ship speed.
// Source: https://ogame.fandom.com/wiki/Fuel_Consumption
func FleetFuelConsumption(
	ships map[ShipType]int,
	distanceUnits int,
	speedPercent int,
) int {
	if len(ships) == 0 {
		return 0
	}
	sp := float64(clampInt(speedPercent, 10, 100)) / 100.0
	speedTerm := (1.0 + sp) * (1.0 + sp)
	total := 0.0
	for st, count := range ships {
		if count <= 0 {
			continue
		}
		stat, ok := ShipStats[st]
		if !ok {
			continue
		}
		total += float64(count) * float64(stat.Fuel) * (float64(distanceUnits) / 35000.0) * speedTerm
	}
	fuel := int(total)
	if fuel < 1 {
		fuel = 1
	}
	return fuel
}

// FleetCargoCapacity sums the cargo holds of every ship in the fleet.
func FleetCargoCapacity(ships map[ShipType]int) int {
	total := 0
	for st, count := range ships {
		if count <= 0 {
			continue
		}
		stat, ok := ShipStats[st]
		if !ok {
			continue
		}
		total += stat.Cargo * count
	}
	return total
}

// SlowestShipSpeed returns the minimum drive-adjusted speed among non-empty
// hulls. An empty fleet falls back to 5000 (small-cargo class) so the time
// formula stays well-defined.
func SlowestShipSpeed(ships map[ShipType]int, techLevels map[TechType]int) int {
	min := -1
	for st, count := range ships {
		if count <= 0 {
			continue
		}
		s := ShipSpeed(st, techLevels)
		if min < 0 || s < min {
			min = s
		}
	}
	if min < 0 {
		return 5000
	}
	return min
}

// -------- Espionage --------------------------------------------------------

// EspionageInfoLevel grades a spy report from 1 (resources only) to 5 (full
// research info). Simplified: floor(probes/2) + max(0, atkTech-defTech) + 1,
// clamped to [1, 5].
func EspionageInfoLevel(attackerProbes, attackerTech, defenderTech int) int {
	delta := attackerTech - defenderTech
	if delta < 0 {
		delta = 0
	}
	level := attackerProbes/2 + delta + 1
	if level < 1 {
		level = 1
	}
	if level > 5 {
		level = 5
	}
	return level
}

// CounterEspionageChance is the probability (0..1) the defender destroys the
// probes. Zero when the attacker out-techs the defender.
func CounterEspionageChance(attackerProbes, defenderTech, attackerTech int) float64 {
	if attackerTech > defenderTech {
		return 0.0
	}
	base := float64(attackerProbes) / 5.0
	diff := defenderTech - attackerTech
	if diff < 0 {
		diff = 0
	}
	chance := base * (0.05 + 0.05*float64(diff))
	if chance > 1.0 {
		chance = 1.0
	}
	if chance < 0 {
		chance = 0
	}
	return chance
}

// -------- Simplified combat -----------------------------------------------

// CombatUnit is a single hull or defense type pooled by name with the combat
// stats already adjusted for the owning player's tech levels.
type CombatUnit struct {
	Name   string
	Count  int
	Weapon int
	Shield int
	Armor  int
}

// applyTechMods bumps weapon, shield, and armor by 10% per matching tech
// level (OGame standard).
func applyTechMods(weapon, shield, armor, weaponsTech, shieldingTech, armourTech int) (int, int, int) {
	return int(float64(weapon) * (1 + 0.10*float64(weaponsTech))),
		int(float64(shield) * (1 + 0.10*float64(shieldingTech))),
		int(float64(armor) * (1 + 0.10*float64(armourTech)))
}

// BuildUnitsFromShips materialises CombatUnits for every non-empty hull,
// applying weapons/shielding/armour tech bonuses.
func BuildUnitsFromShips(
	ships map[ShipType]int,
	weapons, shielding, armour int,
) []CombatUnit {
	units := make([]CombatUnit, 0, len(ships))
	for st, count := range ships {
		if count <= 0 {
			continue
		}
		stat, ok := ShipStats[st]
		if !ok {
			continue
		}
		w, s, a := applyTechMods(stat.Weapon, stat.Shield, stat.Armor, weapons, shielding, armour)
		units = append(units, CombatUnit{Name: string(st), Count: count, Weapon: w, Shield: s, Armor: a})
	}
	return units
}

// BuildUnitsFromDefenses mirrors BuildUnitsFromShips for ground defenses.
func BuildUnitsFromDefenses(
	defenses map[DefenseType]int,
	weapons, shielding, armour int,
) []CombatUnit {
	units := make([]CombatUnit, 0, len(defenses))
	for dt, count := range defenses {
		if count <= 0 {
			continue
		}
		stat, ok := DefenseStats[dt]
		if !ok {
			continue
		}
		w, s, a := applyTechMods(stat.Weapon, stat.Shield, stat.Armor, weapons, shielding, armour)
		units = append(units, CombatUnit{Name: string(dt), Count: count, Weapon: w, Shield: s, Armor: a})
	}
	return units
}

// CombatResult is the aggregated outcome of a single combat resolution.
type CombatResult struct {
	AttackerRemaining         map[string]int
	DefenderShipsRemaining    map[string]int
	DefenderDefensesRemaining map[string]int
	AttackerDestroyed         map[string]int
	DefenderShipsDestroyed    map[string]int
	DefenderDefensesDestroyed map[string]int
	AttackerTotalAttack       int
	DefenderTotalAttack       int
	Winner                    string
	DebrisMetal               int
	DebrisCrystal             int
}

// SimulateCombat applies one round of damage exchange. Each side pools its
// weapon output and distributes the total proportionally across the opposing
// HP pool (shield + armor per unit). Ships produce 30% debris on death;
// defenses produce none. This is intentionally simpler than OGame's full
// rapid-fire chart but is enough for an MVP report.
func SimulateCombat(
	attackerUnits, defenderShipUnits, defenderDefenseUnits []CombatUnit,
) CombatResult {
	res := CombatResult{
		AttackerRemaining:         map[string]int{},
		DefenderShipsRemaining:    map[string]int{},
		DefenderDefensesRemaining: map[string]int{},
		AttackerDestroyed:         map[string]int{},
		DefenderShipsDestroyed:    map[string]int{},
		DefenderDefensesDestroyed: map[string]int{},
		Winner:                    "draw",
	}

	atkAttack := totalAttack(attackerUnits)
	defAttack := totalAttack(defenderShipUnits) + totalAttack(defenderDefenseUnits)
	res.AttackerTotalAttack = atkAttack
	res.DefenderTotalAttack = defAttack

	defHP := totalHPPool(defenderShipUnits) + totalHPPool(defenderDefenseUnits)
	atkHP := totalHPPool(attackerUnits)

	// Combine defender pools so the attacker's damage hits ships and defenses
	// proportionally to their share of total HP.
	defenderPool := make([]CombatUnit, 0, len(defenderShipUnits)+len(defenderDefenseUnits))
	defenderPool = append(defenderPool, defenderShipUnits...)
	defenderPool = append(defenderPool, defenderDefenseUnits...)

	defDestroyed := applyDamage(defenderPool, atkAttack, defHP)
	atkDestroyed := applyDamage(attackerUnits, defAttack, atkHP)

	// Mutating applyDamage already decremented Counts; re-read them here.
	// defenderPool aliased the same backing slices via append, so the same
	// CombatUnit values in defenderShipUnits / defenderDefenseUnits were
	// updated in place.
	for _, u := range attackerUnits {
		res.AttackerRemaining[u.Name] = u.Count
	}
	for _, u := range defenderShipUnits {
		res.DefenderShipsRemaining[u.Name] = u.Count
	}
	for _, u := range defenderDefenseUnits {
		res.DefenderDefensesRemaining[u.Name] = u.Count
	}
	for n, v := range atkDestroyed {
		if v > 0 {
			res.AttackerDestroyed[n] = v
		}
	}

	shipNames := map[string]struct{}{}
	for _, u := range defenderShipUnits {
		shipNames[u.Name] = struct{}{}
	}
	defenseNames := map[string]struct{}{}
	for _, u := range defenderDefenseUnits {
		defenseNames[u.Name] = struct{}{}
	}
	for n, v := range defDestroyed {
		if v <= 0 {
			continue
		}
		if _, ok := shipNames[n]; ok {
			res.DefenderShipsDestroyed[n] = v
			continue
		}
		if _, ok := defenseNames[n]; ok {
			res.DefenderDefensesDestroyed[n] = v
		}
	}

	atkLeft := 0
	for _, u := range attackerUnits {
		atkLeft += u.Count
	}
	defLeft := 0
	for _, u := range defenderShipUnits {
		defLeft += u.Count
	}
	for _, u := range defenderDefenseUnits {
		defLeft += u.Count
	}
	switch {
	case atkLeft > 0 && defLeft == 0:
		res.Winner = "attacker"
	case defLeft > 0 && atkLeft == 0:
		res.Winner = "defender"
	default:
		res.Winner = "draw"
	}

	// Debris: 30% of destroyed ship cost (metal + crystal). Defenses leave none.
	addShipDebris(&res, atkDestroyed)
	addShipDebris(&res, defDestroyed)

	return res
}

func totalAttack(units []CombatUnit) int {
	sum := 0
	for _, u := range units {
		if u.Count > 0 {
			sum += u.Count * u.Weapon
		}
	}
	return sum
}

func totalHPPool(units []CombatUnit) int {
	sum := 0
	for _, u := range units {
		if u.Count > 0 {
			sum += u.Count * (u.Shield + u.Armor)
		}
	}
	return sum
}

// applyDamage distributes totalDamage across units, weighted by each type's
// share of totalHP. Returns destroyed counts keyed by unit name and mutates
// the underlying CombatUnit slice in place.
func applyDamage(units []CombatUnit, totalDamage, totalHP int) map[string]int {
	destroyed := map[string]int{}
	if totalHP <= 0 {
		return destroyed
	}
	for i := range units {
		u := &units[i]
		if u.Count <= 0 {
			continue
		}
		perUnitHP := u.Shield + u.Armor
		if perUnitHP < 1 {
			perUnitHP = 1
		}
		share := float64(u.Count*perUnitHP) / float64(totalHP)
		dmgToPool := int(float64(totalDamage) * share)
		killed := dmgToPool / perUnitHP
		if killed > u.Count {
			killed = u.Count
		}
		destroyed[u.Name] = killed
		u.Count -= killed
	}
	return destroyed
}

func addShipDebris(res *CombatResult, destroyed map[string]int) {
	for name, killed := range destroyed {
		if killed <= 0 {
			continue
		}
		st := ShipType(name)
		stat, ok := ShipStats[st]
		if !ok {
			continue
		}
		res.DebrisMetal += int(0.3 * float64(stat.Metal) * float64(killed))
		res.DebrisCrystal += int(0.3 * float64(stat.Crystal) * float64(killed))
	}
}

func absInt(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func clampInt(x, lo, hi int) int {
	if x < lo {
		return lo
	}
	if x > hi {
		return hi
	}
	return x
}

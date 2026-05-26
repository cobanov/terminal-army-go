package game

import "math"

// Pure formula functions. DB and framework independent: input -> output, no
// side effects. All formulas are sourced from the OGame Fandom Wiki, see the
// per-function comments for the canonical link.

// -------- Mine production (per hour) -------------------------------------

// MetalMineProduction returns hourly metal output for one mine level.
// Source: https://ogame.fandom.com/wiki/Metal_Mine
func MetalMineProduction(level int, speed, positionBonus float64, plasmaTech int) float64 {
	if level <= 0 {
		return 0
	}
	return 30 * float64(level) * math.Pow(1.1, float64(level)) * speed *
		(1 + float64(plasmaTech)*0.01) * (1 + positionBonus)
}

// CrystalMineProduction returns hourly crystal output for one mine level.
// Source: https://ogame.fandom.com/wiki/Crystal_Mine
func CrystalMineProduction(level int, speed, positionBonus float64, plasmaTech int) float64 {
	if level <= 0 {
		return 0
	}
	return 20 * float64(level) * math.Pow(1.1, float64(level)) * speed *
		(1 + float64(plasmaTech)*0.0066) * (1 + positionBonus)
}

// DeuteriumSynthesizerProduction returns hourly deuterium output. Cooler
// planets produce more.
// Source: https://ogame.fandom.com/wiki/Deuterium_Synthesizer
func DeuteriumSynthesizerProduction(level, tempMax int, speed float64, plasmaTech int) float64 {
	if level <= 0 {
		return 0
	}
	return 10 * float64(level) * math.Pow(1.1, float64(level)) *
		(1.28 - 0.002*float64(tempMax)) * speed * (1 + float64(plasmaTech)*0.0033)
}

// BasePassiveProduction is the small baseline output every planet receives,
// even with no mines built.
func BasePassiveProduction(speed float64) (metal, crystal, deut float64) {
	return 30.0 * speed, 15.0 * speed, 0.0
}

// -------- Energy -----------------------------------------------------------

// SolarPlantOutput returns the floor of 20 * L * 1.1^L.
// Source: https://ogame.fandom.com/wiki/Solar_Plant
func SolarPlantOutput(level int) int {
	if level <= 0 {
		return 0
	}
	return int(math.Floor(20 * float64(level) * math.Pow(1.1, float64(level))))
}

// SolarSatelliteOutput returns total satellite energy. Per-unit output is
// capped at SolarSatelliteMax (65).
// Source: https://ogame.fandom.com/wiki/Solar_Satellite
func SolarSatelliteOutput(count, avgTemp int) int {
	if count <= 0 {
		return 0
	}
	perUnit := int(math.Floor(float64(avgTemp+160) / 6.0))
	if perUnit > SolarSatelliteMax {
		perUnit = SolarSatelliteMax
	}
	if perUnit < 0 {
		perUnit = 0
	}
	return perUnit * count
}

// FusionReactorOutput returns hourly energy from a fusion reactor.
// Source: https://ogame.fandom.com/wiki/Fusion_Reactor
func FusionReactorOutput(level, energyTech int) int {
	if level <= 0 {
		return 0
	}
	return int(math.Floor(30 * float64(level) * math.Pow(1.05+float64(energyTech)*0.01, float64(level))))
}

// FusionDeutConsumption is the hourly deuterium burn of a fusion reactor.
func FusionDeutConsumption(level int) int {
	if level <= 0 {
		return 0
	}
	return int(math.Floor(10 * float64(level) * math.Pow(1.1, float64(level))))
}

// MineEnergyConsumption is the energy each mine consumes per hour:
// energy = floor(coeff * level * 1.1^level), with coeff from MineEnergyCoeff.
// Source: https://ogame.fandom.com/wiki/Metal_Mine
func MineEnergyConsumption(b BuildingType, level int) int {
	if level <= 0 {
		return 0
	}
	coeff, ok := MineEnergyCoeff[b]
	if !ok {
		return 0
	}
	return int(math.Floor(float64(coeff) * float64(level) * math.Pow(1.1, float64(level))))
}

// -------- Build / research costs ------------------------------------------

// BuildingLevelCost returns the cost to advance a building to targetLevel.
// cost(L+1) = base * factor^L, so the caller passes target = L + 1.
func BuildingLevelCost(b BuildingType, targetLevel int) (metal, crystal, deut int) {
	if targetLevel <= 0 {
		return 0, 0, 0
	}
	cost, ok := BuildingCosts[b]
	if !ok {
		return 0, 0, 0
	}
	exp := float64(targetLevel - 1)
	metal = int(math.Floor(float64(cost.Metal) * math.Pow(cost.Factor, exp)))
	crystal = int(math.Floor(float64(cost.Crystal) * math.Pow(cost.Factor, exp)))
	deut = int(math.Floor(float64(cost.Deuterium) * math.Pow(cost.Factor, exp)))
	return metal, crystal, deut
}

// ResearchLevelCost mirrors BuildingLevelCost for tech.
func ResearchLevelCost(t TechType, targetLevel int) (metal, crystal, deut int) {
	if targetLevel <= 0 {
		return 0, 0, 0
	}
	cost, ok := ResearchCosts[t]
	if !ok {
		return 0, 0, 0
	}
	exp := float64(targetLevel - 1)
	metal = int(math.Floor(float64(cost.Metal) * math.Pow(cost.Factor, exp)))
	crystal = int(math.Floor(float64(cost.Crystal) * math.Pow(cost.Factor, exp)))
	deut = int(math.Floor(float64(cost.Deuterium) * math.Pow(cost.Factor, exp)))
	return metal, crystal, deut
}

// ShipUnitCost is the per-unit construction cost for a ship.
func ShipUnitCost(s ShipType) (metal, crystal, deut int) {
	stat, ok := ShipStats[s]
	if !ok {
		return 0, 0, 0
	}
	return stat.Metal, stat.Crystal, stat.Deuterium
}

// DefenseUnitCost is the per-unit construction cost for a defense.
func DefenseUnitCost(d DefenseType) (metal, crystal, deut int) {
	stat, ok := DefenseStats[d]
	if !ok {
		return 0, 0, 0
	}
	return stat.Metal, stat.Crystal, stat.Deuterium
}

// -------- Build / research durations --------------------------------------

// BuildTimeSeconds is the in-game build duration in seconds.
// Source: https://ogame.fandom.com/wiki/Formulas
func BuildTimeSeconds(metal, crystal, roboticsLevel, naniteLevel int, speed float64) int {
	hours := float64(metal+crystal) /
		(2500.0 * (1.0 + float64(roboticsLevel)) * speed * math.Pow(2, float64(naniteLevel)))
	secs := int(hours * 3600)
	if secs < 1 {
		secs = 1
	}
	return secs
}

// ResearchTimeSeconds returns the in-game research duration in seconds. labLevel
// is the highest research lab across the user's planets.
func ResearchTimeSeconds(metal, crystal, labLevel int, speed float64) int {
	hours := float64(metal+crystal) / (1000.0 * speed * (1.0 + float64(labLevel)))
	secs := int(hours * 3600)
	if secs < 1 {
		secs = 1
	}
	return secs
}

// -------- Storage capacity ------------------------------------------------

// baseStorageCapacity is the level-0 cap on every planet (5,000 units).
const baseStorageCapacity = 10000

// StorageCapacity returns the maximum units a single resource silo can hold
// at the given level.
// Source: https://ogame.fandom.com/wiki/Metal_Storage
// Capacity = 5000 * floor(2.5 * e^(20 * level / 33))
func StorageCapacity(level int) int {
	if level <= 0 {
		return baseStorageCapacity
	}
	return int(math.Floor(5000 * math.Floor(2.5*math.Exp(20.0*float64(level)/33.0))))
}

package game

import (
	"fmt"
	"math/rand"
)

// PlanetAttributes is the per-planet random profile decided at colonisation
// time: temperature window, total fields, and the position-based bonuses that
// feed the production formulas.
type PlanetAttributes struct {
	Position             int
	TempMin              int
	TempMax              int
	FieldsTotal          int
	MetalPositionBonus   float64
	CrystalPositionBonus float64
}

// MaxPlanets returns the maximum number of planets (including the homeworld) a
// player may own at the given Astrophysics level: 1 + ceil(level/2). So level 0
// allows 1 planet, level 1 allows 2, then +1 at every odd level (3, 5, 7, ...).
// Source: https://ogame.fandom.com/wiki/Astrophysics
func MaxPlanets(astrophysicsLevel int) int {
	if astrophysicsLevel < 0 {
		astrophysicsLevel = 0
	}
	return 1 + (astrophysicsLevel+1)/2 // (level+1)/2 == ceil(level/2) for ints
}

// ColonizablePositionRange returns the inclusive slot-position band a player can
// colonize at the given Astrophysics level. The band widens with the tech:
// levels 1-3 → 4-12, 4-5 → 3-13, 6-7 → 2-14, 8+ → any (1-15).
// Source: https://ogame.fandom.com/wiki/Astrophysics
func ColonizablePositionRange(astrophysicsLevel int) (low, high int) {
	switch {
	case astrophysicsLevel >= 8:
		return 1, 15
	case astrophysicsLevel >= 6:
		return 2, 14
	case astrophysicsLevel >= 4:
		return 3, 13
	default:
		return 4, 12
	}
}

// CanColonizePosition reports whether the given slot position (1-15) is
// colonizable at the given Astrophysics level.
func CanColonizePosition(astrophysicsLevel, position int) bool {
	low, high := ColonizablePositionRange(astrophysicsLevel)
	return position >= low && position <= high
}

// GeneratePlanetAttributes rolls a new planet's attributes for the given slot
// position (1-15). A non-nil rng makes the result deterministic; pass nil to
// use a fresh source seeded by the global PRNG.
func GeneratePlanetAttributes(position int, rng *rand.Rand) (PlanetAttributes, error) {
	if position < 1 || position > 15 {
		return PlanetAttributes{}, fmt.Errorf("invalid position: %d", position)
	}
	if rng == nil {
		rng = rand.New(rand.NewSource(rand.Int63()))
	}

	tr := TemperatureRangesByPosition[position]
	tempMax := rng.Intn(tr.High-tr.Low+1) + tr.Low
	tempMin := tempMax - 40

	fr := FieldsRangesByPosition[position]
	fieldsTotal := rng.Intn(fr.High-fr.Low+1) + fr.Low

	return PlanetAttributes{
		Position:             position,
		TempMin:              tempMin,
		TempMax:              tempMax,
		FieldsTotal:          fieldsTotal,
		MetalPositionBonus:   MetalBonusByPosition[position],
		CrystalPositionBonus: CrystalBonusByPosition[position],
	}, nil
}

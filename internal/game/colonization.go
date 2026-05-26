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

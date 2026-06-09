package svc

import (
	"testing"

	"github.com/cobanov/terminal-army-go/internal/game"
	"github.com/cobanov/terminal-army-go/internal/store"
)

func TestApplyFirstSolarRescueCostAllowsEnergyDeficitEscape(t *testing.T) {
	planet := &store.Planet{
		Metal:      61.5,
		Crystal:    404,
		Deuterium:  0,
		Position:   12,
		TempMin:    -15,
		TempMax:    25,
		UniverseID: 1,
	}
	buildings := map[string]int{
		string(game.BuildingMetalMine): 4,
	}
	got := applyFirstSolarRescueCost(
		game.BuildingSolarPlant,
		1,
		planet,
		buildings,
		nil,
		75, 30, 0,
		1,
	)
	if got != 61 {
		t.Fatalf("applyFirstSolarRescueCost = %d, want 61", got)
	}
}

func TestApplyFirstSolarRescueCostRequiresEnergyDeficit(t *testing.T) {
	planet := &store.Planet{
		Metal:      61.5,
		Crystal:    404,
		Position:   12,
		TempMin:    -15,
		TempMax:    25,
		UniverseID: 1,
	}
	got := applyFirstSolarRescueCost(
		game.BuildingSolarPlant,
		1,
		planet,
		nil,
		nil,
		75, 30, 0,
		1,
	)
	if got != 75 {
		t.Fatalf("applyFirstSolarRescueCost without mine consumption = %d, want 75", got)
	}
}

func TestApplyFirstSolarRescueCostDoesNotDiscountOtherBuilds(t *testing.T) {
	planet := &store.Planet{Metal: 61.5, Crystal: 404}
	buildings := map[string]int{string(game.BuildingMetalMine): 4}
	got := applyFirstSolarRescueCost(
		game.BuildingMetalMine,
		5,
		planet,
		buildings,
		nil,
		303, 75, 0,
		1,
	)
	if got != 303 {
		t.Fatalf("applyFirstSolarRescueCost for metal mine = %d, want 303", got)
	}
}

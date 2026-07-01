package game

import "testing"

// Regression tests for the 2026-07-02 OGame-wiki audit fixes. See
// docs/COVERAGE.md for the finding list.

func TestFlightDurationMatchesWiki(t *testing.T) {
	// Light Fighter (speed 12500), one system apart (distance 2795), 1x
	// universe, 100% throttle. Wiki formula:
	//   (10 + 3500*sqrt(10*2795/12500)) / 1 = 5243s
	got := FlightDurationSeconds(2795, 12500, 1, 100)
	if got != 5243 {
		t.Fatalf("FlightDurationSeconds(2795,12500,1,100) = %d, want 5243", got)
	}
	// Half throttle scales only the sqrt term, so the flight takes longer.
	half := FlightDurationSeconds(2795, 12500, 1, 50)
	if half <= got {
		t.Fatalf("50%% throttle (%d) should be slower than 100%% (%d)", half, got)
	}
	// A faster universe divides the whole duration.
	fast := FlightDurationSeconds(2795, 12500, 5, 100)
	if fast >= got {
		t.Fatalf("5x universe (%d) should be faster than 1x (%d)", fast, got)
	}
}

func TestFleetFuelSpeedFactor(t *testing.T) {
	// Light Fighter base fuel 20, distance 2795, 100% -> (1+1)^2 = 4x.
	// 20 * (2795/35000) * 4 = 6.38 -> 6
	got := FleetFuelConsumption(map[ShipType]int{ShipLightFighter: 1}, 2795, 100)
	if got != 6 {
		t.Fatalf("FleetFuelConsumption = %d, want 6", got)
	}
}

func TestHyperspacePrereqsIncludeShielding(t *testing.T) {
	if !prereqHasTech(TechPrerequisites[TechHyperspace], TechShielding, 5) {
		t.Errorf("Hyperspace Technology must require Shielding 5")
	}
	for _, want := range []TechPrereqEntry{{Tech: TechEnergy, Level: 5}, {Tech: TechShielding, Level: 5}} {
		if !prereqHasTech(TechPrerequisites[TechHyperspaceDrive], want.Tech, want.Level) {
			t.Errorf("Hyperspace Drive must require %s %d", want.Tech, want.Level)
		}
	}
}

func TestBomberFuelIsWikiValue(t *testing.T) {
	if got := ShipStats[ShipBomber].Fuel; got != 700 {
		t.Errorf("Bomber fuel = %d, want 700", got)
	}
}

func prereqHasTech(reqs []TechPrereqEntry, tech TechType, level int) bool {
	for _, r := range reqs {
		if r.Tech == tech && r.Level >= level {
			return true
		}
	}
	return false
}

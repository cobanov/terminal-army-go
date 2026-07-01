package game

import "testing"

// The read-model iterates the ordered display slices (ResearchTechs, Ships,
// Defenses) and the ResourceBuildings/FacilityBuildings tabs. If a new item is
// added to the stat/cost maps but not to its display slice, it would silently
// vanish from the UI. These tests guard that invariant.

func TestResearchTechsCoverAllCosts(t *testing.T) {
	seen := make(map[TechType]bool, len(ResearchTechs))
	for _, tt := range ResearchTechs {
		if seen[tt] {
			t.Errorf("ResearchTechs lists %q twice", tt)
		}
		seen[tt] = true
		if _, ok := ResearchCosts[tt]; !ok {
			t.Errorf("ResearchTechs lists %q which has no ResearchCosts row", tt)
		}
	}
	for tt := range ResearchCosts {
		if !seen[tt] {
			t.Errorf("ResearchCosts has %q but ResearchTechs does not list it", tt)
		}
	}
}

func TestShipsCoverAllStats(t *testing.T) {
	seen := make(map[ShipType]bool, len(Ships))
	for _, s := range Ships {
		seen[s] = true
		if _, ok := ShipStats[s]; !ok {
			t.Errorf("Ships lists %q which has no ShipStats row", s)
		}
	}
	for s := range ShipStats {
		if !seen[s] {
			t.Errorf("ShipStats has %q but Ships does not list it", s)
		}
	}
}

func TestDefensesCoverAllStats(t *testing.T) {
	seen := make(map[DefenseType]bool, len(Defenses))
	for _, d := range Defenses {
		seen[d] = true
		if _, ok := DefenseStats[d]; !ok {
			t.Errorf("Defenses lists %q which has no DefenseStats row", d)
		}
	}
	for d := range DefenseStats {
		if !seen[d] {
			t.Errorf("DefenseStats has %q but Defenses does not list it", d)
		}
	}
}

func TestBuildingTabsCoverAllCosts(t *testing.T) {
	seen := map[BuildingType]bool{}
	for _, b := range ResourceBuildings {
		seen[b] = true
	}
	for _, b := range FacilityBuildings {
		if seen[b] {
			t.Errorf("%q appears in both ResourceBuildings and FacilityBuildings", b)
		}
		seen[b] = true
	}
	for b := range BuildingCosts {
		if !seen[b] {
			t.Errorf("BuildingCosts has %q but no Resource/Facility tab lists it", b)
		}
	}
}

func TestTechTreeParentsAreValid(t *testing.T) {
	for child, parent := range TechTreeParent {
		if _, ok := ResearchCosts[child]; !ok {
			t.Errorf("TechTreeParent child %q is not a known tech", child)
		}
		if _, ok := ResearchCosts[parent]; !ok {
			t.Errorf("TechTreeParent parent %q is not a known tech", parent)
		}
		if child == parent {
			t.Errorf("tech %q is its own parent", child)
		}
	}
}

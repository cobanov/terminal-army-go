package game

import "fmt"

// CheckBuildingPrereqs evaluates a building target against current building
// and tech levels. The bool is true when nothing is missing.
func CheckBuildingPrereqs(
	building BuildingType,
	buildingLevels map[BuildingType]int,
	techLevels map[TechType]int,
) (ok bool, missing []string) {
	reqs := BuildingPrerequisites[building]
	for _, req := range reqs {
		if req.Building != "" {
			if buildingLevels[req.Building] < req.Level {
				missing = append(missing, fmt.Sprintf("%s level %d required", req.Building, req.Level))
			}
			continue
		}
		if req.Tech != "" {
			if techLevels[req.Tech] < req.Level {
				missing = append(missing, fmt.Sprintf("%s level %d required", req.Tech, req.Level))
			}
		}
	}
	return len(missing) == 0, missing
}

// CheckResearchPrereqs evaluates a research tech against the given lab
// ceiling and current tech levels. The bool is true when nothing is missing.
// labLevel is the highest research lab level across the user's planets.
func CheckResearchPrereqs(
	tech TechType,
	labLevel int,
	techLevels map[TechType]int,
) (ok bool, missing []string) {
	reqs := TechPrerequisites[tech]
	for _, req := range reqs {
		if req.LabLevel > 0 {
			if labLevel < req.LabLevel {
				missing = append(missing, fmt.Sprintf("Research Lab level %d required", req.LabLevel))
			}
			continue
		}
		if techLevels[req.Tech] < req.Level {
			missing = append(missing, fmt.Sprintf("%s level %d required", req.Tech, req.Level))
		}
	}
	return len(missing) == 0, missing
}

// CheckShipPrereqs evaluates the prerequisites for one ship hull.
func CheckShipPrereqs(
	ship ShipType,
	shipyardLevel int,
	techLevels map[TechType]int,
) (ok bool, missing []string) {
	reqs := ShipPrerequisites[ship]
	for _, req := range reqs {
		if req.ShipyardLevel > 0 {
			if shipyardLevel < req.ShipyardLevel {
				missing = append(missing, fmt.Sprintf("Shipyard level %d required", req.ShipyardLevel))
			}
			continue
		}
		if techLevels[req.Tech] < req.Level {
			missing = append(missing, fmt.Sprintf("%s level %d required", req.Tech, req.Level))
		}
	}
	return len(missing) == 0, missing
}

// CheckDefensePrereqs evaluates the prerequisites for one defense type.
func CheckDefensePrereqs(
	def DefenseType,
	shipyardLevel int,
	techLevels map[TechType]int,
) (ok bool, missing []string) {
	reqs := DefensePrerequisites[def]
	for _, req := range reqs {
		if req.ShipyardLevel > 0 {
			if shipyardLevel < req.ShipyardLevel {
				missing = append(missing, fmt.Sprintf("Shipyard level %d required", req.ShipyardLevel))
			}
			continue
		}
		if techLevels[req.Tech] < req.Level {
			missing = append(missing, fmt.Sprintf("%s level %d required", req.Tech, req.Level))
		}
	}
	return len(missing) == 0, missing
}

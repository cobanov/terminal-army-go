package svc

import (
	"context"
	"strings"

	"github.com/cobanov/terminal-army-go/internal/game"
	"github.com/cobanov/terminal-army-go/internal/store"
)

// ViewService assembles render-ready read models for the buildings, facilities,
// research, shipyard, and defense screens. It is the boundary that keeps game
// math server-side: it resolves cost, build time, affordability, and
// prerequisite state so the terminal client is pure presentation and never
// imports internal/game.
type ViewService struct{ app *App }

// PlanetBuildings returns the resource-building rows for a planet.
func (s *ViewService) PlanetBuildings(ctx context.Context, userID, planetID int64) ([]BuildingView, error) {
	return s.buildingViews(ctx, userID, planetID, game.ResourceBuildings, "resource")
}

// PlanetFacilities returns the facility-building rows for a planet.
func (s *ViewService) PlanetFacilities(ctx context.Context, userID, planetID int64) ([]BuildingView, error) {
	return s.buildingViews(ctx, userID, planetID, game.FacilityBuildings, "facility")
}

func (s *ViewService) buildingViews(ctx context.Context, userID, planetID int64, keys []game.BuildingType, category string) ([]BuildingView, error) {
	planet, err := s.app.Planet.GetForUser(ctx, userID, planetID)
	if err != nil {
		return nil, err
	}
	researches, err := s.app.Queries.ListResearchesForUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	universe, err := s.app.Queries.GetUniverse(ctx, planet.UniverseID)
	if err != nil {
		return nil, err
	}
	speed := float64(universe.SpeedEconomy)
	bmap := toBuildingMap(planet.Buildings)
	tmap := toTechMap(researches)
	robotics := planet.Buildings[string(game.BuildingRoboticsFactory)]
	nanite := planet.Buildings[string(game.BuildingNaniteFactory)]

	out := make([]BuildingView, 0, len(keys))
	for _, bt := range keys {
		level := planet.Buildings[string(bt)]
		target := level + 1
		m, c, d := game.BuildingLevelCost(bt, target)
		// Marginal energy the upgrade will draw (mines only; 0 otherwise).
		energy := game.MineEnergyConsumption(bt, target) - game.MineEnergyConsumption(bt, level)
		if energy < 0 {
			energy = 0
		}
		ok, missing := game.CheckBuildingPrereqs(bt, bmap, tmap)
		out = append(out, BuildingView{
			Key:          string(bt),
			Label:        game.BuildingLabels[bt],
			Category:     category,
			Level:        level,
			NextCost:     Cost{Metal: float64(m), Crystal: float64(c), Deuterium: float64(d), Energy: energy},
			BuildSeconds: game.BuildingUpgradeTimeSeconds(m, c, robotics, nanite, target, speed),
			Affordable:   int(planet.Metal) >= m && int(planet.Crystal) >= c && int(planet.Deuterium) >= d,
			Locked:       !ok,
			LockedReason: strings.Join(missing, ", "),
		})
	}
	return out, nil
}

// PlanetResearch returns the full research view for a planet (tech levels,
// costs, prerequisites, and tree parents resolved).
func (s *ViewService) PlanetResearch(ctx context.Context, userID, planetID int64) (*ResearchView, error) {
	planet, err := s.app.Planet.GetForUser(ctx, userID, planetID)
	if err != nil {
		return nil, err
	}
	researches, err := s.app.Queries.ListResearchesForUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	universe, err := s.app.Queries.GetUniverse(ctx, planet.UniverseID)
	if err != nil {
		return nil, err
	}
	labLevel, err := s.app.Queries.MaxBuildingLevelAcrossUser(ctx, userID, string(game.BuildingResearchLab))
	if err != nil {
		return nil, err
	}
	tmap := toTechMap(researches)
	speed := float64(universe.SpeedResearch)

	nodes := make([]ResearchNode, 0, len(game.ResearchTechs))
	for _, tt := range game.ResearchTechs {
		level := researches[string(tt)]
		target := level + 1
		m, c, d := game.ResearchLevelCost(tt, target)
		ok, missing := game.CheckResearchPrereqs(tt, labLevel, tmap)
		nodes = append(nodes, ResearchNode{
			Key:          string(tt),
			Label:        game.TechLabels[tt],
			Level:        level,
			NextCost:     Cost{Metal: float64(m), Crystal: float64(c), Deuterium: float64(d)},
			BuildSeconds: game.ResearchTimeSeconds(m, c, labLevel, speed),
			Parent:       string(game.TechTreeParent[tt]),
			Affordable:   int(planet.Metal) >= m && int(planet.Crystal) >= c && int(planet.Deuterium) >= d,
			Locked:       !ok,
			LockedReason: strings.Join(missing, ", "),
		})
	}
	return &ResearchView{LabLevel: labLevel, Nodes: nodes}, nil
}

// PlanetShipyard returns the buildable-ship rows for a planet.
func (s *ViewService) PlanetShipyard(ctx context.Context, userID, planetID int64) ([]UnitView, error) {
	planet, researches, universe, err := s.unitContext(ctx, userID, planetID)
	if err != nil {
		return nil, err
	}
	shipyardLevel := planet.Buildings[string(game.BuildingShipyard)]
	robotics := planet.Buildings[string(game.BuildingRoboticsFactory)]
	nanite := planet.Buildings[string(game.BuildingNaniteFactory)]
	tmap := toTechMap(researches)
	speed := float64(universe.SpeedEconomy)

	out := make([]UnitView, 0, len(game.Ships))
	for _, st := range game.Ships {
		m, c, d := game.ShipUnitCost(st)
		ok, missing := game.CheckShipPrereqs(st, shipyardLevel, tmap)
		out = append(out, UnitView{
			Key:          string(st),
			Label:        game.ShipLabels[st],
			Owned:        planet.Ships[string(st)],
			UnitCost:     Cost{Metal: float64(m), Crystal: float64(c), Deuterium: float64(d)},
			BuildSeconds: game.BuildTimeSeconds(m, c, robotics, nanite, speed),
			BuildableNow: affordableCount(planet.Metal, planet.Crystal, planet.Deuterium, m, c, d),
			Locked:       !ok,
			LockedReason: strings.Join(missing, ", "),
		})
	}
	return out, nil
}

// PlanetDefense returns the buildable-defense rows for a planet.
func (s *ViewService) PlanetDefense(ctx context.Context, userID, planetID int64) ([]UnitView, error) {
	planet, researches, universe, err := s.unitContext(ctx, userID, planetID)
	if err != nil {
		return nil, err
	}
	shipyardLevel := planet.Buildings[string(game.BuildingShipyard)]
	robotics := planet.Buildings[string(game.BuildingRoboticsFactory)]
	nanite := planet.Buildings[string(game.BuildingNaniteFactory)]
	tmap := toTechMap(researches)
	speed := float64(universe.SpeedEconomy)

	out := make([]UnitView, 0, len(game.Defenses))
	for _, dt := range game.Defenses {
		m, c, d := game.DefenseUnitCost(dt)
		ok, missing := game.CheckDefensePrereqs(dt, shipyardLevel, tmap)
		out = append(out, UnitView{
			Key:          string(dt),
			Label:        game.DefenseLabels[dt],
			Owned:        planet.Defense[string(dt)],
			UnitCost:     Cost{Metal: float64(m), Crystal: float64(c), Deuterium: float64(d)},
			BuildSeconds: game.BuildTimeSeconds(m, c, robotics, nanite, speed),
			BuildableNow: affordableCount(planet.Metal, planet.Crystal, planet.Deuterium, m, c, d),
			Locked:       !ok,
			LockedReason: strings.Join(missing, ", "),
		})
	}
	return out, nil
}

// unitContext loads the shared inputs for the ship/defense views.
func (s *ViewService) unitContext(ctx context.Context, userID, planetID int64) (*Planet, map[string]int, *store.Universe, error) {
	planet, err := s.app.Planet.GetForUser(ctx, userID, planetID)
	if err != nil {
		return nil, nil, nil, err
	}
	researches, err := s.app.Queries.ListResearchesForUser(ctx, userID)
	if err != nil {
		return nil, nil, nil, err
	}
	universe, err := s.app.Queries.GetUniverse(ctx, planet.UniverseID)
	if err != nil {
		return nil, nil, nil, err
	}
	return planet, researches, universe, nil
}

// affordableCount returns how many units the given stockpile can afford,
// limited by whichever resource runs out first. Zero-cost resources are
// ignored; a fully free unit returns 0 (there is no such unit in practice).
func affordableCount(metal, crystal, deut float64, m, c, d int) int {
	limit := -1
	consider := func(have float64, cost int) {
		if cost <= 0 {
			return
		}
		n := int(have) / cost
		if limit < 0 || n < limit {
			limit = n
		}
	}
	consider(metal, m)
	consider(crystal, c)
	consider(deut, d)
	if limit < 0 {
		return 0
	}
	return limit
}

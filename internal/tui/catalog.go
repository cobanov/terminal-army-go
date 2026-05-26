// Static catalogs of the keys exposed by the server, paired with display
// labels. The TUI does not need to know costs or prereqs - the server
// rejects illegal queues - but it does need a stable order to render lists.
package tui

import "github.com/cobanov/terminal-army-go/internal/game"

// CatalogItem is a (key, label) pair.
type CatalogItem struct {
	Key   string
	Label string
}

// BuildingCatalog is the order shown in the buildings screen. Mirrors the
// server's known building set so users never queue a key the server rejects.
var BuildingCatalog = []CatalogItem{
	{string(game.BuildingMetalMine), "Metal Mine"},
	{string(game.BuildingCrystalMine), "Crystal Mine"},
	{string(game.BuildingDeuteriumSynthesizer), "Deuterium Synthesizer"},
	{string(game.BuildingSolarPlant), "Solar Plant"},
	{string(game.BuildingFusionReactor), "Fusion Reactor"},
	{string(game.BuildingSolarSatellite), "Solar Satellite"},
	{string(game.BuildingMetalStorage), "Metal Storage"},
	{string(game.BuildingCrystalStorage), "Crystal Storage"},
	{string(game.BuildingDeuteriumTank), "Deuterium Tank"},
	{string(game.BuildingRoboticsFactory), "Robotics Factory"},
	{string(game.BuildingShipyard), "Shipyard"},
	{string(game.BuildingResearchLab), "Research Lab"},
	{string(game.BuildingNaniteFactory), "Nanite Factory"},
	{string(game.BuildingMissileSilo), "Missile Silo"},
	{string(game.BuildingAllianceDepot), "Alliance Depot"},
	{string(game.BuildingTerraformer), "Terraformer"},
}

// ResearchCatalog is the order shown in the research screen.
var ResearchCatalog = []CatalogItem{
	{string(game.TechEnergy), "Energy Technology"},
	{string(game.TechLaser), "Laser Technology"},
	{string(game.TechIon), "Ion Technology"},
	{string(game.TechHyperspace), "Hyperspace Technology"},
	{string(game.TechPlasma), "Plasma Technology"},
	{string(game.TechComputer), "Computer Technology"},
	{string(game.TechAstrophysics), "Astrophysics"},
	{string(game.TechEspionage), "Espionage Technology"},
	{string(game.TechCombustionDrive), "Combustion Drive"},
	{string(game.TechImpulseDrive), "Impulse Drive"},
	{string(game.TechHyperspaceDrive), "Hyperspace Drive"},
	{string(game.TechWeapons), "Weapons Technology"},
	{string(game.TechShielding), "Shielding Technology"},
	{string(game.TechArmour), "Armour Technology"},
}

// ShipCatalog is the order shown in the shipyard screen.
var ShipCatalog = []CatalogItem{
	{string(game.ShipSmallCargo), "Small Cargo"},
	{string(game.ShipLargeCargo), "Large Cargo"},
	{string(game.ShipLightFighter), "Light Fighter"},
	{string(game.ShipHeavyFighter), "Heavy Fighter"},
	{string(game.ShipCruiser), "Cruiser"},
	{string(game.ShipBattleship), "Battleship"},
	{string(game.ShipBattlecruiser), "Battlecruiser"},
	{string(game.ShipBomber), "Bomber"},
	{string(game.ShipDestroyer), "Destroyer"},
	{string(game.ShipRecycler), "Recycler"},
	{string(game.ShipEspionageProbe), "Espionage Probe"},
	{string(game.ShipColonyShip), "Colony Ship"},
}

// DefenseCatalog is the order shown in the defense screen.
var DefenseCatalog = []CatalogItem{
	{string(game.DefenseRocketLauncher), "Rocket Launcher"},
	{string(game.DefenseLightLaser), "Light Laser"},
	{string(game.DefenseHeavyLaser), "Heavy Laser"},
	{string(game.DefenseGaussCannon), "Gauss Cannon"},
	{string(game.DefenseIonCannon), "Ion Cannon"},
	{string(game.DefensePlasmaTurret), "Plasma Turret"},
	{string(game.DefenseSmallShieldDome), "Small Shield Dome"},
	{string(game.DefenseLargeShieldDome), "Large Shield Dome"},
}

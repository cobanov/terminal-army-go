// Static catalogs of the item keys the server accepts, paired with display
// labels. These power command autocomplete and the headless line client. They
// are intentionally plain string literals (not derived from the game-math
// package) so the whole tui package stays presentation-only; the server rejects
// any key it does not recognise, so an accidental drift is caught at the API.
package tui

// CatalogItem is a (key, label) pair.
type CatalogItem struct {
	Key   string
	Label string
}

// BuildingCatalog is the union of resource and facility buildings, in the order
// the server exposes them.
var BuildingCatalog = []CatalogItem{
	{"metal_mine", "Metal Mine"},
	{"crystal_mine", "Crystal Mine"},
	{"deuterium_synthesizer", "Deuterium Synthesizer"},
	{"solar_plant", "Solar Plant"},
	{"fusion_reactor", "Fusion Reactor"},
	{"solar_satellite", "Solar Satellite"},
	{"crawler", "Crawler"},
	{"metal_storage", "Metal Storage"},
	{"crystal_storage", "Crystal Storage"},
	{"deuterium_tank", "Deuterium Tank"},
	{"robotics_factory", "Robotics Factory"},
	{"shipyard", "Shipyard"},
	{"research_lab", "Research Laboratory"},
	{"nanite_factory", "Nanite Factory"},
	{"missile_silo", "Missile Silo"},
	{"alliance_depot", "Alliance Depot"},
	{"terraformer", "Terraformer"},
}

// ResearchCatalog is the order shown in the research screen.
var ResearchCatalog = []CatalogItem{
	{"energy", "Energy Technology"},
	{"laser", "Laser Technology"},
	{"ion", "Ion Technology"},
	{"hyperspace", "Hyperspace Technology"},
	{"plasma", "Plasma Technology"},
	{"computer", "Computer Technology"},
	{"astrophysics", "Astrophysics"},
	{"espionage", "Espionage Technology"},
	{"combustion_drive", "Combustion Drive"},
	{"impulse_drive", "Impulse Drive"},
	{"hyperspace_drive", "Hyperspace Drive"},
	{"weapons", "Weapons Technology"},
	{"shielding", "Shielding Technology"},
	{"armour", "Armour Technology"},
}

// ShipCatalog is the order shown in the shipyard screen.
var ShipCatalog = []CatalogItem{
	{"small_cargo", "Small Cargo"},
	{"large_cargo", "Large Cargo"},
	{"light_fighter", "Light Fighter"},
	{"heavy_fighter", "Heavy Fighter"},
	{"cruiser", "Cruiser"},
	{"battleship", "Battleship"},
	{"battlecruiser", "Battlecruiser"},
	{"bomber", "Bomber"},
	{"destroyer", "Destroyer"},
	{"recycler", "Recycler"},
	{"espionage_probe", "Espionage Probe"},
	{"colony_ship", "Colony Ship"},
}

// DefenseCatalog is the order shown in the defense screen.
var DefenseCatalog = []CatalogItem{
	{"rocket_launcher", "Rocket Launcher"},
	{"light_laser", "Light Laser"},
	{"heavy_laser", "Heavy Laser"},
	{"gauss_cannon", "Gauss Cannon"},
	{"ion_cannon", "Ion Cannon"},
	{"plasma_turret", "Plasma Turret"},
	{"small_shield_dome", "Small Shield Dome"},
	{"large_shield_dome", "Large Shield Dome"},
}

func catalogKeys(rows []CatalogItem) []string {
	keys := make([]string, 0, len(rows))
	for _, row := range rows {
		keys = append(keys, row.Key)
	}
	return keys
}

func allCatalogKeys() []string {
	var keys []string
	keys = append(keys, catalogKeys(BuildingCatalog)...)
	keys = append(keys, catalogKeys(ResearchCatalog)...)
	keys = append(keys, catalogKeys(ShipCatalog)...)
	keys = append(keys, catalogKeys(DefenseCatalog)...)
	return keys
}

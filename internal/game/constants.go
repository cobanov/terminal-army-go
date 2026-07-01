// Package game holds all OGame constants, formulas, and pure simulation logic.
// All values are sourced from the OGame Fandom Wiki:
// https://ogame.fandom.com/wiki/Formulas
package game

// BuildingType identifies one of the structures that can be built on a planet.
type BuildingType string

const (
	BuildingMetalMine            BuildingType = "metal_mine"
	BuildingCrystalMine          BuildingType = "crystal_mine"
	BuildingDeuteriumSynthesizer BuildingType = "deuterium_synthesizer"
	BuildingSolarPlant           BuildingType = "solar_plant"
	BuildingFusionReactor        BuildingType = "fusion_reactor"
	BuildingSolarSatellite       BuildingType = "solar_satellite"
	BuildingCrawler              BuildingType = "crawler"
	BuildingMetalStorage         BuildingType = "metal_storage"
	BuildingCrystalStorage       BuildingType = "crystal_storage"
	BuildingDeuteriumTank        BuildingType = "deuterium_tank"
	BuildingRoboticsFactory      BuildingType = "robotics_factory"
	BuildingShipyard             BuildingType = "shipyard"
	BuildingResearchLab          BuildingType = "research_lab"
	BuildingAllianceDepot        BuildingType = "alliance_depot"
	BuildingMissileSilo          BuildingType = "missile_silo"
	BuildingNaniteFactory        BuildingType = "nanite_factory"
	BuildingTerraformer          BuildingType = "terraformer"
)

// ResourceBuildings is the ordered tab list shown under Resources in the UI.
// Source: https://ogame.fandom.com/wiki/Buildings
var ResourceBuildings = []BuildingType{
	BuildingMetalMine,
	BuildingCrystalMine,
	BuildingDeuteriumSynthesizer,
	BuildingSolarPlant,
	BuildingFusionReactor,
	BuildingSolarSatellite,
	BuildingCrawler,
	BuildingMetalStorage,
	BuildingCrystalStorage,
	BuildingDeuteriumTank,
}

// FacilityBuildings is the ordered tab list shown under Facilities in the UI.
var FacilityBuildings = []BuildingType{
	BuildingRoboticsFactory,
	BuildingShipyard,
	BuildingResearchLab,
	BuildingAllianceDepot,
	BuildingMissileSilo,
	BuildingNaniteFactory,
	BuildingTerraformer,
}

// BuildingLabels maps each building to its display name.
var BuildingLabels = map[BuildingType]string{
	BuildingMetalMine:            "Metal Mine",
	BuildingCrystalMine:          "Crystal Mine",
	BuildingDeuteriumSynthesizer: "Deuterium Synthesizer",
	BuildingSolarPlant:           "Solar Plant",
	BuildingFusionReactor:        "Fusion Reactor",
	BuildingSolarSatellite:       "Solar Satellite",
	BuildingCrawler:              "Crawler",
	BuildingRoboticsFactory:      "Robotics Factory",
	BuildingShipyard:             "Shipyard",
	BuildingResearchLab:          "Research Laboratory",
	BuildingMetalStorage:         "Metal Storage",
	BuildingCrystalStorage:       "Crystal Storage",
	BuildingDeuteriumTank:        "Deuterium Tank",
	BuildingNaniteFactory:        "Nanite Factory",
	BuildingMissileSilo:          "Missile Silo",
	BuildingAllianceDepot:        "Alliance Depot",
	BuildingTerraformer:          "Terraformer",
}

// TechType identifies one of the research technologies.
type TechType string

// ResearchTechs is the canonical display order for the research view. The
// read-model iterates this so clients never hard-code the tech list.
var ResearchTechs = []TechType{
	TechEnergy,
	TechLaser,
	TechIon,
	TechHyperspace,
	TechPlasma,
	TechComputer,
	TechAstrophysics,
	TechEspionage,
	TechCombustionDrive,
	TechImpulseDrive,
	TechHyperspaceDrive,
	TechWeapons,
	TechShielding,
	TechArmour,
}

const (
	TechEnergy          TechType = "energy"
	TechLaser           TechType = "laser"
	TechIon             TechType = "ion"
	TechHyperspace      TechType = "hyperspace"
	TechPlasma          TechType = "plasma"
	TechComputer        TechType = "computer"
	TechAstrophysics    TechType = "astrophysics"
	TechEspionage       TechType = "espionage"
	TechCombustionDrive TechType = "combustion_drive"
	TechImpulseDrive    TechType = "impulse_drive"
	TechHyperspaceDrive TechType = "hyperspace_drive"
	TechWeapons         TechType = "weapons"
	TechShielding       TechType = "shielding"
	TechArmour          TechType = "armour"
)

// TechLabels maps each research tech to its display name.
var TechLabels = map[TechType]string{
	TechEnergy:          "Energy Technology",
	TechLaser:           "Laser Technology",
	TechIon:             "Ion Technology",
	TechHyperspace:      "Hyperspace Technology",
	TechPlasma:          "Plasma Technology",
	TechComputer:        "Computer Technology",
	TechAstrophysics:    "Astrophysics",
	TechEspionage:       "Espionage Technology",
	TechCombustionDrive: "Combustion Drive",
	TechImpulseDrive:    "Impulse Drive",
	TechHyperspaceDrive: "Hyperspace Drive",
	TechWeapons:         "Weapons Technology",
	TechShielding:       "Shielding Technology",
	TechArmour:          "Armour Technology",
}

// ShipType identifies one of the buildable ships.
// Source: https://ogame.fandom.com/wiki/Ships
type ShipType string

const (
	ShipSmallCargo     ShipType = "small_cargo"
	ShipLargeCargo     ShipType = "large_cargo"
	ShipLightFighter   ShipType = "light_fighter"
	ShipHeavyFighter   ShipType = "heavy_fighter"
	ShipCruiser        ShipType = "cruiser"
	ShipBattleship     ShipType = "battleship"
	ShipColonyShip     ShipType = "colony_ship"
	ShipRecycler       ShipType = "recycler"
	ShipEspionageProbe ShipType = "espionage_probe"
	ShipBomber         ShipType = "bomber"
	ShipDestroyer      ShipType = "destroyer"
	ShipBattlecruiser  ShipType = "battlecruiser"
)

// ShipLabels maps each ship to its display name.
var ShipLabels = map[ShipType]string{
	ShipSmallCargo:     "Small Cargo",
	ShipLargeCargo:     "Large Cargo",
	ShipLightFighter:   "Light Fighter",
	ShipHeavyFighter:   "Heavy Fighter",
	ShipCruiser:        "Cruiser",
	ShipBattleship:     "Battleship",
	ShipColonyShip:     "Colony Ship",
	ShipRecycler:       "Recycler",
	ShipEspionageProbe: "Espionage Probe",
	ShipBomber:         "Bomber",
	ShipDestroyer:      "Destroyer",
	ShipBattlecruiser:  "Battlecruiser",
}

// Ships is the canonical display order for the shipyard view.
var Ships = []ShipType{
	ShipSmallCargo,
	ShipLargeCargo,
	ShipLightFighter,
	ShipHeavyFighter,
	ShipCruiser,
	ShipBattleship,
	ShipBattlecruiser,
	ShipBomber,
	ShipDestroyer,
	ShipRecycler,
	ShipEspionageProbe,
	ShipColonyShip,
}

// DefenseType identifies one of the buildable defenses.
// Source: https://ogame.fandom.com/wiki/Defense
type DefenseType string

const (
	DefenseRocketLauncher  DefenseType = "rocket_launcher"
	DefenseLightLaser      DefenseType = "light_laser"
	DefenseHeavyLaser      DefenseType = "heavy_laser"
	DefenseGaussCannon     DefenseType = "gauss_cannon"
	DefenseIonCannon       DefenseType = "ion_cannon"
	DefensePlasmaTurret    DefenseType = "plasma_turret"
	DefenseSmallShieldDome DefenseType = "small_shield_dome"
	DefenseLargeShieldDome DefenseType = "large_shield_dome"
)

// DefenseLabels maps each defense to its display name.
var DefenseLabels = map[DefenseType]string{
	DefenseRocketLauncher:  "Rocket Launcher",
	DefenseLightLaser:      "Light Laser",
	DefenseHeavyLaser:      "Heavy Laser",
	DefenseGaussCannon:     "Gauss Cannon",
	DefenseIonCannon:       "Ion Cannon",
	DefensePlasmaTurret:    "Plasma Turret",
	DefenseSmallShieldDome: "Small Shield Dome",
	DefenseLargeShieldDome: "Large Shield Dome",
}

// Defenses is the canonical display order for the defense view.
var Defenses = []DefenseType{
	DefenseRocketLauncher,
	DefenseLightLaser,
	DefenseHeavyLaser,
	DefenseGaussCannon,
	DefenseIonCannon,
	DefensePlasmaTurret,
	DefenseSmallShieldDome,
	DefenseLargeShieldDome,
}

// TempRange is the inclusive (low, high) range a planet's T_max may fall into
// for a given slot position. T_min = T_max - 40.
// Source: https://ogame.fandom.com/wiki/Temperature
type TempRange struct {
	Low  int
	High int
}

// TemperatureRangesByPosition gives the T_max range for each slot 1-15.
var TemperatureRangesByPosition = map[int]TempRange{
	1:  {220, 260},
	2:  {170, 210},
	3:  {120, 160},
	4:  {70, 110},
	5:  {60, 100},
	6:  {50, 90},
	7:  {40, 80},
	8:  {30, 70},
	9:  {20, 60},
	10: {10, 50},
	11: {0, 40},
	12: {-10, 30},
	13: {-50, -10},
	14: {-90, -50},
	15: {-130, -90},
}

// FieldRange is the inclusive (low, high) range of total surface fields a
// planet may have for a given slot position.
type FieldRange struct {
	Low  int
	High int
}

// FieldsRangesByPosition gives the field range for each slot 1-15.
// Source: https://ogame.fandom.com/wiki/Colonizing_in_Redesigned_Universes
var FieldsRangesByPosition = map[int]FieldRange{
	1:  {40, 80},
	2:  {45, 90},
	3:  {50, 100},
	4:  {90, 175},
	5:  {120, 230},
	6:  {140, 260},
	7:  {140, 260},
	8:  {140, 260},
	9:  {140, 260},
	10: {100, 200},
	11: {100, 200},
	12: {100, 200},
	13: {50, 110},
	14: {50, 110},
	15: {50, 110},
}

// MetalBonusByPosition is the per-position production bonus for metal mines.
// Source: https://ogame.fandom.com/wiki/Metal_Mine
var MetalBonusByPosition = map[int]float64{
	6:  0.17,
	7:  0.23,
	8:  0.35,
	9:  0.23,
	10: 0.17,
}

// CrystalBonusByPosition is the per-position production bonus for crystal mines.
var CrystalBonusByPosition = map[int]float64{
	1: 0.40,
	2: 0.30,
	3: 0.20,
}

// Starting resources granted to every freshly created home planet.
const (
	StartingMetal       = 500
	StartingCrystal     = 500
	StartingDeuterium   = 0
	StartingFieldsUsed  = 0
	BuildQueueMaxActive = 5
)

// BuildingCost holds the base cost and exponential growth factor for one
// building. cost(L+1) = base * factor^L (floor).
type BuildingCost struct {
	Metal     int
	Crystal   int
	Deuterium int
	Factor    float64
}

// BuildingCosts maps every BuildingType to its cost row.
// Source: https://ogame.fandom.com/wiki/Buildings
var BuildingCosts = map[BuildingType]BuildingCost{
	BuildingMetalMine:            {60, 15, 0, 1.5},
	BuildingCrystalMine:          {48, 24, 0, 1.6},
	BuildingDeuteriumSynthesizer: {225, 75, 0, 1.5},
	BuildingSolarPlant:           {75, 30, 0, 1.5},
	BuildingFusionReactor:        {900, 360, 180, 1.8},
	// Solar satellite and crawler are flat per-unit costs.
	BuildingSolarSatellite:  {0, 2000, 500, 1.0},
	BuildingCrawler:         {2000, 2000, 1000, 1.0},
	BuildingRoboticsFactory: {400, 120, 200, 2.0},
	BuildingShipyard:        {400, 200, 100, 2.0},
	BuildingResearchLab:     {200, 400, 200, 2.0},
	BuildingMetalStorage:    {1000, 0, 0, 2.0},
	BuildingCrystalStorage:  {1000, 500, 0, 2.0},
	BuildingDeuteriumTank:   {1000, 1000, 0, 2.0},
	BuildingNaniteFactory:   {1_000_000, 500_000, 100_000, 2.0},
	BuildingMissileSilo:     {20_000, 20_000, 1_000, 2.0},
	BuildingAllianceDepot:   {20_000, 40_000, 0, 2.0},
	BuildingTerraformer:     {0, 50_000, 100_000, 2.0},
}

// MineEnergyCoeff is the multiplier in the mine energy formula:
// energy = floor(coeff * level * 1.1^level).
var MineEnergyCoeff = map[BuildingType]int{
	BuildingMetalMine:            10,
	BuildingCrystalMine:          10,
	BuildingDeuteriumSynthesizer: 20,
}

// ResearchCost mirrors BuildingCost for tech.
type ResearchCost struct {
	Metal     int
	Crystal   int
	Deuterium int
	Factor    float64
}

// ResearchCosts maps each TechType to its cost row.
// Source: https://ogame.fandom.com/wiki/Research
var ResearchCosts = map[TechType]ResearchCost{
	TechEnergy:          {0, 800, 400, 2.0},
	TechLaser:           {200, 100, 0, 2.0},
	TechIon:             {1000, 300, 100, 2.0},
	TechHyperspace:      {0, 4000, 2000, 2.0},
	TechPlasma:          {2000, 4000, 1000, 2.0},
	TechComputer:        {0, 400, 600, 2.0},
	TechAstrophysics:    {4000, 8000, 4000, 1.75},
	TechEspionage:       {200, 1000, 200, 2.0},
	TechCombustionDrive: {400, 0, 600, 2.0},
	TechImpulseDrive:    {2000, 4000, 600, 2.0},
	TechHyperspaceDrive: {10000, 20000, 6000, 2.0},
	TechWeapons:         {800, 200, 0, 2.0},
	TechShielding:       {200, 600, 0, 2.0},
	TechArmour:          {1000, 0, 0, 2.0},
}

// TechPrereqEntry is one prerequisite line: either a research dependency
// (Tech non-empty) or a research lab dependency (LabLevel > 0).
type TechPrereqEntry struct {
	Tech     TechType
	Level    int
	LabLevel int
}

// TechPrerequisites lists what must be in place before each research can start.
// The "lab" entry uses the highest research lab level across all the user's
// planets.
var TechPrerequisites = map[TechType][]TechPrereqEntry{
	TechEnergy: {{LabLevel: 1}},
	TechLaser:  {{LabLevel: 1}, {Tech: TechEnergy, Level: 2}},
	TechIon: {
		{LabLevel: 4},
		{Tech: TechEnergy, Level: 4},
		{Tech: TechLaser, Level: 5},
	},
	TechHyperspace: {{LabLevel: 7}, {Tech: TechEnergy, Level: 5}, {Tech: TechShielding, Level: 5}},
	TechPlasma: {
		{LabLevel: 4},
		{Tech: TechEnergy, Level: 8},
		{Tech: TechLaser, Level: 10},
		{Tech: TechIon, Level: 5},
	},
	TechComputer:        {{LabLevel: 1}},
	TechAstrophysics:    {{LabLevel: 3}, {Tech: TechEspionage, Level: 4}},
	TechEspionage:       {{LabLevel: 3}},
	TechCombustionDrive: {{LabLevel: 1}, {Tech: TechEnergy, Level: 1}},
	TechImpulseDrive:    {{LabLevel: 2}, {Tech: TechEnergy, Level: 1}},
	TechHyperspaceDrive: {
		{LabLevel: 7},
		{Tech: TechEnergy, Level: 5},
		{Tech: TechShielding, Level: 5},
		{Tech: TechHyperspace, Level: 3},
	},
	TechWeapons:   {{LabLevel: 4}},
	TechShielding: {{LabLevel: 6}, {Tech: TechEnergy, Level: 3}},
	TechArmour:    {{LabLevel: 2}},
}

// BuildingPrereqEntry is one prerequisite line for a building. A building can
// require either another building at a minimum level (Building non-empty) or a
// research at a minimum level (Tech non-empty).
type BuildingPrereqEntry struct {
	Building BuildingType
	Tech     TechType
	Level    int
}

// BuildingPrerequisites lists what must be in place before each building can
// be constructed (level 1+). Empty entry means no prerequisite.
// Source: https://ogame.fandom.com/wiki/Buildings
var BuildingPrerequisites = map[BuildingType][]BuildingPrereqEntry{
	BuildingFusionReactor: {
		{Building: BuildingDeuteriumSynthesizer, Level: 5},
		{Tech: TechEnergy, Level: 3},
	},
	BuildingShipyard: {
		{Building: BuildingRoboticsFactory, Level: 2},
	},
	BuildingResearchLab: {},
	BuildingMissileSilo: {
		{Building: BuildingShipyard, Level: 1},
	},
	BuildingNaniteFactory: {
		{Building: BuildingRoboticsFactory, Level: 10},
		{Tech: TechComputer, Level: 10},
	},
	BuildingTerraformer: {
		{Building: BuildingNaniteFactory, Level: 1},
		{Tech: TechEnergy, Level: 12},
	},
}

// ShipStat is the full combat + cost + movement profile for one ship hull.
// Source: https://ogame.fandom.com/wiki/Ships
type ShipStat struct {
	Metal     int
	Crystal   int
	Deuterium int
	Armor     int
	Shield    int
	Weapon    int
	Speed     int
	Cargo     int
	Fuel      int
}

// ShipStats indexes ship profiles by type. Speed values are at drive tech 0.
var ShipStats = map[ShipType]ShipStat{
	ShipSmallCargo:     {2000, 2000, 0, 4000, 10, 5, 5000, 5000, 10},
	ShipLargeCargo:     {6000, 6000, 0, 12000, 25, 5, 7500, 25000, 50},
	ShipLightFighter:   {3000, 1000, 0, 4000, 10, 50, 12500, 50, 20},
	ShipHeavyFighter:   {6000, 4000, 0, 10000, 25, 150, 10000, 100, 75},
	ShipCruiser:        {20000, 7000, 2000, 27000, 50, 400, 15000, 800, 300},
	ShipBattleship:     {45000, 15000, 0, 60000, 200, 1000, 10000, 1500, 500},
	ShipColonyShip:     {10000, 20000, 10000, 30000, 100, 50, 2500, 7500, 1000},
	ShipRecycler:       {10000, 6000, 2000, 16000, 10, 1, 2000, 20000, 300},
	ShipEspionageProbe: {0, 1000, 0, 1000, 1, 1, 100_000_000, 5, 1},
	ShipBomber:         {50000, 25000, 15000, 75000, 500, 1000, 4000, 500, 700},
	ShipDestroyer:      {60000, 50000, 15000, 110000, 500, 2000, 5000, 2000, 1000},
	ShipBattlecruiser:  {30000, 40000, 15000, 70000, 400, 700, 10000, 750, 250},
}

// ShipPrereqEntry is one prerequisite line for a ship: either a shipyard
// requirement (ShipyardLevel > 0) or a tech requirement.
type ShipPrereqEntry struct {
	ShipyardLevel int
	Tech          TechType
	Level         int
}

// ShipPrerequisites lists what must be in place before each ship can be built.
var ShipPrerequisites = map[ShipType][]ShipPrereqEntry{
	ShipSmallCargo: {{ShipyardLevel: 2}, {Tech: TechCombustionDrive, Level: 2}},
	ShipLargeCargo: {{ShipyardLevel: 4}, {Tech: TechCombustionDrive, Level: 6}},
	ShipLightFighter: {
		{ShipyardLevel: 1},
		{Tech: TechCombustionDrive, Level: 1},
	},
	ShipHeavyFighter: {
		{ShipyardLevel: 3},
		{Tech: TechArmour, Level: 2},
		{Tech: TechImpulseDrive, Level: 2},
	},
	ShipCruiser: {
		{ShipyardLevel: 5},
		{Tech: TechImpulseDrive, Level: 4},
		{Tech: TechIon, Level: 2},
	},
	ShipBattleship: {{ShipyardLevel: 7}, {Tech: TechHyperspaceDrive, Level: 4}},
	ShipColonyShip: {{ShipyardLevel: 4}, {Tech: TechImpulseDrive, Level: 3}},
	ShipRecycler: {
		{ShipyardLevel: 4},
		{Tech: TechCombustionDrive, Level: 6},
		{Tech: TechShielding, Level: 2},
	},
	ShipEspionageProbe: {
		{ShipyardLevel: 3},
		{Tech: TechCombustionDrive, Level: 3},
		{Tech: TechEspionage, Level: 2},
	},
	ShipBomber: {
		{ShipyardLevel: 8},
		{Tech: TechImpulseDrive, Level: 6},
		{Tech: TechPlasma, Level: 5},
	},
	ShipDestroyer: {
		{ShipyardLevel: 9},
		{Tech: TechHyperspaceDrive, Level: 6},
		{Tech: TechHyperspace, Level: 5},
	},
	ShipBattlecruiser: {
		{ShipyardLevel: 8},
		{Tech: TechHyperspaceDrive, Level: 5},
		{Tech: TechLaser, Level: 12},
		{Tech: TechHyperspace, Level: 5},
	},
}

// ShipDrive picks the drive tech that grants each ship its speed bonus.
var ShipDrive = map[ShipType]TechType{
	ShipSmallCargo:     TechCombustionDrive,
	ShipLargeCargo:     TechCombustionDrive,
	ShipLightFighter:   TechCombustionDrive,
	ShipRecycler:       TechCombustionDrive,
	ShipEspionageProbe: TechCombustionDrive,
	ShipHeavyFighter:   TechImpulseDrive,
	ShipCruiser:        TechImpulseDrive,
	ShipColonyShip:     TechImpulseDrive,
	ShipBomber:         TechImpulseDrive,
	ShipBattleship:     TechHyperspaceDrive,
	ShipDestroyer:      TechHyperspaceDrive,
	ShipBattlecruiser:  TechHyperspaceDrive,
}

// DefenseStat carries the cost and combat profile for one defense unit.
type DefenseStat struct {
	Metal     int
	Crystal   int
	Deuterium int
	Armor     int
	Shield    int
	Weapon    int
}

// DefenseStats indexes defense profiles by type.
var DefenseStats = map[DefenseType]DefenseStat{
	DefenseRocketLauncher:  {2000, 0, 0, 2000, 20, 80},
	DefenseLightLaser:      {1500, 500, 0, 2000, 25, 100},
	DefenseHeavyLaser:      {6000, 2000, 0, 8000, 100, 250},
	DefenseGaussCannon:     {20000, 15000, 2000, 35000, 200, 1100},
	DefenseIonCannon:       {5000, 3000, 0, 8000, 500, 150},
	DefensePlasmaTurret:    {50000, 50000, 30000, 100000, 300, 3000},
	DefenseSmallShieldDome: {10000, 10000, 0, 20000, 2000, 1},
	DefenseLargeShieldDome: {50000, 50000, 0, 100000, 10000, 1},
}

// DefensePrereqEntry mirrors ShipPrereqEntry for defenses.
type DefensePrereqEntry struct {
	ShipyardLevel int
	Tech          TechType
	Level         int
}

// DefensePrerequisites lists what must be in place before each defense can be built.
var DefensePrerequisites = map[DefenseType][]DefensePrereqEntry{
	DefenseRocketLauncher: {{ShipyardLevel: 1}},
	DefenseLightLaser: {
		{ShipyardLevel: 2},
		{Tech: TechEnergy, Level: 1},
		{Tech: TechLaser, Level: 3},
	},
	DefenseHeavyLaser: {
		{ShipyardLevel: 4},
		{Tech: TechEnergy, Level: 3},
		{Tech: TechLaser, Level: 6},
	},
	DefenseGaussCannon: {
		{ShipyardLevel: 6},
		{Tech: TechWeapons, Level: 3},
		{Tech: TechShielding, Level: 1},
		{Tech: TechEnergy, Level: 6},
	},
	DefenseIonCannon: {{ShipyardLevel: 4}, {Tech: TechIon, Level: 4}},
	DefensePlasmaTurret: {
		{ShipyardLevel: 8},
		{Tech: TechPlasma, Level: 7},
	},
	DefenseSmallShieldDome: {
		{ShipyardLevel: 1},
		{Tech: TechShielding, Level: 2},
	},
	DefenseLargeShieldDome: {
		{ShipyardLevel: 6},
		{Tech: TechShielding, Level: 6},
	},
}

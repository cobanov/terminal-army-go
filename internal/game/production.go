package game

import "math"

// ProductionReport summarises the hourly economic state of one planet.
type ProductionReport struct {
	MetalPerHour     float64
	CrystalPerHour   float64
	DeuteriumPerHour float64
	EnergyProduced   int
	EnergyConsumed   int
	ProductionFactor float64
	GrossMetal       float64
	GrossCrystal     float64
	GrossDeuterium   float64
}

// EnergyBalance is produced minus consumed (negative = deficit).
func (r ProductionReport) EnergyBalance() int {
	return r.EnergyProduced - r.EnergyConsumed
}

// AvgTemp is the arithmetic mean of min and max temperature, floored.
func AvgTemp(tempMin, tempMax int) int {
	return int(math.Floor(float64(tempMin+tempMax) / 2.0))
}

// ComputePlanetProduction aggregates every building and research bonus into a
// single hourly snapshot of metal, crystal, deuterium, and the energy balance.
// All inputs are pure values; this function does not touch the DB.
func ComputePlanetProduction(
	buildings map[BuildingType]int,
	researches map[TechType]int,
	tempMin, tempMax int,
	metalPositionBonus, crystalPositionBonus, speed float64,
) ProductionReport {
	plasma := researches[TechPlasma]
	energyTech := researches[TechEnergy]

	metalLvl := buildings[BuildingMetalMine]
	crystalLvl := buildings[BuildingCrystalMine]
	deutLvl := buildings[BuildingDeuteriumSynthesizer]

	baseM, baseC, baseD := BasePassiveProduction(speed)
	grossMetal := baseM + MetalMineProduction(metalLvl, speed, metalPositionBonus, plasma)
	grossCrystal := baseC + CrystalMineProduction(crystalLvl, speed, crystalPositionBonus, plasma)
	grossDeuterium := baseD + DeuteriumSynthesizerProduction(deutLvl, tempMax, speed, plasma)

	energyConsumed := MineEnergyConsumption(BuildingMetalMine, metalLvl) +
		MineEnergyConsumption(BuildingCrystalMine, crystalLvl) +
		MineEnergyConsumption(BuildingDeuteriumSynthesizer, deutLvl)

	solarLvl := buildings[BuildingSolarPlant]
	fusionLvl := buildings[BuildingFusionReactor]
	satCount := buildings[BuildingSolarSatellite]
	avgT := AvgTemp(tempMin, tempMax)

	energyProduced := SolarPlantOutput(solarLvl) +
		SolarSatelliteOutput(satCount, avgT) +
		FusionReactorOutput(fusionLvl, energyTech)

	fusionDeut := float64(FusionDeutConsumption(fusionLvl))
	grossDeuterium = math.Max(0, grossDeuterium-fusionDeut)

	productionFactor := 1.0
	if energyConsumed > 0 {
		productionFactor = math.Min(1.0, float64(energyProduced)/float64(energyConsumed))
	}

	netMetalMine := (grossMetal - baseM) * productionFactor
	netCrystalMine := (grossCrystal - baseC) * productionFactor
	deutMineRaw := DeuteriumSynthesizerProduction(deutLvl, tempMax, speed, plasma)
	netDeutMine := deutMineRaw * productionFactor

	// Source: https://ogame.fandom.com/wiki/Crawler - each crawler adds +0.02%
	// mine output, capped at +50%. Applies only to mine output, not to the
	// base passive trickle.
	crawlerCount := buildings[BuildingCrawler]
	crawlerBonus := math.Min(0.5, float64(crawlerCount)*0.0002)
	if crawlerBonus > 0 {
		netMetalMine *= 1 + crawlerBonus
		netCrystalMine *= 1 + crawlerBonus
		netDeutMine *= 1 + crawlerBonus
	}

	netMetal := baseM + netMetalMine
	netCrystal := baseC + netCrystalMine
	netDeut := baseD + netDeutMine - fusionDeut

	if netMetal < 0 {
		netMetal = 0
	}
	if netCrystal < 0 {
		netCrystal = 0
	}
	if netDeut < 0 {
		netDeut = 0
	}

	return ProductionReport{
		MetalPerHour:     netMetal,
		CrystalPerHour:   netCrystal,
		DeuteriumPerHour: netDeut,
		EnergyProduced:   energyProduced,
		EnergyConsumed:   energyConsumed,
		ProductionFactor: productionFactor,
		GrossMetal:       grossMetal,
		GrossCrystal:     grossCrystal,
		GrossDeuterium:   grossDeuterium,
	}
}

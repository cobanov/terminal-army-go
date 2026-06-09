package game

import (
	"math"
	"testing"
)

func TestMineEnergyConsumptionRoundsUpToWikiTable(t *testing.T) {
	tests := []struct {
		name     string
		building BuildingType
		level    int
		want     int
	}{
		{name: "metal L2", building: BuildingMetalMine, level: 2, want: 25},
		{name: "metal L4", building: BuildingMetalMine, level: 4, want: 59},
		{name: "crystal L2", building: BuildingCrystalMine, level: 2, want: 25},
		{name: "deuterium L2", building: BuildingDeuteriumSynthesizer, level: 2, want: 49},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MineEnergyConsumption(tt.building, tt.level); got != tt.want {
				t.Fatalf("MineEnergyConsumption(%s, %d) = %d, want %d", tt.building, tt.level, got, tt.want)
			}
		})
	}
}

func TestDeuteriumSynthesizerUsesAverageTemperature(t *testing.T) {
	got := DeuteriumSynthesizerProduction(1, 20, 1, 0)
	want := 10 * 1.1 * (1.36 - 0.004*20.0)
	if math.Abs(got-want) > 0.000001 {
		t.Fatalf("DeuteriumSynthesizerProduction = %.6f, want %.6f", got, want)
	}

	if got := DeuteriumSynthesizerProduction(5, 340, 1, 0); math.Abs(got) > 0.000001 {
		t.Fatalf("DeuteriumSynthesizerProduction at 340C avg = %.6f, want 0", got)
	}
}

func TestSolarSatelliteOutputMatchesAverageTemperatureTable(t *testing.T) {
	tests := []struct {
		avgTemp int
		want    int
	}{
		{avgTemp: 100, want: 43},
		{avgTemp: 0, want: 26},
		{avgTemp: -40, want: 20},
	}
	for _, tt := range tests {
		if got := SolarSatelliteOutput(1, tt.avgTemp); got != tt.want {
			t.Fatalf("SolarSatelliteOutput(1, %d) = %d, want %d", tt.avgTemp, got, tt.want)
		}
	}
}

func TestFusionDeutConsumptionScalesWithEconomyAndRoundsAsNegative(t *testing.T) {
	if got := FusionDeutConsumption(2, 1); got != 25 {
		t.Fatalf("FusionDeutConsumption(2, 1) = %d, want 25", got)
	}
	if got := FusionDeutConsumption(2, 2); got != 49 {
		t.Fatalf("FusionDeutConsumption(2, 2) = %d, want 49", got)
	}
}

func TestCrawlerActiveCapAndEnergyConsumption(t *testing.T) {
	active := ActiveCrawlerCount(999, 21, 20, 19)
	if active != 480 {
		t.Fatalf("ActiveCrawlerCount = %d, want 480", active)
	}
	if got := CrawlerEnergyConsumption(active); got != 24000 {
		t.Fatalf("CrawlerEnergyConsumption = %d, want 24000", got)
	}
}

func TestSolarPlantLevelOneMatchesFandomCostAndRedesignedTime(t *testing.T) {
	metal, crystal, deut := BuildingLevelCost(BuildingSolarPlant, 1)
	if metal != 75 || crystal != 30 || deut != 0 {
		t.Fatalf("Solar Plant L1 cost = %d/%d/%d, want 75/30/0", metal, crystal, deut)
	}

	if got := BuildTimeSeconds(metal, crystal, 0, 0, 1); got != 151 {
		t.Fatalf("base BuildTimeSeconds for Solar Plant L1 = %d, want 151", got)
	}
	if got := BuildingUpgradeTimeSeconds(metal, crystal, 0, 0, 1, 1); got != 43 {
		t.Fatalf("redesigned Solar Plant L1 build time = %d, want 43", got)
	}
}

func TestBuildingUpgradeTimeUsesEarlyLevelReductionThroughLevelFive(t *testing.T) {
	tests := []struct {
		name   string
		target int
		want   int
	}{
		{name: "level 1", target: 1, want: 43},
		{name: "level 5", target: 5, want: 508},
		{name: "level 6", target: 6, want: 1146},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metal, crystal, _ := BuildingLevelCost(BuildingSolarPlant, tt.target)
			got := BuildingUpgradeTimeSeconds(metal, crystal, 0, 0, tt.target, 1)
			if got != tt.want {
				t.Fatalf("BuildingUpgradeTimeSeconds target %d = %d, want %d", tt.target, got, tt.want)
			}
		})
	}
}

func TestProductionIncludesCrawlerCapAndEnergyUse(t *testing.T) {
	report := ComputePlanetProduction(
		map[BuildingType]int{
			BuildingMetalMine:            1,
			BuildingCrystalMine:          1,
			BuildingDeuteriumSynthesizer: 1,
			BuildingCrawler:              999,
		},
		nil,
		0, 40,
		0, 0,
		1,
	)
	wantEnergy := MineEnergyConsumption(BuildingMetalMine, 1) +
		MineEnergyConsumption(BuildingCrystalMine, 1) +
		MineEnergyConsumption(BuildingDeuteriumSynthesizer, 1) +
		CrawlerEnergyConsumption(24)
	if report.EnergyConsumed != wantEnergy {
		t.Fatalf("EnergyConsumed = %d, want %d", report.EnergyConsumed, wantEnergy)
	}
	if report.ProductionFactor != 0 {
		t.Fatalf("ProductionFactor = %.2f, want 0 with no energy production", report.ProductionFactor)
	}
}

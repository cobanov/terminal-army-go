package tui

import (
	"os"
	"testing"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/x/ansi"
	"github.com/cobanov/terminal-army-go/internal/svc"
)

// TestRenderPreview dumps the buildings view at a fixed size (ANSI stripped) so
// layout/alignment can be eyeballed without a real terminal. Run with:
//
//	go test ./internal/tui -run TestRenderPreview -v
func TestRenderPreview(t *testing.T) {
	planet := &svc.Planet{Name: "Homeworld", Galaxy: 4, System: 440, Position: 5, Metal: 396, Crystal: 463, EnergyUsed: 11}
	sess := &replSession{planets: []svc.Planet{*planet}, currentIndex: 0, user: &svc.User{Username: "cobanov"}}
	b := []svc.BuildingView{
		{Key: "metal_mine", Label: "Metal Mine", Level: 1, NextCost: svc.Cost{Metal: 90, Crystal: 22}, BuildSeconds: 53, Affordable: true},
		{Key: "crystal_mine", Label: "Crystal Mine", Level: 0, NextCost: svc.Cost{Metal: 48, Crystal: 24}, BuildSeconds: 29, Affordable: true},
		{Key: "deuterium_synthesizer", Label: "Deuterium Synthesizer", Level: 0, NextCost: svc.Cost{Metal: 225, Crystal: 75}, BuildSeconds: 123, Affordable: true},
		{Key: "solar_plant", Label: "Solar Plant", Level: 0, NextCost: svc.Cost{Metal: 75, Crystal: 30}, BuildSeconds: 43, Affordable: true},
		{Key: "fusion_reactor", Label: "Fusion Reactor", Level: 0, NextCost: svc.Cost{Metal: 900, Crystal: 360, Deuterium: 180}, BuildSeconds: 518, Locked: true, LockedReason: "deuterium_synthesizer level 5 required, energy level 3 required"},
		{Key: "solar_satellite", Label: "Solar Satellite", Level: 0, NextCost: svc.Cost{Crystal: 2000, Deuterium: 500}, BuildSeconds: 822, Affordable: false},
		{Key: "crawler", Label: "Crawler", Level: 0, NextCost: svc.Cost{Metal: 2000, Crystal: 2000, Deuterium: 1000}, BuildSeconds: 1645, Affordable: false},
		{Key: "metal_storage", Label: "Metal Storage", Level: 0, NextCost: svc.Cost{Metal: 1000}, BuildSeconds: 411, Affordable: false},
	}
	ti := textinput.New()
	ti.Prompt = "tarmy> "
	ti.Focus()
	m := appModel{
		session: sess, active: viewBuildings, hover: -1, width: 140, height: 45,
		input: ti, status: "ready", cmdAt: -1,
		data: viewData{loaded: viewBuildings, buildings: b, planet: planet},
		log:  []string{"› /upgrade metal_mine", "queued metal_mine level 2, finishes 2:33AM"},
	}
	if out := os.Getenv("PREVIEW_OUT"); out != "" {
		_ = os.WriteFile(out, []byte(ansi.Strip(m.View())), 0o644)
	}
}

package tui

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"
	"github.com/cobanov/terminal-army-go/internal/svc"
	"github.com/cobanov/terminal-army-go/internal/tui/client"
)

func TestParseCoord(t *testing.T) {
	g, s, p, err := parseCoord("4:128:6")
	if err != nil {
		t.Fatalf("parseCoord returned error: %v", err)
	}
	if g != 4 || s != 128 || p != 6 {
		t.Fatalf("parseCoord = %d:%d:%d, want 4:128:6", g, s, p)
	}

	if _, _, _, err := parseCoord("4:128"); err == nil {
		t.Fatal("parseCoord accepted an incomplete coordinate")
	}
}

func TestParseKVArgsSplitsShipsAndCargo(t *testing.T) {
	ships, cargo := parseKVArgs([]string{
		"small_cargo=2",
		"espionage_probe=1",
		"m=100",
		"crystal=50",
		"d=25",
		"bad=not-a-number",
		"zero=0",
	})

	if ships["small_cargo"] != 2 || ships["espionage_probe"] != 1 {
		t.Fatalf("ships = %#v", ships)
	}
	if cargo["metal"] != 100 || cargo["crystal"] != 50 || cargo["deuterium"] != 25 {
		t.Fatalf("cargo = %#v", cargo)
	}
	if _, ok := ships["zero"]; ok {
		t.Fatal("zero-value ship count should be ignored")
	}
}

func TestSuggestionsForInput(t *testing.T) {
	t.Run("slashless command", func(t *testing.T) {
		got := suggestionsForInput("gal", nil)
		if len(got) == 0 || got[0].value != "/galaxy " {
			t.Fatalf("suggestionsForInput(gal) = %#v", got)
		}
	})

	t.Run("building argument", func(t *testing.T) {
		got := suggestionsForInput("/upgrade metal", nil)
		if len(got) == 0 || got[0].value != "/upgrade metal_mine" {
			t.Fatalf("suggestionsForInput(/upgrade metal) = %#v", got)
		}
	})

	t.Run("ship build argument", func(t *testing.T) {
		got := suggestionsForInput("/ships build small", nil)
		if len(got) == 0 || got[0].value != "/ships build small_cargo " {
			t.Fatalf("suggestionsForInput(/ships build small) = %#v", got)
		}
	})

	t.Run("planet switch", func(t *testing.T) {
		planets := []svc.Planet{{Code: "ABCD", Name: "Homeworld", Galaxy: 1, System: 2, Position: 3}}
		got := suggestionsForInput("/switch home", planets)
		if len(got) != 1 || got[0].value != "/switch ABCD" {
			t.Fatalf("suggestionsForInput(/switch home) = %#v", got)
		}
	})
}

func TestConsoleCockpitViewContainsMajorRegions(t *testing.T) {
	now := time.Now()
	input := textinput.New()
	input.Prompt = "tarmy> "
	model := consoleModel{
		ctx:   context.Background(),
		input: input,
		session: &replSession{
			client: client.New("https://terminal.army"),
			user:   &svc.User{Username: "cobanov", Email: "cobanov@example.test", Role: "player"},
			planets: []svc.Planet{{
				ID:                     1,
				Code:                   "3LS5",
				Name:                   "Homeworld",
				Galaxy:                 6,
				System:                 262,
				Position:               11,
				FieldsUsed:             37,
				FieldsTotal:            138,
				TempMin:                -28,
				TempMax:                12,
				Metal:                  209000,
				Crystal:                178700,
				Deuterium:              101900,
				EnergyProduced:         753,
				EnergyUsed:             554,
				ResourcesLastUpdatedAt: now,
			}},
		},
		width:  140,
		height: 42,
		log:    []string{"resources", "metal_mine 11"},
		side: consoleSideState{
			planetID: 1,
			production: &svc.ProductionReport{
				MetalPerHour:     4858,
				CrystalPerHour:   880,
				DeuteriumPerHour: 506,
				ProductionFactor: 1,
			},
			queues: []svc.QueueItem{{
				ID:          9,
				QueueType:   "building",
				ItemKey:     "metal_mine",
				TargetLevel: 12,
				FinishedAt:  now.Add(2 * time.Minute),
			}},
		},
	}
	view := model.View()
	for _, want := range []string{"PLANET", "RESEARCH", "FLEET", "PLANETS", "QUEUES (1/5)", "metal_mine L12", "terminal.army", "Homeworld", "M 209.0k"} {
		if !strings.Contains(view, want) {
			t.Fatalf("cockpit view missing %q\n%s", want, view)
		}
	}
	assertMaxLineWidth(t, view, model.width)
}

func TestConsoleCockpitViewDoesNotOverflowWideTerminal(t *testing.T) {
	now := time.Now()
	input := textinput.New()
	input.Prompt = "tarmy> "
	model := consoleModel{
		ctx:   context.Background(),
		input: input,
		session: &replSession{
			client: client.New("https://terminal.army"),
			user:   &svc.User{Username: "cobanov", Email: "cobanov@example.test", Role: "player"},
			planets: []svc.Planet{
				{ID: 1, Code: "3SXQ7YSQ", Name: "Homeworld", Galaxy: 7, System: 447, Position: 12, FieldsUsed: 3, FieldsTotal: 108, TempMin: -15, TempMax: 25, Metal: 225, Crystal: 435, EnergyProduced: 0, EnergyUsed: 0},
			},
		},
		width:  238,
		height: 60,
		side: consoleSideState{
			planetID: 1,
			queues: []svc.QueueItem{{
				ID:          4,
				QueueType:   "building",
				ItemKey:     "metal_mine",
				TargetLevel: 4,
				StartedAt:   now.Add(-time.Minute),
				FinishedAt:  now.Add(3 * time.Minute),
			}},
		},
	}
	assertMaxLineWidth(t, model.View(), model.width)
}

func assertMaxLineWidth(t *testing.T, view string, maxWidth int) {
	t.Helper()
	for i, line := range strings.Split(view, "\n") {
		if w := lipgloss.Width(line); w > maxWidth {
			t.Fatalf("line %d width = %d, want <= %d\n%s", i+1, w, maxWidth, line)
		}
	}
}

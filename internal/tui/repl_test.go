package tui

import (
	"context"
	"fmt"
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

func TestAppViewContainsMajorRegions(t *testing.T) {
	m := testAppModel(t, 140, 42)
	m.active = viewBuildings
	m.data = viewData{
		loaded: viewBuildings,
		buildings: []svc.BuildingView{
			{Key: "metal_mine", Label: "Metal Mine", Level: 11, NextCost: svc.Cost{Metal: 24500, Crystal: 6100}, BuildSeconds: 1930, Affordable: true},
		},
	}
	m.rail = railData{queues: []svc.QueueItem{{ID: 9, QueueType: "building", ItemKey: "metal_mine", TargetLevel: 12, FinishedAt: time.Now().Add(2 * time.Minute)}}}
	view := m.View()
	for _, want := range []string{"EMPIRE", "OPS", "SOCIAL", "BUILDINGS", "QUEUE", "tarmy", "HOMEWORLD", "Metal Mine"} {
		if !strings.Contains(view, want) {
			t.Fatalf("app view missing %q\n%s", want, view)
		}
	}
	assertMaxLineWidth(t, view, m.width)
}

func TestAppViewDoesNotOverflowWideTerminal(t *testing.T) {
	m := testAppModel(t, 238, 60)
	assertMaxLineWidth(t, m.View(), m.width)
}

func TestAppViewRespondsToTerminalSizes(t *testing.T) {
	for _, size := range []struct{ width, height int }{
		{160, 48}, {120, 40}, {96, 36}, {80, 32}, {60, 26}, {40, 18},
	} {
		t.Run(fmt.Sprintf("%dx%d", size.width, size.height), func(t *testing.T) {
			m := testAppModel(t, size.width, size.height)
			view := m.View()
			assertMaxLineWidth(t, view, size.width)
			assertMaxLineCount(t, view, size.height)
		})
	}
}

func testAppModel(t *testing.T, width, height int) appModel {
	t.Helper()
	m := newAppModel(context.Background(), &replSession{
		client: client.New("https://terminal.army"),
		user:   &svc.User{Username: "cobanov", Email: "cobanov@example.test", Role: "player"},
		planets: []svc.Planet{
			{ID: 1, Code: "3SXQ7YSQ", Name: "Homeworld", Galaxy: 7, System: 447, Position: 12, FieldsUsed: 3, FieldsTotal: 108, TempMin: -15, TempMax: 25, Metal: 209000, Crystal: 178700, Deuterium: 101900, EnergyProduced: 753, EnergyUsed: 554},
		},
	})
	m.width, m.height = width, height
	m.log = []string{"resources", "metal_mine 11", "very long command output that should be truncated rather than wrapping past the terminal edge and never overflow"}
	_ = textinput.New()
	return m
}

func assertMaxLineWidth(t *testing.T, view string, maxWidth int) {
	t.Helper()
	for i, line := range strings.Split(view, "\n") {
		if w := lipgloss.Width(line); w > maxWidth {
			t.Fatalf("line %d width = %d, want <= %d\n%s", i+1, w, maxWidth, line)
		}
	}
}

func assertMaxLineCount(t *testing.T, view string, maxHeight int) {
	t.Helper()
	lines := strings.Split(strings.TrimRight(view, "\n"), "\n")
	if len(lines) > maxHeight {
		t.Fatalf("line count = %d, want <= %d\n%s", len(lines), maxHeight, view)
	}
}

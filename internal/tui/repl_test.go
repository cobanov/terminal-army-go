package tui

import (
	"testing"

	"github.com/cobanov/terminal-army-go/internal/svc"
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

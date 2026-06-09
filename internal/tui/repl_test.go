package tui

import "testing"

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

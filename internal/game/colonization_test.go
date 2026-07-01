package game

import "testing"

func TestMaxPlanets(t *testing.T) {
	// Source: OGame wiki — 1 homeworld + ceil(astro/2) colonies.
	want := map[int]int{0: 1, 1: 2, 2: 2, 3: 3, 4: 3, 5: 4, 6: 4, 7: 5, 8: 5}
	for level, exp := range want {
		if got := MaxPlanets(level); got != exp {
			t.Errorf("MaxPlanets(%d) = %d, want %d", level, got, exp)
		}
	}
}

func TestColonizablePositionRange(t *testing.T) {
	cases := []struct {
		level, low, high int
	}{
		{0, 4, 12}, {1, 4, 12}, {3, 4, 12},
		{4, 3, 13}, {5, 3, 13},
		{6, 2, 14}, {7, 2, 14},
		{8, 1, 15}, {12, 1, 15},
	}
	for _, c := range cases {
		lo, hi := ColonizablePositionRange(c.level)
		if lo != c.low || hi != c.high {
			t.Errorf("ColonizablePositionRange(%d) = %d-%d, want %d-%d", c.level, lo, hi, c.low, c.high)
		}
	}
	if CanColonizePosition(1, 3) {
		t.Error("position 3 must not be colonizable at Astrophysics 1")
	}
	if !CanColonizePosition(8, 1) {
		t.Error("position 1 must be colonizable at Astrophysics 8")
	}
}

func TestFieldsRangesCoverAllPositions(t *testing.T) {
	for pos := 1; pos <= 15; pos++ {
		fr, ok := FieldsRangesByPosition[pos]
		if !ok {
			t.Errorf("position %d missing from FieldsRangesByPosition", pos)
			continue
		}
		if fr.Low <= 0 || fr.High < fr.Low {
			t.Errorf("position %d has invalid range %d-%d", pos, fr.Low, fr.High)
		}
	}
}

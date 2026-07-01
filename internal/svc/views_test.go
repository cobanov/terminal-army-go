package svc

import "testing"

func TestAffordableCount(t *testing.T) {
	cases := []struct {
		name                 string
		metal, crystal, deut float64
		m, c, d              int
		want                 int
	}{
		{"metal-limited", 1000, 5000, 5000, 100, 50, 0, 10},
		{"crystal-limited", 5000, 1000, 5000, 100, 50, 0, 20},
		{"deut-limited", 5000, 5000, 90, 100, 50, 25, 3},
		{"probe crystal only", 0, 1000, 0, 0, 1000, 0, 1},
		{"exact one", 100, 50, 0, 100, 50, 0, 1},
		{"cannot afford", 50, 50, 0, 100, 50, 0, 0},
		{"all zero cost yields zero", 50, 50, 50, 0, 0, 0, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := affordableCount(tc.metal, tc.crystal, tc.deut, tc.m, tc.c, tc.d)
			if got != tc.want {
				t.Fatalf("affordableCount(%v,%v,%v, %d,%d,%d) = %d, want %d",
					tc.metal, tc.crystal, tc.deut, tc.m, tc.c, tc.d, got, tc.want)
			}
		})
	}
}

package core

import (
	"testing"
)

func TestSitePower(t *testing.T) {
	tc := []struct {
		grid, battery, site float64
	}{
		{0, 0, 0},    // silent night
		{0, 1, 1},    // battery discharging
		{0, -1, -1},  // battery charging -> negative result cannot occur in reality
		{1, 0, 1},    // grid import
		{1, 1, 2},    // grid import + battery discharging
		{-1, 0, -1},  // grid export
		{-1, -1, -2}, // grid export + battery charging
	}

	for _, tc := range tc {
		res := sitePower(tc.grid, tc.battery, 0)
		if res != tc.site {
			t.Errorf("sitePower wanted %.f, got %.f", tc.site, res)
		}
	}
}

// TODO add test case for battery priority charging

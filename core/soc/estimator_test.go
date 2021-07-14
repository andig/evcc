package soc

import (
	"testing"
	"time"

	"github.com/andig/evcc/util"
)

func TestRemainingChargeDuration(t *testing.T) {
	// 9 kWh userBatCap => 10 kWh virtualBatCap
	ce := NewEstimator(util.NewLogger("foo"), 9, false)
	ce.estimatedSoC = 20.0

	chargePower := 1000.0
	targetSoC := 80

	if remaining := ce.RemainingChargeDuration(targetSoC, chargePower, 0); remaining != 6*time.Hour {
		t.Error("wrong remaining charge duration")
	}
}

func TestSoCEstimation(t *testing.T) {
	// 9 kWh user battery capacity is converted to initial value of 10 kWh virtual capacity
	ce := NewEstimator(util.NewLogger("foo"), 9, true)
	ce.estimatedSoC = 20.0

	tc := []struct {
		chargedEnergy   float64
		vehicleSoC      float64
		estimatedSoC    float64
		virtualCapacity float64
	}{
		{10, 20.0, 20.0, 10000},
		{0, 20.0, 20.0, 10000},
		{123, 20.0, 21.23, 10000},
		{1000, 20.0, 30.0, 10000},
		{1100, 31.0, 31.0, 10000},
		{1200, 32.0, 32.0, 10000},
		{1900, 39.0, 39.0, 10000},
		{2000, 40.0, 40.0, 10000},
		{4000, 50.0, 50.0, 20000}, // 2kWh add 10% -> 20kWh battery
		{6000, 60.0, 60.0, 20000}, // 2kWh add 10% -> 20kWh battery
		{6500, 65.0, 65.0, 10000},
		{7000, 65.0, 70.0, 10000},
		{7100, 71.0, 71.0, 10000},
		{7300, 72.0, 72.0, 10000},
		{7400, 73.0, 73.0, 10000},
		{7700, 75.0, 75.0, 10000},
		{8200, 80.0, 80.0, 10000},
		{0, 25.0, 25.0, 10000},
		{2500, 25.0, 50.0, 10000},
		{0, 50.0, 50.0, 10000}, // -10000
		{4990, 50.0, 99.9, 10000},
		{5000, 50.0, 100.0, 10000},
		{5001, 50.0, 100.0, 10000},
		{0, 0.0, 0.0, 10000},
		{1000, 0.0, 10.0, 10000},
	}

	for _, tc := range tc {
		t.Logf("%+v", tc)

		soc := ce.SoC(tc.vehicleSoC, tc.chargedEnergy, false)

		// validate soc estimate
		if tc.estimatedSoC != soc {
			t.Errorf("expected estimated soc: %g, got: %g", tc.estimatedSoC, soc)
		}

		// validate capacity estimate
		if tc.virtualCapacity != ce.virtualCapacity {
			t.Errorf("expected virtual capacity: %v, got: %v", tc.virtualCapacity, ce.virtualCapacity)
		}

		// validate duration estimate
		chargePower := 1e3
		targetSoC := 100
		remainingHours := (float64(targetSoC) - soc) / 100 * tc.virtualCapacity / chargePower
		remainingDuration := time.Duration(float64(time.Hour) * remainingHours).Round(time.Second)

		if rm := ce.RemainingChargeDuration(targetSoC, chargePower, 0); rm != remainingDuration {
			t.Errorf("expected estimated duration: %v, got: %v", remainingDuration, rm)
		}
	}
}

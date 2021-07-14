package soc

import (
	"math"
	"time"

	"github.com/andig/evcc/util"
)

const chargeEfficiency = 0.9 // assume charge 90% efficiency

// Estimator provides vehicle soc and charge duration
// Vehicle SoC can be estimated to provide more granularity
type Estimator struct {
	log      *util.Logger
	estimate bool

	capacity          float64 // vehicle capacity in Wh cached to simplify testing
	virtualCapacity   float64 // estimated virtual vehicle capacity in Wh
	estimatedSoC      float64 // estimated vehicle SoC
	measuredSoC       float64 // previous vehicle SoC in %
	prevChargedEnergy float64 // previous charged energy in Wh
	energyPerSocStep  float64 // Energy per SoC percent in Wh
}

// NewEstimator creates new estimator
func NewEstimator(log *util.Logger, capacity int64, estimate bool) *Estimator {
	s := &Estimator{
		log:      log,
		estimate: estimate,
	}

	s.ResetCapacity(capacity)

	return s
}

// Reset resets the estimation process to default values
func (s *Estimator) ResetCapacity(capacity int64) {
	s.measuredSoC = 0
	s.prevChargedEnergy = 0

	s.capacity = float64(capacity) * 1e3              // cache to simplify debugging
	s.virtualCapacity = s.capacity / chargeEfficiency // initial capacity taking efficiency into account
	s.energyPerSocStep = s.virtualCapacity / 100
}

// SoC replaces the api.Vehicle.SoC interface to take charged energy into account
// func (s *Estimator) SoC(chargedEnergy float64) (float64, error) {
// 	f, err := s.vehicle.SoC()

// 	if err != nil {
// 		if errors.Is(err, api.ErrMustRetry) {
// 			return 0, err
// 		}

// 		s.log.WARN.Printf("updating soc failed: %v", err)

// 		// try to recover from temporary vehicle-api errors
// 		if s.measuredSoC == 0 { // never received a soc value
// 			return s.estimatedSoC, err
// 		}

// 		f = s.measuredSoC // recover last received soc
// 	}

// 	return s.UpdateSoC(f, chargedEnergy), err
// }

func (s *Estimator) SoC(soc, chargedEnergy float64, socKnown bool) float64 {
	estimatedSoC := soc

	if socKnown {
		s.measuredSoC = soc
	} else if s.estimate {
		socDelta := soc - s.measuredSoC
		energyDelta := math.Max(chargedEnergy, 0) - s.prevChargedEnergy

		if socDelta != 0 || energyDelta < 0 { // soc value change or unexpected energy reset
			// calculate gradient, wh per soc %
			// TODO: drop samples with unmatching state of evse and vehicle
			if socDelta > 2 && energyDelta > 0 && s.measuredSoC > 0 {
				s.energyPerSocStep = energyDelta / socDelta
				s.virtualCapacity = s.energyPerSocStep * 100
				s.log.TRACE.Printf("soc gradient updated: energyPerSocStep: %0.0fWh, virtualCapacity: %0.0fWh", s.energyPerSocStep, s.virtualCapacity)
			}

			// sample charged energy at soc change, reset energy delta
			s.prevChargedEnergy = math.Max(chargedEnergy, 0)
		} else {
			estimatedSoC = math.Min(soc+energyDelta/s.energyPerSocStep, 100)
			s.log.TRACE.Printf("soc estimated: %.2f%% (vehicle: %.2f%%)", estimatedSoC, soc)
		}

		s.measuredSoC = soc
	}

	// store estimate for use with remaining time/energy
	s.estimatedSoC = estimatedSoC

	return estimatedSoC
}

// RemainingChargeEnergy returns the remaining charge energy in kWh
func (s *Estimator) RemainingChargeEnergy(targetSoC int) float64 {
	percentRemaining := float64(targetSoC) - s.estimatedSoC
	if percentRemaining <= 0 {
		return 0
	}

	// estimate remaining energy
	whRemaining := percentRemaining / 100 * s.virtualCapacity
	return whRemaining / 1e3
}

// RemainingChargeDuration returns the remaining duration estimate based on SoC, target and charge power
func (s *Estimator) RemainingChargeDuration(targetSoC int, chargePower float64, timeRemaining time.Duration) time.Duration {
	if chargePower <= 50 {
		return -1
	}

	percentRemaining := float64(targetSoC) - s.estimatedSoC
	if percentRemaining <= 0 {
		return 0
	}

	// use vehicle api value if available
	if timeRemaining > 0 {
		return time.Duration(float64(timeRemaining) * percentRemaining / (100 - s.estimatedSoC))
	}

	// estimate remaining time
	whRemaining := percentRemaining / 100 * s.virtualCapacity
	return time.Duration(float64(time.Hour) * whRemaining / chargePower).Round(time.Second)
}

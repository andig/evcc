package charger

// Code generated by github.com/andig/cmd/tools/decorate.go. DO NOT EDIT.

import (
	"github.com/andig/evcc/api"
)

func decorateWallbe(base api.Charger, meter func() (float64, error), meterEnergy func() (float64, error), meterCurrent func() (float64, float64, float64, error)) api.Charger {
	switch {
	case meter == nil && meterCurrent == nil && meterEnergy == nil:
		return base

	case meter != nil && meterCurrent == nil && meterEnergy == nil:
		return &struct{
			api.Charger
			api.Meter
		}{
			Charger: base,
			Meter: &decorateWallbeMeterImpl{
				meter: meter,
			},
		}

	case meter == nil && meterCurrent == nil && meterEnergy != nil:
		return &struct{
			api.Charger
			api.MeterEnergy
		}{
			Charger: base,
			MeterEnergy: &decorateWallbeMeterEnergyImpl{
				meterEnergy: meterEnergy,
			},
		}

	case meter != nil && meterCurrent == nil && meterEnergy != nil:
		return &struct{
			api.Charger
			api.Meter
			api.MeterEnergy
		}{
			Charger: base,
			Meter: &decorateWallbeMeterImpl{
				meter: meter,
			},
			MeterEnergy: &decorateWallbeMeterEnergyImpl{
				meterEnergy: meterEnergy,
			},
		}

	case meter == nil && meterCurrent != nil && meterEnergy == nil:
		return &struct{
			api.Charger
			api.MeterCurrent
		}{
			Charger: base,
			MeterCurrent: &decorateWallbeMeterCurrentImpl{
				meterCurrent: meterCurrent,
			},
		}

	case meter != nil && meterCurrent != nil && meterEnergy == nil:
		return &struct{
			api.Charger
			api.Meter
			api.MeterCurrent
		}{
			Charger: base,
			Meter: &decorateWallbeMeterImpl{
				meter: meter,
			},
			MeterCurrent: &decorateWallbeMeterCurrentImpl{
				meterCurrent: meterCurrent,
			},
		}

	case meter == nil && meterCurrent != nil && meterEnergy != nil:
		return &struct{
			api.Charger
			api.MeterCurrent
			api.MeterEnergy
		}{
			Charger: base,
			MeterCurrent: &decorateWallbeMeterCurrentImpl{
				meterCurrent: meterCurrent,
			},
			MeterEnergy: &decorateWallbeMeterEnergyImpl{
				meterEnergy: meterEnergy,
			},
		}

	case meter != nil && meterCurrent != nil && meterEnergy != nil:
		return &struct{
			api.Charger
			api.Meter
			api.MeterCurrent
			api.MeterEnergy
		}{
			Charger: base,
			Meter: &decorateWallbeMeterImpl{
				meter: meter,
			},
			MeterCurrent: &decorateWallbeMeterCurrentImpl{
				meterCurrent: meterCurrent,
			},
			MeterEnergy: &decorateWallbeMeterEnergyImpl{
				meterEnergy: meterEnergy,
			},
		}
	}

	return nil
}

type decorateWallbeMeterImpl struct {
	meter func() (float64, error)
}

func (impl *decorateWallbeMeterImpl) CurrentPower() (float64, error) {
	return impl.meter()
}

type decorateWallbeMeterCurrentImpl struct {
	meterCurrent func() (float64, float64, float64, error)
}

func (impl *decorateWallbeMeterCurrentImpl) Currents() (float64, float64, float64, error) {
	return impl.meterCurrent()
}

type decorateWallbeMeterEnergyImpl struct {
	meterEnergy func() (float64, error)
}

func (impl *decorateWallbeMeterEnergyImpl) TotalEnergy() (float64, error) {
	return impl.meterEnergy()
}

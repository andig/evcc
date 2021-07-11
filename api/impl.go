package api

import "strings"

// ChargeModeString converts string to ChargeMode
func ChargeModeString(mode string) ChargeMode {
	switch strings.ToLower(mode) {
	case string(ModeNow):
		return ModeNow
	case string(ModeMinPV):
		return ModeMinPV
	case string(ModePV):
		return ModePV
	case string(ModeOff):
		return ModeOff
	default:
		return ""
	}
}

// GetMeterEnergy return the MeterEnergy if meter provides energy
func GetMeterEnergy(meter Meter) (MeterEnergy, bool) {
	m, ok := meter.(MeterEnergy)
	if !ok {
		return nil, false
	}

	optional, ok := m.(OptionalMeterEnergy)
	if !ok {
		return m, true
	}
	if optional.HasEnergy() {
		return m, true
	}
	return nil, false
}

// GetMeterCurrent return the MeterCurrent if meter provides currents
func GetMeterCurrent(meter Meter) (MeterCurrent, bool) {
	m, ok := meter.(MeterCurrent)
	if !ok {
		return nil, false
	}

	optional, ok := m.(OptionalMeterCurrent)
	if !ok {
		return m, true
	}
	if optional.HasCurrent() {
		return m, true
	}
	return nil, false
}

// GetBattery return the Battery if meter provides currents
func GetBattery(meter interface{}) (Battery, bool) {
	m, ok := meter.(Battery)
	if !ok {
		return nil, false
	}

	optional, ok := m.(OptionalBattery)
	if !ok {
		return m, true
	}
	if optional.HasSoC() {
		return m, true
	}
	return nil, false
}

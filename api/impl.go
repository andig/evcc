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

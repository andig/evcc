package loadpoint

import (
	"time"

	"github.com/andig/evcc/api"
)

// API is the external loadpoint API
type API interface {
	Name() string
	HasChargeMeter() bool

	// status
	GetStatus() api.ChargeStatus

	// settings
	GetMode() api.ChargeMode
	SetMode(api.ChargeMode)
	GetTargetSoC() int
	SetTargetSoC(int) error
	GetMinSoC() int
	SetMinSoC(int) error
	SetTargetCharge(time.Time, int)
	RemoteControl(string, RemoteDemand)

	// energy
	GetChargePower() float64
	GetMinCurrent() float64
	SetMinCurrent(float64)
	GetMaxCurrent() float64
	SetMaxCurrent(float64)
	GetMinPower() float64
	GetMaxPower() float64
}

package api

import "time"

//go:generate mockgen -package mock -destination ../mock/mock_api.go github.com/andig/evcc/api Charger,Meter,MeterEnergy,Vehicle,ChargeRater

// ChargeMode are charge modes modeled after OpenWB
type ChargeMode string

// Charge modes
const (
	ModeOff   ChargeMode = "off"
	ModeNow   ChargeMode = "now"
	ModeMinPV ChargeMode = "minpv"
	ModePV    ChargeMode = "pv"
)

// String implements Stringer
func (c ChargeMode) String() string {
	return string(c)
}

// ChargeStatus is the EV's charging status from A to F
type ChargeStatus string

// Charging states
const (
	StatusNone ChargeStatus = ""
	StatusA    ChargeStatus = "A" // Fzg. angeschlossen: nein    Laden aktiv: nein    - Kabel nicht angeschlossen
	StatusB    ChargeStatus = "B" // Fzg. angeschlossen:   ja    Laden aktiv: nein    - Kabel angeschlossen
	StatusC    ChargeStatus = "C" // Fzg. angeschlossen:   ja    Laden aktiv:   ja    - Laden
	StatusD    ChargeStatus = "D" // Fzg. angeschlossen:   ja    Laden aktiv:   ja    - Laden mit Lüfter
	StatusE    ChargeStatus = "E" // Fzg. angeschlossen:   ja    Laden aktiv: nein    - Fehler (Kurzschluss)
	StatusF    ChargeStatus = "F" // Fzg. angeschlossen:   ja    Laden aktiv: nein    - Fehler (Ausfall Wallbox)
)

// String implements Stringer
func (c ChargeStatus) String() string {
	return string(c)
}

// Meter is able to provide current power in W
type Meter interface {
	CurrentPower() (float64, error)
}

// MeterEnergy is able to provide current energy in kWh
type MeterEnergy interface {
	TotalEnergy() (float64, error)
}

// MeterCurrent is able to provide per-line current A
type MeterCurrent interface {
	Currents() (float64, float64, float64, error)
}

// Battery is able to provide battery SoC in %
type Battery interface {
	SoC() (float64, error)
}

// ChargeState provides current charging status
type ChargeState interface {
	Status() (ChargeStatus, error)
}

// Charger is able to provide current charging status and enable/disable charging
type Charger interface {
	ChargeState
	Enabled() (bool, error)
	Enable(enable bool) error
	MaxCurrent(current int64) error
}

// ChargerEx provides milli-amp precision charger current control
type ChargerEx interface {
	MaxCurrentMillis(current float64) error
}

// Diagnosis is a helper interface that allows to dump diagnostic data to console
type Diagnosis interface {
	Diagnose()
}

// ChargeTimer provides current charge cycle duration
type ChargeTimer interface {
	ChargingTime() (time.Duration, error)
}

// ChargeRater provides charged energy amount in kWh
type ChargeRater interface {
	ChargedEnergy() (float64, error)
}

// Identifier identifies a vehicle and is implemented by the charger
type Identifier interface {
	Identify() (string, error)
}

// Vehicle represents the EV and it's battery
type Vehicle interface {
	Battery
	Identifier
	Title() string
	Capacity() int64
}

// VehicleFinishTimer provides estimated charge cycle finish time
type VehicleFinishTimer interface {
	FinishTime() (time.Time, error)
}

// VehicleRange provides the vehicles remaining km range
type VehicleRange interface {
	Range() (int64, error)
}

// VehicleClimater provides climatisation data
type VehicleClimater interface {
	Climater() (active bool, outsideTemp float64, targetTemp float64, err error)
}

// VehicleStartCharge starts the charging session on the vehicle side
type VehicleStartCharge interface {
	StartCharge() error
}

// VehicleStopCharge stops the charging session on the vehicle side
type VehicleStopCharge interface {
	StopCharge() error
}

type Tariff interface {
	IsCheap() bool
}

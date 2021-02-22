package charger

import (
	"encoding/binary"
	"fmt"
	"time"

	"github.com/andig/evcc/api"
	"github.com/andig/evcc/util"
	"github.com/andig/evcc/util/modbus"
)

const (
	phETHRegStatus     = 100 // Input
	phETHRegChargeTime = 102 // Input
	phETHRegMaxCurrent = 300 // Holding
	phETHRegEnable     = 400 // Coil

	phETHRegPower  = 120 // power reading
	phETHRegEnergy = 128 // energy reading
)

var phETHRegCurrents = []uint16{114, 116, 118} // current readings

// PhoenixETH is an api.ChargeController implementation for Phoenix Contact ETH (Ethernet) controllers.
// It uses Modbus/TCP to communicate with the controller at modbus client id 180 or 255 (default).
type PhoenixETH struct {
	conn *modbus.Connection
}

func init() {
	registry.Add("phoenix-eth", NewPhoenixETHFromConfig)
}

//go:generate go run ../cmd/tools/decorate.go -p charger -f decoratePhoenixETH -o phoenix-eth_decorators -b *PhoenixETH -r api.Charger -t "api.Meter,CurrentPower,func() (float64, error)" -t "api.MeterEnergy,TotalEnergy,func() (float64, error)" -t "api.MeterCurrent,Currents,func() (float64, float64, float64, error)"

// NewPhoenixETHFromConfig creates a Phoenix charger from generic config
func NewPhoenixETHFromConfig(other map[string]interface{}) (api.Charger, error) {
	cc := struct {
		URI   string
		ID    uint8
		Meter struct {
			Power, Energy, Currents bool
		}
	}{
		URI: "192.168.0.8:502", // default
		ID:  255,               // default
	}

	if err := util.DecodeOther(other, &cc); err != nil {
		return nil, err
	}

	wb, err := NewPhoenixETH(cc.URI, cc.ID)

	var currentPower func() (float64, error)
	if cc.Meter.Power {
		currentPower = wb.currentPower
	}

	var totalEnergy func() (float64, error)
	if cc.Meter.Energy {
		totalEnergy = wb.totalEnergy
	}

	var currents func() (float64, float64, float64, error)
	if cc.Meter.Currents {
		currents = wb.currents
	}

	return decoratePhoenixETH(wb, currentPower, totalEnergy, currents), err
}

// NewPhoenixETH creates a Phoenix charger
func NewPhoenixETH(uri string, id uint8) (*PhoenixETH, error) {
	conn, err := modbus.NewConnection(uri, "", "", 0, false, id)
	if err != nil {
		return nil, err
	}

	log := util.NewLogger("phoenix-eth")
	conn.Logger(log.TRACE)

	wb := &PhoenixETH{
		conn: conn,
	}

	return wb, nil
}

// Status implements the Charger.Status interface
func (wb *PhoenixETH) Status() (api.ChargeStatus, error) {
	b, err := wb.conn.ReadInputRegisters(phETHRegStatus, 1)
	if err != nil {
		return api.StatusNone, err
	}

	return api.ChargeStatus(string(b[1])), nil
}

// Enabled implements the Charger.Enabled interface
func (wb *PhoenixETH) Enabled() (bool, error) {
	b, err := wb.conn.ReadCoils(phETHRegEnable, 1)
	if err != nil {
		return false, err
	}

	return b[0] == 1, nil
}

// Enable implements the Charger.Enable interface
func (wb *PhoenixETH) Enable(enable bool) error {
	var u uint16
	if enable {
		u = 0xFF00
	}

	_, err := wb.conn.WriteSingleCoil(phETHRegEnable, u)

	return err
}

// MaxCurrent implements the Charger.MaxCurrent interface
func (wb *PhoenixETH) MaxCurrent(current int64) error {
	if current < 6 {
		return fmt.Errorf("invalid current %d", current)
	}

	_, err := wb.conn.WriteSingleRegister(phETHRegMaxCurrent, uint16(current))

	return err
}

// ChargingTime yields current charge run duration
func (wb *PhoenixETH) ChargingTime() (time.Duration, error) {
	b, err := wb.conn.ReadInputRegisters(phETHRegChargeTime, 2)
	if err != nil {
		return 0, err
	}

	// 2 words, least significant word first
	secs := uint64(b[3])<<16 | uint64(b[2])<<24 | uint64(b[1]) | uint64(b[0])<<8
	return time.Duration(time.Duration(secs) * time.Second), nil
}

func (wb *PhoenixETH) decodeReading(b []byte) float64 {
	v := binary.BigEndian.Uint32(b)
	return float64(v)
}

// CurrentPower implements the Meter.CurrentPower interface
func (wb *PhoenixETH) currentPower() (float64, error) {
	b, err := wb.conn.ReadInputRegisters(phETHRegPower, 2)
	if err != nil {
		return 0, err
	}

	return wb.decodeReading(b), err
}

// totalEnergy implements the Meter.TotalEnergy interface
func (wb *PhoenixETH) totalEnergy() (float64, error) {
	b, err := wb.conn.ReadInputRegisters(phETHRegEnergy, 2)
	if err != nil {
		return 0, err
	}

	return wb.decodeReading(b), err
}

// currents implements the Meter.Currents interface
func (wb *PhoenixETH) currents() (float64, float64, float64, error) {
	var currents []float64
	for _, regCurrent := range phETHRegCurrents {
		b, err := wb.conn.ReadInputRegisters(regCurrent, 2)
		if err != nil {
			return 0, 0, 0, err
		}

		currents = append(currents, wb.decodeReading(b))
	}

	return currents[0], currents[1], currents[2], nil
}

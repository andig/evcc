package meter

import (
	"errors"
	"fmt"
	"time"

	"github.com/andig/evcc/util"
	"github.com/andig/evcc/util/modbus"
	"github.com/volkszaehler/mbmd/meters"
	"github.com/volkszaehler/mbmd/meters/rs485"
	"github.com/volkszaehler/mbmd/meters/sunspec"
)

func init() {
	registry.Add("modbus", "ModBus", new(modbusMeter))
}

// modbusMeter is an api.Meter implementation with configurable getters and setters.
type modbusMeter struct {
	modbus.Settings `mapstructure:",squash"`

	Model   string
	Power   string `default:"Power"`
	Energy  string
	SoCConf string `mapstructure:"soc"`
	Timeout time.Duration

	log      *util.Logger
	conn     *modbus.Connection
	device   meters.Device
	opPower  modbus.Operation
	opEnergy modbus.Operation
	opSoC    modbus.Operation
}

func (m *modbusMeter) Connect() error {
	// assume RTU if not set and this is a known RS485 meter model
	if m.RTU == nil {
		b := modbus.IsRS485(m.Model)
		m.RTU = &b
	}

	m.log = util.NewLogger("modbus")

	var err error
	m.conn, err = modbus.NewConnection(m.URI, m.Device, m.Comset, m.Baudrate, *m.RTU, m.ID)
	if err != nil {
		return err
	}

	// set non-default timeout
	if m.Timeout > 0 {
		m.conn.Timeout(m.Timeout)
	}

	m.conn.Logger(m.log.TRACE)

	// prepare device
	m.device, err = modbus.NewDevice(m.Model, m.SubDevice)
	if err != nil {
		return err
	}

	err = m.device.Initialize(m.conn)

	// silence Kostal implementation errors
	if errors.Is(err, meters.ErrPartiallyOpened) {
		err = nil
	}

	if err != nil {
		return err
	}

	m.Power = modbus.ReadingName(m.Power)
	if err := modbus.ParseOperation(m.device, m.Power, &m.opPower); err != nil {
		return fmt.Errorf("invalid measurement for power: %s", m.Power)
	}

	if m.Energy != "" {
		m.Energy = modbus.ReadingName(m.Energy)
		if err := modbus.ParseOperation(m.device, m.Energy, &m.opEnergy); err != nil {
			return fmt.Errorf("invalid measurement for energy: %s", m.Energy)
		}
	}

	// decorate energy reading
	if m.SoCConf != "" {
		m.SoCConf = modbus.ReadingName(m.SoCConf)
		if err := modbus.ParseOperation(m.device, m.SoCConf, &m.opSoC); err != nil {
			return fmt.Errorf("invalid measurement for soc: %s", m.SoCConf)
		}
	}
	return nil
}

// floatGetter executes configured modbus read operation and implements func() (float64, error)
func (m *modbusMeter) floatGetter(op modbus.Operation) (float64, error) {
	var res meters.MeasurementResult
	var err error

	if dev, ok := m.device.(*rs485.RS485); ok {
		res, err = dev.QueryOp(m.conn, op.MBMD)
	}

	if dev, ok := m.device.(*sunspec.SunSpec); ok {
		if op.MBMD.IEC61850 != 0 {
			res, err = dev.QueryOp(m.conn, op.MBMD.IEC61850)
		} else {
			res, err = dev.QueryPoint(
				m.conn,
				op.SunSpec.Model,
				op.SunSpec.Block,
				op.SunSpec.Point,
			)
		}
	}

	// silence NaN reading errors by assuming zero
	if err != nil && errors.Is(err, meters.ErrNaN) {
		res.Value = 0
		err = nil
	}

	if err == nil {
		m.log.TRACE.Printf("%+v", res)
	}

	return res.Value, err
}

// CurrentPower implements the api.Meter interface
func (m *modbusMeter) CurrentPower() (float64, error) {
	return m.floatGetter(m.opPower)
}

// TotalEnergy implements the api.MeterEnergy interface
func (m *modbusMeter) TotalEnergy() (float64, error) {
	return m.floatGetter(m.opEnergy)
}

// HasEnergy implements the api.OptionalMeterEnergy interface
func (m *modbusMeter) HasEnergy() bool {
	return m.Energy != ""
}

// SoC implements the api.Battery interface
func (m *modbusMeter) SoC() (float64, error) {
	return m.floatGetter(m.opSoC)
}

// HasSoC implements the api.OptionalBattery interface
func (m *modbusMeter) HasSoC() bool {
	return m.SoCConf != ""
}

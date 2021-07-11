package modbus

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/andig/evcc/util"
	"github.com/volkszaehler/mbmd/meters"
	"github.com/volkszaehler/mbmd/meters/rs485"
	"github.com/volkszaehler/mbmd/meters/sunspec"
)

// WriteSingleRegister 16-bit wise write access
const WriteSingleRegister = 6 // modbus.FuncCodeWriteSingleRegister

// Settings contains the ModBus settings
type Settings struct {
	ID        uint8 `default:"1" validate:"required"`
	SubDevice int
	URI       string
	Device    string
	Comset    string `validate:"oneof=8E1 8N1"`
	Baudrate  int
	RTU       *bool // indicates RTU over TCP if true
}

// Connection decorates a meters.Connection with transparent slave id and error handling
type Connection struct {
	slaveID uint8
	conn    meters.Connection
	delay   time.Duration
}

func (mb *Connection) prepare() {
	mb.conn.Slave(mb.slaveID)
	if mb.delay > 0 {
		time.Sleep(mb.delay)
	}
}

func (mb *Connection) handle(res []byte, err error) ([]byte, error) {
	if err != nil {
		mb.conn.Close()
	}
	return res, err
}

// Delay sets delay so use between subsequent modbus operations
func (mb *Connection) Delay(delay time.Duration) {
	mb.delay = delay
}

// Logger sets logger implementation
func (mb *Connection) Logger(logger meters.Logger) {
	mb.conn.Logger(logger)
}

// Timeout sets the connection timeout (not idle timeout)
func (mb *Connection) Timeout(timeout time.Duration) {
	mb.conn.Timeout(timeout)
}

// ReadCoils wraps the underlying implementation
func (mb *Connection) ReadCoils(address, quantity uint16) ([]byte, error) {
	mb.prepare()
	return mb.handle(mb.conn.ModbusClient().ReadCoils(address, quantity))
}

// WriteSingleCoil wraps the underlying implementation
func (mb *Connection) WriteSingleCoil(address, quantity uint16) ([]byte, error) {
	mb.prepare()
	return mb.handle(mb.conn.ModbusClient().WriteSingleCoil(address, quantity))
}

// ReadInputRegisters wraps the underlying implementation
func (mb *Connection) ReadInputRegisters(address, quantity uint16) ([]byte, error) {
	mb.prepare()
	return mb.handle(mb.conn.ModbusClient().ReadInputRegisters(address, quantity))
}

// ReadHoldingRegisters wraps the underlying implementation
func (mb *Connection) ReadHoldingRegisters(address, quantity uint16) ([]byte, error) {
	mb.prepare()
	return mb.handle(mb.conn.ModbusClient().ReadHoldingRegisters(address, quantity))
}

// WriteSingleRegister wraps the underlying implementation
func (mb *Connection) WriteSingleRegister(address, value uint16) ([]byte, error) {
	mb.prepare()
	return mb.handle(mb.conn.ModbusClient().WriteSingleRegister(address, value))
}

// WriteMultipleRegisters wraps the underlying implementation
func (mb *Connection) WriteMultipleRegisters(address, quantity uint16, value []byte) ([]byte, error) {
	mb.prepare()
	return mb.handle(mb.conn.ModbusClient().WriteMultipleRegisters(address, quantity, value))
}

// ReadDiscreteInputs wraps the underlying implementation
func (mb *Connection) ReadDiscreteInputs(address, quantity uint16) (results []byte, err error) {
	mb.prepare()
	return mb.handle(mb.conn.ModbusClient().ReadDiscreteInputs(address, quantity))
}

// WriteMultipleCoils wraps the underlying implementation
func (mb *Connection) WriteMultipleCoils(address, quantity uint16, value []byte) (results []byte, err error) {
	mb.prepare()
	return mb.handle(mb.conn.ModbusClient().WriteMultipleCoils(address, quantity, value))
}

// ReadWriteMultipleRegisters wraps the underlying implementation
func (mb *Connection) ReadWriteMultipleRegisters(readAddress, readQuantity, writeAddress, writeQuantity uint16, value []byte) (results []byte, err error) {
	mb.prepare()
	return mb.handle(mb.conn.ModbusClient().ReadWriteMultipleRegisters(readAddress, readQuantity, writeAddress, writeQuantity, value))
}

// MaskWriteRegister wraps the underlying implementation
func (mb *Connection) MaskWriteRegister(address, andMask, orMask uint16) (results []byte, err error) {
	mb.prepare()
	return mb.handle(mb.conn.ModbusClient().MaskWriteRegister(address, andMask, orMask))
}

// ReadFIFOQueue wraps the underlying implementation
func (mb *Connection) ReadFIFOQueue(address uint16) (results []byte, err error) {
	mb.prepare()
	return mb.handle(mb.conn.ModbusClient().ReadFIFOQueue(address))
}

var connections = make(map[string]meters.Connection)

func registeredConnection(key string, newConn meters.Connection) meters.Connection {
	if conn, ok := connections[key]; ok {
		return conn
	}

	connections[key] = newConn

	return newConn
}

// NewConnection creates physical modbus device from config
func NewConnection(uri, device, comset string, baudrate int, rtu bool, slaveID uint8) (*Connection, error) {
	var conn meters.Connection

	if device != "" && uri != "" {
		return nil, errors.New("invalid modbus configuration: can only have either uri or device")
	}

	if device != "" {
		if baudrate == 0 || comset == "" {
			return nil, errors.New("invalid modbus configuration: need baudrate and comset")
		}
		conn = registeredConnection(device, meters.NewRTU(device, baudrate, comset))
	}

	if uri != "" {
		uri = util.DefaultPort(uri, 502)

		if rtu {
			conn = registeredConnection(uri, meters.NewRTUOverTCP(uri))
		} else {
			conn = registeredConnection(uri, meters.NewTCP(uri))
		}
	}

	if conn == nil {
		return nil, errors.New("invalid modbus configuration: need either uri or device")
	}

	slaveConn := &Connection{
		slaveID: slaveID,
		conn:    conn,
	}

	return slaveConn, nil
}

// NewDevice creates physical modbus device from config
func NewDevice(model string, subdevice int) (device meters.Device, err error) {
	if IsRS485(model) {
		device, err = rs485.NewDevice(strings.ToUpper(model))
	} else {
		device = sunspec.NewDevice(strings.ToUpper(model), subdevice)
	}

	if device == nil {
		err = errors.New("invalid modbus configuration: need either uri or device")
	}

	return device, err
}

// IsRS485 determines if model is a known MBMD rs485 device model
func IsRS485(model string) bool {
	for k := range rs485.Producers {
		if strings.ToUpper(model) == k {
			return true
		}
	}
	return false
}

// RS485FindDeviceOp checks is RS485 device supports operation
func RS485FindDeviceOp(device *rs485.RS485, measurement meters.Measurement) (op rs485.Operation, err error) {
	ops := device.Producer().Produce()

	for _, op := range ops {
		if op.IEC61850 == measurement {
			return op, nil
		}
	}

	return op, fmt.Errorf("unsupported measurement: %s", measurement.String())
}

// Register contains the ModBus register configuration
type Register struct {
	Address uint16 // Length  uint16
	Type    string
	Decode  string
}

// RegisterOperation creates a read operation from a register definition
func RegisterOperation(r Register) (rs485.Operation, error) {
	op := rs485.Operation{
		OpCode:  r.Address,
		ReadLen: 2,
	}

	switch strings.ToLower(r.Type) {
	case "holding":
		op.FuncCode = rs485.ReadHoldingReg
	case "input":
		op.FuncCode = rs485.ReadInputReg
	case "writesingle":
		op.FuncCode = WriteSingleRegister // modbus.FuncCodeWriteSingleRegister
	default:
		return rs485.Operation{}, fmt.Errorf("invalid register type: %s", r.Type)
	}

	switch strings.ToLower(r.Decode) {
	case "float32", "ieee754":
		op.Transform = rs485.RTUIeee754ToFloat64
	case "float32s", "ieee754s":
		op.Transform = rs485.RTUIeee754ToFloat64Swapped
	case "float64":
		op.Transform = rs485.RTUUint64ToFloat64
		op.ReadLen = 4
	case "uint16":
		op.Transform = rs485.RTUUint16ToFloat64
		op.ReadLen = 1
	case "uint32":
		op.Transform = rs485.RTUUint32ToFloat64
	case "uint32s":
		op.Transform = rs485.RTUUint32ToFloat64Swapped
	case "uint64":
		op.Transform = rs485.RTUUint64ToFloat64
		op.ReadLen = 4
	case "int16":
		op.Transform = rs485.RTUInt16ToFloat64
		op.ReadLen = 1
	case "int32":
		op.Transform = rs485.RTUInt32ToFloat64
	case "int32s":
		op.Transform = rs485.RTUInt32ToFloat64Swapped
	default:
		return rs485.Operation{}, fmt.Errorf("invalid register decoding: %s", r.Decode)
	}

	return op, nil
}

// SunSpecOperation is a sunspec modbus operation
type SunSpecOperation struct {
	Model, Block int
	Point        string
}

// ParsePoint parses sunspec point from string
func ParsePoint(selector string) (model int, block int, point string, err error) {
	err = fmt.Errorf("invalid point: %s", selector)

	el := strings.Split(selector, ":")
	if len(el) < 2 || len(el) > 3 {
		return
	}

	if model, err = strconv.Atoi(el[0]); err != nil {
		return
	}

	if len(el) == 3 {
		// block is the middle element
		if block, err = strconv.Atoi(el[1]); err != nil {
			return
		}
	}

	point = el[len(el)-1]

	return model, block, point, nil
}

// Operation is a register-based or sunspec modbus operation
type Operation struct {
	MBMD    rs485.Operation
	SunSpec SunSpecOperation
}

// ParseOperation parses an MBMD measurement or SunsSpec point definition into a modbus operation
func ParseOperation(dev meters.Device, measurement string, op *Operation) (err error) {
	// if measurement cannot be parsed it could be SunSpec model/block/point
	if op.MBMD.IEC61850, err = meters.MeasurementString(measurement); err != nil {
		op.SunSpec.Model, op.SunSpec.Block, op.SunSpec.Point, err = ParsePoint(measurement)
		return err
	}

	// for RS485 check if producer supports the measurement
	if dev, ok := dev.(*rs485.RS485); ok {
		op.MBMD, err = RS485FindDeviceOp(dev, op.MBMD.IEC61850)
	}

	return err
}

package sma

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/andig/evcc/util"
	"gitlab.com/bboehmke/sunny"
)

const udpTimeout = 10 * time.Second

// map of created discover instances
var discoverers = make(map[string]*Discoverer)
var discoverersMutex sync.Mutex

// initialize sunny logger only once
var once sync.Once

// GetDiscoverer fo the given interface
func GetDiscoverer(iface string) (*Discoverer, error) {
	// on time initialization of sunny logger
	log := util.NewLogger("sma")
	once.Do(func() {
		sunny.Log = log.TRACE
	})

	discoverersMutex.Lock()
	defer discoverersMutex.Unlock()

	// get or create discoverer
	discoverer, ok := discoverers[iface]
	if !ok {
		conn, err := sunny.NewConnection(iface)
		if err != nil {
			return nil, fmt.Errorf("connection failed: %w", err)
		}

		discoverer = &Discoverer{
			log:     log,
			conn:    conn,
			devices: make(map[uint32]*Device),
		}

		go discoverer.run()

		discoverers[iface] = discoverer
	}
	return discoverer, nil
}

// Discoverer discovers SMA devicesBySerial in background while providing already found devicesBySerial
type Discoverer struct {
	log     *util.Logger
	conn    *sunny.Connection
	devices map[uint32]*Device
	mux     sync.RWMutex
	done    uint32
}

func (d *Discoverer) createDevice(device *sunny.Device) *Device {
	return &Device{
		Device: device,
		log:    d.log,
		mux:    util.NewWaiter(udpTimeout, func() { d.log.TRACE.Println("wait for initial value") }),
		values: make(map[sunny.ValueID]interface{}),
	}
}

func (d *Discoverer) addDevice(device *sunny.Device) {
	d.mux.Lock()
	defer d.mux.Unlock()

	if _, ok := d.devices[device.SerialNumber()]; !ok {
		d.devices[device.SerialNumber()] = d.createDevice(device)
	} else {
		device.Close()
	}
}

// run discover and store found devicesBySerial
func (d *Discoverer) run() {
	devices := make(chan *sunny.Device)

	go func() {
		for device := range devices {
			d.addDevice(device)
		}
	}()

	// discover devicesBySerial and wait for results
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	d.conn.DiscoverDevices(ctx, devices, "")
	cancel()
	close(devices)

	// mark discover as done
	atomic.AddUint32(&d.done, 1)
}

func (d *Discoverer) get(serial uint32, password string) *Device {
	d.mux.RLock()
	defer d.mux.RUnlock()

	device := d.devices[serial]
	if device != nil {
		device.SetPassword(password)
	}
	return device
}

// DeviceBySerial with the given serial number
func (d *Discoverer) DeviceBySerial(serial uint32, password string) *Device {
	start := time.Now()
	for time.Since(start) < time.Second*3 {
		// discover done -> return immediately regardless of result
		if atomic.LoadUint32(&d.done) != 0 {
			return d.get(serial, password)
		}

		// device with serial found -> return
		if device := d.get(serial, password); device != nil {
			return device
		}

		time.Sleep(time.Millisecond * 10)
	}
	return d.get(serial, password)
}

// DeviceByIP with the given serial number
func (d *Discoverer) DeviceByIP(ip, password string) (*Device, error) {
	d.mux.Lock()
	defer d.mux.Unlock()

	for _, device := range d.devices {
		if device.Address().IP.String() == ip {
			device.SetPassword(password)
			return device, nil
		}
	}

	device, err := d.conn.NewDevice(ip, password)
	if err != nil {
		return nil, fmt.Errorf("failed to get device: %w", err)
	}

	dev := d.createDevice(device)
	d.devices[device.SerialNumber()] = dev
	return dev, err
}

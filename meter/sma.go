package meter

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"text/tabwriter"

	"github.com/andig/evcc/api"
	"github.com/andig/evcc/provider/sma"
	"github.com/andig/evcc/util"
	"gitlab.com/bboehmke/sunny"
)

func init() {
	registry.Add("sma", "SMA/Speedwire device (Home Manager/Energy Meter/Inverter)", new(smaMeter))
}

// smaMeter supporting SMA Home Manager 2.0, SMA Energy Meter 30 and SMA inverter
type smaMeter struct {
	URI       string  `validate:"required_without=Serial"`
	Interface string  `label:"Name of network interface device is connected to"`
	Password  string  `default:"0000" meta:"secret"`
	Serial    uint32  `validate:"required_without=URI"`
	Power     string  `meta:"hide"`
	Energy    string  `meta:"hide"`
	Scale     float64 `default:"1"` // power only

	hasSoc bool
	log    *util.Logger
	device *sma.Device
}

func (sm *smaMeter) Connect() error {
	sm.log = util.NewLogger("sma")

	if sm.Power != "" || sm.Energy != "" {
		util.NewLogger("sma").WARN.Println("energy and power setting are deprecated and will be removed in a future release")
	}

	discoverer, err := sma.GetDiscoverer(sm.Interface)
	if err != nil {
		return fmt.Errorf("failed to get discoverer failed: %w", err)
	}

	switch {
	case sm.URI != "":
		sm.device, err = discoverer.DeviceByIP(sm.URI, sm.Password)
		if err != nil {
			return err
		}

	case sm.Serial > 0:
		sm.device = discoverer.DeviceBySerial(sm.Serial, sm.Password)
		if sm.device == nil {
			return fmt.Errorf("device not found: %d", sm.Serial)
		}

	default:
		return errors.New("missing uri or serial")
	}
	// start update loop manually to get values as fast as possible
	sm.device.StartUpdateLoop()

	// decorate api.Battery in case of inverter
	if !sm.device.IsEnergyMeter() {
		vals, err := sm.device.Values()
		if err != nil {
			return err
		}

		if _, ok := vals[sunny.BatteryCharge]; ok {
			sm.hasSoc = true
		}
	}

	return nil
}

// CurrentPower implements the api.Meter interface
func (sm *smaMeter) CurrentPower() (float64, error) {
	values, err := sm.device.Values()
	return sm.Scale * (sma.AsFloat(values[sunny.ActivePowerPlus]) - sma.AsFloat(values[sunny.ActivePowerMinus])), err
}

var _ api.MeterEnergy = (*smaMeter)(nil)

// TotalEnergy implements the api.MeterEnergy interface
func (sm *smaMeter) TotalEnergy() (float64, error) {
	values, err := sm.device.Values()
	return sma.AsFloat(values[sunny.ActiveEnergyPlus]) / 3600000, err
}

var _ api.MeterCurrent = (*smaMeter)(nil)

// Currents implements the api.MeterCurrent interface
func (sm *smaMeter) Currents() (float64, float64, float64, error) {
	values, err := sm.device.Values()

	var currents [3]float64
	for i, id := range []sunny.ValueID{sunny.CurrentL1, sunny.CurrentL2, sunny.CurrentL3} {
		currents[i] = sma.AsFloat(values[id])
	}

	return currents[0], currents[1], currents[2], err
}

// SoC implements the api.Battery interface
func (sm *smaMeter) SoC() (float64, error) {
	values, err := sm.device.Values()
	return sma.AsFloat(values[sunny.BatteryCharge]), err
}

// HasSoC implements the api.OptionalBattery interface
func (sm *smaMeter) HasSoC() bool {
	return sm.hasSoc
}

var _ api.Diagnosis = (*smaMeter)(nil)

// Diagnose implements the api.Diagnosis interface
func (sm *smaMeter) Diagnose() {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', 0)

	fmt.Fprintf(w, "  IP:\t%s\n", sm.device.Address())
	fmt.Fprintf(w, "  Serial:\t%d\n", sm.device.SerialNumber())
	fmt.Fprintf(w, "  EnergyMeter:\t%v\n", sm.device.IsEnergyMeter())
	fmt.Fprintln(w)

	if name, err := sm.device.GetDeviceName(); err == nil {
		fmt.Fprintf(w, "  Name:\t%s\n", name)
	}

	if devClass, err := sm.device.GetDeviceClass(); err == nil {
		fmt.Fprintf(w, "  Device Class:\t0x%X\n", devClass)
	}
	fmt.Fprintln(w)

	if values, err := sm.device.Values(); err == nil {
		ids := make([]sunny.ValueID, 0, len(values))
		for k := range values {
			ids = append(ids, k)
		}

		sort.Slice(ids, func(i, j int) bool {
			return ids[i].String() < ids[j].String()
		})

		for _, id := range ids {
			switch values[id].(type) {
			case float64:
				fmt.Fprintf(w, "  %s:\t%f %s\n", id.String(), values[id], sunny.GetValueInfo(id).Unit)
			default:
				fmt.Fprintf(w, "  %s:\t%v %s\n", id.String(), values[id], sunny.GetValueInfo(id).Unit)
			}
		}
	}
	w.Flush()
}

package core

import (
	"errors"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/andig/evcc/api"
	"github.com/andig/evcc/push"
	"github.com/andig/evcc/util"
	"github.com/avast/retry-go"
)

//go:generate mockgen -package mock -destination ../mock/mock_loadpoint.go github.com/andig/evcc/core Updater

// Updater abstracts the LoadPoint implementation for testing
type Updater interface {
	Update(float64, bool)
}

// Site is the main configuration container. A site can host multiple loadpoints.
type Site struct {
	uiChan       chan<- util.Param // client push messages
	lpUpdateChan chan *LoadPoint

	*Health

	sync.Mutex
	log *util.Logger

	// configuration
	Title         string       `mapstructure:"title"`         // UI title
	Voltage       float64      `mapstructure:"voltage"`       // Operating voltage. 230V for Germany.
	ResidualPower float64      `mapstructure:"residualPower"` // PV meter only: household usage. Grid meter: household safety margin
	Meters        MetersConfig // Meter references
	PrioritySoC   float64      `mapstructure:"prioritySoC"` // prefer battery up to this SoC

	// meters
	gridMeter    api.Meter // Grid usage meter
	pvMeter      api.Meter // PV generation meter
	batteryMeter api.Meter // Battery charging meter

	tariff     api.Tariff   // Tariff
	loadpoints []*LoadPoint // Loadpoints

	// cached state
	gridPower    float64 // Grid power
	pvPower      float64 // PV power
	batteryPower float64 // Battery charge power
}

// MetersConfig contains the loadpoint's meter configuration
type MetersConfig struct {
	GridMeterRef    string `mapstructure:"grid"`    // Grid usage meter reference
	PVMeterRef      string `mapstructure:"pv"`      // PV generation meter reference
	BatteryMeterRef string `mapstructure:"battery"` // Battery charging meter reference
}

// NewSiteFromConfig creates a new site
func NewSiteFromConfig(
	log *util.Logger,
	cp configProvider,
	other map[string]interface{},
	loadpoints []*LoadPoint,
	tariff api.Tariff,
) (*Site, error) {
	site := NewSite()
	if err := util.DecodeOther(other, &site); err != nil {
		return nil, err
	}

	Voltage = site.Voltage
	site.tariff = tariff
	site.loadpoints = loadpoints

	if site.Meters.GridMeterRef != "" {
		site.gridMeter = cp.Meter(site.Meters.GridMeterRef)
	}
	if site.Meters.PVMeterRef != "" {
		site.pvMeter = cp.Meter(site.Meters.PVMeterRef)
	}
	if site.Meters.BatteryMeterRef != "" {
		site.batteryMeter = cp.Meter(site.Meters.BatteryMeterRef)
	}

	// configure meter from references
	if site.gridMeter == nil && site.pvMeter == nil {
		return nil, errors.New("missing either grid or pv meter")
	}

	return site, nil
}

// NewSite creates a Site with sane defaults
func NewSite() *Site {
	lp := &Site{
		log:     util.NewLogger("site"),
		Health:  NewHealth(60 * time.Second),
		Voltage: 230, // V
	}

	return lp
}

// LoadPoints returns the array of associated loadpoints
func (site *Site) LoadPoints() []LoadPointAPI {
	res := make([]LoadPointAPI, len(site.loadpoints))
	for id, lp := range site.loadpoints {
		res[id] = lp
	}
	return res
}

func meterCapabilities(name string, meter interface{}) string {
	_, power := meter.(api.Meter)
	_, energy := meter.(api.MeterEnergy)
	_, currents := meter.(api.MeterCurrent)

	name += ":"
	return fmt.Sprintf("    %-8s power %s energy %s currents %s",
		name,
		presence[power],
		presence[energy],
		presence[currents],
	)
}

// DumpConfig site configuration
func (site *Site) DumpConfig() {
	site.publish("siteTitle", site.Title)

	site.log.INFO.Println("site config:")
	site.log.INFO.Printf("  meters:    grid %s pv %s battery %s",
		presence[site.gridMeter != nil],
		presence[site.pvMeter != nil],
		presence[site.batteryMeter != nil],
	)

	site.publish("gridConfigured", site.gridMeter != nil)
	if site.gridMeter != nil {
		site.log.INFO.Println(meterCapabilities("grid", site.gridMeter))
	}

	site.publish("pvConfigured", site.pvMeter != nil)
	if site.pvMeter != nil {
		site.log.INFO.Println(meterCapabilities("pv", site.pvMeter))
	}

	site.publish("batteryConfigured", site.batteryMeter != nil)
	if site.batteryMeter != nil {
		_, ok := site.batteryMeter.(api.Battery)
		site.log.INFO.Println(
			meterCapabilities("battery", site.batteryMeter),
			fmt.Sprintf("soc %s", presence[ok]),
		)

		if ok {
			site.publish("prioritySoC", site.PrioritySoC)
		}
	}

	for i, lp := range site.loadpoints {
		lp.log.INFO.Printf("loadpoint %d:", i+1)

		lp.log.INFO.Printf("  mode:      %s", lp.GetMode())

		_, power := lp.charger.(api.Meter)
		_, energy := lp.charger.(api.MeterEnergy)
		_, currents := lp.charger.(api.MeterCurrent)
		_, timer := lp.charger.(api.ChargeTimer)

		lp.log.INFO.Printf("  charger:   power %s energy %s currents %s timer %s",
			presence[power],
			presence[energy],
			presence[currents],
			presence[timer],
		)

		lp.log.INFO.Printf("  meters:    charge %s", presence[lp.HasChargeMeter()])

		lp.publish("chargeConfigured", lp.HasChargeMeter())
		if lp.HasChargeMeter() {
			lp.log.INFO.Printf(meterCapabilities("charge", lp.chargeMeter))
		}

		lp.log.INFO.Printf("  vehicles:  %s", presence[len(lp.vehicles) > 0])

		for i, v := range lp.vehicles {
			_, rng := v.(api.VehicleRange)
			_, finish := v.(api.VehicleFinishTimer)
			_, status := v.(api.ChargeState)
			_, climate := v.(api.VehicleClimater)
			lp.log.INFO.Printf("    car %d:   range %s finish %s status %s climate %s",
				i, presence[rng], presence[finish], presence[status], presence[climate],
			)
		}
	}
}

// publish sends values to UI and databases
func (site *Site) publish(key string, val interface{}) {
	// test helper
	if site.uiChan == nil {
		return
	}

	site.uiChan <- util.Param{
		Key: key,
		Val: val,
	}
}

// updateMeter updates and publishes single meter
func (site *Site) updateMeter(name string, meter api.Meter, power *float64) error {
	value, err := meter.CurrentPower()
	if err != nil {
		return err
	}

	*power = value // update value if no error

	site.log.DEBUG.Printf("%s power: %.0fW", name, *power)
	site.publish(name+"Power", *power)

	return nil
}

// updateMeter updates and publishes single meter
func (site *Site) updateMeters() error {
	retryMeter := func(s string, m api.Meter, f *float64) error {
		if m == nil {
			return nil
		}

		err := retry.Do(func() error {
			return site.updateMeter(s, m, f)
		}, retryOptions...)

		if err != nil {
			err = fmt.Errorf("updating %s meter: %v", s, err)
			site.log.ERROR.Println(err)
		}

		return err
	}

	// pv meter is not critical for operation
	_ = retryMeter("pv", site.pvMeter, &site.pvPower)

	err := retryMeter("grid", site.gridMeter, &site.gridPower)
	if err == nil {
		err = retryMeter("battery", site.batteryMeter, &site.batteryPower)
	}

	// currents
	if phaseMeter, ok := site.gridMeter.(api.MeterCurrent); err == nil && ok {
		i1, i2, i3, err := phaseMeter.Currents()
		if err == nil {
			site.log.TRACE.Printf("grid currents: %.3gA", []float64{i1, i2, i3})
			site.publish("gridCurrents", []float64{i1, i2, i3})
		}
	}

	// allow using PV as estimate for grid power
	if site.gridMeter == nil {
		site.gridPower = -site.pvPower

		for _, lp := range site.loadpoints {
			site.gridPower += lp.GetChargePower()
		}
	}

	return err
}

// consumedPower estimates how much power the charger might have consumed given it was the only load
// func (site *Site) consumedPower() float64 {
// 	return consumedPower(lp.pvPower, lp.batteryPower, lp.gridPower)
// }

// sitePower returns the net power exported by the site minus a residual margin.
// negative values mean grid: export, battery: charging
func (site *Site) sitePower() (float64, error) {
	if err := site.updateMeters(); err != nil {
		return 0, err
	}

	// honour battery priority
	batteryPower := site.batteryPower
	if battery, ok := site.batteryMeter.(api.Battery); ok {
		soc, err := battery.SoC()
		if err != nil {
			site.log.ERROR.Printf("updating battery soc: %v", err)
		} else {
			site.log.DEBUG.Printf("battery soc: %.0f%%", soc)
			site.publish("batterySoC", math.Trunc(soc))

			site.Lock()
			defer site.Unlock()

			// if battery is charging give it priority
			if soc < site.PrioritySoC && batteryPower < 0 {
				site.log.DEBUG.Printf("giving priority to battery at soc: %.0f", soc)
				batteryPower = 0
			}
		}
	}

	sitePower := sitePower(site.gridPower, batteryPower, site.ResidualPower)
	site.log.DEBUG.Printf("site power: %.0fW", sitePower)

	return sitePower, nil
}

func (site *Site) update(lp Updater) {
	site.log.DEBUG.Println("----")

	var cheap bool
	if site.tariff != nil {
		cheap = site.tariff.IsCheap()
	}

	if sitePower, err := site.sitePower(); err == nil {
		lp.Update(sitePower, cheap)
		site.Health.Update()
	}
}

// Prepare attaches communication channels to site and loadpoints
func (site *Site) Prepare(uiChan chan<- util.Param, pushChan chan<- push.Event) {
	site.uiChan = uiChan
	site.lpUpdateChan = make(chan *LoadPoint, 1) // 1 capacity to avoid deadlock

	for id, lp := range site.loadpoints {
		lpUIChan := make(chan util.Param)
		lpPushChan := make(chan push.Event)

		// pipe messages through go func to add id
		go func(id int) {
			for {
				select {
				case param := <-lpUIChan:
					param.LoadPoint = &id
					uiChan <- param
				case ev := <-lpPushChan:
					ev.LoadPoint = &id
					pushChan <- ev
				}
			}
		}(id)

		lp.Prepare(lpUIChan, lpPushChan, site.lpUpdateChan)
	}
}

// loopLoadpoints keeps iterating across loadpoints sending the next to the given channel
func (site *Site) loopLoadpoints(next chan<- Updater) {
	for {
		for _, lp := range site.loadpoints {
			next <- lp
		}
	}
}

// Run is the main control loop. It reacts to trigger events by
// updating measurements and executing control logic.
func (site *Site) Run(stopC chan struct{}, interval time.Duration) {
	loadpointChan := make(chan Updater)
	go site.loopLoadpoints(loadpointChan)

	ticker := time.NewTicker(interval)
	site.update(<-loadpointChan) // start immediately

	for {
		select {
		case <-ticker.C:
			site.update(<-loadpointChan)
		case lp := <-site.lpUpdateChan:
			site.update(lp)
		case <-stopC:
			return
		}
	}
}

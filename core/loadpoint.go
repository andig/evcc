package core

import (
	"errors"
	"math"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/andig/evcc/api"
	"github.com/andig/evcc/core/soc"
	"github.com/andig/evcc/core/wrapper"
	"github.com/andig/evcc/provider"
	"github.com/andig/evcc/push"
	"github.com/andig/evcc/util"

	evbus "github.com/asaskevich/EventBus"
	"github.com/avast/retry-go"
	"github.com/benbjohnson/clock"
)

const (
	evChargeStart       = "start"      // update chargeTimer
	evChargeStop        = "stop"       // update chargeTimer
	evChargeCurrent     = "current"    // update fakeChargeMeter
	evChargePower       = "power"      // update chargeRater
	evVehicleConnect    = "connect"    // vehicle connected
	evVehicleDisconnect = "disconnect" // vehicle disconnected

	minActiveCurrent = 1.0 // minimum current at which a phase is treated as active
)

// PollConfig defines the vehicle polling mode and interval
type PollConfig struct {
	Mode     string        `mapstructure:"mode"`     // polling mode charging (default), connected, always
	Interval time.Duration `mapstructure:"interval"` // interval when not charging
}

// SoCConfig defines soc settings, estimation and update behaviour
type SoCConfig struct {
	Poll         PollConfig `mapstructure:"poll"`
	AlwaysUpdate bool       `mapstructure:"alwaysUpdate"`
	Estimate     bool       `mapstructure:"estimate"`
	Min          int        `mapstructure:"min"`    // Default minimum SoC, guarded by mutex
	Target       int        `mapstructure:"target"` // Default target SoC, guarded by mutex
	Levels       []int      `mapstructure:"levels"` // deprecated
}

// Poll modes
const (
	pollCharging  = "charging"
	pollConnected = "connected"
	pollAlways    = "always"

	pollInterval = 60 * time.Minute
)

// ThresholdConfig defines enable/disable hysteresis parameters
type ThresholdConfig struct {
	Delay     time.Duration
	Threshold float64
}

// LoadPoint is responsible for controlling charge depending on
// SoC needs and power availability.
type LoadPoint struct {
	clock    clock.Clock       // mockable time
	bus      evbus.Bus         // event bus
	pushChan chan<- push.Event // notifications
	uiChan   chan<- util.Param // client push messages
	lpChan   chan<- *LoadPoint // update requests
	log      *util.Logger

	// exposed public configuration
	sync.Mutex                // guard status
	Mode       api.ChargeMode `mapstructure:"mode"` // Charge mode, guarded by mutex

	Title       string   `mapstructure:"title"`    // UI title
	Phases      int64    `mapstructure:"phases"`   // Phases- required for converting power and current
	ChargerRef  string   `mapstructure:"charger"`  // Charger reference
	VehicleRef  string   `mapstructure:"vehicle"`  // Vehicle reference
	VehiclesRef []string `mapstructure:"vehicles"` // Vehicles reference
	Meters      struct {
		ChargeMeterRef string `mapstructure:"charge"` // Charge meter reference
	}
	SoC          SoCConfig
	OnDisconnect struct {
		Mode      api.ChargeMode `mapstructure:"mode"`      // Charge mode to apply when car disconnected
		TargetSoC int            `mapstructure:"targetSoC"` // Target SoC to apply when car disconnected
	}
	Enable, Disable ThresholdConfig

	MinCurrent    float64       // PV mode: start current	Min+PV mode: min current
	MaxCurrent    float64       // Max allowed current. Physically ensured by the charger
	GuardDuration time.Duration // charger enable/disable minimum holding time

	enabled       bool      // Charger enabled state
	chargeCurrent float64   // Charger current limit
	guardUpdated  time.Time // Charger enabled/disabled timestamp
	socUpdated    time.Time // SoC updated timestamp (poll: connected)

	charger     api.Charger
	chargeTimer api.ChargeTimer
	chargeRater api.ChargeRater

	chargeMeter    api.Meter     // Charger usage meter
	vehicle        api.Vehicle   // Currently active vehicle
	vehicles       []api.Vehicle // Assigned vehicles
	socEstimator   *soc.Estimator
	socTimer       *soc.Timer
	vehicleIdError error // state of last vehicle identification

	// cached state
	status         api.ChargeStatus // Charger status
	remoteDemand   RemoteDemand     // External status demand
	chargePower    float64          // Charging power
	chargeCurrents []float64        // Phase currents
	connectedTime  time.Time        // Time when vehicle was connected
	pvTimer        time.Time        // PV enabled/disable timer

	socCharge      float64       // Vehicle SoC
	chargedEnergy  float64       // Charged energy while connected in Wh
	chargeDuration time.Duration // Charge duration
}

// NewLoadPointFromConfig creates a new loadpoint
func NewLoadPointFromConfig(log *util.Logger, cp configProvider, other map[string]interface{}) (*LoadPoint, error) {
	lp := NewLoadPoint(log)
	if err := util.DecodeOther(other, &lp); err != nil {
		return nil, err
	}

	// set sane defaults
	lp.Mode = api.ChargeModeString(string(lp.Mode))
	lp.OnDisconnect.Mode = api.ChargeModeString(string(lp.OnDisconnect.Mode))

	// set vehicle polling mode
	switch lp.SoC.Poll.Mode = strings.ToLower(lp.SoC.Poll.Mode); lp.SoC.Poll.Mode {
	case pollCharging:
	case pollConnected, pollAlways:
		log.WARN.Printf("poll mode '%s' may deplete your battery or lead to API misuse. USE AT YOUR OWN RISK.", lp.SoC.Poll)
	default:
		if lp.SoC.Poll.Mode != "" {
			log.WARN.Printf("invalid poll mode: %s", lp.SoC.Poll.Mode)
		}
		if lp.SoC.AlwaysUpdate {
			log.WARN.Println("alwaysUpdate is deprecated and will be removed in a future release. Use poll instead.")
		} else {
			lp.SoC.Poll.Mode = pollConnected
		}
	}

	// set vehicle polling interval
	if lp.SoC.Poll.Interval < pollInterval {
		if lp.SoC.Poll.Interval == 0 {
			lp.SoC.Poll.Interval = pollInterval
		} else {
			log.WARN.Printf("poll interval '%v' is lower than %v and may deplete your battery or lead to API misuse. USE AT YOUR OWN RISK.", lp.SoC.Poll.Interval, pollInterval)
		}
	}

	if len(lp.SoC.Levels) > 0 {
		log.WARN.Printf("soc.levels are deprecated and will be removed in an upcoming release")
	}

	if lp.SoC.Target == 0 {
		lp.SoC.Target = lp.OnDisconnect.TargetSoC // use disconnect value as default soc
		if lp.SoC.Target == 0 {
			lp.SoC.Target = 100
		}
	}

	if lp.MinCurrent == 0 {
		log.WARN.Println("minCurrent must not be zero")
	}

	if lp.MaxCurrent <= lp.MinCurrent {
		log.WARN.Println("maxCurrent must be larger than minCurrent")
	}

	if lp.Meters.ChargeMeterRef != "" {
		lp.chargeMeter = cp.Meter(lp.Meters.ChargeMeterRef)
	}

	// multiple vehicles
	for _, ref := range lp.VehiclesRef {
		vehicle := cp.Vehicle(ref)
		lp.vehicles = append(lp.vehicles, vehicle)
	}

	// single vehicle
	if lp.VehicleRef != "" {
		vehicle := cp.Vehicle(lp.VehicleRef)
		lp.vehicles = append(lp.vehicles, vehicle)
	}

	if lp.ChargerRef == "" {
		return nil, errors.New("missing charger")
	}
	lp.charger = cp.Charger(lp.ChargerRef)
	lp.configureChargerType(lp.charger)

	// allow target charge handler to access loadpoint
	lp.socTimer = soc.NewTimer(lp.log, lp.adapter(), lp.MaxCurrent)
	if lp.Enable.Threshold > lp.Disable.Threshold {
		log.WARN.Printf("PV mode enable threshold (%.0fW) is larger than disable threshold (%.0fW)", lp.Enable.Threshold, lp.Disable.Threshold)
	}

	return lp, nil
}

// NewLoadPoint creates a LoadPoint with sane defaults
func NewLoadPoint(log *util.Logger) *LoadPoint {
	clock := clock.New()
	bus := evbus.New()

	lp := &LoadPoint{
		log:           log,   // logger
		clock:         clock, // mockable time
		bus:           bus,   // event bus
		Mode:          api.ModeOff,
		Phases:        1,
		status:        api.StatusNone,
		MinCurrent:    6,  // A
		MaxCurrent:    16, // A
		GuardDuration: 5 * time.Minute,
	}

	return lp
}

// requestUpdate requests site to update this loadpoint
func (lp *LoadPoint) requestUpdate() {
	select {
	case lp.lpChan <- lp: // request loadpoint update
	default:
	}
}

// configureChargerType ensures that chargeMeter, Rate and Timer can use charger capabilities
func (lp *LoadPoint) configureChargerType(charger api.Charger) {
	// ensure charge meter exists
	if lp.chargeMeter == nil {
		if mt, ok := charger.(api.Meter); ok {
			lp.chargeMeter = mt
		} else {
			mt := &wrapper.ChargeMeter{}
			_ = lp.bus.Subscribe(evChargeCurrent, lp.evChargeCurrentWrappedMeterHandler)
			_ = lp.bus.Subscribe(evChargeStop, func() { mt.SetPower(0) })
			lp.chargeMeter = mt
		}
	}

	// ensure charge rater exists
	if rt, ok := charger.(api.ChargeRater); ok {
		lp.chargeRater = rt
	} else {
		rt := wrapper.NewChargeRater(lp.log, lp.chargeMeter)
		_ = lp.bus.Subscribe(evChargePower, rt.SetChargePower)
		_ = lp.bus.Subscribe(evVehicleConnect, func() { rt.StartCharge(false) })
		_ = lp.bus.Subscribe(evChargeStart, func() { rt.StartCharge(true) })
		_ = lp.bus.Subscribe(evChargeStop, rt.StopCharge)
		lp.chargeRater = rt
	}

	// ensure charge timer exists
	if ct, ok := charger.(api.ChargeTimer); ok {
		lp.chargeTimer = ct
	} else {
		ct := wrapper.NewChargeTimer()
		_ = lp.bus.Subscribe(evVehicleConnect, func() { ct.StartCharge(false) })
		_ = lp.bus.Subscribe(evChargeStart, func() { ct.StartCharge(true) })
		_ = lp.bus.Subscribe(evChargeStop, ct.StopCharge)
		lp.chargeTimer = ct
	}
}

// pushEvent sends push messages to clients
func (lp *LoadPoint) pushEvent(event string) {
	lp.pushChan <- push.Event{Event: event}
}

// publish sends values to UI and databases
func (lp *LoadPoint) publish(key string, val interface{}) {
	if lp.uiChan != nil {
		lp.uiChan <- util.Param{Key: key, Val: val}
	}
}

// evChargeStartHandler sends external start event
func (lp *LoadPoint) evChargeStartHandler() {
	lp.log.INFO.Println("start charging ->")
	lp.pushEvent(evChargeStart)

	// soc update reset
	lp.socUpdated = time.Time{}
}

// evChargeStopHandler sends external stop event
func (lp *LoadPoint) evChargeStopHandler() {
	lp.log.INFO.Println("stop charging <-")
	lp.pushEvent(evChargeStop)

	// soc update reset
	lp.socUpdated = time.Time{}
}

// evVehicleConnectHandler sends external start event
func (lp *LoadPoint) evVehicleConnectHandler() {
	lp.log.INFO.Printf("car connected")

	// energy
	lp.chargedEnergy = 0
	lp.publish("chargedEnergy", lp.chargedEnergy)

	// duration
	lp.connectedTime = lp.clock.Now()
	lp.publish("connectedDuration", time.Duration(0))

	// soc update reset
	lp.socUpdated = time.Time{}

	// soc update reset on car change
	if lp.socEstimator != nil {
		lp.socEstimator.Reset()
	}

	// flush all vehicles before updating state
	provider.ResetCached()

	// identify active vehicle
	lp.findActiveVehicle()

	// immediately allow pv mode activity
	lp.pvDisableTimer()

	lp.pushEvent(evVehicleConnect)
}

// evVehicleDisconnectHandler sends external start event
func (lp *LoadPoint) evVehicleDisconnectHandler() {
	lp.log.INFO.Println("car disconnected")

	// energy and duration
	lp.publish("chargedEnergy", lp.chargedEnergy)
	lp.publish("connectedDuration", lp.clock.Since(lp.connectedTime))

	lp.pushEvent(evVehicleDisconnect)

	// set default mode on disconnect
	if lp.OnDisconnect.Mode != "" && lp.GetMode() != api.ModeOff {
		lp.SetMode(lp.OnDisconnect.Mode)
	}
	if lp.OnDisconnect.TargetSoC != 0 {
		_ = lp.SetTargetSoC(lp.OnDisconnect.TargetSoC)
	}

	// soc update reset
	lp.socUpdated = time.Time{}
}

// evChargeCurrentHandler publishes the charge current
func (lp *LoadPoint) evChargeCurrentHandler(current float64) {
	if !lp.enabled {
		current = 0
	}
	lp.publish("chargeCurrent", current)
}

// evChargeCurrentWrappedMeterHandler updates the dummy charge meter's charge power.
// This simplifies the main flow where the charge meter can always be treated as present.
// It assumes that the charge meter cannot consume more than total household consumption.
// If physical charge meter is present this handler is not used.
// The actual value is published by the evChargeCurrentHandler
func (lp *LoadPoint) evChargeCurrentWrappedMeterHandler(current float64) {
	power := current * float64(lp.Phases) * Voltage

	if !lp.enabled || lp.GetStatus() != api.StatusC {
		// if disabled we cannot be charging
		power = 0
	}

	// handler only called if charge meter was replaced by dummy
	lp.chargeMeter.(*wrapper.ChargeMeter).SetPower(power)
}

// Name returns the human-readable loadpoint title
func (lp *LoadPoint) Name() string {
	return lp.Title
}

// Prepare loadpoint configuration by adding missing helper elements
func (lp *LoadPoint) Prepare(uiChan chan<- util.Param, pushChan chan<- push.Event, lpChan chan<- *LoadPoint) {
	lp.uiChan = uiChan
	lp.pushChan = pushChan
	lp.lpChan = lpChan

	// event handlers
	_ = lp.bus.Subscribe(evChargeStart, lp.evChargeStartHandler)
	_ = lp.bus.Subscribe(evChargeStop, lp.evChargeStopHandler)
	_ = lp.bus.Subscribe(evVehicleConnect, lp.evVehicleConnectHandler)
	_ = lp.bus.Subscribe(evVehicleDisconnect, lp.evVehicleDisconnectHandler)
	_ = lp.bus.Subscribe(evChargeCurrent, lp.evChargeCurrentHandler)

	// publish initial values
	lp.publish("title", lp.Title)
	lp.publish("minCurrent", lp.MinCurrent)
	lp.publish("maxCurrent", lp.MaxCurrent)
	lp.publish("phases", lp.Phases)
	lp.publish("activePhases", lp.Phases)
	lp.publish("hasVehicle", len(lp.vehicles) > 0)

	lp.Lock()
	lp.publish("mode", lp.Mode)
	lp.publish("targetSoC", lp.SoC.Target)
	lp.publish("minSoC", lp.SoC.Min)
	lp.Unlock()

	// use first vehicle for estimator
	// run during prepare() to ensure cache has been attached
	if len(lp.vehicles) > 0 {
		lp.setActiveVehicle(lp.vehicles[0])
		lp.findActiveVehicle()
	}

	// read initial charger state to prevent immediately disabling charger
	if enabled, err := lp.charger.Enabled(); err == nil {
		if lp.enabled = enabled; enabled {
			lp.guardUpdated = lp.clock.Now()
			// set defined current for use by pv mode
			_ = lp.setLimit(lp.GetMinCurrent(), false)
		}
	} else {
		lp.log.ERROR.Printf("charger: %v", err)
	}

	// allow charger to  access loadpoint
	if ctrl, ok := lp.charger.(LoadpointController); ok {
		ctrl.LoadpointControl(lp)
	}
}

// syncCharger updates charger status and synchronizes it with expectations
func (lp *LoadPoint) syncCharger() {
	enabled, err := lp.charger.Enabled()
	if err == nil {
		if enabled != lp.enabled {
			lp.log.WARN.Printf("charger out of sync: expected %vd, got %vd", status[lp.enabled], status[enabled])
			err = lp.charger.Enable(lp.enabled)
		}

		if !enabled && lp.GetStatus() == api.StatusC {
			lp.log.WARN.Println("charger logic error: disabled but charging")
		}
	}

	if err != nil {
		lp.log.ERROR.Printf("charger: %v", err)
	}
}

// setLimit applies charger current limits and enables/disables accordingly
func (lp *LoadPoint) setLimit(chargeCurrent float64, force bool) (err error) {
	// set current
	if chargeCurrent != lp.chargeCurrent && chargeCurrent >= lp.GetMinCurrent() {
		if charger, ok := lp.charger.(api.ChargerEx); ok {
			lp.log.DEBUG.Printf("max charge current: %.2g", chargeCurrent)
			err = charger.MaxCurrentMillis(chargeCurrent)
		} else {
			lp.log.DEBUG.Printf("max charge current: %d", int64(chargeCurrent))
			err = lp.charger.MaxCurrent(int64(chargeCurrent))
		}

		if err == nil {
			lp.chargeCurrent = chargeCurrent
			lp.bus.Publish(evChargeCurrent, chargeCurrent)
		} else {
			lp.log.ERROR.Printf("max charge current %.2g: %v", chargeCurrent, err)
		}
	}

	// set enabled
	if enabled := chargeCurrent >= lp.GetMinCurrent(); enabled != lp.enabled && err == nil {
		if remaining := (lp.GuardDuration - lp.clock.Since(lp.guardUpdated)).Truncate(time.Second); remaining > 0 && !force {
			lp.log.DEBUG.Printf("charger %s: contactor delay %v", status[enabled], remaining)
			return nil
		}

		// sleep vehicle
		if car, ok := lp.vehicle.(api.VehicleStopCharge); !enabled && ok {
			if err := car.StopCharge(); err != nil {
				lp.log.ERROR.Printf("vehicle remote charge stop: %v", err)
			}
		}

		lp.log.DEBUG.Printf("charger %s", status[enabled])
		if err = lp.charger.Enable(enabled); err == nil {
			lp.enabled = enabled
			lp.guardUpdated = lp.clock.Now()

			lp.bus.Publish(evChargeCurrent, chargeCurrent)

			// wake up vehicle
			if car, ok := lp.vehicle.(api.VehicleStartCharge); enabled && ok {
				if err := car.StartCharge(); err != nil {
					lp.log.ERROR.Printf("vehicle remote charge start: %v", err)
				}
			}
		} else {
			lp.log.ERROR.Printf("charger %s: %v", status[enabled], err)
		}
	}

	return err
}

// connected returns the EVs connection state
func (lp *LoadPoint) connected() bool {
	status := lp.GetStatus()
	return status == api.StatusB || status == api.StatusC
}

// charging returns the EVs charging state
func (lp *LoadPoint) charging() bool {
	return lp.GetStatus() == api.StatusC
}

// charging returns the EVs charging state
func (lp *LoadPoint) setStatus(status api.ChargeStatus) {
	lp.Lock()
	defer lp.Unlock()
	lp.status = status
}

// targetSocReached checks if target is configured and reached.
// If vehicle is not configured this will always return false
func (lp *LoadPoint) targetSocReached() bool {
	return lp.vehicle != nil &&
		lp.SoC.Target > 0 &&
		lp.SoC.Target < 100 &&
		lp.socCharge >= float64(lp.SoC.Target)
}

// minSocNotReached checks if minimum is configured and not reached.
// If vehicle is not configured this will always return true
func (lp *LoadPoint) minSocNotReached() bool {
	return lp.vehicle != nil &&
		lp.SoC.Min > 0 &&
		lp.socCharge < float64(lp.SoC.Min)
}

// climateActive checks if vehicle has active climate request
func (lp *LoadPoint) climateActive() bool {
	if cl, ok := lp.vehicle.(api.VehicleClimater); ok {
		active, outsideTemp, targetTemp, err := cl.Climater()
		if err == nil {
			lp.log.DEBUG.Printf("climater active: %v, target temp: %.1f°C, outside temp: %.1f°C", active, targetTemp, outsideTemp)

			status := "off"
			if active {
				status = "on"

				switch {
				case outsideTemp < targetTemp:
					status = "heating"
				case outsideTemp > targetTemp:
					status = "cooling"
				}
			}

			lp.publish("climater", status)
			return active
		}

		if !errors.Is(err, api.ErrNotAvailable) {
			lp.log.ERROR.Printf("climater: %v", err)
		}
	}

	return false
}

// remoteControlled returns true if remote control status is active
func (lp *LoadPoint) remoteControlled(demand RemoteDemand) bool {
	lp.Lock()
	defer lp.Unlock()

	return lp.remoteDemand == demand
}

// setActiveVehicle assigns currently active vehicle and configures soc estimator
func (lp *LoadPoint) setActiveVehicle(vehicle api.Vehicle) {
	if lp.vehicle != nil {
		lp.log.INFO.Printf("vehicle updated: %s -> %s", lp.vehicle.Title(), vehicle.Title())
	}

	// update successful
	lp.vehicleIdError = nil

	lp.vehicle = vehicle
	lp.socEstimator = soc.NewEstimator(lp.log, vehicle, lp.SoC.Estimate)

	lp.publish("socTitle", lp.vehicle.Title())
	lp.publish("socCapacity", lp.vehicle.Capacity())
}

// vehicleIdentificationAllowed returns true if active vehicle has not yet been identified
func (lp *LoadPoint) vehicleIdentificationAllowed() bool {
	return errors.Is(lp.vehicleIdError, api.ErrMustRetry)
}

// findActiveVehicle validates if the active vehicle is still connected to the loadpoint
func (lp *LoadPoint) findActiveVehicle() {
	// find vehicles by id
	if identifier, ok := lp.charger.(api.Identifier); ok {
		id, err := identifier.Identify()

		if err != nil {
			lp.vehicleIdError = err
			lp.log.ERROR.Println("charger vehicle id:", err)
			return
		}

		if id != "" {
			lp.log.DEBUG.Println("charger vehicle id:", id)

			// find exact match
			for _, vehicle := range lp.vehicles {
				if vid, err := vehicle.Identify(); err == nil && vid == id {
					lp.setActiveVehicle(vehicle)
					return
				}
			}

			// find placeholder match
			for _, vehicle := range lp.vehicles {
				if vid, err := vehicle.Identify(); err == nil {
					re, err := regexp.Compile(strings.ReplaceAll(vid, "*", ".*?"))
					if err != nil {
						lp.log.ERROR.Printf("vehicle identity: %v", err)
						continue
					}

					if re.MatchString(id) {
						lp.setActiveVehicle(vehicle)
						return
					}
				}
			}

			// TODO implement removing vehicle
			// lp.setActiveVehicle(nil)
		}
	}

	if len(lp.vehicles) <= 1 {
		return
	}

	// find vehicles by charge state - current vehicle
	if vs, ok := lp.vehicle.(api.ChargeState); ok {
		status, err := vs.Status()

		if err != nil {
			lp.vehicleIdError = err
			lp.log.ERROR.Println("vehicle charge state:", err)
			return
		}

		lp.log.DEBUG.Printf("vehicle status: %s (%s)", status, lp.vehicle.Title())

		// vehicle is plugged or charging, so it should be the right one
		if status == api.StatusB || status == api.StatusC {
			lp.vehicleIdError = nil
			return
		}
	}

	// find vehicles by charge state
	for _, vehicle := range lp.vehicles {
		if vehicle == lp.vehicle {
			continue
		}

		if vs, ok := vehicle.(api.ChargeState); ok {
			status, err := vs.Status()

			if err != nil {
				lp.vehicleIdError = err
				lp.log.ERROR.Println("vehicle charge state:", err)
				return
			}

			lp.log.DEBUG.Printf("vehicle status: %s (%s)", status, vehicle.Title())

			// vehicle is plugged or charging, so it should be the right one
			if status == api.StatusB || status == api.StatusC {
				lp.setActiveVehicle(vehicle)
				return
			}
		}
	}
}

// updateChargerStatus updates charger status and detects car connected/disconnected events
func (lp *LoadPoint) updateChargerStatus() error {
	status, err := lp.charger.Status()
	if err != nil {
		return err
	}

	lp.log.DEBUG.Printf("charger status: %s", status)

	if prevStatus := lp.GetStatus(); status != prevStatus {
		lp.setStatus(status)

		// changed from empty (initial startup) - set connected without sending message
		if prevStatus == api.StatusNone {
			lp.connectedTime = lp.clock.Now()
			lp.publish("connectedDuration", time.Duration(0))
		}

		// changed from A - connected
		if prevStatus == api.StatusA {
			lp.bus.Publish(evVehicleConnect)
		}

		// changed to C - start/stop charging cycle - handle before disconnect to update energy
		if lp.charging() {
			lp.bus.Publish(evChargeStart)
		} else if prevStatus == api.StatusC {
			lp.bus.Publish(evChargeStop)
		}

		// changed to A - disconnected
		if status == api.StatusA {
			lp.bus.Publish(evVehicleDisconnect)
		}

		// update whenever there is a state change
		lp.bus.Publish(evChargeCurrent, lp.chargeCurrent)
	}

	return nil
}

// effectiveCurrent returns the currently effective charging current
// it does not take measured currents into account
func (lp *LoadPoint) effectiveCurrent() float64 {
	// use measured L1 current
	if lp.chargeCurrents != nil {
		return lp.chargeCurrents[0]
	}

	if lp.GetStatus() != api.StatusC {
		return 0
	}

	return lp.chargeCurrent
}

// pvDisableTimer puts the pv enable/disable timer into elapsed state
func (lp *LoadPoint) pvDisableTimer() {
	lp.pvTimer = time.Now().Add(-lp.Disable.Delay)
}

// pvMaxCurrent calculates the maximum target current for PV mode
func (lp *LoadPoint) pvMaxCurrent(mode api.ChargeMode, sitePower float64) float64 {
	// calculate target charge current from delta power and actual current
	effectiveCurrent := lp.effectiveCurrent()
	deltaCurrent := powerToCurrent(-sitePower, lp.Phases)
	targetCurrent := math.Max(math.Min(effectiveCurrent+deltaCurrent, lp.GetMaxCurrent()), 0)

	lp.log.DEBUG.Printf("max charge current: %.1fA = %.1fA + %.1fA (%.0fW @ %dp)", targetCurrent, effectiveCurrent, deltaCurrent, sitePower, lp.Phases)

	// in MinPV mode return at least minCurrent
	minCurrent := lp.GetMinCurrent()
	if mode == api.ModeMinPV && targetCurrent < minCurrent {
		return minCurrent
	}

	// read only once to simplify testing
	if mode == api.ModePV && lp.enabled && targetCurrent < minCurrent {
		// kick off disable sequence
		if sitePower >= lp.Disable.Threshold {
			lp.log.DEBUG.Printf("site power %.0fW >= disable threshold %.0fW", sitePower, lp.Disable.Threshold)

			if lp.pvTimer.IsZero() {
				lp.log.DEBUG.Printf("start pv disable timer: %v", lp.Disable.Delay)
				lp.pvTimer = lp.clock.Now()
			}

			elapsed := lp.clock.Since(lp.pvTimer)
			if elapsed >= lp.Disable.Delay {
				lp.log.DEBUG.Println("pv disable timer elapsed")
				return 0
			}

			lp.log.DEBUG.Printf("pv disable timer remaining: %v", (lp.Disable.Delay - elapsed).Round(time.Second))
		} else {
			// reset timer
			lp.log.DEBUG.Printf("reset pv disable timer: %v", lp.Disable.Delay)
			lp.pvTimer = lp.clock.Now()
		}

		lp.log.DEBUG.Println("pv enable timer: keep enabled")
		return minCurrent
	}

	if mode == api.ModePV && !lp.enabled {
		// kick off enable sequence
		if (lp.Enable.Threshold == 0 && targetCurrent >= minCurrent) ||
			(lp.Enable.Threshold != 0 && sitePower <= lp.Enable.Threshold) {
			lp.log.DEBUG.Printf("site power %.0fW < enable threshold %.0fW", sitePower, lp.Enable.Threshold)

			if lp.pvTimer.IsZero() {
				lp.log.DEBUG.Printf("start pv enable timer: %v", lp.Enable.Delay)
				lp.pvTimer = lp.clock.Now()
			}

			elapsed := lp.clock.Since(lp.pvTimer)
			if elapsed >= lp.Enable.Delay {
				lp.log.DEBUG.Println("pv enable timer elapsed")
				return minCurrent
			}

			lp.log.DEBUG.Printf("pv enable timer remaining: %v", (lp.Enable.Delay - elapsed).Round(time.Second))
		} else {
			// reset timer
			lp.log.DEBUG.Printf("reset pv enable timer: %v", lp.Enable.Delay)
			lp.pvTimer = lp.clock.Now()
		}

		lp.log.DEBUG.Println("pv enable timer: keep disabled")
		return 0
	}

	// reset timer to disabled state
	lp.log.DEBUG.Printf("pv timer reset")
	lp.pvTimer = time.Time{}

	return targetCurrent
}

// updateChargePower updates charge meter power
func (lp *LoadPoint) updateChargePower() {
	err := retry.Do(func() error {
		value, err := lp.chargeMeter.CurrentPower()
		if err != nil {
			return err
		}

		lp.chargePower = value // update value if no error
		lp.log.DEBUG.Printf("charge power: %.0fW", value)
		lp.publish("chargePower", value)

		return nil
	}, retryOptions...)

	if err != nil {
		lp.log.ERROR.Printf("charge meter: %v", err)
	}
}

// updateChargeCurrents uses MeterCurrent interface to count phases with current >=1A
func (lp *LoadPoint) updateChargeCurrents() {
	lp.chargeCurrents = nil
	phaseMeter, ok := lp.chargeMeter.(api.MeterCurrent)
	if !ok {
		return
	}

	i1, i2, i3, err := phaseMeter.Currents()
	if err != nil {
		lp.log.ERROR.Printf("charge meter: %v", err)
		return
	}

	lp.chargeCurrents = []float64{i1, i2, i3}
	lp.log.DEBUG.Printf("charge currents: %.3gA", lp.chargeCurrents)
	lp.publish("chargeCurrents", lp.chargeCurrents)

	if lp.charging() {
		var phases int64
		for _, i := range lp.chargeCurrents {
			if i >= minActiveCurrent {
				phases++
			}
		}

		if phases > 0 {
			lp.Phases = phases
			lp.log.DEBUG.Printf("detected phases: %dp %.3gA", lp.Phases, lp.chargeCurrents)

			lp.publish("activePhases", lp.Phases)
		}
	}
}

// publish charged energy and duration
func (lp *LoadPoint) publishChargeProgress() {
	if f, err := lp.chargeRater.ChargedEnergy(); err == nil {
		lp.chargedEnergy = 1e3 * f // convert to Wh
	} else {
		lp.log.ERROR.Printf("charge rater: %v", err)
	}

	if d, err := lp.chargeTimer.ChargingTime(); err == nil {
		lp.chargeDuration = d.Round(time.Second)
	} else {
		lp.log.ERROR.Printf("charge timer: %v", err)
	}

	lp.publish("chargedEnergy", lp.chargedEnergy)
	lp.publish("chargeDuration", lp.chargeDuration)
}

// socPollAllowed validates charging state against polling mode
func (lp *LoadPoint) socPollAllowed() bool {
	remaining := lp.SoC.Poll.Interval - lp.clock.Since(lp.socUpdated)

	honourUpdateInterval := lp.SoC.Poll.Mode == pollAlways ||
		lp.SoC.Poll.Mode == pollConnected && lp.connected()

	if honourUpdateInterval && remaining > 0 {
		lp.log.DEBUG.Printf("next soc poll remaining time: %v", remaining.Truncate(time.Second))
	}

	return lp.charging() || honourUpdateInterval && (remaining <= 0) || lp.connected() && lp.socUpdated.IsZero()
}

// publish state of charge, remaining charge duration and range
func (lp *LoadPoint) publishSoCAndRange() {
	if lp.socEstimator == nil {
		return
	}

	if lp.socPollAllowed() {
		lp.socUpdated = lp.clock.Now()

		f, err := lp.socEstimator.SoC(lp.chargedEnergy)
		if err == nil {
			lp.socCharge = math.Trunc(f)
			lp.log.DEBUG.Printf("vehicle soc: %.0f%%", lp.socCharge)
			lp.publish("socCharge", lp.socCharge)

			chargeEstimate := time.Duration(-1)
			if lp.charging() {
				chargeEstimate = lp.socEstimator.RemainingChargeDuration(lp.chargePower, lp.SoC.Target)
			}
			lp.publish("chargeEstimate", chargeEstimate)

			chargeRemainingEnergy := 1e3 * lp.socEstimator.RemainingChargeEnergy(lp.SoC.Target)
			lp.publish("chargeRemainingEnergy", chargeRemainingEnergy)
		} else {
			if errors.Is(err, api.ErrMustRetry) {
				lp.socUpdated = time.Time{}
				lp.log.DEBUG.Printf("vehicle: waiting for update")
			} else {
				lp.log.ERROR.Printf("vehicle: %v", err)
			}
		}

		// range
		if vs, ok := lp.vehicle.(api.VehicleRange); ok {
			if rng, err := vs.Range(); err == nil {
				lp.log.DEBUG.Printf("vehicle range: %vkm", rng)
				lp.publish("range", rng)
			}
		}

		return
	}

	// reset if poll: connected/charging and not connected
	if lp.SoC.Poll.Mode != pollAlways && !lp.connected() {
		lp.publish("socCharge", -1)
		lp.publish("chargeEstimate", time.Duration(-1))

		// range
		lp.publish("range", -1)
	}
}

// Update is the main control function. It reevaluates meters and charger state
func (lp *LoadPoint) Update(sitePower float64, cheap bool) {
	mode := lp.GetMode()
	lp.publish("mode", mode)

	// read and publish meters first
	lp.updateChargePower()
	lp.updateChargeCurrents()

	// update ChargeRater here to make sure initial meter update is caught
	lp.bus.Publish(evChargeCurrent, lp.chargeCurrent)
	lp.bus.Publish(evChargePower, lp.chargePower)

	// update progress and soc before status is updated
	lp.publishChargeProgress()

	// read and publish status
	if err := lp.updateChargerStatus(); err != nil {
		lp.log.ERROR.Printf("charger: %v", err)
		return
	}

	lp.publish("connected", lp.connected())
	lp.publish("charging", lp.charging())
	lp.publish("enabled", lp.enabled)

	// update active vehicle if not yet done
	if lp.vehicleIdentificationAllowed() {
		lp.findActiveVehicle()
	}

	// publish soc after updating charger status to make sure
	// initial update of connected state matches charger status
	lp.publishSoCAndRange()

	// sync settings with charger
	lp.syncCharger()

	// check if car connected and ready for charging
	var err error

	// track if remote disabled is actually active
	remoteDisabled := RemoteEnable

	// execute loading strategy
	switch {
	case !lp.connected():
		// always disable charger if not connected
		// https://github.com/andig/evcc/issues/105
		err = lp.setLimit(0, false)

	case lp.targetSocReached():
		lp.log.DEBUG.Printf("targetSoC reached: %.1f > %d", lp.socCharge, lp.SoC.Target)
		var targetCurrent float64 // zero disables
		if lp.climateActive() {
			lp.log.DEBUG.Println("climater active")
			targetCurrent = lp.GetMinCurrent()
		}
		err = lp.setLimit(targetCurrent, true)
		lp.socTimer.Reset() // once SoC is reached, the target charge request is removed

	// OCPP has priority over target charging
	case lp.remoteControlled(RemoteHardDisable):
		remoteDisabled = RemoteHardDisable
		fallthrough

	case mode == api.ModeOff:
		err = lp.setLimit(0, true)

	case lp.minSocNotReached():
		err = lp.setLimit(lp.GetMaxCurrent(), true)
		lp.pvDisableTimer() // let PV mode disable immediately afterwards

	case mode == api.ModeNow:
		err = lp.setLimit(lp.GetMaxCurrent(), true)

	// target charging
	case lp.socTimer.StartRequired():
		targetCurrent := lp.socTimer.Handle()
		err = lp.setLimit(targetCurrent, false)

	case mode == api.ModeMinPV || mode == api.ModePV:
		targetCurrent := lp.pvMaxCurrent(mode, sitePower)
		lp.log.DEBUG.Printf("pv max charge current: %.3gA", targetCurrent)

		var required bool // false
		if targetCurrent == 0 && lp.climateActive() {
			targetCurrent = lp.GetMaxCurrent()
			required = true
		}

		// tariff
		if cheap {
			targetCurrent = lp.GetMaxCurrent()
			lp.log.DEBUG.Printf("cheap tariff: %.3gA", targetCurrent)
			required = true
		}

		// Sunny Home Manager
		if lp.remoteControlled(RemoteSoftDisable) {
			remoteDisabled = RemoteSoftDisable
			targetCurrent = 0
			required = true
		}

		err = lp.setLimit(targetCurrent, required)
	}

	// effective disabled status
	if remoteDisabled != RemoteEnable {
		lp.publish("remoteDisabled", remoteDisabled)
	}

	if err != nil {
		lp.log.ERROR.Println(err)
	}
}

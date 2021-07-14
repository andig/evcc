package soc

import (
	"math"
	"time"

	"github.com/andig/evcc/util"
)

const (
	deviation = 30 * time.Minute
)

// Adapter provides the required methods for interacting with the loadpoint
type Adapter interface {
	Publish(key string, val interface{})
	Estimator() *Estimator
	ActivePhases() int64
	Voltage() float64
}

// Timer is the target charging handler
type Timer struct {
	Adapter
	log            *util.Logger
	maxCurrent     float64
	current        float64
	SoC            int
	Time           time.Time
	finishAt       time.Time
	chargeRequired bool
}

// NewTimer creates a Timer
func NewTimer(log *util.Logger, adapter Adapter, maxCurrent float64) *Timer {
	lp := &Timer{
		log:        log,
		Adapter:    adapter,
		maxCurrent: maxCurrent,
	}

	return lp
}

// Reset resets the target charging request
func (lp *Timer) Reset() {
	if lp == nil {
		return
	}

	lp.current = float64(lp.maxCurrent)
	lp.Time = time.Time{}
	lp.SoC = 0
}

// StartRequired calculates remaining charge duration and returns true if charge start is required to achieve target soc in time
func (lp *Timer) StartRequired() bool {
	if lp == nil {
		return false
	}

	se := lp.Estimator()
	if !lp.active() || se == nil {
		return false
	}

	power := float64(lp.ActivePhases()) * lp.maxCurrent * lp.Voltage()

	// time
	remainingDuration := se.RemainingChargeDuration(lp.SoC, power, -1)
	lp.finishAt = time.Now().Add(remainingDuration).Round(time.Minute)
	lp.log.DEBUG.Printf("target charging active for %v: projected %v (%v remaining)", lp.Time, lp.finishAt, remainingDuration.Round(time.Minute))

	lp.chargeRequired = lp.finishAt.After(lp.Time)
	lp.Publish("timerActive", lp.chargeRequired)

	return lp.chargeRequired
}

// active returns true if there is an active target charging request
func (lp *Timer) active() bool {
	inactive := lp.Time.IsZero() || lp.Time.Before(time.Now())
	lp.Publish("timerSet", !inactive)

	// reset active
	if inactive && lp.chargeRequired {
		lp.chargeRequired = false
		lp.Publish("timerActive", lp.chargeRequired)
	}

	return !inactive
}

// Handle adjusts current up/down to achieve desired target time taking.
func (lp *Timer) Handle() float64 {
	switch {
	case lp.finishAt.Before(lp.Time.Add(-deviation)):
		lp.current--
		lp.log.DEBUG.Printf("target charging: slowdown")

	case lp.finishAt.After(lp.Time):
		lp.current++
		lp.log.DEBUG.Printf("target charging: speedup")
	}

	lp.current = math.Max(math.Min(lp.current, float64(lp.maxCurrent)), 0)

	return lp.current
}

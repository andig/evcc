package core

import "github.com/andig/evcc/core/soc"

type adapter struct {
	lp *LoadPoint
}

var _ soc.Adapter = (*adapter)(nil)

func (lp *LoadPoint) adapter() soc.Adapter {
	return &adapter{lp: lp}
}

func (a *adapter) Publish(key string, val interface{}) {
	a.lp.publish(key, val)
}

func (a *adapter) Estimator() *soc.Estimator {
	return a.lp.estimator
}

func (a *adapter) ActivePhases() int64 {
	return a.lp.Phases
}

func (a *adapter) Voltage() float64 {
	return Voltage
}

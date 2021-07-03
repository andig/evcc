package core

import (
	"github.com/andig/evcc/core/soc"
)

type adapter struct {
	*LoadPoint
}

func (a *adapter) Publish(key string, val interface{}) {
	a.LoadPoint.publish(key, val)
}

func (a *adapter) SocEstimator() *soc.Estimator {
	return a.LoadPoint.socEstimator
}

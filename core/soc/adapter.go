package soc

import "github.com/andig/evcc/core/loadpoint"

type Adapter interface {
	loadpoint.API
	Publish(key string, val interface{})
	SocEstimator() *Estimator
}

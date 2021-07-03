package metrics

import "github.com/prometheus/client_golang/prometheus"

var (
	PushRegistry = prometheus.NewRegistry()
)

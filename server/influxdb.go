package server

import (
	"fmt"
	"sync"
	"time"

	"github.com/andig/evcc/core"
	"github.com/andig/evcc/util"
	influxdb2 "github.com/influxdata/influxdb-client-go"
)

// Influx is a influx publisher
type Influx struct {
	sync.Mutex
	log      *util.Logger
	client   influxdb2.Client
	org      string
	database string
}

// NewInfluxClient creates new publisher for influx
func NewInfluxClient(url, token, org, user, password, database string) *Influx {
	log := util.NewLogger("iflx")

	if token == "" && user != "" {
		// InfluxDB v1 compatibility
		token = fmt.Sprintf("%s:%s", user, password)
	}

	client := influxdb2.NewClient(url, token)

	return &Influx{
		log:      log,
		client:   client,
		org:      org,
		database: database,
	}
}

// Run Influx publisher
func (m *Influx) Run(in <-chan core.Param) {
	writer := m.client.WriteApi(m.org, m.database)

	// log errors
	go func() {
		for err := range writer.Errors() {
			m.log.ERROR.Println(err)
		}
	}()

	// add points to batch for async writing
	for param := range in {
		// allow nil value to be written as series gaps
		if _, ok := param.Val.(float64); param.Val != nil && !ok {
			continue
		}

		p := influxdb2.NewPoint(
			param.Key,
			map[string]string{
				"loadpoint": param.LoadPoint,
			},
			map[string]interface{}{
				"value": param.Val,
			},
			time.Now(),
		)

		// write asynchronously
		writer.WritePoint(p)
	}

	m.client.Close()
}

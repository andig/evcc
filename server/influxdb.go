package server

import (
	"fmt"
	"sync"
	"time"

	"github.com/andig/evcc/core/loadpoint"
	"github.com/andig/evcc/util"
	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	influxlog "github.com/influxdata/influxdb-client-go/v2/log"
)

// InfluxConfig is the influx db configuration
type InfluxConfig struct {
	URL      string
	Database string
	Token    string
	Org      string
	User     string
	Password string
	Interval time.Duration
}

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
	log := util.NewLogger("influx")

	// InfluxDB v1 compatibility
	if token == "" && user != "" {
		token = fmt.Sprintf("%s:%s", user, password)
	}

	options := influxdb2.DefaultOptions().SetPrecision(time.Second)
	client := influxdb2.NewClientWithOptions(url, token, options)

	// handle error logging in writer
	influxlog.Log = nil

	return &Influx{
		log:      log,
		client:   client,
		org:      org,
		database: database,
	}
}

// supportedType checks if type can be written as influx value
func (m *Influx) supportedType(p util.Param) bool {
	if p.Val == nil {
		return true
	}

	switch val := p.Val.(type) {
	case float64:
		return true
	case [3]float64:
		return true
	case []float64:
		return len(val) == 3
	default:
		return false
	}
}

// Run Influx publisher
func (m *Influx) Run(loadPoints []loadpoint.API, in <-chan util.Param) {
	writer := m.client.WriteAPI(m.org, m.database)

	// log errors
	go func() {
		for err := range writer.Errors() {
			m.log.ERROR.Println(err)
		}
	}()

	// track active vehicle per loadpoint
	vehicles := make(map[int]string)

	// add points to batch for async writing
	for param := range in {
		// vehicle name
		if param.LoadPoint != nil {
			if name, ok := param.Val.(string); ok && param.Key == "socTitle" {
				vehicles[*param.LoadPoint] = name
				continue
			}
		}

		if !m.supportedType(param) {
			continue
		}

		tags := map[string]string{}
		if param.LoadPoint != nil {
			tags["loadpoint"] = loadPoints[*param.LoadPoint].Name()
			tags["vehicle"] = vehicles[*param.LoadPoint]
		}

		fields := map[string]interface{}{}

		// array to slice
		val := param.Val
		if v, ok := val.([3]float64); ok {
			val = v[:]
		}

		// add slice as phase values
		if phases, ok := val.([]float64); ok {
			var total float64
			for i, v := range phases {
				total += v
				fields[fmt.Sprintf("l%d", i+1)] = v
			}

			// add total as "value"
			val = total
		}

		fields["value"] = val

		// write asynchronously
		m.log.TRACE.Printf("write %s=%v (%v)", param.Key, param.Val, tags)
		p := influxdb2.NewPoint(param.Key, tags, fields, time.Now())
		writer.WritePoint(p)
	}

	m.client.Close()
}

package server

import (
	"fmt"
	"strconv"
	"time"

	"github.com/andig/evcc/api"
	"github.com/andig/evcc/core/loadpoint"
	"github.com/andig/evcc/core/site"
	"github.com/andig/evcc/provider/mqtt"
	"github.com/andig/evcc/util"
)

// MQTT is the MQTT server. It uses the MQTT client for publishing.
type MQTT struct {
	Handler *mqtt.Client
	root    string
}

// NewMQTT creates MQTT server
func NewMQTT(root string) *MQTT {
	if root == "" {
		root = "evcc"
	}

	return &MQTT{
		Handler: mqtt.Instance,
		root:    root,
	}
}

func (m *MQTT) encode(v interface{}) string {
	var s string
	switch val := v.(type) {
	case time.Time:
		s = strconv.FormatInt(val.Unix(), 10)
	case time.Duration:
		// must be before stringer to convert to seconds instead of string
		s = fmt.Sprintf("%d", int64(val.Seconds()))
	case fmt.Stringer, string:
		s = fmt.Sprintf("%s", val)
	case float64:
		s = fmt.Sprintf("%.5g", val)
	default:
		s = fmt.Sprintf("%v", val)
	}
	return s
}

func (m *MQTT) publishSingleValue(topic string, retained bool, payload interface{}) {
	token := m.Handler.Client.Publish(topic, m.Handler.Qos, retained, m.encode(payload))
	go m.Handler.WaitForToken(token)
}

func (m *MQTT) publish(topic string, retained bool, payload interface{}) {
	if slice, ok := payload.([]float64); ok && len(slice) == 3 {
		// publish phase values
		var total float64
		for i, v := range slice {
			total += v
			m.publishSingleValue(fmt.Sprintf("%s/l%d", topic, i+1), retained, v)
		}

		// publish sum value
		payload = total
	}

	m.publishSingleValue(topic, retained, payload)
}

func (m *MQTT) listenSetters(topic string, apiHandler loadpoint.API) {
	m.Handler.Listen(topic+"/mode/set", func(payload string) {
		apiHandler.SetMode(api.ChargeMode(payload))
	})
	m.Handler.Listen(topic+"/minSoC/set", func(payload string) {
		soc, err := strconv.Atoi(payload)
		if err == nil {
			_ = apiHandler.SetMinSoC(soc)
		}
	})
	m.Handler.Listen(topic+"/targetSoC/set", func(payload string) {
		soc, err := strconv.Atoi(payload)
		if err == nil {
			_ = apiHandler.SetTargetSoC(soc)
		}
	})
}

// Run starts the MQTT publisher for the MQTT API
func (m *MQTT) Run(site site.API, in <-chan util.Param) {
	// site setters
	m.Handler.Listen(fmt.Sprintf("%s/site/prioritySoC/set", m.root), func(payload string) {
		soc, err := strconv.Atoi(payload)
		if err == nil {
			_ = site.SetPrioritySoC(float64(soc))
		}
	})

	// number of loadpoints
	topic := fmt.Sprintf("%s/loadpoints", m.root)
	m.publish(topic, true, len(site.LoadPoints()))

	// loadpoint setters
	for id, lp := range site.LoadPoints() {
		topic := fmt.Sprintf("%s/loadpoints/%d", m.root, id+1)
		m.listenSetters(topic, lp)
	}

	// alive indicator
	updated := time.Now().Unix()
	m.publish(fmt.Sprintf("%s/updated", m.root), true, updated)

	// publish
	for p := range in {
		topic := fmt.Sprintf("%s/site", m.root)
		if p.LoadPoint != nil {
			id := *p.LoadPoint + 1
			topic = fmt.Sprintf("%s/loadpoints/%d", m.root, id)
		}

		// alive indicator
		if now := time.Now().Unix(); now != updated {
			updated = now
			m.publish(fmt.Sprintf("%s/updated", m.root), true, updated)
		}

		// value
		topic += "/" + p.Key
		m.publish(topic, false, p.Val)
	}
}

package meter

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/andig/evcc/charger/openwb"
	"github.com/andig/evcc/provider"
	"github.com/andig/evcc/provider/mqtt"
	"github.com/andig/evcc/util"
)

func init() {
	registry.Add("openwb", "openWB", new(openwbMeter))
}

type openwbMeter struct {
	mqtt.Config `mapstructure:",squash"`

	Topic   string        `default:"openWB"`
	Timeout time.Duration `default:"15s"`
	Usage   string        `validate:"required,oneof=grid pv battery"`

	power    func() (float64, error)
	soc      func() (float64, error)
	currents []func() (float64, error)
}

func (m *openwbMeter) Connect() error {
	log := util.NewLogger("openwb")

	client, err := mqtt.RegisteredClientOrDefault(log, m.Config)
	if err != nil {
		return err
	}

	// timeout handler
	timer := provider.NewMqtt(log, client,
		fmt.Sprintf("%s/system/%s", m.Topic, openwb.TimestampTopic), 1, m.Timeout,
	).IntGetter()

	// getters
	boolG := func(topic string) func() (bool, error) {
		g := provider.NewMqtt(log, client, topic, 1, 0).BoolGetter()
		return func() (val bool, err error) {
			if val, err = g(); err == nil {
				_, err = timer()
			}
			return val, err
		}
	}

	floatG := func(topic string) func() (float64, error) {
		g := provider.NewMqtt(log, client, topic, 1, 0).FloatGetter()
		return func() (val float64, err error) {
			if val, err = g(); err == nil {
				_, err = timer()
			}
			return val, err
		}
	}

	switch strings.ToLower(m.Usage) {
	case "grid":
		m.power = floatG(fmt.Sprintf("%s/evu/%s", m.Topic, openwb.PowerTopic))

		for i := 1; i <= 3; i++ {
			current := floatG(fmt.Sprintf("%s/evu/%s%d", m.Topic, openwb.CurrentTopic, i))
			m.currents = append(m.currents, current)
		}

	case "pv":
		configuredG := boolG(fmt.Sprintf("%s/pv/%s", m.Topic, openwb.PvConfigured))
		configured, err := configuredG()
		if err != nil {
			return err
		}

		if !configured {
			return errors.New("pv not available")
		}

		m.power = floatG(fmt.Sprintf("%s/pv/%s", m.Topic, openwb.PowerTopic))

	case "battery":
		configuredG := boolG(fmt.Sprintf("%s/housebattery/%s", m.Topic, openwb.BatteryConfigured))
		configured, err := configuredG()
		if err != nil {
			return err
		}

		if !configured {
			return errors.New("battery not available")
		}

		m.power = floatG(fmt.Sprintf("%s/housebattery/%s", m.Topic, openwb.PowerTopic))
		m.soc = floatG(fmt.Sprintf("%s/housebattery/%s", m.Topic, openwb.SoCTopic))

	default:
		return fmt.Errorf("invalid usage: %s", m.Usage)
	}
	return nil
}

// CurrentPower implements the api.Meter interface
func (m *openwbMeter) CurrentPower() (float64, error) {
	return m.power()
}

// Currents implements the api.MeterCurrent interface
func (m *openwbMeter) Currents() (float64, float64, float64, error) {
	var currents []float64
	for _, currentG := range m.currents {
		c, err := currentG()
		if err != nil {
			return 0, 0, 0, err
		}

		currents = append(currents, c)
	}

	return currents[0], currents[1], currents[2], nil
}

// HasCurrent implements the api.OptionalMeterCurrent interface
func (m *openwbMeter) HasCurrent() bool {
	return m.currents != nil
}

// SoC implements the api.Battery interface
func (m *openwbMeter) SoC() (float64, error) {
	return m.soc()
}

// HasSoC implements the api.OptionalBattery interface
func (m *openwbMeter) HasSoC() bool {
	return m.soc != nil
}

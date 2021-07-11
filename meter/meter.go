package meter

import (
	"fmt"

	"github.com/andig/evcc/api"
	"github.com/andig/evcc/provider"
)

func init() {
	registry.Add("default", "Generic configurable Meter", new(genericMeter))
	registry.Add(api.Custom, "Generic configurable Meter", new(genericMeter))
}

// genericMeter is an api.Meter implementation with configurable getters and setters.
type genericMeter struct {
	Power        provider.Config `validate:"required"`
	Energy       *provider.Config
	CurrentsConf []provider.Config `mapstructure:"currents" validate:"len=0|len=3"`
	SoCConf      *provider.Config  `mapstructure:"soc"`

	currentPowerG func() (float64, error)
	totalEnergyG  func() (float64, error)
	currentsG     []func() (float64, error)
	batterySoCG   func() (float64, error)
}

func (m *genericMeter) Connect() error {
	for k, v := range map[string]string{"power": m.Power.PluginType()} {
		if v == "" {
			return fmt.Errorf("missing plugin configuration: %s", k)
		}
	}

	var err error
	m.currentPowerG, err = provider.NewFloatGetterFromConfig(m.Power)
	if err != nil {
		return fmt.Errorf("power: %w", err)
	}

	// decorate Meter with MeterEnergy
	if m.Energy != nil {
		m.totalEnergyG, err = provider.NewFloatGetterFromConfig(*m.Energy)
		if err != nil {
			return fmt.Errorf("energy: %w", err)
		}
	}

	// decorate Meter with MeterCurrent
	if len(m.CurrentsConf) > 0 {
		for idx, cc := range m.CurrentsConf {
			c, err := provider.NewFloatGetterFromConfig(cc)
			if err != nil {
				return fmt.Errorf("currents[%d]: %w", idx, err)
			}

			m.currentsG = append(m.currentsG, c)
		}
	}

	// decorate Meter with BatterySoC
	if m.SoCConf != nil {
		m.batterySoCG, err = provider.NewFloatGetterFromConfig(*m.SoCConf)
		if err != nil {
			return fmt.Errorf("battery: %w", err)
		}
	}
	return nil
}

// CurrentPower implements the api.Meter interface
func (m *genericMeter) CurrentPower() (float64, error) {
	return m.currentPowerG()
}

// TotalEnergy implements the api.MeterEnergy interface
func (m *genericMeter) TotalEnergy() (float64, error) {
	return m.totalEnergyG()
}

// HasEnergy implements the api.OptionalMeterEnergy interface
func (m *genericMeter) HasEnergy() bool {
	return m.totalEnergyG != nil
}

// Currents implements the api.MeterCurrent interface
func (m *genericMeter) Currents() (float64, float64, float64, error) {
	var currents []float64
	for _, currentG := range m.currentsG {
		c, err := currentG()
		if err != nil {
			return 0, 0, 0, err
		}

		currents = append(currents, c)
	}

	return currents[0], currents[1], currents[2], nil
}

// HasCurrent implements the api.OptionalMeterCurrent interface
func (m *genericMeter) HasCurrent() bool {
	return m.currentsG != nil
}

// SoC implements the api.Battery interface
func (m *genericMeter) SoC() (float64, error) {
	return m.batterySoCG()
}

// HasSoC implements the api.OptionalBattery interface
func (m *genericMeter) HasSoC() bool {
	return m.batterySoCG != nil
}

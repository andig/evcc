package meter

import (
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"

	"github.com/andig/evcc/meter/powerwall"
	"github.com/andig/evcc/util"
	"github.com/andig/evcc/util/request"
)

// credits to https://github.com/vloschiavo/powerwall2

func init() {
	registry.Add("tesla", "Tesla Powerwall", new(teslaMeter))
}

// teslaMeter is the tesla powerwall meter
type teslaMeter struct {
	URI      string `validate:"required"`
	Usage    string `validate:"required,oneof=grid pv battery"`
	Password string `validate:"required"`

	helper *request.Helper
}

func (m *teslaMeter) Connect() error {
	_, err := url.Parse(m.URI)
	if err != nil {
		return fmt.Errorf("%s is invalid: %s", m.URI, err)
	}

	// support default meter names
	switch strings.ToLower(m.Usage) {
	case "grid":
		m.Usage = "site"
	case "pv":
		m.Usage = "solar"
	default:
		m.Usage = strings.ToLower(m.Usage)
	}

	log := util.NewLogger("tesla")

	m.helper = request.NewHelper(log)
	m.URI = util.DefaultScheme(strings.TrimSuffix(m.URI, "/"), "https")

	// ignore the self signed certificate
	m.helper.Client.Transport = request.NewTripper(log, request.InsecureTransport())
	// create cookie jar to save login tokens
	m.helper.Client.Jar, _ = cookiejar.New(nil)

	if err := m.Login(); err != nil {
		return err
	}
	return nil
}

// Login calls login and saves the returned cookie
func (m *teslaMeter) Login() error {
	data := map[string]interface{}{
		"username": "customer",
		"password": m.Password,
	}

	req, err := request.New(http.MethodPost, m.URI+powerwall.LoginURI, request.MarshalJSON(data), request.JSONEncoding)
	if err == nil {
		// use DoBody as it will close the response body
		if _, err = m.helper.DoBody(req); err != nil {
			err = fmt.Errorf("login failed: %w", err)
		}
	}

	return err
}

// CurrentPower implements the api.Meter interface
func (m *teslaMeter) CurrentPower() (float64, error) {
	var res powerwall.MeterResponse
	if err := m.helper.GetJSON(m.URI+powerwall.MeterURI, &res); err != nil {
		return 0, err
	}

	if o, ok := res[m.Usage]; ok {
		return o.InstantPower, nil
	}

	return 0, fmt.Errorf("invalid usage: %s", m.Usage)
}

// TotalEnergy implements the api.MeterEnergy interface
func (m *teslaMeter) TotalEnergy() (float64, error) {
	var res powerwall.MeterResponse
	if err := m.helper.GetJSON(m.URI+powerwall.MeterURI, &res); err != nil {
		return 0, err
	}

	if o, ok := res[m.Usage]; ok {
		if m.Usage == "load" {
			return o.EnergyImported, nil
		}
		if m.Usage == "solar" {
			return o.EnergyExported, nil
		}
	}

	return 0, fmt.Errorf("invalid usage: %s", m.Usage)
}

// HasEnergy implements the api.OptionalMeterEnergy interface
func (m *teslaMeter) HasEnergy() bool {
	return m.Usage == "load" || m.Usage == "solar"
}

// SoC implements the api.Battery interface
func (m *teslaMeter) SoC() (float64, error) {
	var res powerwall.BatteryResponse
	err := m.helper.GetJSON(m.URI+powerwall.BatteryURI, &res)

	return res.Percentage, err
}

// HasSoC implements the api.OptionalBattery interface
func (m *teslaMeter) HasSoC() bool {
	return m.Usage == "battery"
}

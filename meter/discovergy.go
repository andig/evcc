package meter

import (
	"fmt"
	"net/http"

	"github.com/andig/evcc/provider"
	"github.com/andig/evcc/util"
	"github.com/andig/evcc/util/request"
	"github.com/thoas/go-funk"
)

const discovergyAPI = "https://api.discovergy.com/public/v1"

func init() {
	registry.Add("discovergy", "Discovergy", new(discovergyMeter))
}

type discovergyMeterEntry struct {
	MeterID          string `json:"meterId"`
	SerialNumber     string `json:"serialNumber"`
	FullSerialNumber string `json:"fullSerialNumber"`
}

type discovergyMeter struct {
	User     string `validate:"required"`
	Password string `validate:"required"`
	Meter    string
	Scale    float64 `default:"1"`

	currentPowerG func() (float64, error)
}

func (m *discovergyMeter) Connect() error {
	log := util.NewLogger("discgy")

	headers := make(map[string]string)
	if err := provider.AuthHeaders(log, provider.Auth{
		Type:     "Basic",
		User:     m.User,
		Password: m.Password,
	}, headers); err != nil {
		return err
	}

	req, err := request.New(http.MethodGet, fmt.Sprintf("%s/meters", discovergyAPI), nil, headers)
	if err != nil {
		return err
	}

	var meters []discovergyMeterEntry
	if err := request.NewHelper(log).DoJSON(req, &meters); err != nil {
		return err
	}

	var meterID string
	if m.Meter != "" {
		for _, meter := range meters {
			if matchesIdentifier(m.Meter, meter) {
				meterID = meter.MeterID
				break
			}
		}
	} else if len(meters) == 1 {
		meterID = meters[0].MeterID
	}

	if meterID == "" {
		return fmt.Errorf("could not determine meter id: %v", funk.Map(meters, func(m discovergyMeterEntry) string {
			return m.FullSerialNumber
		}))
	}

	uri := fmt.Sprintf("%s/last_reading?meterId=%s", discovergyAPI, meterID)
	power, err := provider.NewHTTP(log, http.MethodGet, uri, headers, "", false, ".values.power", 0.001*m.Scale)
	if err != nil {
		return err
	}
	m.currentPowerG = power.FloatGetter()

	return nil
}

func matchesIdentifier(id string, m discovergyMeterEntry) bool {
	return id == m.MeterID || id == m.SerialNumber || id == m.FullSerialNumber
}

// CurrentPower implements the api.Meter interface
func (m *discovergyMeter) CurrentPower() (float64, error) {
	return m.currentPowerG()
}

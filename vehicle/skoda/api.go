package skoda

import (
	"fmt"
	"net/http"

	"github.com/andig/evcc/util"
	"github.com/andig/evcc/util/request"
	"golang.org/x/oauth2"
)

const BaseURI = "https://api.connect.skoda-auto.cz/api"

// API is the Skoda api client
type API struct {
	*request.Helper
}

// NewAPI creates a new api client
func NewAPI(log *util.Logger, identity oauth2.TokenSource) *API {
	v := &API{
		Helper: request.NewHelper(log, request.WithMetricsPush),
	}

	v.Client.Transport = &oauth2.Transport{
		Source: identity,
		Base:   v.Client.Transport,
	}

	return v
}

func (v *API) getJSON(uri string, res interface{}) error {
	req, err := request.New(http.MethodGet, uri, nil, request.AcceptJSON)

	if err == nil {
		err = v.DoJSON(req, &res)
	}

	return err
}

// Vehicle is the /v2/garage/vehicles api
type Vehicle struct {
	ID, VIN       string
	LastUpdatedAt string
	Specification struct {
		Title, Brand, Model string
		Battery             struct {
			CapacityInKWh int
		}
	}
	// Connectivities
	// Capabilities
}

// Vehicles implements the /vehicles response
func (v *API) Vehicles() ([]string, error) {
	var res []Vehicle

	uri := fmt.Sprintf("%s/v2/garage/vehicles", BaseURI)
	err := v.getJSON(uri, &res)

	var vehicles []string
	if err == nil {
		for _, v := range res {
			vehicles = append(vehicles, v.VIN)
		}
	}

	return vehicles, err
}

// ChargerResponse is the /v1/charging/<vin>/status api
type ChargerResponse struct {
	Plug struct {
		ConnectionState string // Connected
		LockState       string // Unlocked
	}
	Charging struct {
		State                           string // Error
		RemainingToCompleteInSeconds    int64
		ChargingPowerInWatts            float64
		ChargingRateInKilometersPerHour float64
		ChargingType                    string // Invalid
		ChargeMode                      string // MANUAL
	}
	Battery struct {
		CruisingRangeElectricInMeters int64
		StateOfChargeInPercent        int
	}
}

// charging/settings
// {
//     "autoUnlockPlugWhenCharged": "Permanent",
//     "maxChargeCurrentAc": "Maximum",
//     "targetStateOfChargeInPercent": 100
// }

// Charger implements the /v1/charging/<vin>/status response
func (v *API) Charger(vin string) (ChargerResponse, error) {
	var res ChargerResponse
	uri := fmt.Sprintf("%s/v1/charging/%s/status", BaseURI, vin)
	err := v.getJSON(uri, &res)
	return res, err
}

// ClimaterResponse is the /bs/climatisation/v1/%s/%s/vehicles/%s/climater api
// type ClimaterResponse struct {
// }

// // Climater implements the /climater response
// func (v *API) Climater(vin string) (ClimaterResponse, error) {
// 	var res ClimaterResponse
// 	uri := fmt.Sprintf("%s/bs/climatisation/v1/%s/%s/vehicles/%s/climater", BaseURI, v.brand, v.country, vin)
// 	err := v.getJSON(uri, &res)
// 	return res, err
// }

// const (
// 	ActionCharge      = "batterycharge"
// 	ActionChargeStart = "start"
// 	ActionChargeStop  = "stop"
// )

// type actionDefinition struct {
// 	contentType string
// 	appendix    string
// }

// var actionDefinitions = map[string]actionDefinition{
// 	ActionCharge: {
// 		"application/vnd.vwg.mbb.ChargerAction_v1_0_0+xml",
// 		"charger/actions",
// 	},
// }

// // Action implements vehicle actions
// func (v *API) Action(vin, action, value string) error {
// 	def := actionDefinitions[action]

// 	uri := fmt.Sprintf("%s/bs/%s/v1/%s/%s/vehicles/%s/%s", BaseURI, action, v.brand, v.country, vin, def.appendix)
// 	body := "<?xml version=\"1.0\" encoding=\"UTF-8\" ?><action><type>" + value + "</type></action>"

// 	req, err := request.New(http.MethodPost, uri, strings.NewReader(body), map[string]string{
// 		"Content-type": def.contentType,
// 	})

// 	if err == nil {
// 		var resp *http.Response
// 		if resp, err = v.Do(req); err == nil {
// 			resp.Body.Close()
// 		}
// 	}

// 	return err
// }

// // Any implements any api response
// func (v *API) Any(base, vin string) (interface{}, error) {
// 	var res interface{}
// 	uri := fmt.Sprintf("%s/"+strings.TrimLeft(base, "/"), BaseURI, v.brand, v.country, vin)
// 	err := v.getJSON(uri, &res)
// 	return res, err
// }

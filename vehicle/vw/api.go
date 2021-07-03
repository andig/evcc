package vw

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/andig/evcc/util"
	"github.com/andig/evcc/util/request"
	"golang.org/x/oauth2"
)

// DefaultBaseURI is the VW api base URI
const DefaultBaseURI = "https://msg.volkswagen.de/fs-car"

// RegionAPI is the VW api used for determining the home region
const RegionAPI = "https://mal-1a.prd.ece.vwg-connect.com/api"

// API is the VW api client
type API struct {
	*request.Helper
	brand, country string
	baseURI        string
}

// NewAPI creates a new api client
func NewAPI(log *util.Logger, identity oauth2.TokenSource, brand, country string) *API {
	v := &API{
		Helper:  request.NewHelper(log, request.WithMetricsPush),
		brand:   brand,
		country: country,
		baseURI: DefaultBaseURI,
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

// VehiclesResponse is the /usermanagement/users/v1/%s/%s/vehicles api
type VehiclesResponse struct {
	UserVehicles struct {
		Vehicle []string
	}
}

// Vehicles implements the /vehicles response
func (v *API) Vehicles() ([]string, error) {
	var res VehiclesResponse
	uri := fmt.Sprintf("%s/usermanagement/users/v1/%s/%s/vehicles", v.baseURI, v.brand, v.country)
	err := v.getJSON(uri, &res)
	return res.UserVehicles.Vehicle, err
}

// HomeRegion is the home region API response
type HomeRegion struct {
	HomeRegion struct {
		BaseURI struct {
			SystemID string
			Content  string // api url
		}
	}
}

// HomeRegion updates the home region for the given vehicle
func (v *API) HomeRegion(vin string) error {
	var res HomeRegion
	uri := fmt.Sprintf("%s/cs/vds/v1/vehicles/%s/homeRegion", RegionAPI, vin)

	err := v.getJSON(uri, &res)
	if err == nil {
		if api := res.HomeRegion.BaseURI.Content; strings.HasPrefix(api, "https://mal-3a.prd.eu.dp.vwg-connect.com") {
			api = "https://fal" + strings.TrimPrefix(api, "https://mal")
			api = strings.TrimSuffix(api, "/api") + "/fs-car"
			v.baseURI = api
		}
	}

	return err
}

// RolesRights updates the home region for the given vehicle
func (v *API) RolesRights(vin string) (string, error) {
	var res json.RawMessage
	uri := fmt.Sprintf("%s/rolesrights/operationlist/v3/vehicles/%s", RegionAPI, vin)
	err := v.getJSON(uri, &res)
	return string(res), err
}

// ChargerResponse is the /bs/batterycharge/v1/%s/%s/vehicles/%s/charger api
type ChargerResponse struct {
	Charger struct {
		Status struct {
			BatteryStatusData struct {
				StateOfCharge         TimedInt
				RemainingChargingTime TimedInt
			}
			ChargingStatusData struct {
				ChargingState            TimedString // off, charging
				ChargingMode             TimedString // invalid, AC
				ChargingReason           TimedString // invalid, immediate
				ExternalPowerSupplyState TimedString // unavailable, available
				EnergyFlow               TimedString // on, off
			}
			PlugStatusData struct {
				PlugState TimedString // connected
			}
			CruisingRangeStatusData struct {
				EngineTypeFirstEngine  TimedString // typeIsElectric, petrolGasoline
				EngineTypeSecondEngine TimedString // typeIsElectric, petrolGasoline
				PrimaryEngineRange     TimedInt
				SecondaryEngineRange   TimedInt
				HybridRange            TimedInt
			}
		}
	}
}

// Charger implements the /charger response
func (v *API) Charger(vin string) (ChargerResponse, error) {
	var res ChargerResponse
	uri := fmt.Sprintf("%s/bs/batterycharge/v1/%s/%s/vehicles/%s/charger", v.baseURI, v.brand, v.country, vin)
	err := v.getJSON(uri, &res)
	return res, err
}

// ClimaterResponse is the /bs/climatisation/v1/%s/%s/vehicles/%s/climater api
type ClimaterResponse struct {
	Climater struct {
		Settings struct {
			TargetTemperature TimedTemperature
			HeaterSource      TimedString
		}
		Status struct {
			ClimatisationStatusData struct {
				ClimatisationState         TimedString
				ClimatisationReason        TimedString
				RemainingClimatisationTime TimedInt
			}
			TemperatureStatusData struct {
				OutdoorTemperature TimedTemperature
			}
			VehicleParkingClockStatusData struct {
				VehicleParkingClock TimedString
			}
		}
	}
}

// Climater implements the /climater response
func (v *API) Climater(vin string) (ClimaterResponse, error) {
	var res ClimaterResponse
	uri := fmt.Sprintf("%s/bs/climatisation/v1/%s/%s/vehicles/%s/climater", v.baseURI, v.brand, v.country, vin)
	err := v.getJSON(uri, &res)
	return res, err
}

const (
	ActionCharge      = "batterycharge"
	ActionChargeStart = "start"
	ActionChargeStop  = "stop"
)

type actionDefinition struct {
	contentType string
	appendix    string
}

var actionDefinitions = map[string]actionDefinition{
	ActionCharge: {
		"application/vnd.vwg.mbb.ChargerAction_v1_0_0+xml",
		"charger/actions",
	},
}

// Action implements vehicle actions
func (v *API) Action(vin, action, value string) error {
	def := actionDefinitions[action]

	uri := fmt.Sprintf("%s/bs/%s/v1/%s/%s/vehicles/%s/%s", v.baseURI, action, v.brand, v.country, vin, def.appendix)
	body := "<?xml version=\"1.0\" encoding=\"UTF-8\" ?><action><type>" + value + "</type></action>"

	req, err := request.New(http.MethodPost, uri, strings.NewReader(body), map[string]string{
		"Content-type": def.contentType,
	})

	if err == nil {
		var resp *http.Response
		if resp, err = v.Do(req); err == nil {
			resp.Body.Close()
		}
	}

	return err
}

// Any implements any api response
func (v *API) Any(base, vin string) (interface{}, error) {
	var res interface{}
	uri := fmt.Sprintf("%s/"+strings.TrimLeft(base, "/"), v.baseURI, v.brand, v.country, vin)
	err := v.getJSON(uri, &res)
	return res, err
}

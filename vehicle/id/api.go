package id

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/andig/evcc/util"
	"github.com/andig/evcc/util/request"
	"golang.org/x/oauth2"
)

// https://identity-userinfo.vwgroup.io/oidc/userinfo
// https://customer-profile.apps.emea.vwapps.io/v1/customers/<userId>/realCarData

// BaseURL is the API base url
const BaseURL = "https://mobileapi.apps.emea.vwapps.io"

// API is an api.Vehicle implementation for VW ID cars
type API struct {
	*request.Helper
}

// Actions and action values
const (
	ActionCharge         = "charging"
	ActionChargeStart    = "start"
	ActionChargeStop     = "stop"
	ActionChargeSettings = "settings" // body: targetSOC_pct

	ActionClimatisation      = "climatisation"
	ActionClimatisationStart = "start"
	ActionClimatisationStop  = "stop"
)

// NewAPI creates a new vehicle
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

// Vehicles implements the /vehicles response
func (v *API) Vehicles() (res []string, err error) {
	uri := fmt.Sprintf("%s/vehicles", BaseURL)

	req, err := request.New(http.MethodGet, uri, nil, request.AcceptJSON)

	var vehicles struct {
		Data []struct {
			VIN      string
			Model    string
			Nickname string
		}
	}

	if err == nil {
		err = v.DoJSON(req, &vehicles)

		for _, v := range vehicles.Data {
			res = append(res, v.VIN)
		}
	}

	return res, err
}

// Status is the /status api
type Status struct {
	Data struct {
		BatteryStatus         BatteryStatus
		ChargingStatus        ChargingStatus
		ChargingSettings      ChargingSettings
		PlugStatus            PlugStatus
		RangeStatus           RangeStatus
		ClimatisationSettings ClimatisationSettings
		ClimatisationStatus   ClimatisationStatus // may be currently not available
	}
}

// BatteryStatus is the /status.batteryStatus api
type BatteryStatus struct {
	CarCapturedTimestamp    string
	CurrentSOCPercent       int `json:"currentSOC_pct"`
	CruisingRangeElectricKm int `json:"cruisingRangeElectric_km"`
}

// ChargingStatus is the /status.chargingStatus api
type ChargingStatus struct {
	CarCapturedTimestamp               string
	ChargingState                      string  // readyForCharging
	RemainingChargingTimeToCompleteMin int     `json:"remainingChargingTimeToComplete_min"`
	ChargePowerKW                      float64 `json:"chargePower_kW"`
	ChargeRateKmph                     int     `json:"chargeRate_kmph"`
}

// ChargingSettings is the /status.chargingSettings api
type ChargingSettings struct {
	CarCapturedTimestamp      string
	MaxChargeCurrentAC        string // reduced, maximum
	AutoUnlockPlugWhenCharged string
	TargetSOCPercent          int `json:"targetSOC_pct"`
}

// PlugStatus is the /status.plugStatus api
type PlugStatus struct {
	CarCapturedTimestamp string
	PlugConnectionState  string // connected, disconnected
	PlugLockState        string
}

// ClimatisationStatus is the /status.climatisationStatus api
type ClimatisationStatus struct {
	CarCapturedTimestamp          string
	RemainingClimatisationTimeMin int    `json:"remainingClimatisationTime_min"`
	ClimatisationState            string // off
}

// ClimatisationSettings is the /status.climatisationSettings api
type ClimatisationSettings struct {
	CarCapturedTimestamp              string
	TargetTemperatureK                float64 `json:"targetTemperature_K"`
	TargetTemperatureC                float64 `json:"targetTemperature_C"`
	ClimatisationWithoutExternalPower bool
	ClimatisationAtUnlock             bool // ClimatizationAtUnlock?
	WindowHeatingEnabled              bool
	ZoneFrontLeftEnabled              bool
	ZoneFrontRightEnabled             bool
	ZoneRearLeftEnabled               bool
	ZoneRearRightEnabled              bool
}

// RangeStatus is the /status.rangeStatus api
type RangeStatus struct {
	CarCapturedTimestamp string
	CarType              string
	PrimaryEngine        struct {
		Type              string
		CurrentSOCPercent int `json:"currentSOC_pct"`
		RemainingRangeKm  int `json:"remainingRange_km"`
	}
	TotalRangeKm int `json:"totalRange_km"`
}

// Status implements the /status response
func (v *API) Status(vin string) (res Status, err error) {
	uri := fmt.Sprintf("%s/vehicles/%s/status", BaseURL, vin)

	req, err := request.New(http.MethodGet, uri, nil, request.AcceptJSON)

	if err == nil {
		err = v.DoJSON(req, &res)
	}

	return res, err
}

// Action implements vehicle actions
func (v *API) Action(vin, action, value string) error {
	uri := fmt.Sprintf("%s/vehicles/%s/%s/%s", BaseURL, vin, action, value)

	req, err := request.New(http.MethodPost, uri, nil, request.AcceptJSON)

	if err == nil {
		var res interface{}
		err = v.DoJSON(req, &res)
	}

	return err
}

// Any implements any api response
func (v *API) Any(uri, vin string) (interface{}, error) {
	if strings.Contains(uri, "%s") {
		uri = fmt.Sprintf(uri, vin)
	}

	req, err := request.New(http.MethodGet, uri, nil, request.AcceptJSON)

	var res interface{}
	if err == nil {
		err = v.DoJSON(req, &res)
	}

	return res, err
}

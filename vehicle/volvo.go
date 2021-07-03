package vehicle

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"time"

	"github.com/andig/evcc/api"
	"github.com/andig/evcc/provider"
	"github.com/andig/evcc/util"
	"github.com/andig/evcc/util/request"
)

const (
	volvoAPI = "https://vocapi.wirelesscar.net/customerapi/rest/v3.0"
)

type volvoAccountResponse struct {
	FirstName        string   `json:"firstName"`
	LastName         string   `json:"lastName"`
	VehicleRelations []string `json:"accountVehicleRelations"`
}

type volvoVehicleRelation struct {
	Account                   string `json:"account"`
	AccountID                 string `json:"accountId"`
	Vehicle                   string `json:"vehicle"`
	AccountVehicleRelation    string `json:"accountVehicleRelation"`
	VehicleID                 string `json:"vehicleId"`
	Username                  string `json:"username"`
	Status                    string `json:"status"`
	CustomerVehicleRelationID int    `json:"customerVehicleRelationId"`
}

type volvoStatus struct {
	AverageFuelConsumption          float32 `json:"averageFuelConsumption"`
	AverageFuelConsumptionTimestamp string  `json:"averageFuelConsumptionTimestamp"`
	AverageSpeed                    int     `json:"averageSpeed"`
	AverageSpeedTimestamp           string  `json:"averageSpeedTimestamp"`
	BrakeFluid                      string  `json:"brakeFluid"`
	BrakeFluidTimestamp             string  `json:"brakeFluidTimestamp"`
	CarLocked                       bool    `json:"carLocked"`
	CarLockedTimestamp              string  `json:"carLockedTimestamp"`
	ConnectionStatus                string  `json:"connectionStatus"` // Disconnected
	ConnectionStatusTimestamp       string  `json:"connectionStatusTimestamp"`
	DistanceToEmpty                 int     `json:"distanceToEmpty"`
	DistanceToEmptyTimestamp        string  `json:"distanceToEmptyTimestamp"`
	EngineRunning                   bool    `json:"engineRunning"`
	EngineRunningTimestamp          string  `json:"engineRunningTimestamp"`
	FuelAmount                      int     `json:"fuelAmount"`
	FuelAmountLevel                 int     `json:"fuelAmountLevel"`
	FuelAmountLevelTimestamp        string  `json:"fuelAmountLevelTimestamp"`
	FuelAmountTimestamp             string  `json:"fuelAmountTimestamp"`
	HvBattery                       struct {
		HvBatteryChargeStatusDerived          string `json:"hvBatteryChargeStatusDerived"` // CableNotPluggedInCar, CablePluggedInCar, Charging
		HvBatteryChargeStatusDerivedTimestamp string `json:"hvBatteryChargeStatusDerivedTimestamp"`
		HvBatteryChargeModeStatus             string `json:"hvBatteryChargeModeStatus"`
		HvBatteryChargeModeStatusTimestamp    string `json:"hvBatteryChargeModeStatusTimestamp"`
		HvBatteryChargeStatus                 string `json:"hvBatteryChargeStatus"` // Started, ChargeProgress, ChargeEnd, Interrupted
		HvBatteryChargeStatusTimestamp        string `json:"hvBatteryChargeStatusTimestamp"`
		HvBatteryLevel                        int    `json:"hvBatteryLevel"`
		HvBatteryLevelTimestamp               string `json:"hvBatteryLevelTimestamp"`
		DistanceToHVBatteryEmpty              int    `json:"distanceToHVBatteryEmpty"`
		DistanceToHVBatteryEmptyTimestamp     string `json:"distanceToHVBatteryEmptyTimestamp"`
		TimeToHVBatteryFullyCharged           int    `json:"timeToHVBatteryFullyCharged"`
		TimeToHVBatteryFullyChargedTimestamp  string `json:"timeToHVBatteryFullyChargedTimestamp"`
	} `json:"hvBattery"`
	Odometer                           int    `json:"odometer"`
	OdometerTimestamp                  string `json:"odometerTimestamp"`
	PrivacyPolicyEnabled               bool   `json:"privacyPolicyEnabled"`
	PrivacyPolicyEnabledTimestamp      string `json:"privacyPolicyEnabledTimestamp"`
	RemoteClimatizationStatus          string `json:"remoteClimatizationStatus"` // CableConnectedWithoutPower
	RemoteClimatizationStatusTimestamp string `json:"remoteClimatizationStatusTimestamp"`
	ServiceWarningStatus               string `json:"serviceWarningStatus"`
	ServiceWarningStatusTimestamp      string `json:"serviceWarningStatusTimestamp"`
	TimeFullyAccessibleUntil           string `json:"timeFullyAccessibleUntil"`
	TimePartiallyAccessibleUntil       string `json:"timePartiallyAccessibleUntil"`
	TripMeter1                         int    `json:"tripMeter1"`
	TripMeter1Timestamp                string `json:"tripMeter1Timestamp"`
	TripMeter2                         int    `json:"tripMeter2"`
	TripMeter2Timestamp                string `json:"tripMeter2Timestamp"`
}

// Volvo is an api.Vehicle implementation for Volvo cars
type Volvo struct {
	*embed
	*request.Helper
	user, password, vin string
	statusG             func() (interface{}, error)
}

func init() {
	registry.Add("volvo", NewVolvoFromConfig)
}

// NewVolvoFromConfig creates a new vehicle
func NewVolvoFromConfig(other map[string]interface{}) (api.Vehicle, error) {
	cc := struct {
		embed               `mapstructure:",squash"`
		User, Password, VIN string
		Cache               time.Duration
	}{
		Cache: interval,
	}

	if err := util.DecodeOther(other, &cc); err != nil {
		return nil, err
	}

	log := util.NewLogger("volvo")

	v := &Volvo{
		embed:    &cc.embed,
		Helper:   request.NewHelper(log, request.WithMetricsPush),
		user:     cc.User,
		password: cc.Password,
		vin:      cc.VIN,
	}

	v.statusG = provider.NewCached(v.status, cc.Cache).InterfaceGetter()

	var err error
	if cc.VIN == "" {
		v.vin, err = findVehicle(v.vehicles())
		if err == nil {
			log.DEBUG.Printf("found vehicle: %v", v.vin)
		}
	}

	return v, err
}

func (v *Volvo) request(uri string) (*http.Request, error) {
	basicAuth := base64.StdEncoding.EncodeToString([]byte(v.user + ":" + v.password))

	return request.New(http.MethodGet, uri, nil, map[string]string{
		"Authorization":     fmt.Sprintf("Basic %s", basicAuth),
		"Content-Type":      "application/json",
		"X-Device-Id":       "Device",
		"X-OS-Type":         "Android",
		"X-Originator-Type": "App",
		"X-OS-Version":      "22",
	})
}

// vehicles implements returns the list of user vehicles
func (v *Volvo) vehicles() ([]string, error) {
	var vehicles []string

	req, err := v.request(fmt.Sprintf("%s/customeraccounts", volvoAPI))
	if err == nil {
		var res volvoAccountResponse
		err = v.DoJSON(req, &res)

		for _, rel := range res.VehicleRelations {
			var vehicle volvoVehicleRelation
			if req, err := v.request(rel); err == nil {
				if err = v.DoJSON(req, &vehicle); err != nil {
					return vehicles, err
				}

				vehicles = append(vehicles, vehicle.VehicleID)
			}
		}
	}

	return vehicles, err
}

func (v *Volvo) status() (interface{}, error) {
	var res volvoStatus

	req, err := v.request(fmt.Sprintf("%s/vehicles/%s/status", volvoAPI, v.vin))
	if err == nil {
		err = v.DoJSON(req, &res)
	}

	return res, err
}

// SoC implements the api.Vehicle interface
func (v *Volvo) SoC() (float64, error) {
	res, err := v.statusG()
	if res, ok := res.(volvoStatus); err == nil && ok {
		return float64(res.HvBattery.HvBatteryLevel), nil
	}

	return 0, err
}

var _ api.ChargeState = (*Volvo)(nil)

// Status implements the api.ChargeState interface
func (v *Volvo) Status() (api.ChargeStatus, error) {
	res, err := v.statusG()
	if res, ok := res.(volvoStatus); err == nil && ok {
		switch res.HvBattery.HvBatteryChargeStatusDerived {
		case "CableNotPluggedInCar":
			return api.StatusA, nil
		case "CablePluggedInCar":
			return api.StatusB, nil
		case "Charging":
			return api.StatusC, nil
		}
	}

	return api.StatusNone, err
}

var _ api.VehicleRange = (*Volvo)(nil)

// VehicleRange implements the api.VehicleRange interface
func (v *Volvo) Range() (int64, error) {
	res, err := v.statusG()
	if res, ok := res.(volvoStatus); err == nil && ok {
		return int64(res.HvBattery.DistanceToHVBatteryEmpty), nil
	}

	return 0, err
}

var _ api.VehicleFinishTimer = (*Volvo)(nil)

// FinishTime implements the VehicleFinishTimer interface
func (v *Volvo) FinishTime() (time.Time, error) {
	res, err := v.statusG()
	if res, ok := res.(volvoStatus); err == nil && ok {
		timestamp, err := time.Parse("2006-01-02T15:04:05-0700", res.HvBattery.TimeToHVBatteryFullyChargedTimestamp)

		if err == nil {
			timestamp = timestamp.Add(time.Duration(res.HvBattery.DistanceToHVBatteryEmpty) * time.Minute)
			if timestamp.Before(time.Now()) {
				return time.Time{}, api.ErrNotAvailable
			}
		}

		return timestamp, err
	}

	return time.Time{}, err
}

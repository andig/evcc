package bluelink

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/andig/evcc/api"
	"github.com/andig/evcc/util"
	"github.com/andig/evcc/util/request"
)

const (
	VehiclesURL     = "/api/v1/spa/vehicles"
	StatusURL       = "/api/v1/spa/vehicles/%s/status"
	StatusLatestURL = "/api/v1/spa/vehicles/%s/status/latest"
)

const (
	resOK          = "S"                    // auth fail: F
	timeFormat     = "20060102150405 -0700" // Note: must add timeOffset
	timeOffset     = " +0100"
	refreshTimeout = time.Minute
	statusExpiry   = 5 * time.Minute
)

// ErrAuthFail indicates authorization failure
var ErrAuthFail = errors.New("authorization failed")

// API implements the Kia/Hyundai bluelink api.
// Based on https://github.com/Hacksore/bluelinky.
type API struct {
	*request.Helper
	log         *util.Logger
	identity    *Identity
	refresh     bool
	refreshTime time.Time
}

// New creates a new BlueLink API
func NewAPI(log *util.Logger, identity *Identity, cache time.Duration) *API {
	v := &API{
		log:      log,
		identity: identity,
		Helper:   request.NewHelper(log, request.WithMetricsPush),
	}

	// api is unbelievably slow when retrieving status
	v.Helper.Client.Timeout = 120 * time.Second

	return v
}

type VehiclesResponse struct {
	RetCode string
	ResMsg  struct {
		Vehicles []Vehicle
	}
}

type Vehicle struct {
	Vin, VehicleName, VehicleID string
}

func (v *API) Vehicles() ([]Vehicle, error) {
	req, err := v.identity.Request(http.MethodGet, VehiclesURL)

	var resp VehiclesResponse
	if err == nil {
		err = v.DoJSON(req, &resp)
	}

	return resp.ResMsg.Vehicles, err
}

type StatusResponse struct {
	RetCode string
	ResCode string
	ResMsg  StatusData
}

type StatusLatestResponse struct {
	RetCode string
	ResCode string
	ResMsg  struct {
		VehicleStatusInfo struct {
			VehicleStatus StatusData
		}
	}
}

type StatusData struct {
	Time     string
	EvStatus struct {
		BatteryStatus float64
		RemainTime2   struct {
			Atc struct {
				Value, Unit int
			}
		}
		DrvDistance []DrivingDistance
	}
	Vehicles []Vehicle
}

func (d *StatusData) Updated() (time.Time, error) {
	return time.Parse(timeFormat, d.Time+timeOffset)
}

type DrivingDistance struct {
	RangeByFuel struct {
		EvModeRange struct {
			Value int
		}
	}
}

func (v *API) Status(vid string) (StatusData, error) {
	var resp StatusLatestResponse

	req, err := v.identity.Request(http.MethodGet, fmt.Sprintf(StatusLatestURL, vid))
	if err == nil {
		if err = v.DoJSON(req, &resp); err == nil && resp.RetCode != resOK {
			err = fmt.Errorf("unexpected response: %s", resp.RetCode)
		}

		var ts time.Time
		if err == nil {
			ts, err = resp.ResMsg.VehicleStatusInfo.VehicleStatus.Updated()

			// return the current value
			if time.Since(ts) <= statusExpiry {
				v.refresh = false
				return resp.ResMsg.VehicleStatusInfo.VehicleStatus, err
			}
		}
	}

	// request a refresh, irrespective of a previous error
	if !v.refresh {
		if err = v.refreshRequest(vid); err == nil {
			err = api.ErrMustRetry
		}

		return StatusData{}, err
	}

	// refresh finally expired
	if time.Since(v.refreshTime) > refreshTimeout {
		v.refresh = false
		if err == nil {
			err = api.ErrTimeout
		}
	} else {
		// wait for refresh, irrespective of a previous error
		err = api.ErrMustRetry
	}

	return resp.ResMsg.VehicleStatusInfo.VehicleStatus, err
}

func (v *API) refreshRequest(vid string) error {
	req, err := v.identity.Request(http.MethodGet, fmt.Sprintf(StatusURL, vid))
	if err == nil {
		v.refresh = true
		v.refreshTime = time.Now()

		// run the actual update asynchronously
		go func() {
			var resp StatusResponse
			if err := v.DoJSON(req, &resp); err == nil && resp.RetCode != resOK {
				v.log.ERROR.Printf("unexpected response: %s", resp.RetCode)
			}
		}()
	}

	return err
}

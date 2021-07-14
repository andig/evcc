package vehicle

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/andig/evcc/api"
	"github.com/andig/evcc/provider"
	"github.com/andig/evcc/util"
	"github.com/andig/evcc/util/oauth"
	"github.com/andig/evcc/util/request"
	"golang.org/x/oauth2"
)

const (
	FiatAuth           = "https://fcis.ice.ibmcloud.com"
	FiatAPI            = "https://usapi.cv.Fiat.com"
	FiatVehicleList    = "https://api.mps.Fiat.com/api/users/vehicles"
	FiatStatusExpiry   = 5 * time.Minute       // if returned status value is older, evcc will init refresh
	FiatRefreshTimeout = time.Minute           // timeout to get status after refresh
	FiatTimeFormat     = "01-02-2006 15:04:05" // time format used by Fiat API, time is in UTC
)

// Fiat is an api.Vehicle implementation for Fiat cars
type Fiat struct {
	*embed
	*request.Helper
	log                 *util.Logger
	user, password, vin string
	tokenSource         oauth2.TokenSource
	statusG             func() (interface{}, error)
	refreshId           string
	refreshTime         time.Time
}

func init() {
	registry.Add("fiat", NewFiatFromConfig)
}

// NewFiatFromConfig creates a new vehicle
func NewFiatFromConfig(other map[string]interface{}) (api.Vehicle, error) {
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

	if cc.User == "" || cc.Password == "" {
		return nil, errors.New("missing credentials")
	}

	log := util.NewLogger("Fiat")

	v := &Fiat{
		embed:    &cc.embed,
		Helper:   request.NewHelper(log),
		log:      log,
		user:     cc.User,
		password: cc.Password,
		vin:      strings.ToUpper(cc.VIN),
	}

	token, err := v.login()
	if err == nil {
		v.tokenSource = oauth.RefreshTokenSource((*oauth2.Token)(&token), v)
	}

	v.statusG = provider.NewCached(func() (interface{}, error) {
		return v.status()
	}, cc.Cache).InterfaceGetter()

	if err == nil && cc.VIN == "" {
		v.vin, err = findVehicle(v.vehicles())
		if err == nil {
			log.DEBUG.Printf("found vehicle: %v", v.vin)
		}
	}

	return v, err
}

// login authenticates with username/password to get new token
func (v *Fiat) login() (oauth.Token, error) {
	data := url.Values{
		"client_id":  []string{"9fb503e0-715b-47e8-adfd-ad4b7770f73b"},
		"grant_type": []string{"password"},
		"username":   []string{v.user},
		"password":   []string{v.password},
	}

	uri := FiatAuth + "/v1.0/endpoint/default/token"
	req, err := request.New(http.MethodPost, uri, strings.NewReader(data.Encode()), request.URLEncoding)

	var res oauth.Token
	if err == nil {
		err = v.DoJSON(req, &res)
	}

	return res, err
}

// Refresh implements the oauth.TokenRefresher interface
func (v *Fiat) RefreshToken(token *oauth2.Token) (*oauth2.Token, error) {
	data := url.Values{
		"client_id":     []string{"9fb503e0-715b-47e8-adfd-ad4b7770f73b"},
		"grant_type":    []string{"refresh_token"},
		"refresh_token": []string{token.RefreshToken},
	}

	uri := FiatAuth + "/v1.0/endpoint/default/token"
	req, err := request.New(http.MethodPost, uri, strings.NewReader(data.Encode()), request.URLEncoding)

	var res oauth.Token
	if err == nil {
		err = v.DoJSON(req, &res)
	}

	if err != nil {
		res, err = v.login()
	}

	return (*oauth2.Token)(&res), err
}

// request is a helper to send API requests, sets header the Fiat API expects
func (v *Fiat) request(method, uri string) (*http.Request, error) {
	token, err := v.tokenSource.Token()

	var req *http.Request
	if err == nil {
		req, err = request.New(method, uri, nil, map[string]string{
			"Content-type":   "application/json",
			"Application-Id": "71A3AD0A-CF46-4CCF-B473-FC7FE5BC4592",
			"Auth-Token":     token.AccessToken,
		})
	}

	return req, err
}

// FiatVehicleStatus holds the relevant data extracted from JSON that the server sends
// on vehicle status request
type FiatVehicleStatus struct {
	VehicleStatus struct {
		BatteryFillLevel struct {
			Value     float64
			Timestamp string
		}
		ElVehDTE struct {
			Value     float64
			Timestamp string
		}
		ChargingStatus struct {
			Value     string
			Timestamp string
		}
		PlugStatus struct {
			Value     int
			Timestamp string
		}
		LastRefresh string
	}
	Status int
}

// vehicles returns the list of user vehicles
func (v *Fiat) vehicles() ([]string, error) {
	var resp struct {
		Vehicles struct {
			Values []struct {
				VIN string
			} `json:"$values"`
		}
	}

	req, err := v.request(http.MethodGet, FiatVehicleList)
	if err == nil {
		err = v.DoJSON(req, &resp)
	}

	var vehicles []string
	if err == nil {
		for _, v := range resp.Vehicles.Values {
			vehicles = append(vehicles, v.VIN)
		}
	}

	return vehicles, err
}

// status performs a /status request to the Fiat API and triggers a refresh if
// the received status is too old
func (v *Fiat) status() (res FiatVehicleStatus, err error) {
	// follow up requested refresh
	if v.refreshId != "" {
		return v.refreshResult()
	}

	// otherwise start normal workflow
	uri := fmt.Sprintf("%s/api/vehicles/v3/%s/status", FiatAPI, v.vin)
	req, err := v.request(http.MethodGet, uri)
	if err == nil {
		err = v.DoJSON(req, &res)
	}

	if err == nil {
		var lastUpdate time.Time
		lastUpdate, err = time.Parse(FiatTimeFormat, res.VehicleStatus.LastRefresh)

		if elapsed := time.Since(lastUpdate); err == nil && elapsed > FiatStatusExpiry {
			v.log.DEBUG.Printf("vehicle status is outdated (age %v > %v), requesting refresh", elapsed, FiatStatusExpiry)

			if err = v.refreshRequest(); err == nil {
				err = api.ErrMustRetry
			}
		}
	}

	return res, err
}

// refreshResult triggers an update if not already in progress, otherwise gets result
func (v *Fiat) refreshResult() (res FiatVehicleStatus, err error) {
	uri := fmt.Sprintf("%s/api/vehicles/v3/%s/statusrefresh/%s", FiatAPI, v.vin, v.refreshId)

	var req *http.Request
	if req, err = v.request(http.MethodGet, uri); err == nil {
		err = v.DoJSON(req, &res)
	}

	// update successful and completed
	if err == nil && res.Status == 200 {
		v.refreshId = ""
		return res, nil
	}

	// update still in progress, keep retrying
	if time.Since(v.refreshTime) < FiatRefreshTimeout {
		return res, api.ErrMustRetry
	}

	// give up
	v.refreshId = ""
	if err == nil {
		err = api.ErrTimeout
	}

	return res, err
}

// refreshRequest requests status refresh tracked by commandId
func (v *Fiat) refreshRequest() error {
	var resp struct {
		CommandId string
	}

	uri := fmt.Sprintf("%s/api/vehicles/v2/%s/status", FiatAPI, v.vin)
	req, err := v.request(http.MethodPut, uri)
	if err == nil {
		err = v.DoJSON(req, &resp)
	}

	if err == nil {
		v.refreshId = resp.CommandId
		v.refreshTime = time.Now()

		if resp.CommandId == "" {
			err = errors.New("refresh failed")
		}
	}

	return err
}

var _ api.Battery = (*Fiat)(nil)

// SoC implements the api.Battery interface
func (v *Fiat) SoC() (float64, error) {
	res, err := v.statusG()
	if res, ok := res.(FiatVehicleStatus); err == nil && ok {
		return float64(res.VehicleStatus.BatteryFillLevel.Value), nil
	}

	return 0, err
}

var _ api.VehicleRange = (*Fiat)(nil)

// Range implements the api.VehicleRange interface
func (v *Fiat) Range() (int64, error) {
	res, err := v.statusG()
	if res, ok := res.(FiatVehicleStatus); err == nil && ok {
		return int64(res.VehicleStatus.ElVehDTE.Value), nil
	}

	return 0, err
}

var _ api.ChargeState = (*Fiat)(nil)

// Status implements the api.ChargeState interface
func (v *Fiat) Status() (api.ChargeStatus, error) {
	status := api.StatusA // disconnected

	res, err := v.statusG()
	if res, ok := res.(FiatVehicleStatus); err == nil && ok {
		if res.VehicleStatus.PlugStatus.Value == 1 {
			status = api.StatusB // connected, not charging
		}
		if res.VehicleStatus.ChargingStatus.Value == "ChargingAC" {
			status = api.StatusC // charging
		}
	}

	return status, err
}

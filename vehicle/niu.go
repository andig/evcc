package vehicle

import (
	"crypto/md5"
	"encoding/hex"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/andig/evcc/api"
	"github.com/andig/evcc/provider"
	"github.com/andig/evcc/util"
	"github.com/andig/evcc/util/request"
	"github.com/andig/evcc/vehicle/niu"
)

// Niu is an api.Vehicle implementation for Niu vehicles
type Niu struct {
	*embed
	*request.Helper
	user, password string
	serial         string
	token          niu.Token
	apiG           func() (interface{}, error)
}

func init() {
	registry.Add("niu", NewNiuFromConfig)
}

// NewFordFromConfig creates a new vehicle
func NewNiuFromConfig(other map[string]interface{}) (api.Vehicle, error) {
	cc := struct {
		embed                  `mapstructure:",squash"`
		User, Password, Serial string
		Cache                  time.Duration
	}{
		Cache: interval,
	}

	if err := util.DecodeOther(other, &cc); err != nil {
		return nil, err
	}

	if cc.User == "" || cc.Password == "" || cc.Serial == "" {
		return nil, errors.New("missing user, password or serial")
	}

	log := util.NewLogger("niu")

	v := &Niu{
		embed:    &cc.embed,
		Helper:   request.NewHelper(log, request.WithMetricsPush),
		user:     cc.User,
		password: cc.Password,
		serial:   strings.ToUpper(cc.Serial),
	}

	v.apiG = provider.NewCached(v.batteryAPI, cc.Cache).InterfaceGetter()

	return v, nil
}

// login implements the Niu oauth2 api
func (v *Niu) login() error {
	md5hash, err := md5Hash(v.password)
	if err != nil {
		return err
	}

	data := url.Values{
		"account":    []string{v.user},
		"password":   []string{md5hash},
		"grant_type": []string{"password"},
		"scope":      []string{"base"},
		"app_id":     []string{"niu_8xt1afu6"},
	}

	uri := niu.AuthURI + "/v3/api/oauth2/token"
	req, err := request.New(http.MethodPost, uri, strings.NewReader(data.Encode()), map[string]string{
		"Content-Type": "application/x-www-form-urlencoded",
	})

	if err == nil {
		var token niu.Token
		if err = v.DoJSON(req, &token); err == nil {
			v.token = token
		}
	}

	return err
}

// md5Hash creates a MD5 hash based on a string
func md5Hash(text string) (string, error) {
	hasher := md5.New()
	_, err := hasher.Write([]byte(text))
	return hex.EncodeToString(hasher.Sum(nil)), err
}

// request implements the Niu web request
func (v *Niu) request(uri string) (*http.Request, error) {
	if v.token.AccessToken == "" || time.Until(v.token.Expiry) < time.Minute {
		if err := v.login(); err != nil {
			return nil, err
		}
	}

	req, err := request.New(http.MethodGet, uri, nil, map[string]string{
		"token": v.token.AccessToken,
	})

	return req, err
}

// batteryAPI provides battery api response
func (v *Niu) batteryAPI() (interface{}, error) {
	var res niu.Response

	req, err := v.request(niu.ApiURI + "/v3/motor_data/index_info?sn=" + v.serial)
	if err == nil {
		err = v.DoJSON(req, &res)
	}

	return res, err
}

// SoC implements the api.Vehicle interface
func (v *Niu) SoC() (float64, error) {
	res, err := v.apiG()

	if res, ok := res.(niu.Response); err == nil && ok {
		return float64(res.Data.Batteries.CompartmentA.BatteryCharging), nil
	}

	return 0, err
}

var _ api.ChargeState = (*Niu)(nil)

// Status implements the api.ChargeState interface
func (v *Niu) Status() (api.ChargeStatus, error) {
	status := api.StatusA // disconnected

	res, err := v.apiG()
	if res, ok := res.(niu.Response); err == nil && ok {
		if res.Data.IsConnected {
			status = api.StatusB
		}
		if res.Data.IsCharging > 0 {
			status = api.StatusC
		}
	}

	return status, err
}

var _ api.VehicleRange = (*Niu)(nil)

// Range implements the api.VehicleRange interface
func (v *Niu) Range() (int64, error) {
	res, err := v.apiG()

	if res, ok := res.(niu.Response); err == nil && ok {
		return int64(res.Data.EstimatedMileage), nil
	}

	return 0, err
}

package vehicle

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/andig/evcc/api"
	"github.com/andig/evcc/provider"
	"github.com/andig/evcc/util"
	"github.com/andig/evcc/util/request"
)

const (
	bmwAuth = "https://customer.bmwgroup.com/gcdm/oauth/authenticate"
	bmwAPI  = "https://www.bmw-connecteddrive.com/api"
)

type bmwDynamicResponse struct {
	AttributesMap struct {
		ChargingLevelHv float64 `json:"chargingLevelHv,string"`
	}
}

type bmwVehiclesResponse []struct {
	VIN string `json:"vin"`
}

// BMW is an api.Vehicle implementation for BMW cars
type BMW struct {
	*embed
	*request.Helper
	user, password, vin string
	token               string
	tokenValid          time.Time
	chargeStateG        func() (float64, error)
}

func init() {
	registry.Add("bmw", NewBMWFromConfig)
}

// NewBMWFromConfig creates a new vehicle
func NewBMWFromConfig(other map[string]interface{}) (api.Vehicle, error) {
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

	log := util.NewLogger("bmw")

	v := &BMW{
		embed:    &cc.embed,
		Helper:   request.NewHelper(log, request.WithMetricsPush),
		user:     cc.User,
		password: cc.Password,
		vin:      strings.ToUpper(cc.VIN),
	}

	v.chargeStateG = provider.NewCached(v.chargeState, cc.Cache).FloatGetter()

	var err error
	if cc.VIN == "" {
		v.vin, err = findVehicle(v.vehicles())
		if err == nil {
			log.DEBUG.Printf("found vehicle: %v", v.vin)
		}
	}

	return v, err
}

func (v *BMW) login(user, password string) error {
	data := url.Values{
		"username":      []string{user},
		"password":      []string{password},
		"client_id":     []string{"dbf0a542-ebd1-4ff0-a9a7-55172fbfce35"},
		"redirect_uri":  []string{"https://www.bmw-connecteddrive.com/app/default/static/external-dispatch.html"},
		"response_type": []string{"token"},
		"scope":         []string{"authenticate_user fupo"},
		"state":         []string{"eyJtYXJrZXQiOiJkZSIsImxhbmd1YWdlIjoiZGUiLCJkZXN0aW5hdGlvbiI6ImxhbmRpbmdQYWdlIn0"},
		"locale":        []string{"DE-de"},
	}

	req, err := request.New(http.MethodPost, bmwAuth, strings.NewReader(data.Encode()), request.URLEncoding)
	if err != nil {
		return err
	}

	client := &http.Client{
		Timeout:       v.Helper.Client.Timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse }, // don't follow redirects
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	query, err := url.ParseQuery(resp.Header.Get("Location"))
	if err != nil {
		return err
	}

	token := query.Get("access_token")
	expires, err := strconv.Atoi(query.Get("expires_in"))
	if err != nil || token == "" || expires == 0 {
		return errors.New("could not obtain token")
	}

	v.token = token
	v.tokenValid = time.Now().Add(time.Duration(expires) * time.Second)

	return nil
}

func (v *BMW) request(uri string) (*http.Request, error) {
	if v.token == "" || time.Since(v.tokenValid) > 0 {
		if err := v.login(v.user, v.password); err != nil {
			return nil, err
		}
	}

	req, err := http.NewRequest(http.MethodGet, uri, nil)
	if err == nil {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", v.token))
	}

	return req, nil
}

// vehicles implements returns the list of user vehicles
func (v *BMW) vehicles() ([]string, error) {
	var resp bmwVehiclesResponse
	uri := fmt.Sprintf("%s/me/vehicles/v2/", bmwAPI)

	var vehicles []string

	req, err := v.request(uri)
	if err == nil {
		err = v.DoJSON(req, &resp)
	}

	if err == nil {
		for _, v := range resp {
			vehicles = append(vehicles, v.VIN)
		}
	}

	return vehicles, err
}

// chargeState implements the api.Vehicle interface
func (v *BMW) chargeState() (float64, error) {
	var resp bmwDynamicResponse
	uri := fmt.Sprintf("%s/vehicle/dynamic/v1/%s", bmwAPI, v.vin)

	req, err := v.request(uri)
	if err != nil {
		return 0, err
	}

	err = v.DoJSON(req, &resp)
	return resp.AttributesMap.ChargingLevelHv, err
}

// SoC implements the api.Vehicle interface
func (v *BMW) SoC() (float64, error) {
	return v.chargeStateG()
}

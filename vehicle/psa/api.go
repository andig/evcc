package psa

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/andig/evcc/util"
	"github.com/andig/evcc/util/request"
	"golang.org/x/oauth2"
)

// https://github.com/flobz/psa_car_controller
// https://developer.groupe-psa.io/webapi/b2c/api-reference/specification

// BaseURL is the API base url
const BaseURL = "https://api.groupe-psa.com/connectedcar/v4"

// API is an api.Vehicle implementation for PSA cars
type API struct {
	*request.Helper
	brand, realm string
	id, secret   string // client
}

// NewAPI creates a new vehicle
func NewAPI(log *util.Logger, brand, realm, id, secret string) *API {
	v := &API{
		Helper: request.NewHelper(log, request.WithMetricsPush),
		brand:  brand,
		realm:  realm,
		id:     id,
		secret: secret,
	}

	return v
}

// Login performs the login
func (v *API) Login(user, password string) error {
	config := oauth2.Config{
		ClientID:     v.id,
		ClientSecret: v.secret,
		Endpoint: oauth2.Endpoint{
			AuthURL:   "https://api.mpsa.com/api/connectedcar/v2/oauth/authorize",
			TokenURL:  fmt.Sprintf("https://idpcvs.%s/am/oauth2/access_token", v.brand),
			AuthStyle: oauth2.AuthStyleInHeader,
		},
		Scopes: []string{"openid profile"},
	}

	ctx := context.WithValue(
		context.Background(),
		oauth2.HTTPClient,
		v.Client,
	)

	// replace client with authenticated oauth client
	token, err := config.PasswordCredentialsToken(ctx, user, password)
	if err == nil {
		v.Client = config.Client(ctx, token)
	}

	return err
}

// Vehicle is a single vehicle
type Vehicle struct {
	ID       string   `json:"id"`
	Label    string   `json:"label"`
	Pictures []string `json:"pictures"`
	VIN      string   `json:"vin"`
}

// Vehicles implements the /vehicles response
func (v *API) Vehicles() ([]Vehicle, error) {
	data := url.Values{
		"client_id": []string{v.id},
	}

	uri := fmt.Sprintf("%s/user/vehicles?%s", BaseURL, data.Encode())
	req, err := request.New(http.MethodGet, uri, nil, map[string]string{
		"Accept":             "application/hal+json",
		"X-Introspect-Realm": v.realm,
	})

	var res struct {
		Embedded struct {
			Vehicles []Vehicle
		} `json:"_embedded"`
	}
	if err == nil {
		err = v.DoJSON(req, &res)
	}

	return res.Embedded.Vehicles, err
}

// Status is the /status response
type Status struct {
	Battery struct {
		Capacity int64
		Health   struct {
			Capacity   int64
			Resistance int64
		}
	}
	Charging struct {
		ChargingMode    string
		ChargingRate    int64
		NextDelayedTime string
		Plugged         bool
		RemainingTime   string
		Status          string
	}
	Preconditionning struct {
		AirConditioning struct {
			UpdatedAt time.Time
			Status    string // Disabled
		}
	}
	Energy []Energy
}

// Energy is the /status partial energy response
type Energy struct {
	UpdatedAt time.Time
	Type      string // Electric
	Level     int
	Autonomy  int
	Charging  struct {
		Plugged         bool
		Status          string // InProgress
		RemainingTime   Duration
		ChargingRate    int
		ChargingMode    string // "Slow"
		NextDelayedTime Duration
	}
}

// Status implements the /vehicles/<vid>/status response
func (v *API) Status(vid string) (Status, error) {
	data := url.Values{
		"client_id": []string{v.id},
	}

	// BaseURL is the API base url
	uri := fmt.Sprintf("%s/user/vehicles/%s/status?%s", BaseURL, vid, data.Encode())
	req, err := request.New(http.MethodGet, uri, nil, map[string]string{
		"Accept":             "application/hal+json",
		"X-Introspect-Realm": v.realm,
	})

	var status Status
	if err == nil {
		err = v.DoJSON(req, &status)
	}

	return status, err
}

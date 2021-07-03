package vehicle

import (
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"strconv"
	"time"

	"github.com/andig/evcc/api"
	"github.com/andig/evcc/provider"
	"github.com/andig/evcc/util"
	"github.com/andig/evcc/util/request"
	"golang.org/x/net/publicsuffix"
)

type ovmsChargeResponse struct {
	ChargeEtrFull    string `json:"charge_etr_full"`
	ChargeState      string `json:"chargestate"`
	ChargePortOpen   int    `json:"cp_dooropen"`
	EstimatedRange   string `json:"estimatedrange"`
	MessageAgeServer int    `json:"m_msgage_s"`
	Soc              string `json:"soc"`
}

type ovmsConnectResponse struct {
	NetConnected int `json:"v_net_connected"`
}

// OVMS is an api.Vehicle implementation for dexters-web server requests
type Ovms struct {
	*embed
	*request.Helper
	user, password, vehicleId, server string
	interval                          time.Duration
	chargeG                           func() (interface{}, error)
}

func init() {
	registry.Add("ovms", NewOvmsFromConfig)
}

// NewOVMSFromConfig creates a new vehicle
func NewOvmsFromConfig(other map[string]interface{}) (api.Vehicle, error) {
	cc := struct {
		embed                             `mapstructure:",squash"`
		User, Password, VehicleID, Server string
		Cache                             time.Duration
	}{
		Cache: interval,
	}

	if err := util.DecodeOther(other, &cc); err != nil {
		return nil, err
	}

	log := util.NewLogger("ovms")

	v := &Ovms{
		embed:     &cc.embed,
		Helper:    request.NewHelper(log, request.WithMetricsPush),
		user:      cc.User,
		password:  cc.Password,
		vehicleId: cc.VehicleID,
		server:    cc.Server,
		interval:  cc.Cache,
	}

	v.chargeG = provider.NewCached(v.batteryAPI, cc.Cache).InterfaceGetter()

	var err error
	v.Jar, err = cookiejar.New(&cookiejar.Options{
		PublicSuffixList: publicsuffix.List,
	})

	return v, err
}

func (v *Ovms) loginToServer() (err error) {
	uri := fmt.Sprintf("http://%s:6868/api/cookie?username=%s&password=%s", v.server, v.user, v.password)

	var resp *http.Response
	if resp, err = v.Get(uri); err == nil {
		resp.Body.Close()
	}

	return err
}

func (v *Ovms) delete(url string) error {
	req, err := request.New(http.MethodDelete, url, nil)
	if err == nil {
		var resp *http.Response
		if resp, err = v.Do(req); err == nil {
			resp.Body.Close()
		}
	}
	return err
}

func (v *Ovms) authFlow() (bool, error) {
	var resp ovmsConnectResponse
	err := v.loginToServer()
	if err == nil {
		resp, err = v.connectRequest()
	}
	return resp.NetConnected == 1, err
}

func (v *Ovms) connectRequest() (ovmsConnectResponse, error) {
	uri := fmt.Sprintf("http://%s:6868/api/vehicle/%s", v.server, v.vehicleId)
	var res ovmsConnectResponse
	err := v.GetJSON(uri, &res)
	return res, err
}

func (v *Ovms) chargeRequest() (ovmsChargeResponse, error) {
	uri := fmt.Sprintf("http://%s:6868/api/charge/%s", v.server, v.vehicleId)
	var res ovmsChargeResponse
	err := v.GetJSON(uri, &res)
	return res, err
}

func (v *Ovms) disconnect() error {
	uri := fmt.Sprintf("http://%s:6868/api/vehicle/%s", v.server, v.vehicleId)

	err := v.delete(uri)
	if err == nil {
		uri = fmt.Sprintf("http://%s:6868/api/cookie", v.server)
		return v.delete(uri)
	}

	return err
}

// batteryAPI provides battery-status api response
func (v *Ovms) batteryAPI() (interface{}, error) {
	var resp ovmsChargeResponse

	ovmsConnected, err := v.authFlow()
	if err == nil {
		resp, err = v.chargeRequest()

		if err == nil {
			err = v.disconnect()
		}

		messageAge := time.Duration(resp.MessageAgeServer) * time.Second
		if err == nil && messageAge > v.interval+time.Minute && ovmsConnected {
			err = api.ErrMustRetry
		}
	}

	return resp, err
}

// SoC implements the api.Vehicle interface
func (v *Ovms) SoC() (float64, error) {
	res, err := v.chargeG()

	if res, ok := res.(ovmsChargeResponse); err == nil && ok {
		return strconv.ParseFloat(res.Soc, 64)
	}

	return 0, err
}

var _ api.ChargeState = (*Ovms)(nil)

// Status implements the api.ChargeState interface
func (v *Ovms) Status() (api.ChargeStatus, error) {
	status := api.StatusA // disconnected

	res, err := v.chargeG()
	if res, ok := res.(ovmsChargeResponse); err == nil && ok {
		if res.ChargePortOpen > 0 {
			status = api.StatusB
		}
		if res.ChargeState == "charging" {
			status = api.StatusC
		}
	}

	return status, nil
}

var _ api.VehicleRange = (*Ovms)(nil)

// Range implements the api.VehicleRange interface
func (v *Ovms) Range() (int64, error) {
	res, err := v.chargeG()

	if res, ok := res.(ovmsChargeResponse); err == nil && ok {
		return strconv.ParseInt(res.EstimatedRange, 0, 64)
	}

	return 0, nil
}

var _ api.VehicleFinishTimer = (*Ovms)(nil)

// FinishTime implements the api.VehicleFinishTimer interface
func (v *Ovms) FinishTime() (time.Time, error) {
	res, err := v.chargeG()

	if res, ok := res.(ovmsChargeResponse); err == nil && ok {
		cef, err := strconv.ParseInt(res.ChargeEtrFull, 0, 64)
		if err == nil {
			return time.Now().Add(time.Duration(cef) * time.Minute), err
		}
	}

	return time.Time{}, api.ErrNotAvailable
}
